package store_test

import (
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

func TestVisitorDailyLastOnAFreshTableIsEmptyNotNil(t *testing.T) {
	ctx, _, s := newStore(t)
	got, err := s.VisitorDailyLast(ctx, 30)
	if err != nil {
		t.Fatalf("VisitorDailyLast: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("got %#v, want empty non-nil slice", got)
	}
}

func TestVisitorDailyLastReturnsTheNewestRowsOldestFirst(t *testing.T) {
	ctx, _, s := newStore(t)
	var rows []store.VisitorDaily
	for d := 1; d <= 5; d++ {
		day := time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC)
		rows = append(rows, store.VisitorDaily{Day: day, Uniques: d, Requests: int64(d * 10), PageViews: int64(d * 5), FetchedAt: day})
	}
	// Inserted out of order so the ordering comes from the query, not the insert.
	rows[0], rows[4] = rows[4], rows[0]
	if _, err := s.UpsertVisitorDaily(ctx, rows); err != nil {
		t.Fatalf("UpsertVisitorDaily: %v", err)
	}

	got, err := s.VisitorDailyLast(ctx, 3)
	if err != nil {
		t.Fatalf("VisitorDailyLast: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for i, want := range []int{3, 4, 5} {
		g := got[i]
		if g.Day.Day() != want || g.Uniques != want || g.Requests != int64(want*10) || g.PageViews != int64(want*5) {
			t.Errorf("row %d = %+v, want day %d", i, g, want)
		}
	}
}
