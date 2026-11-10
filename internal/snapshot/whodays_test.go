package snapshot

import (
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

func dayPoint(day int, v float64) store.Point {
	return store.Point{Time: time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC), Value: v}
}

// whoNow is late on 2026-01-20 UTC, so the 2026-01-20 bucket is the partial current day.
var whoNow = time.Date(2026, 1, 20, 2, 0, 0, 0, time.UTC)

func TestSummariseWHODays(t *testing.T) {
	pts := []store.Point{dayPoint(1, 10), dayPoint(2, 15), dayPoint(3, 15.01), dayPoint(4, 40.5), dayPoint(5, 22), dayPoint(6, 40.5)}
	got := summariseWHODays(pts, time.Time{}, whoNow)
	if got == nil {
		t.Fatal("got nil summary")
	}
	// 15.0 is not over the guideline, 15.01 is. The first of two equal worst days wins.
	if got.DaysWithData != 6 || got.DaysOver != 4 || got.WorstMean != 40.5 || !got.WorstDay.Equal(pts[3].Time) {
		t.Errorf("got %+v", *got)
	}
}

func TestSummariseWHODaysExactlyAtGuideline(t *testing.T) {
	got := summariseWHODays([]store.Point{dayPoint(1, WHODayPM25), dayPoint(2, 3)}, time.Time{}, whoNow)
	if got.DaysOver != 0 || got.WorstMean != WHODayPM25 {
		t.Errorf("got %+v", *got)
	}
}

func TestSummariseWHODaysEmpty(t *testing.T) {
	if got := summariseWHODays(nil, time.Time{}, whoNow); got != nil {
		t.Errorf("got %+v, want nil", *got)
	}
}

// The bucket of the current UTC day holds a few hours and must not count or be the worst day.
func TestSummariseWHODaysSkipsPartialToday(t *testing.T) {
	pts := []store.Point{dayPoint(18, 20), dayPoint(19, 30), dayPoint(20, 400)}
	got := summariseWHODays(pts, time.Time{}, whoNow)
	if got.DaysWithData != 2 || got.DaysOver != 2 || got.WorstMean != 30 || !got.WorstDay.Equal(pts[1].Time) {
		t.Errorf("got %+v", *got)
	}
}

// The first bucket of the window is cut at since, so only buckets from the next UTC midnight count.
func TestSummariseWHODaysSkipsCutFirstBucket(t *testing.T) {
	since := time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC)
	pts := []store.Point{dayPoint(10, 400), dayPoint(11, 20), dayPoint(12, 5)}
	got := summariseWHODays(pts, since, whoNow)
	if got.DaysWithData != 2 || got.DaysOver != 1 || got.WorstMean != 20 {
		t.Errorf("got %+v", *got)
	}
	// A window starting exactly at midnight keeps its first bucket.
	full := summariseWHODays(pts, dayPoint(10, 0).Time, whoNow)
	if full.DaysWithData != 3 {
		t.Errorf("midnight start: got %+v", *full)
	}
}

func TestSummariseWHODaysOnlyPartialBuckets(t *testing.T) {
	if got := summariseWHODays([]store.Point{dayPoint(20, 400)}, time.Time{}, whoNow); got != nil {
		t.Errorf("got %+v, want nil", *got)
	}
}
