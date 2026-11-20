package api_test

import (
	"encoding/json"
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

// The partial flag marks the current UTC day only, and only that day carries
// fetched_at. The clock is fixed to a moment that is already the next day in
// Sofia, so a zone slip would mark the wrong day.
func TestVisitorsMarksOnlyTheCurrentUTCDayPartial(t *testing.T) {
	now := time.Date(2026, 3, 28, 22, 30, 0, 0, time.UTC)
	today := time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)
	fetched := time.Date(2026, 3, 28, 22, 5, 0, 0, time.UTC)
	rows := []store.VisitorDaily{
		{Day: today.AddDate(0, 0, -2), Uniques: 1, FetchedAt: fetched.AddDate(0, 0, -1)},
		{Day: today.AddDate(0, 0, -1), Uniques: 2, FetchedAt: fetched.AddDate(0, 0, -1)},
		{Day: today, Uniques: 3, FetchedAt: fetched},
	}
	d := visitorsDeps(t, &stubSource{visitors: rows})
	d.Clock = func() time.Time { return now }
	rec := serve(t, d, get("/api/v1/visitors?days=30", "203.0.113.7"))
	var b visitorsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if len(b.Days) != 3 {
		t.Fatalf("days = %d, want 3", len(b.Days))
	}
	for i, day := range b.Days {
		last := i == 2
		if got, _ := day["partial"].(bool); got != last {
			t.Errorf("day %v partial = %v, want %v", day["day"], day["partial"], last)
		}
		if _, has := day["fetched_at"]; has != last {
			t.Errorf("day %v has fetched_at = %v, want %v", day["day"], has, last)
		}
	}
	if got := b.Days[2]["fetched_at"]; got != "2026-03-28T22:05:00Z" {
		t.Errorf("fetched_at = %v, want 2026-03-28T22:05:00Z", got)
	}
}

// When today has no row yet, the newest stored day is closed and nothing is
// partial.
func TestVisitorsNoPartialWhenTodayHasNoRow(t *testing.T) {
	now := time.Date(2026, 3, 28, 0, 10, 0, 0, time.UTC)
	rows := []store.VisitorDaily{{Day: time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC), Uniques: 2}}
	d := visitorsDeps(t, &stubSource{visitors: rows})
	d.Clock = func() time.Time { return now }
	rec := serve(t, d, get("/api/v1/visitors", "203.0.113.7"))
	var b visitorsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if len(b.Days) != 1 || b.Days[0]["partial"] != false {
		t.Errorf("days = %v, want one day with partial false", b.Days)
	}
	if _, has := b.Days[0]["fetched_at"]; has {
		t.Errorf("closed day carries fetched_at")
	}
}
