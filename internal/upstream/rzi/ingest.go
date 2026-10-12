package rzi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/store"
)

// Subdirectories of the outbox. A record leaves the outbox root exactly once.
const (
	dirDone   = "done"   // ok, rejected and duplicate records
	dirReview = "review" // needs_review records, kept for approval
	dirBad    = "bad"    // records that could not be tied to a file row
)

// maxReasonBytes caps bathing_rzi_file.reason.
const maxReasonBytes = 500

// recordName is the only outbox file name the loop owns.
var recordName = regexp.MustCompile(`^[0-9a-f]{64}\.json$`)

// Sink is the store side of the ingest loop and of approval.
type Sink interface {
	LoadBathingSites(ctx context.Context) ([]store.BathingSite, error)
	LoadRZI(ctx context.Context) ([]store.RZIRow, error)
	RZILastGood(ctx context.Context, source string) (store.RZIFile, bool, error)
	RZILatest(ctx context.Context, source string) (store.RZIFile, bool, error)
	RZIFileBySHA(ctx context.Context, sha string) (store.RZIFile, bool, error)
	ReplaceRZISource(ctx context.Context, source string, f store.RZIFile, xs []store.RZISample) error
	RecordRZIFile(ctx context.Context, f store.RZIFile) (int64, error)
	ApproveRZIFile(ctx context.Context, sha, source string, xs []store.RZISample, keep []store.RZIKey, at time.Time, excluded int) error
}

// Stats counts what one Process pass did with outbox records.
type Stats struct {
	OK, NeedsReview, Rejected int
	// Duplicate is a record whose hash and extractor version are already stored.
	Duplicate int
	// Refused is a record that could not be read or tied to a source.
	Refused int
}

// fetchMeta is what a fetch in this process learned about a document.
type fetchMeta struct {
	url          string
	etag         string
	lastModified *time.Time
}

// Ingester turns outbox records into file rows and, for ok files, samples.
// A file is all or nothing: only an ok verdict writes samples (I13).
type Ingester struct {
	sink  Sink
	cfg   config.SeaRZI
	clock func() time.Time
	mu    sync.Mutex
	meta  map[string]fetchMeta
	prior map[string]Validators
}

func NewIngester(cfg config.SeaRZI, sink Sink, clock func() time.Time) *Ingester {
	if clock == nil {
		clock = time.Now
	}
	return &Ingester{sink: sink, cfg: cfg, clock: clock,
		meta: map[string]fetchMeta{}, prior: map[string]Validators{}}
}

// NoteFetch remembers a fetch result: its metadata goes on the file row when
// the record for that hash arrives, and its validators go on the next request.
func (g *Ingester) NoteFetch(res Result) {
	if res.Status != StatusWritten && res.Status != StatusUnchanged {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	m := fetchMeta{url: res.URL, etag: res.ETag}
	if t, err := http.ParseTime(res.LastModified); err == nil {
		m.lastModified = &t
	}
	g.meta[res.SHA256] = m
	g.prior[res.Source] = Validators{ETag: res.ETag, LastModified: res.LastModified, SHA256: res.SHA256}
}

// Prior is what to send with the next request for source.
func (g *Ingester) Prior(ctx context.Context, source string) Validators {
	g.mu.Lock()
	v, ok := g.prior[source]
	g.mu.Unlock()
	if ok {
		return v
	}
	f, found, err := g.sink.RZILastGood(ctx, source)
	if err != nil || !found {
		return Validators{}
	}
	v = Validators{ETag: f.ETag, SHA256: f.SHA256}
	if f.LastModified != nil {
		v.LastModified = f.LastModified.UTC().Format(http.TimeFormat)
	}
	return v
}

// Process reads every record in the outbox root once. Records with a store
// error stay where they are and the pass stops with that error.
func (g *Ingester) Process(ctx context.Context) (Stats, error) {
	var st Stats
	entries, err := os.ReadDir(g.cfg.OutboxDir)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("rzi outbox: %w", err)
	}
	for _, e := range entries {
		// Only plain files with the extractor's names. Symlinks, directories
		// and temp files are not ours.
		if !e.Type().IsRegular() || !recordName.MatchString(e.Name()) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return st, err
		}
		if err := g.one(ctx, e.Name(), &st); err != nil {
			return st, err
		}
	}
	return st, nil
}

