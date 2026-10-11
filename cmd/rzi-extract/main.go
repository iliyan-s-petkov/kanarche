// Command rzi-extract turns the RZI documents in an inbox directory into
// outbox records. It runs inside the no-network sandbox: it reads only
// validated <sha256>.(pdf|xls|xlsx) names, and every document, accepted or
// not, ends as one contract record. Record errors are fixed texts.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"kanarche.eu/internal/docparse/pdf"
	"kanarche.eu/internal/upstream/rzi"
	"kanarche.eu/internal/upstream/rzi/burgas"
	"kanarche.eu/internal/upstream/rzi/varna"
)

const (
	// maxInputBytes matches the fetcher's per-file cap.
	maxInputBytes = 5 << 20
	// rejectVersion tags records written by this binary itself.
	rejectVersion = "rzi-extract/1"
	varnaVersion  = "varna-pdftotext/1"
)

var reName = regexp.MustCompile(`^([0-9a-f]{64})\.(pdf|xls|xlsx)$`)

// kind describes one input format. A nil extract means the format is
// recognised but has no extractor yet.
type kind struct {
	source  string
	magic   []byte
	extract func(ctx context.Context, path string, data []byte) rzi.File
}

// newKinds is the one place formats are wired to extractors.
func newKinds(tools pdf.Tools) map[string]kind {
	return map[string]kind{
		"pdf": {
			source:  rzi.SourceVarna,
			magic:   []byte("%PDF-"),
			extract: varnaExtract(tools),
		},
		"xls": {
			source:  rzi.SourceBurgas,
			magic:   []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1},
			extract: func(_ context.Context, _ string, data []byte) rzi.File { return burgas.Extract(data) },
		},
		// Dobrich: set extract to the XLSX extractor when it exists.
		"xlsx": {source: rzi.SourceDobrich, magic: []byte("PK\x03\x04")},
	}
}

func varnaExtract(tools pdf.Tools) func(context.Context, string, []byte) rzi.File {
	return func(ctx context.Context, path string, data []byte) rzi.File {
		text, err := tools.Text(ctx, path)
		if err != nil {
			return rejected(rzi.SourceVarna, sha(data), err.Error())
		}
		f := varna.Parse(text)
		f.SHA256 = sha(data)
		f.ExtractorVersion = varnaVersion
		return f
	}
}

func sha(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

// rejected is the record for a document that produced no blocks.
func rejected(source, hash, reason string) rzi.File {
	return rzi.File{
		Schema:           rzi.SchemaVersion,
		Source:           source,
		SHA256:           hash,
		ExtractorVersion: rejectVersion,
		Error:            reason,
		Blocks:           []rzi.Block{},
		Warnings:         []string{},
	}
}

func main() {
	in := flag.String("in", "/in", "inbox directory")
	out := flag.String("out", "/out", "outbox directory")
	flag.Parse()
	os.Exit(process(*in, *out, newKinds(pdf.Default())))
}

// process handles every validated name in in and returns the exit code:
// 0 when each of them produced a record, 1 otherwise.
func process(in, out string, kinds map[string]kind) int {
	ents, err := os.ReadDir(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rzi-extract: cannot read input directory")
		return 1
	}
	code := 0
	for _, e := range ents {
		m := reName.FindStringSubmatch(e.Name())
		if m == nil || !e.Type().IsRegular() {
			continue
		}
		if err := handle(in, out, m[1], m[2], kinds[m[2]]); err != nil {
			fmt.Fprintf(os.Stderr, "rzi-extract: %s: %v\n", m[1], err)
			code = 1
		}
	}
	return code
}

// handle reads one document, builds its record and writes it.
func handle(in, out, hash, ext string, k kind) error {
	path := filepath.Join(in, hash+"."+ext)
	rec := build(path, hash, k)
	body, err := encode(rec)
	if err != nil || len(body) > rzi.MaxRecordBytes {
		body, err = encode(rejected(k.source, hash, "output over size cap"))
	}
	if err != nil {
		return fmt.Errorf("encode failed")
	}
	if _, err := rzi.Decode(bytes.NewReader(body)); err != nil {
		body, err = encode(rejected(k.source, hash, "output not a valid record"))
		if err != nil {
			return fmt.Errorf("encode failed")
		}
	}
	return writeAtomic(out, hash, body)
}

func encode(f rzi.File) ([]byte, error) { return json.Marshal(f) }

// build reads the file and runs its extractor. The extractor never sees a
// document whose size, name hash or magic is wrong.
func build(path, hash string, k kind) (rec rzi.File) {
	defer func() {
		if recover() != nil {
			rec = rejected(k.source, hash, "internal extractor error")
		}
	}()
	fh, err := os.Open(path)
	if err != nil {
		return rejected(k.source, hash, "input not readable")
	}
	defer fh.Close()
	data, err := io.ReadAll(io.LimitReader(fh, maxInputBytes+1))
	if err != nil {
		return rejected(k.source, hash, "input not readable")
	}
	if len(data) > maxInputBytes {
		return rejected(k.source, hash, "input over size cap")
	}
	if sha(data) != hash {
		return rejected(k.source, hash, "name does not match content hash")
	}
	if !bytes.HasPrefix(data, k.magic) {
		return rejected(k.source, hash, "magic mismatch")
	}
	if k.extract == nil {
		return rejected(k.source, hash, "unsupported format")
	}
	rec = k.extract(context.Background(), path, data)
	if rec.SHA256 != hash {
		return rejected(k.source, hash, "extractor hash mismatch")
	}
	return rec
}

// writeAtomic writes <hash>.json.tmp, then renames it into place.
func writeAtomic(dir, hash string, body []byte) error {
	final := filepath.Join(dir, hash+".json")
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return fmt.Errorf("write failed")
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename failed")
	}
	return nil
}
