package snapshot

import (
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

// TestBuildDayRange pins buildDayRange's three rules: it takes the min/max of
// the series' own points, it ignores a bucket whose station count falls below
// the coverage threshold, and it refuses to publish anything at all once too
// few buckets survive that gate.
func TestBuildDayRange(t *testing.T) {
	base := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	at := func(i int) time.Time { return base.Add(time.Duration(i) * 5 * time.Minute) }

	const floor = 72 // six hours of 5-minute buckets
	const threshold = 3

	// 80 buckets, all at the coverage threshold: enough to publish, and the
	// min/max come from the series itself.
	points := make([]store.Point, 80)
	counts := make(map[time.Time]int, 80)
	for i := range points {
		points[i] = store.Point{Time: at(i), Value: 10}
		counts[at(i)] = threshold
	}
	points[5].Value = 3.5 // the low point
	points[40].Value = 60 // the high point

	day := buildDayRange(points, counts, threshold, floor)
	if day == nil {
		t.Fatal("day = nil, want a range")
	}
	if day.Min != 3.5 || !day.MinAt.Equal(at(5)) {
		t.Errorf("Min = %v at %v, want 3.5 at %v", day.Min, day.MinAt, at(5))
	}
	if day.Max != 60 || !day.MaxAt.Equal(at(40)) {
		t.Errorf("Max = %v at %v, want 60 at %v", day.Max, day.MaxAt, at(40))
	}
	if day.Buckets != 80 {
		t.Errorf("Buckets = %d, want 80", day.Buckets)
	}
}

// A single one-sensor bucket at 3 a.m. must not set the day's extreme on an
// area that is otherwise well covered — the whole point of gating by count.
func TestBuildDayRangeExcludesLowCoverageBuckets(t *testing.T) {
	base := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	at := func(i int) time.Time { return base.Add(time.Duration(i) * 5 * time.Minute) }
	const floor, threshold = 72, 3

	points := make([]store.Point, 80)
	counts := make(map[time.Time]int, 80)
	for i := range points {
		points[i] = store.Point{Time: at(i), Value: 10}
		counts[at(i)] = threshold
	}
	// One sensor, reading 900 at 3 a.m.: below the coverage threshold, so it
	// must not become the day's high.
	points[36].Value = 900
	counts[at(36)] = 1

	day := buildDayRange(points, counts, threshold, floor)
	if day == nil {
		t.Fatal("day = nil, want a range")
	}
	if day.Max != 10 {
		t.Errorf("Max = %v, want 10 — the low-coverage bucket must be excluded", day.Max)
	}
	if day.Buckets != 79 {
		t.Errorf("Buckets = %d, want 79 (80 minus the excluded one)", day.Buckets)
	}
}

// Fewer than six hours of coverage-gated buckets: no range at all, not a
// range built from a handful of scattered points.
func TestBuildDayRangeNilBelowFloor(t *testing.T) {
	base := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	at := func(i int) time.Time { return base.Add(time.Duration(i) * 5 * time.Minute) }
	const floor, threshold = 72, 3

	points := make([]store.Point, 71)
	counts := make(map[time.Time]int, 71)
	for i := range points {
		points[i] = store.Point{Time: at(i), Value: 10}
		counts[at(i)] = threshold
	}
	if day := buildDayRange(points, counts, threshold, floor); day != nil {
		t.Errorf("day = %+v, want nil — 71 buckets is under the 72 floor", day)
	}
}

func TestMinDayBuckets(t *testing.T) {
	if got, want := minDayBuckets(5*time.Minute), 72; got != want {
		t.Errorf("minDayBuckets(5m) = %d, want %d", got, want)
	}
	if got := minDayBuckets(0); got != 0 {
		t.Errorf("minDayBuckets(0) = %d, want 0", got)
	}
}
