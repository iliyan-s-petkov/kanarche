package snapshot

import (
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

func dayPoint(day int, v float64) store.Point {
	return store.Point{Time: time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC), Value: v}
}

func TestSummariseWHODays(t *testing.T) {
	pts := []store.Point{dayPoint(1, 10), dayPoint(2, 15), dayPoint(3, 15.01), dayPoint(4, 40.5), dayPoint(5, 22), dayPoint(6, 40.5)}
	got := summariseWHODays(pts)
	if got == nil {
		t.Fatal("got nil summary")
	}
	// 15.0 is not over the guideline, 15.01 is. The first of two equal worst days wins.
	if got.DaysWithData != 6 || got.DaysOver != 4 || got.WorstMean != 40.5 || !got.WorstDay.Equal(pts[3].Time) {
		t.Errorf("got %+v", *got)
	}
}

func TestSummariseWHODaysExactlyAtGuideline(t *testing.T) {
	got := summariseWHODays([]store.Point{dayPoint(1, WHODayPM25), dayPoint(2, 3)})
	if got.DaysOver != 0 || got.WorstMean != WHODayPM25 {
		t.Errorf("got %+v", *got)
	}
}

func TestSummariseWHODaysEmpty(t *testing.T) {
	if got := summariseWHODays(nil); got != nil {
		t.Errorf("got %+v, want nil", *got)
	}
}
