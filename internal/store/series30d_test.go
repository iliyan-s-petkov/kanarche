package store_test

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/store"
)

func shippedPeriod(t *testing.T, name string) config.Period {
	t.Helper()
	t.Setenv(config.DatabaseURLEnv, "postgres://user:pass@localhost:5432/airbg")
	cfg, err := config.LoadFile(filepath.Join("..", "..", "airbg.yaml"))
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	p, ok := cfg.Series.Periods[name]
	if !ok {
		t.Fatalf("period %q missing from airbg.yaml", name)
	}
	return p
}

// The 30d chart must be answerable from reading_hourly alone. Raw readings are
// deleted after the rollup, so a period that still aggregates `reading` returns
// nothing here, which is what timed out against the 5s series statement budget
// in production.
func TestSensorSeries30dServedFromRollup(t *testing.T) {
	ctx, pool := migrated(t)
	s := store.New(pool, testStoreConfig(), testSeriesTimeout)
	p := shippedPeriod(t, "30d")

	seedSensor(t, ctx, pool, 31, 23.0, 42.0)
	// A reading every 30 minutes for 30 days, plus one flagged spike per day that
	// the rollup must keep out of the average.
	_, err := pool.Exec(ctx,
		`INSERT INTO reading (time, sensor_id, metric, value, quality)
		 SELECT t, 31, 'P2', 10, 'ok'::quality_flag
		   FROM generate_series(date_trunc('hour', now()) - interval '30 days',
		                        date_trunc('hour', now()), interval '30 minutes') t
		 UNION ALL
		 SELECT t + interval '10 minutes', 31, 'P2', 900, 'out_of_range'::quality_flag
		   FROM generate_series(date_trunc('hour', now()) - interval '30 days',
		                        date_trunc('hour', now()), interval '1 day') t`)
	if err != nil {
		t.Fatalf("seed readings: %v", err)
	}
	if _, err := s.RollupAll(ctx); err != nil {
		t.Fatalf("RollupAll: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM reading WHERE sensor_id = 31`); err != nil {
		t.Fatalf("drop raw: %v", err)
	}

	now := time.Now().UTC()
	pts, err := s.SensorSeries(ctx, 31, "P2", now.Add(-p.Window), nil, p.Hourly, p.Bucket)
	if err != nil {
		t.Fatalf("SensorSeries: %v", err)
	}
	// 30 days of 6h buckets is 120, give or take the partial first and last.
	if len(pts) < 118 || len(pts) > 122 {
		t.Fatalf("got %d points, want about 120", len(pts))
	}
	for _, pt := range pts {
		if pt.Value != 10 {
			t.Fatalf("point %v = %v, want 10 (a flagged 900 leaked in)", pt.Time, pt.Value)
		}
	}
	if age := now.Sub(pts[len(pts)-1].Time); age > p.Bucket+time.Hour {
		t.Errorf("newest point is %v old, want within one bucket: the current hour must be in the rollup", age)
	}
}

// Re-bucketing hourly rows into 6h must weight by sample_count, or it differs
// from what the raw table would have answered: 1 sample at 10 and 9 samples at
// 30 average 28, not 20.
func TestSensorSeriesRebucketsHourlyWeightedBySampleCount(t *testing.T) {
	ctx, pool := migrated(t)
	s := store.New(pool, testStoreConfig(), testSeriesTimeout)

	seedSensor(t, ctx, pool, 32, 23.0, 42.0)
	// Midnight UTC is on a 6h bucket boundary.
	day := time.Now().UTC().Add(-10 * 24 * time.Hour).Truncate(24 * time.Hour)
	seedHourly(t, ctx, pool, 32, "P2", day.Add(time.Hour), 10, 1)
	seedHourly(t, ctx, pool, 32, "P2", day.Add(2*time.Hour), 30, 9)

	pts, err := s.SensorSeries(ctx, 32, "P2", day, nil, true, 6*time.Hour)
	if err != nil {
		t.Fatalf("SensorSeries: %v", err)
	}
	if len(pts) != 1 {
		t.Fatalf("got %d points, want 1", len(pts))
	}
	if math.Abs(pts[0].Value-28) > 1e-9 {
		t.Errorf("value = %v, want 28 (20 is the unweighted mean of means)", pts[0].Value)
	}
}

// An area's per-sensor value over a re-bucketed hour range must weight by
// sample_count, as the sensor series does. 1 sample at 10 and 9 at 30 are 28.
// One sensor, so the cross-sensor median does not mask the per-sensor step.
func TestAreaHourlySeriesWeightsSensorHoursBySampleCount(t *testing.T) {
	ctx, pool := migrated(t)
	s := store.New(pool, testStoreConfig(), testSeriesTimeout)

	seedArea(t, ctx, pool, "weighted", "oblast", 23.0, 42.0)
	seedSensor(t, ctx, pool, 33, 23.0, 42.0)
	assignAreas(t, ctx, pool)
	day := time.Now().UTC().Add(-10 * 24 * time.Hour).Truncate(24 * time.Hour)
	seedHourly(t, ctx, pool, 33, "P2", day.Add(time.Hour), 10, 1)
	seedHourly(t, ctx, pool, 33, "P2", day.Add(2*time.Hour), 30, 9)

	check := func(name string, got float64) {
		t.Helper()
		if math.Abs(got-28) > 1e-9 {
			t.Errorf("%s = %v, want 28 (20 is the unweighted mean of means)", name, got)
		}
	}

	pts, err := s.AreaSeries(ctx, "weighted", "P2", day, nil, true, 6*time.Hour)
	if err != nil || len(pts) != 1 {
		t.Fatalf("AreaSeries: %v, %d points", err, len(pts))
	}
	check("AreaSeries", pts[0].Value)

	band, err := s.AreaSeriesBand(ctx, "weighted", "P2", day, nil, true, 6*time.Hour)
	if err != nil || len(band) != 1 {
		t.Fatalf("AreaSeriesBand: %v, %d rows", err, len(band))
	}
	check("band median", band[0].Median)
	check("band min", band[0].Low)
	check("band max", band[0].High)

	all, err := s.AllAreaSeries(ctx, "P2", day, true, 6*time.Hour)
	if err != nil || len(all["weighted"]) != 1 {
		t.Fatalf("AllAreaSeries: %v, %v", err, all)
	}
	check("AllAreaSeries", all["weighted"][0].Value)
}