func (g *Ingester) one(ctx context.Context, name string, st *Stats) error {
	sha := strings.TrimSuffix(name, ".json")
	rec, recSum, err := g.readRecord(filepath.Join(g.cfg.OutboxDir, name))
	if err == nil && rec.SHA256 != sha {
		err = errNameMismatch
	}
	if err == nil {
		if _, ok := specFor(rec.Source); !ok {
			err = ErrSource
		}
	}
	if err != nil {
		slog.Warn("rzi outbox record refused", "sha256", sha, "reason", refusal(err))
		st.Refused++
		return g.move(name, dirBad)
	}

	if old, found, err := g.sink.RZIFileBySHA(ctx, sha); err != nil {
		return fmt.Errorf("rzi file lookup: %w", err)
	} else if found && (old.Status == store.RZIStatusOK || old.ApprovedAt != nil || old.ExtractorVersion == rec.ExtractorVersion) {
		// An ok file is final: a later extractor never re-judges it.
		st.Duplicate++
		slog.Info("rzi outbox record already stored", "source", rec.Source, "sha256", sha, "status", old.Status)
		if old.Status == store.RZIStatusNeedsReview {
			return g.move(name, dirReview)
		}
		return g.move(name, dirDone)
	}

	sites, err := g.sink.LoadBathingSites(ctx)
	if err != nil {
		return fmt.Errorf("rzi load sites: %w", err)
	}
	last, err := g.lastGood(ctx, rec.Source)
	if err != nil {
		return err
	}
	accepted, rejects, status := ValidateRows(rec, sites, last, g.clock())

	file := g.fileRow(rec, status, len(accepted), rows(rec), rejects)
	file.RecordSHA256 = recSum
	switch status {
	case StatusOK:
		if err := g.sink.ReplaceRZISource(ctx, rec.Source, file, toStore(accepted)); err != nil {
			return fmt.Errorf("rzi replace %s: %w", rec.Source, err)
		}
		st.OK++
		slog.Info("rzi file accepted", "source", rec.Source, "sha256", sha, "rows", len(accepted))
		return g.move(name, dirDone)
	case StatusNeedsReview:
		// Nothing is written but the file row. The accepted rows wait for
		// an operator (rzi-approve).
		if _, err := g.sink.RecordRZIFile(ctx, file); err != nil {
			return fmt.Errorf("rzi record %s: %w", rec.Source, err)
		}
		st.NeedsReview++
		slog.Warn("rzi file needs review", "source", rec.Source, "sha256", sha,
			"rows_ok", len(accepted), "rows_flagged", file.RowsRejected, "reasons", file.Reason)
		return g.move(name, dirReview)
	default:
		if _, err := g.sink.RecordRZIFile(ctx, file); err != nil {
			return fmt.Errorf("rzi record %s: %w", rec.Source, err)
		}
		st.Rejected++
		slog.Warn("rzi file rejected", "source", rec.Source, "sha256", sha, "reasons", file.Reason)
		return g.move(name, dirDone)
	}
}

var (
	errNameMismatch = errors.New("file name does not match sha256")
	errNotRegular   = errors.New("not a regular file")
)

// refusal maps a read error to a fixed string. Decode errors can quote the
// record, so they never reach the log as they are.
func refusal(err error) string {
	switch {
	case errors.Is(err, ErrTooLarge):
		return "record over size cap"
	case errors.Is(err, ErrSchema):
		return "unsupported schema"
	case errors.Is(err, ErrRecord):
		return "malformed record"
	case errors.Is(err, errNameMismatch):
		return errNameMismatch.Error()
	case errors.Is(err, ErrSource):
		return "unknown source"
	case errors.Is(err, errNotRegular):
		return "not a regular file"
	}
	return "unreadable record"
}

// readRecord decodes a record file and returns the sha256 of its bytes.
// Anything but a regular file is refused, a symlink included.
func (g *Ingester) readRecord(path string) (File, string, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return File{}, "", err
	}
	if !fi.Mode().IsRegular() {
		return File{}, "", errNotRegular
	}
	fh, err := os.Open(path)
	if err != nil {
		return File{}, "", err
	}
	defer fh.Close()
	h := sha256.New()
	f, err := Decode(io.TeeReader(fh, h))
	if err != nil {
		return File{}, "", err
	}
	return f, hex.EncodeToString(h.Sum(nil)), nil
}

// fileRow builds the row for a verdict. Reason holds the fixed validator
// strings only, never the extractor's own text.
func (g *Ingester) fileRow(rec File, status Status, accepted, total int, rejects []Reject) store.RZIFile {
	now := g.clock().UTC()
	g.mu.Lock()
	m, known := g.meta[rec.SHA256]
	g.mu.Unlock()
	if !known {
		m.url = g.pageURL(rec.Source)
	}
	f := store.RZIFile{
		Source: rec.Source, URL: m.url, SHA256: rec.SHA256, Bytes: g.inboxBytes(rec.SHA256),
		ETag: m.etag, LastModified: m.lastModified, FetchedAt: now,
		Status: string(status), RowsOK: accepted, RowsRejected: total - accepted,
		Reason: joinReasons(rejects),
	}
	if validVersion(rec.ExtractorVersion) {
		f.ExtractorVersion = rec.ExtractorVersion
	}
	if status == StatusRejected {
		f.RowsOK, f.RowsRejected = 0, 0
	}
	return f
}

