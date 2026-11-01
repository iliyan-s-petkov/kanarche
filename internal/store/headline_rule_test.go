package store_test

import (
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

// Rule 3: a metric with a few citizen stations and no official one still shows
// the citizen median, as before the split. Covered stays the plain station
// threshold, plus any metric that took the official branch.
func TestHeadlineFallsBackToFewCitizenStations(t *testing.T) {
	ctx, pool := migrated(t)
	s := store.New(pool, testStoreConfig(), testSeriesTimeout)
	now := time.Now().UTC().Truncate(time.Minute)
	off := store.OfficialSensorIDFloor

	// Three PM stations make the area covered; temperature has one station.
	seedHeadlineArea(t, ctx, pool, "weather", 23.0, []headlineSeed{{901, false, 10}, {902, false, 20}, {903, false, 30}}, now)
	seedSensorReading(t, ctx, pool, 904, 23.004, 42.0, "temperature", 21, "ok", now)
	seedHourly(t, ctx, pool, 904, "temperature", now.Truncate(time.Hour), 21, 1)
	// One citizen sensor alone: below the threshold, so no number at all.
	seedHeadlineArea(t, ctx, pool, "lone-citizen", 25.0, []headlineSeed{{911, false, 33}}, now)
	// One citizen sensor and one official CO station: the official branch covers it.
	seedHeadlineArea(t, ctx, pool, "co-official", 27.0, []headlineSeed{{921, false, 33}}, now)
	seedOfficialSensorReading(t, ctx, pool, off+91, 27.002, 42.0, "CO", 800, now)
	seedHourly(t, ctx, pool, off+91, "CO", now.Truncate(time.Hour), 800, 1)
	assignAreas(t, ctx, pool)

	check := func(t *testing.T, aggs []store.AreaAggregate) {
		t.Helper()
		a := findAgg(t, aggs, "weather")
		if a.Values["temperature"] != 21 || a.HeadlineSources["temperature"] != "community" || a.Values["P2"] != 20 || !a.Covered {
			t.Errorf("weather = %v %v covered=%v, want temperature 21 community beside P2 20", a.Values, a.HeadlineSources, a.Covered)
		}
		a = findAgg(t, aggs, "lone-citizen")
		if a.Covered || len(a.Values) != 0 {
			t.Errorf("lone citizen = %v covered=%v, want uncovered and empty", a.Values, a.Covered)
		}
		a = findAgg(t, aggs, "co-official")
		if !a.Covered || a.Values["CO"] != 800 || a.HeadlineSources["CO"] != "eea" || a.Values["P2"] != 33 || a.HeadlineSources["P2"] != "community" {
			t.Errorf("co-official = %v %v covered=%v, want CO 800 eea, P2 33 community, covered", a.Values, a.HeadlineSources, a.Covered)
		}
	}
	aggs, err := s.AreaAggregates(ctx, []string{"oblast"})
	if err != nil {
		t.Fatalf("AreaAggregates: %v", err)
	}
	check(t, aggs)
	aggs, err = s.WindowedAreaAggregates(ctx, []string{"oblast"}, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("WindowedAreaAggregates: %v", err)
	}
	check(t, aggs)
}

// Exactly one source per (area, metric): the jsonb maps hide a duplicate.
func TestHeadlineHasOneSourcePerMetric(t *testing.T) {
	ctx, pool, _ := headlineFixture(t)
	s := store.New(pool, testStoreConfig(), testSeriesTimeout)
	rows, err := s.HeadlineRowsPerMetric(ctx, []string{"oblast"})
	if err != nil {
		t.Fatalf("HeadlineRowsPerMetric: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("no headline rows")
	}
	for k, n := range rows {
		if n != 1 {
			t.Errorf("%s has %d headline rows, want 1", k, n)
		}
	}
}

// The series takes the same side as the headline: 2 citizen + 1 official is the
// official series, 3 + 1 and 2 alone are citizen.
func TestAreaSeriesFollowsThePerMetricChoice(t *testing.T) {
	ctx, pool := migrated(t)
	s := store.New(pool, testStoreConfig(), testSeriesTimeout)
	now := time.Now().UTC().Truncate(time.Minute)
	off := store.OfficialSensorIDFloor
	seedHeadlineArea(t, ctx, pool, "two-plus-one", 23.0, []headlineSeed{{801, false, 10}, {802, false, 20}, {off + 81, true, 40}}, now)
	seedHeadlineArea(t, ctx, pool, "three-plus-one", 25.0, []headlineSeed{{811, false, 10}, {812, false, 20}, {813, false, 30}, {off + 82, true, 100}}, now)
	seedHeadlineArea(t, ctx, pool, "two-only", 27.0, []headlineSeed{{821, false, 10}, {822, false, 20}}, now)
	assignAreas(t, ctx, pool)

	want := map[string]float64{"two-plus-one": 40, "three-plus-one": 20, "two-only": 15}
	since := now.Add(-2 * time.Hour)
	for _, hourly := range []bool{false, true} {
		bucket := 5 * time.Minute
		if hourly {
			bucket = time.Hour
		}
		all, err := s.AllAreaSeries(ctx, "P2", since, hourly, bucket)
		if err != nil {
			t.Fatalf("AllAreaSeries: %v", err)
		}
		counts, err := s.AllAreaSeriesCounts(ctx, "P2", since, hourly, bucket)
		if err != nil {
			t.Fatalf("AllAreaSeriesCounts: %v", err)
		}
		for slug, v := range want {
			if pts := all[slug]; len(pts) != 1 || pts[0].Value != v {
				t.Errorf("hourly=%v all-area %s = %v, want %v", hourly, slug, pts, v)
			}
			pts, err := s.AreaSeries(ctx, slug, "P2", since, nil, hourly, bucket)
			if err != nil {
				t.Fatalf("AreaSeries: %v", err)
			}
			if len(pts) != 1 || pts[0].Value != v {
				t.Errorf("hourly=%v area %s = %v, want %v", hourly, slug, pts, v)
			}
		}
		// The official series is one station, not the three behind the area.
		for _, n := range counts["two-plus-one"] {
			if n != 1 {
				t.Errorf("hourly=%v two-plus-one count = %d, want 1", hourly, n)
			}
		}
		band, err := s.AreaSeriesBand(ctx, "two-plus-one", "P2", since, nil, hourly, bucket)
		if err != nil {
			t.Fatalf("AreaSeriesBand: %v", err)
		}
		if len(band) != 1 || band[0].Low != 40 || band[0].High != 40 {
			t.Errorf("hourly=%v two-plus-one band = %v, want 40..40", hourly, band)
		}
	}
}
