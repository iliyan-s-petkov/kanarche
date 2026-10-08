package pollen_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"kanarche.eu/internal/pollen"
	"kanarche.eu/internal/store"
)

func TestNextRun(t *testing.T) {
	times := []time.Duration{9*time.Hour + 15*time.Minute, 21*time.Hour + 15*time.Minute}
	day := func(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, time.UTC) }
	for _, tc := range []struct {
		now, want time.Time
	}{
		{day(4, 3, 0), day(4, 9, 15)},
		{day(4, 9, 15), day(4, 21, 15)},
		{day(4, 12, 0), day(4, 21, 15)},
		{day(4, 23, 0), day(5, 9, 15)},
		// 12:00 EEST is 09:00 UTC: the times are UTC, not local.
		{time.Date(2026, 10, 4, 12, 0, 0, 0, time.FixedZone("EEST", 3*3600)), day(4, 9, 15)},
	} {
		if got := pollen.NextRun(tc.now, times); !got.Equal(tc.want) {
			t.Errorf("NextRun(%v) = %v, want %v", tc.now, got, tc.want)
		}
	}
}

func TestDueOnStart(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	stale := 13 * time.Hour
	for _, tc := range []struct {
		name   string
		latest time.Time
		ok     bool
		want   bool
	}{
		{"nothing stored", time.Time{}, false, true},
		{"fresh run", now.Add(-3 * time.Hour), true, false},
		{"stale run", now.Add(-14 * time.Hour), true, true},
	} {
		if got := pollen.DueOnStart(tc.latest, tc.ok, now, stale); got != tc.want {
			t.Errorf("%s: DueOnStart = %v, want %v", tc.name, got, tc.want)
		}
	}
}

type fakeStore struct {
	cells     []store.PollenCell
	written   []store.PollenForecast
	fetchedAt time.Time
	country   string
	stepC     int
	marginM   float64
}

func (f *fakeStore) PollenLattice(_ context.Context, country string, stepC int, marginM float64) ([]store.PollenCell, error) {
	f.country, f.stepC, f.marginM = country, stepC, marginM
	return f.cells, nil
}

func (f *fakeStore) WritePollen(_ context.Context, rows []store.PollenForecast, fetchedAt time.Time) (int64, error) {
	f.written, f.fetchedAt = rows, fetchedAt
	return int64(len(rows)), nil
}

func (f *fakeStore) LatestPollenFetch(context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func TestRunOnceFetchesTheLatticeAndStoresIt(t *testing.T) {
	var requests atomic.Int32
	srv := fakeAPI(t, &requests, nil)
	cfg := testConfig()
	cfg.URL = srv.URL
	fs := &fakeStore{cells: []store.PollenCell{{LonC: 2320, LatC: 4260}, {LonC: 2340, LatC: 4260}}}
	c := pollen.NewCollector(cfg, fs)
	now := time.Date(2026, 10, 4, 9, 15, 0, 0, time.UTC)
	c.SetClockForTesting(func() time.Time { return now })

	n, err := c.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if n != 4 || len(fs.written) != 4 {
		t.Errorf("wrote %d (%d rows), want 2 cells x 2 species", n, len(fs.written))
	}
	if !fs.fetchedAt.Equal(now) {
		t.Errorf("fetched_at = %v, want %v", fs.fetchedAt, now)
	}
	if fs.country != "BG" || fs.stepC != 20 || fs.marginM != 10000 {
		t.Errorf("lattice asked for %q step %d margin %v, want BG 20 10000", fs.country, fs.stepC, fs.marginM)
	}
}

// No country row means no lattice. That is an error, not a quiet empty run.
func TestRunOnceWithoutALatticeFails(t *testing.T) {
	var requests atomic.Int32
	srv := fakeAPI(t, &requests, nil)
	cfg := testConfig()
	cfg.URL = srv.URL
	if _, err := pollen.NewCollector(cfg, &fakeStore{}).RunOnce(context.Background()); err == nil {
		t.Error("RunOnce succeeded with an empty lattice")
	}
	if requests.Load() != 0 {
		t.Errorf("made %d requests for an empty lattice", requests.Load())
	}
}