func (g *Ingester) pageURL(source string) string {
	switch source {
	case SourceVarna:
		return g.cfg.Varna.PageURL
	case SourceBurgas:
		return g.cfg.Burgas.PageURL
	case SourceDobrich:
		return g.cfg.Dobrich.PageURL
	}
	return ""
}

// inboxBytes is the size of the fetched document, 0 if it is not in the inbox.
func (g *Ingester) inboxBytes(sha string) int {
	// sha is 64 hex characters here, so the pattern has no metacharacters.
	matches, err := filepath.Glob(filepath.Join(g.cfg.InboxDir, sha+".*"))
	if err != nil || len(matches) == 0 {
		return 0
	}
	fi, err := os.Lstat(matches[0])
	if err != nil || !fi.Mode().IsRegular() {
		return 0
	}
	return int(fi.Size())
}

// joinReasons is the distinct reason strings in a stable order, capped.
func joinReasons(rejects []Reject) string {
	seen := map[string]bool{}
	var rs []string
	for _, r := range rejects {
		if !seen[r.Reason] {
			seen[r.Reason] = true
			rs = append(rs, r.Reason)
		}
	}
	slices.Sort(rs)
	s := strings.Join(rs, "; ")
	if len(s) > maxReasonBytes {
		s = s[:maxReasonBytes]
	}
	return s
}

// lastGood is the newest ok file of a source as the validator wants it.
// The store keeps no block count, so Blocks is the number of distinct sites:
// each block maps to one site and a site is mapped by one block.
func (g *Ingester) lastGood(ctx context.Context, source string) (LastGood, error) {
	_, ok, err := g.sink.RZILastGood(ctx, source)
	if err != nil {
		return LastGood{}, fmt.Errorf("rzi last good: %w", err)
	}
	if !ok {
		return LastGood{}, nil
	}
	all, err := g.sink.LoadRZI(ctx)
	if err != nil {
		return LastGood{}, fmt.Errorf("rzi load rows: %w", err)
	}
	var lg LastGood
	sites := map[string]bool{}
	for _, r := range all {
		if r.File.Source != source {
			continue
		}
		sites[r.SiteID] = true
		lg.Samples = append(lg.Samples, Sample{
			SiteID: r.SiteID, Date: r.Date, Season: r.Season,
			EC: r.EC, ECBelow: r.ECBelow, IE: r.IE, IEBelow: r.IEBelow, Unit: r.Unit,
		})
	}
	lg.Blocks = len(sites)
	return lg, nil
}

// move puts a record into an outbox subdirectory, then drops its document
// from the inbox. Approval reads the review record only, so review keeps no
// original.
func (g *Ingester) move(name, dir string) error {
	dst := filepath.Join(g.cfg.OutboxDir, dir)
	if err := os.MkdirAll(dst, 0o750); err != nil {
		return fmt.Errorf("rzi outbox: %w", err)
	}
	if err := os.Rename(filepath.Join(g.cfg.OutboxDir, name), filepath.Join(dst, name)); err != nil {
		return fmt.Errorf("rzi outbox: %w", err)
	}
	g.pruneInbox(strings.TrimSuffix(name, ".json"))
	return nil
}

// pruneInbox removes inbox/<sha>.* regular files. A failure is logged: the
// document is only disk, and the next fetch of it is skipped as settled.
func (g *Ingester) pruneInbox(sha string) {
	// sha is 64 hex characters here, so the pattern has no metacharacters.
	matches, err := filepath.Glob(filepath.Join(g.cfg.InboxDir, sha+".*"))
	if err != nil {
		return
	}
	for _, p := range matches {
		if fi, err := os.Lstat(p); err != nil || !fi.Mode().IsRegular() {
			continue
		}
		if err := os.Remove(p); err != nil {
			slog.Warn("rzi inbox file not removed", "sha256", sha)
		}
	}
}

// rows is the number of sample rows in a record.
func rows(f File) int {
	n := 0
	for _, b := range f.Blocks {
		n += len(b.Rows)
	}
	return n
}

func toStore(xs []Sample) []store.RZISample {
	out := make([]store.RZISample, len(xs))
	for i, x := range xs {
		out[i] = store.RZISample{
			SiteID: x.SiteID, Date: x.Date, Season: x.Season,
			EC: x.EC, ECBelow: x.ECBelow, IE: x.IE, IEBelow: x.IEBelow,
			Unit: x.Unit, Remarks: x.Remarks,
		}
	}
	return out
}
