package store_test

import (
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

// seedAt seeds one reading and its hourly rollup at the given time.
func seedAt(t *testing.T, ctx contextT, pool poolT, id int64, x float64, value float64, at time.Time) {
	t.Helper()
	if id >= store.OfficialSensorIDFloor {
		seedOfficialSensorReading(t, ctx, pool, id, x, 42.0, "P2", value, at)
	} else {
		seedSensorReading(t, ctx, pool, id, x, 42.0, "P2", value, "ok", at)
	}
	seedHourly(t, ctx, pool, id, "P2", at.Truncate(time.Hour), value, 1)
}

// wantSeriesValues checks every series query returns exactly these values, in time order.
func wantSeriesValues(t *testing.T, ctx contextT, s *store.Store, slug string, since time.Time, want []float64) {
	t.Helper()
	for _, hourly := range []bool{false, true} {
		bucket := 5 * time.Minute
		if hourly {
			bucket = time.Hour
		}
		all, err := s.AllAreaSeries(ctx, "P2", since, hourly, bucket)
		if err != nil {
			t.Fatalf("AllAreaSeries: %v", err)
		}
		one, err := s.AreaSeries(ctx, slug, "P2", since, nil, hourly, bucket)
		if err != nil {
			t.Fatalf("AreaSeries: %v", err)
		}
		for name, pts := range map[string][]store.Point{"all-area": all[slug], "per-area": one} {
			var got []float64
			for _, p := range pts {
				got = append(got, p.Value)
			}
			if len(got) != len(want) {
				t.Errorf("hourly=%v %s %s = %v, want %v", hourly, name, slug, got, want)
				continue
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("hourly=%v %s %s = %v, want %v", hourly, name, slug, got, want)
					break
				}
			}
		}
	}
}

// The official side of the series needs a station that is fresh now, the same
// freshness the headline uses, not just any official reading in the window.
func TestSeriesOfficialMustBeFreshNotJustInTheWindow(t *testing.T) {
	ctx, pool := migrated(t)
	s := store.New(pool, testStoreConfig(), testSeriesTimeout)
	now := time.Now().UTC().Truncate(time.Minute)
	stale := now.Add(-48 * time.Hour)
	off := store.OfficialSensorIDFloor

	// 2 citizen stations and an official one stale for 2 days.
	seedArea(t, ctx, pool, "stale-official", "oblast", 23.0, 42.0)
	seedAt(t, ctx, pool, 1001, 23.000, 10, now)
	seedAt(t, ctx, pool, 1002, 23.001, 20, now)
	seedAt(t, ctx, pool, off+101, 23.002, 40, stale)
	// Rule 3: one citizen station and an old official one.
	seedArea(t, ctx, pool, "rule-three", "oblast", 25.0, 42.0)
	seedAt(t, ctx, pool, 1011, 25.000, 33, now)
	seedAt(t, ctx, pool, off+102, 25.002, 40, stale)
	// Only an official station, fresh.
	seedArea(t, ctx, pool, "official-alone", "oblast", 27.0, 42.0)
	seedAt(t, ctx, pool, off+103, 27.000, 55, now)
	// Only an official station, and a stale one: its history is all there is.
	seedArea(t, ctx, pool, "official-stale-alone", "oblast", 29.0, 42.0)
	seedAt(t, ctx, pool, off+104, 29.000, 44, stale)
	assignAreas(t, ctx, pool)

	since := now.Add(-72 * time.Hour)
	wantSeriesValues(t, ctx, s, "official-stale-alone", since, []float64{44})
	wantSeriesValues(t, ctx, s, "stale-official", since, []float64{15})
	wantSeriesValues(t, ctx, s, "rule-three", since, []float64{33})
	wantSeriesValues(t, ctx, s, "official-alone", since, []float64{55})
}

// The citizen count behind the series is over the whole window, not just the
// stations fresh now, so a station that went quiet still keeps the series citizen.
func TestSeriesCitizenCountSpansTheWindow(t *testing.T) {
	ctx, pool := migrated(t)
	s := store.New(pool, testStoreConfig(), testSeriesTimeout)
	now := time.Now().UTC().Truncate(time.Minute)
	off := store.OfficialSensorIDFloor

	seedArea(t, ctx, pool, "went-quiet", "oblast", 23.0, 42.0)
	seedAt(t, ctx, pool, 1101, 23.000, 10, now)
	seedAt(t, ctx, pool, 1102, 23.001, 20, now)
	seedAt(t, ctx, pool, 1103, 23.002, 30, now.Add(-10*time.Hour))
	seedAt(t, ctx, pool, off+111, 23.003, 90, now)
	assignAreas(t, ctx, pool)

	aggs, err := s.AreaAggregates(ctx, []string{"oblast"})
	if err != nil {
		t.Fatalf("AreaAggregates: %v", err)
	}
	if a := findAgg(t, aggs, "went-quiet"); a.HeadlineSource != "eea" {
		t.Fatalf("headline = %q, want eea: only two citizen stations are fresh", a.HeadlineSource)
	}
	wantSeriesValues(t, ctx, s, "went-quiet", now.Add(-24*time.Hour), []float64{30, 15})
}
