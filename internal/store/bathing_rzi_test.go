package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"kanarche.eu/internal/store"
)

const (
	siteA = "BG3310610135003001"
	siteB = "BG3310610135003002"
	siteC = "BG3310610135003003"
)

func rziFile(source, sha string) store.RZIFile {
	return store.RZIFile{
		Source: source, URL: "https://example.test/" + source + ".pdf", SHA256: sha, Bytes: 1234,
		FetchedAt: time.Date(2026, 6, 3, 8, 0, 0, 0, time.UTC), ExtractorVersion: "rzi-extract 1",
		Status: store.RZIStatusOK, RowsOK: 2,
	}
}

func hash(c string) string { return strings.Repeat(c, 64) }

func rziSample(site string, day int) store.RZISample {
	return store.RZISample{
		SiteID: site, Date: time.Date(2026, 6, day, 0, 0, 0, 0, time.UTC), Season: 2026,
		EC: 15, ECBelow: true, IE: 40, Unit: "MPN/100ml", Remarks: "clear",
	}
}

func mustReplace(t *testing.T, ctx context.Context, s *store.Store, f store.RZIFile, xs ...store.RZISample) {
	t.Helper()
	if err := s.ReplaceRZISource(ctx, f.Source, f, xs); err != nil {
		t.Fatalf("ReplaceRZISource(%s): %v", f.Source, err)
	}
}

