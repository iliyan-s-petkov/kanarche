package store_test

import (
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

func TestVisitorDailyMaxDayOnAFreshTable(t *testing.T) {
	ctx, _, s := newStore(t)
	_, ok, err := s.VisitorDailyMaxDay(ctx)
	if err != nil {
		t.Fatalf("VisitorDailyMaxDay: %v", err)
	}
	if ok {
		t.Errorf("ok = true, want false on a fresh table")
	}
}

func TestVisitorDailyMaxDayReturnsTheLatestDay(t *testing.T) {
	ctx, _, s := newStore(t)
	older := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	latest := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	if _, err := s.UpsertVisitorDaily(ctx, []store.VisitorDaily{
		{Day: older, Uniques: 1, Requests: 1, PageViews: 1, FetchedAt: older},
		{Day: latest, Uniques: 2, Requests: 2, PageViews: 2, FetchedAt: latest},
	}); err != nil {
		t.Fatalf("UpsertVisitorDaily: %v", err)
	}

	day, ok, err := s.VisitorDailyMaxDay(ctx)
	if err != nil {
		t.Fatalf("VisitorDailyMaxDay: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !day.Equal(latest) {
		t.Errorf("day = %v, want %v", day, latest)
	}
}

func TestUpsertVisitorDailyWritesRows(t *testing.T) {
	ctx, pool, s := newStore(t)
	day := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	fetchedAt := time.Date(2026, 9, 28, 0, 5, 0, 0, time.UTC)

	n, err := s.UpsertVisitorDaily(ctx, []store.VisitorDaily{
		{Day: day, Uniques: 104, Requests: 500, PageViews: 162, FetchedAt: fetchedAt},
	})
	if err != nil {
		t.Fatalf("UpsertVisitorDaily: %v", err)
	}
	if n != 1 {
		t.Fatalf("n = %d, want 1", n)
	}

	var uniques int
	var requests, pageViews int64
	if err := pool.QueryRow(ctx, `SELECT uniques, requests, page_views FROM visitor_daily WHERE day = $1`, day).
		Scan(&uniques, &requests, &pageViews); err != nil {
		t.Fatalf("query: %v", err)
	}
	if uniques != 104 || requests != 500 || pageViews != 162 {
		t.Errorf("got (%d, %d, %d), want (104, 500, 162)", uniques, requests, pageViews)
	}
}

// A re-fetch of the same day is a correction, not a second opinion: Cloudflare
// itself says late-arriving data lands within a day or two.
func TestUpsertVisitorDailyReplacesAnEarlierValueForTheSameDay(t *testing.T) {
	ctx, pool, s := newStore(t)
	day := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)

	if _, err := s.UpsertVisitorDaily(ctx, []store.VisitorDaily{
		{Day: day, Uniques: 100, Requests: 400, PageViews: 150, FetchedAt: day},
	}); err != nil {
		t.Fatalf("UpsertVisitorDaily (first): %v", err)
	}
	if _, err := s.UpsertVisitorDaily(ctx, []store.VisitorDaily{
		{Day: day, Uniques: 181, Requests: 878, PageViews: 878, FetchedAt: day.Add(time.Hour)},
	}); err != nil {
		t.Fatalf("UpsertVisitorDaily (second): %v", err)
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM visitor_daily`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("row count = %d, want 1", n)
	}

	var uniques int
	if err := pool.QueryRow(ctx, `SELECT uniques FROM visitor_daily WHERE day = $1`, day).Scan(&uniques); err != nil {
		t.Fatalf("query: %v", err)
	}
	if uniques != 181 {
		t.Errorf("uniques = %d, want 181 (the second write's value)", uniques)
	}
}

func TestUpsertVisitorDailyEmptySliceIsANoOp(t *testing.T) {
	ctx, _, s := newStore(t)
	n, err := s.UpsertVisitorDaily(ctx, nil)
	if err != nil {
		t.Fatalf("UpsertVisitorDaily: %v", err)
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
}

func TestSeedVisitorDailyFillsAnEmptyDay(t *testing.T) {
	ctx, pool, s := newStore(t)
	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	n, err := s.SeedVisitorDaily(ctx, []store.VisitorDaily{
		{Day: day, Uniques: 50, Requests: 200, PageViews: 90, FetchedAt: day},
	})
	if err != nil {
		t.Fatalf("SeedVisitorDaily: %v", err)
	}
	if n != 1 {
		t.Fatalf("n = %d, want 1", n)
	}

	var uniques int
	if err := pool.QueryRow(ctx, `SELECT uniques FROM visitor_daily WHERE day = $1`, day).Scan(&uniques); err != nil {
		t.Fatalf("query: %v", err)
	}
	if uniques != 50 {
		t.Errorf("uniques = %d, want 50", uniques)
	}
}

// The collector's own numbers are ground truth; a seed file must never
// clobber a day it already wrote.
func TestSeedVisitorDailyDoesNotOverwriteAnExistingRow(t *testing.T) {
	ctx, pool, s := newStore(t)
	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	if _, err := s.UpsertVisitorDaily(ctx, []store.VisitorDaily{
		{Day: day, Uniques: 181, Requests: 900, PageViews: 878, FetchedAt: day},
	}); err != nil {
		t.Fatalf("UpsertVisitorDaily: %v", err)
	}

	n, err := s.SeedVisitorDaily(ctx, []store.VisitorDaily{
		{Day: day, Uniques: 1, Requests: 1, PageViews: 1, FetchedAt: day},
	})
	if err != nil {
		t.Fatalf("SeedVisitorDaily: %v", err)
	}
	if n != 0 {
		t.Errorf("n = %d, want 0: the existing row must not be touched", n)
	}

	var uniques int
	if err := pool.QueryRow(ctx, `SELECT uniques FROM visitor_daily WHERE day = $1`, day).Scan(&uniques); err != nil {
		t.Fatalf("query: %v", err)
	}
	if uniques != 181 {
		t.Errorf("uniques = %d, want 181 (the collector's row, untouched)", uniques)
	}
}

func TestSeedVisitorDailyEmptySliceIsANoOp(t *testing.T) {
	ctx, _, s := newStore(t)
	n, err := s.SeedVisitorDaily(ctx, nil)
	if err != nil {
		t.Fatalf("SeedVisitorDaily: %v", err)
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
}
