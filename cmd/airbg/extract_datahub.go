package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/upstream/bathing/datahub"
	"kanarche.eu/internal/xlsx"
)

const (
	defaultSnapshotPath = "internal/upstream/bathing/datahub/bg.json"
	datahubLicence      = "CC BY 4.0"
)

// runExtractDatahub turns the pinned EEA Excel into the committed BG snapshot.
// It opens no database and reads no credential. The file is verified against
// sea.datahub.sha256 and sea.datahub.size before the first byte is parsed.
//
// --file reads a local copy (dev, offline). --fetch downloads the pinned URL.
func runExtractDatahub(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("extract-bathing-datahub", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", "", "local copy of the pinned workbook")
	fetch := fs.Bool("fetch", false, "download the pinned URL instead of --file")
	out := fs.String("out", defaultSnapshotPath, "snapshot path")
	edition := fs.String("edition", "2025 v1.0", "edition recorded in the header")
	published := fs.String("published", "2026-06-02", "publication date recorded in the header")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if (*file != "") == *fetch {
		fmt.Fprintln(stderr, "extract-bathing-datahub: give exactly one of --file or --fetch")
		return 2
	}
	if err := extractDatahub(context.Background(), extractArgs{
		file: *file, out: *out, edition: *edition, published: *published,
	}, stdout); err != nil {
		fmt.Fprintln(stderr, "extract-bathing-datahub:", err)
		return 1
	}
	return 0
}

type extractArgs struct {
	file, out, edition, published string
}

func extractDatahub(ctx context.Context, a extractArgs, stdout io.Writer) error {
	cfg, err := config.LoadOffline()
	if err != nil {
		return err
	}
	d := cfg.Sea.Datahub
	// Verify before parsing, on both paths.
	var meta datahub.Meta
	if a.file == "" {
		dir, err := os.MkdirTemp("", "extract-datahub-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		meta, err = datahub.Fetch(ctx, datahub.FetchConfig{
			URL: d.URL, SHA256: d.SHA256, Size: d.Size, AllowedHosts: d.AllowedHosts,
			Timeout: d.RequestTimeout, MaxBytes: d.MaxDownloadBytes, Client: &http.Client{},
		}, dir)
		if err != nil {
			return err
		}
	} else if meta, err = datahub.VerifyFile(a.file, d.SHA256, d.Size); err != nil {
		return err
	}
	f, err := os.Open(meta.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	book, err := xlsx.OpenContext(ctx, f, meta.Size)
	if err != nil {
		return err
	}
	snap, rej, err := datahub.Parse(book, cfg.Sea.Country, time.Now().UTC())
	if err != nil {
		return err
	}
	doc := datahub.NewFile(datahub.Header{
		SourceURL: d.URL, SHA256: meta.SHA256, Size: meta.Size,
		Edition: a.edition, Published: a.published, Licence: datahubLicence, Country: cfg.Sea.Country,
	}, snap)
	data, err := doc.Encode()
	if err != nil {
		return err
	}
	if err := writeAtomic(a.out, data); err != nil {
		return err
	}
	report(stdout, a.out, len(data), snap, rej)
	return nil
}

// writeAtomic leaves no partial file behind: the target appears whole or not at all.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".extract-*.part")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func report(w io.Writer, path string, n int, snap datahub.Snapshot, rej datahub.Rejects) {
	bySeason := map[int]int{}
	for _, c := range snap.Classes {
		bySeason[c.Season]++
	}
	seasons := make([]int, 0, len(bySeason))
	for s := range bySeason {
		seasons = append(seasons, s)
	}
	sort.Ints(seasons)
	fmt.Fprintf(w, "%s: %d bytes, %d classes, %d rejected, %d other-country rows\n", path, n, len(snap.Classes), rej.Rejected(), rej.OtherCountry)
	for _, s := range seasons {
		fmt.Fprintf(w, "season %d: %d\n", s, bySeason[s])
	}
}
