package rzi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/db"
	"kanarche.eu/internal/store"
	"kanarche.eu/internal/testsupport"
	"kanarche.eu/internal/upstream/rzi"
)

// rig is a migrated database with the EEA sites loaded, an empty inbox and
// outbox, and an ingester on a fixed clock.
type rig struct {
	t      *testing.T
	ctx    context.Context
	st     *store.Store
	cfg    config.SeaRZI
	ing    *rzi.Ingester
	now    time.Time
	outbox string
}

// rigSites mirrors the synthetic sites the fixtures were written against.
func rigSites() []store.BathingSite {
	return []store.BathingSite{
		{ID: "BG3310610135003001", Zone: "coastal", Lat: 43.3033, Lon: 28.0533},
		{ID: "BG3310610135003002", Zone: "coastal", Lat: 43.2900, Lon: 28.0450},
		{ID: "BG3310610135003003", Zone: "coastal", Lat: 43.2700, Lon: 28.0300},
		{ID: "BG3410210135002001", Zone: "coastal", Lat: 42.6500, Lon: 27.7200},
	}
}

const (
	siteA = "BG3310610135003001"
	siteB = "BG3310610135003002"
	siteC = "BG3310610135003003"
)

func testCfg(t *testing.T) config.SeaRZI {
	t.Helper()
	return config.SeaRZI{
		Enabled:           true,
		InboxDir:          t.TempDir(),
		OutboxDir:         t.TempDir(),
		InSeasonInterval:  6 * time.Hour,
		OffSeasonInterval: 24 * time.Hour,
		MaxFileBytes:      5 << 20,
		RequestTimeout:    30 * time.Second,
		Varna:             config.SeaRZISource{PageURL: "https://rzi-varna.com/?page_id=2635"},
		Burgas:            config.SeaRZISource{PageURL: "https://rzi-burgas.bg/page"},
		Dobrich:           config.SeaRZISource{PageURL: "https://rzi-dobrich.egov.bg/page"},
	}
}

func newRig(t *testing.T) *rig {
	t.Helper()
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st := store.New(pool, testsupport.StoreConfig(), 5*time.Second)
	now := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	if err := st.ReplaceBathing(ctx, store.BathingData{Sites: rigSites()}, now); err != nil {
		t.Fatalf("seed sites: %v", err)
	}
	cfg := testCfg(t)
	return &rig{t: t, ctx: ctx, st: st, cfg: cfg, now: now, outbox: cfg.OutboxDir,
		ing: rzi.NewIngester(cfg, st, func() time.Time { return now })}
}

func (r *rig) process() rzi.Stats {
	r.t.Helper()
	s, err := r.ing.Process(r.ctx)
	if err != nil {
		r.t.Fatalf("Process: %v", err)
	}
	return s
}

// fixture decodes a committed outbox record.
func fixture(t testing.TB, name string) rzi.File {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "outbox", name))
	if err != nil {
		t.Fatal(err)
	}
	f, err := rzi.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// shaOf is a valid sha256 made of one hex digit.
func shaOf(c string) string { return strings.Repeat(c, 64) }

// varna is the valid fixture with its own document hash.
func varna(t testing.TB, sha string) rzi.File {
	t.Helper()
	f := fixture(t, "varna-valid.json")
	f.SHA256 = sha
	return f
}

// withBadUnit breaks the first row of the first block.
func withBadUnit(f rzi.File) rzi.File {
	f.Blocks = append([]rzi.Block(nil), f.Blocks...)
	f.Blocks[0].Rows = append([]rzi.Row(nil), f.Blocks[0].Rows...)
	f.Blocks[0].Rows[0].Unit = "mg/l"
	return f
}

// drop writes f to dir as <sha>.json and returns the path.
func drop(t testing.TB, dir string, f rzi.File) string {
	t.Helper()
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, f.SHA256+".json")
	if err := os.WriteFile(p, b, 0o640); err != nil {
		t.Fatal(err)
	}
	return p
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// captureLog routes slog to a buffer for the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func (r *rig) rows() []store.RZIRow {
	r.t.Helper()
	rows, err := r.st.LoadRZI(r.ctx)
	if err != nil {
		r.t.Fatal(err)
	}
	return rows
}

func (r *rig) file(sha string) store.RZIFile {
	r.t.Helper()
	f, ok, err := r.st.RZIFileBySHA(r.ctx, sha)
	if err != nil || !ok {
		r.t.Fatalf("file row %s: found %v, err %v", sha[:4], ok, err)
	}
	return f
}

// seedOK stores an ok varna file with one sample, as an earlier run would.
func (r *rig) seedOK(sha string, site string, day int) {
	r.t.Helper()
	f := store.RZIFile{
		Source: rzi.SourceVarna, URL: "https://example.test/", SHA256: sha, Bytes: 10,
		FetchedAt: r.now.Add(-48 * time.Hour), ExtractorVersion: "rzi-extract/1",
		Status: store.RZIStatusOK, RowsOK: 1,
	}
	x := store.RZISample{
		SiteID: site, Date: time.Date(2026, 6, day, 0, 0, 0, 0, time.UTC), Season: 2026,
		EC: 30, IE: 15, IEBelow: true, Unit: "MPN/100ml",
	}
	if err := r.st.ReplaceRZISource(r.ctx, rzi.SourceVarna, f, []store.RZISample{x}); err != nil {
		r.t.Fatal(err)
	}
}