func countRows(t *testing.T, ctx context.Context, p *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestReplaceRZISourceTouchesOnlyThatSource(t *testing.T) {
	ctx, _, s := newStore(t)
	mustReplace(t, ctx, s, rziFile("rzi_varna", hash("a")), rziSample(siteA, 2), rziSample(siteA, 9))
	mustReplace(t, ctx, s, rziFile("rzi_burgas", hash("b")), rziSample(siteB, 2))

	mustReplace(t, ctx, s, rziFile("rzi_varna", hash("c")), rziSample(siteA, 16))

	got, err := s.LoadRZI(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2 (new varna + untouched burgas): %+v", len(got), got)
	}
	by := map[string]store.RZIRow{}
	for _, r := range got {
		by[r.File.Source] = r
	}
	if by["rzi_varna"].Date.Day() != 16 {
		t.Errorf("varna row = %+v, want only the 16th", by["rzi_varna"])
	}
	if by["rzi_burgas"].SiteID != siteB {
		t.Errorf("burgas row = %+v, want it untouched", by["rzi_burgas"])
	}
}

func TestReplaceRZISourceIsAtomic(t *testing.T) {
	ctx, pool, s := newStore(t)
	mustReplace(t, ctx, s, rziFile("rzi_varna", hash("a")), rziSample(siteA, 2))

	bad := rziSample(siteB, 2)
	bad.EC = -1
	err := s.ReplaceRZISource(ctx, "rzi_varna", rziFile("rzi_varna", hash("c")), []store.RZISample{rziSample(siteA, 9), bad})
	if err == nil {
		t.Fatal("ReplaceRZISource accepted a negative value")
	}
	got, _ := s.LoadRZI(ctx)
	if len(got) != 1 || got[0].Date.Day() != 2 {
		t.Errorf("rows = %+v, want the old set kept", got)
	}
	if n := countRows(t, ctx, pool, "bathing_rzi_file"); n != 1 {
		t.Errorf("files = %d, want 1: the failed file must roll back", n)
	}
}

func TestReplaceRZISourceRejectsMismatchedSource(t *testing.T) {
	ctx, pool, s := newStore(t)
	err := s.ReplaceRZISource(ctx, "rzi_burgas", rziFile("rzi_varna", hash("a")), []store.RZISample{rziSample(siteA, 2)})
	if err == nil {
		t.Fatal("source and file.Source disagree, want an error")
	}
	if n := countRows(t, ctx, pool, "bathing_rzi_sample"); n != 0 {
		t.Errorf("samples = %d, want 0", n)
	}
}

func TestReplaceRZISourceRefusesNonOKFile(t *testing.T) {
	ctx, pool, s := newStore(t)
	f := rziFile("rzi_varna", hash("a"))
	f.Status = store.RZIStatusNeedsReview
	err := s.ReplaceRZISource(ctx, "rzi_varna", f, []store.RZISample{rziSample(siteA, 2)})
	if !errors.Is(err, store.ErrRZIFileNotOK) {
		t.Fatalf("err = %v, want ErrRZIFileNotOK", err)
	}
	if n := countRows(t, ctx, pool, "bathing_rzi_sample"); n != 0 {
		t.Errorf("samples = %d, want 0", n)
	}
}

func TestRecordRZIFileNeedsReviewWritesNoSamples(t *testing.T) {
	ctx, pool, s := newStore(t)
	mustReplace(t, ctx, s, rziFile("rzi_varna", hash("a")), rziSample(siteA, 2))

	f := rziFile("rzi_varna", hash("d"))
	f.Status, f.Reason, f.RowsOK = store.RZIStatusNeedsReview, "block count dropped by 3", 0
	id, err := s.RecordRZIFile(ctx, f)
	if err != nil || id == 0 {
		t.Fatalf("RecordRZIFile = %d, %v", id, err)
	}
	if n := countRows(t, ctx, pool, "bathing_rzi_file"); n != 2 {
		t.Errorf("files = %d, want 2", n)
	}
	got, _ := s.LoadRZI(ctx)
	if len(got) != 1 || got[0].Date.Day() != 2 {
		t.Errorf("rows = %+v, want the old set kept", got)
	}
}

func TestRecordRZIFileIsIdempotentOnHash(t *testing.T) {
	ctx, pool, s := newStore(t)
	f := rziFile("rzi_varna", hash("a"))
	id1, err := s.RecordRZIFile(ctx, f)
	if err != nil {
		t.Fatal(err)
	}
	f.ETag = "v2"
	id2, err := s.RecordRZIFile(ctx, f)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 || countRows(t, ctx, pool, "bathing_rzi_file") != 1 {
		t.Errorf("ids %d, %d; want one row for one hash", id1, id2)
	}
}

func TestLoadRZICarriesFileProvenance(t *testing.T) {
	ctx, _, s := newStore(t)
	f := rziFile("rzi_dobrich", hash("e"))
	lm := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	f.LastModified, f.ETag = &lm, `"abc"`
	mustReplace(t, ctx, s, f, rziSample(siteC, 2))

	got, err := s.LoadRZI(ctx)
	if err != nil || len(got) != 1 {
		t.Fatalf("LoadRZI = %+v, %v", got, err)
	}
	r := got[0]
	if r.SiteID != siteC || r.EC != 15 || !r.ECBelow || r.IE != 40 || r.IEBelow || r.Unit != "MPN/100ml" || r.Remarks != "clear" {
		t.Errorf("sample = %+v", r.RZISample)
	}
	if r.File.ID == 0 || r.File.Source != "rzi_dobrich" || r.File.URL != f.URL || r.File.SHA256 != hash("e") ||
		r.File.ETag != `"abc"` || r.File.LastModified == nil || !r.File.LastModified.Equal(lm) ||
		!r.File.FetchedAt.Equal(f.FetchedAt) || r.File.ExtractorVersion != "rzi-extract 1" {
		t.Errorf("file = %+v, want provenance of %+v", r.File, f)
	}
}

func TestRZILastGood(t *testing.T) {
	ctx, _, s := newStore(t)
	if _, ok, err := s.RZILastGood(ctx, "rzi_varna"); err != nil || ok {
		t.Fatalf("empty: ok=%v err=%v, want false", ok, err)
	}
	old := rziFile("rzi_varna", hash("a"))
	mustReplace(t, ctx, s, old, rziSample(siteA, 2))
	other := rziFile("rzi_burgas", hash("b"))
	other.FetchedAt = old.FetchedAt.Add(time.Hour)
	mustReplace(t, ctx, s, other, rziSample(siteB, 2))
	bad := rziFile("rzi_varna", hash("d"))
	bad.Status, bad.FetchedAt = store.RZIStatusNeedsReview, old.FetchedAt.Add(2*time.Hour)
	if _, err := s.RecordRZIFile(ctx, bad); err != nil {
		t.Fatal(err)
	}

	got, ok, err := s.RZILastGood(ctx, "rzi_varna")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.SHA256 != hash("a") || got.RowsOK != 2 {
		t.Errorf("last good = %+v, want the ok varna file, not the newer needs_review or burgas", got)
	}
}

// I14: the EEA replace must leave RZI rows alone.
func TestReplaceBathingLeavesRZIRowsAlone(t *testing.T) {
	ctx, pool, s := newStore(t)
	if err := s.ReplaceBathing(ctx, bathingFixture(), time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	mustReplace(t, ctx, s, rziFile("rzi_varna", hash("a")), rziSample(siteA, 2), rziSample(siteA, 9))
	before, _ := s.LoadRZI(ctx)

	// An EEA import that no longer lists the site must not cascade either.
	d := bathingFixture()
	d.Sites = d.Sites[1:]
	d.Classes = d.Classes[2:]
	d.Samples = nil
	if err := s.ReplaceBathing(ctx, d, time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("ReplaceBathing: %v", err)
	}
	if err := s.ReplaceBathing(ctx, bathingFixture(), time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("ReplaceBathing: %v", err)
	}

	after, err := s.LoadRZI(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 2 || len(after) != 2 {
		t.Fatalf("rzi rows before %d, after %d; want 2 and 2", len(before), len(after))
	}
	for i := range before {
		if before[i].RZISample != after[i].RZISample || before[i].File.ID != after[i].File.ID {
			t.Errorf("row %d changed: %+v -> %+v", i, before[i], after[i])
		}
	}
	if n := countRows(t, ctx, pool, "bathing_rzi_file"); n != 1 {
		t.Errorf("files = %d, want 1", n)
	}
}

func TestRZIConstraints(t *testing.T) {
	ctx, pool, s := newStore(t)
	id, err := s.RecordRZIFile(ctx, rziFile("rzi_varna", hash("a")))
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	sample := func(site string, ec, ie int, unit, remarks string, fileID int64) error {
		_, err := pool.Exec(ctx, `INSERT INTO bathing_rzi_sample
			(site_id, sample_date, season, ec_value, ec_below, ie_value, ie_below, unit, remarks, file_id)
			VALUES ($1, $2, 2026, $3, false, $4, false, $5, $6, $7)`, site, day, ec, ie, unit, remarks, fileID)
		return err
	}
	file := func(source, sha string, status string) error {
		_, err := pool.Exec(ctx, `INSERT INTO bathing_rzi_file
			(source, url, sha256, bytes, fetched_at, extractor_version, status, rows_ok, rows_rejected)
			VALUES ($1, 'u', $2, 1, now(), 'v', $3, 0, 0)`, source, sha, status)
		return err
	}
	if err := sample(siteA, 1, 1, "MPN/100ml", "", id); err != nil {
		t.Fatalf("baseline insert: %v", err)
	}

	bad := map[string]error{
		"duplicate site and date": sample(siteA, 1, 1, "MPN/100ml", "", id),
		"ec above cap":            sample(siteB, 1000001, 1, "MPN/100ml", "", id),
		"ie negative":             sample(siteB, 1, -1, "MPN/100ml", "", id),
		"unit":                    sample(siteB, 1, 1, "mg/l", "", id),
		"remarks 501 runes":       sample(siteB, 1, 1, "MPN/100ml", strings.Repeat("ж", 501), id),
		"unknown file":            sample(siteB, 1, 1, "MPN/100ml", "", id+100),
		"duplicate sha":           file("rzi_burgas", hash("a"), "ok"),
		"short sha":               file("rzi_burgas", "abc", "ok"),
		"upper sha":               file("rzi_burgas", strings.Repeat("A", 64), "ok"),
		"source":                  file("rzi_sofia", hash("f"), "ok"),
		"status":                  file("rzi_varna", hash("f"), "maybe"),
	}
	for name, err := range bad {
		if err == nil {
			t.Errorf("%s: insert succeeded, want a constraint error", name)
		}
	}

	if err := sample(siteC, 1, 1, "MPN/100ml", strings.Repeat("ж", 500), id); err != nil {
		t.Errorf("500 runes rejected: %v", err)
	}
}
