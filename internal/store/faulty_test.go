package store_test

import (
	"testing"
	"time"

	"airbg.org/internal/store"
)

// faultyStore is newStore under testStoreConfig's faulty rule (airbg.yaml's).
func faultyStore(t *testing.T) (contextT, poolT, *store.Store) {
	t.Helper()
	return newStore(t)
}

// seedMixed writes ok readings then flagged ones for one sensor and metric,
// ten seconds apart from start, all inside start's hour.
func seedMixed(t *testing.T, ctx contextT, pool poolT, id int64, metric string, ok, flagged int, start time.Time) {
	t.Helper()
	at := start
	for range ok {
		seedReadingWithQuality(t, ctx, pool, id, metric, 10, "ok", at)
		at = at.Add(10 * time.Second)
	}
	for range flagged {
		seedReadingWithQuality(t, ctx, pool, id, metric, 900, "stuck", at)
		at = at.Add(10 * time.Second)
	}
}

func rollupAndRefresh(t *testing.T, ctx contextT, s *store.Store, now time.Time, buckets ...time.Time) {
	t.Helper()
	for _, b := range buckets {
		if _, err := s.RollupHour(ctx, b); err != nil {
			t.Fatalf("RollupHour(%v): %v", b, err)
		}
	}
	if _, err := s.RefreshFaulty(ctx, now); err != nil {
		t.Fatalf("RefreshFaulty: %v", err)
	}
}

func faultyOf(t *testing.T, ctx contextT, s *store.Store, id int64) []string {
	t.Helper()
	sensors, err := s.LatestSensors(ctx)
	if err != nil {
		t.Fatalf("LatestSensors: %v", err)
	}
	for _, sr := range sensors {
		if sr.SensorID == id {
			return sr.Faulty
		}
	}
	t.Fatalf("sensor %d not in LatestSensors", id)
	return nil
}

func TestFaultyShareThreshold(t *testing.T) {
	tests := []struct {
		name        string
		ok, flagged int
		want        bool
	}{
		{"exactly half of the minimum", 3, 3, true},
		{"49 percent", 51, 49, false},
		{"all flagged but below the minimum count", 0, 5, false},
		{"every reading flagged", 0, 6, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, pool, s := faultyStore(t)
			now := time.Now().UTC()
			hour := store.TruncateHour(now).Add(-2 * time.Hour)
			seedSensor(t, ctx, pool, 1, 23.3, 42.7)
			seedMixed(t, ctx, pool, 1, "P2", tc.ok, tc.flagged, hour)
			// A fresh row so the sensor is listed by LatestSensors at all.
			seedReadingWithQuality(t, ctx, pool, 1, "temperature", 20, "ok", now.Add(-time.Minute))
			rollupAndRefresh(t, ctx, s, now, hour)

			got := faultyOf(t, ctx, s, 1)
			isFaulty := len(got) == 1 && got[0] == "P2"
			if isFaulty != tc.want {
				t.Errorf("Faulty = %v, want P2 faulty = %v", got, tc.want)
			}
		})
	}
}

// The window is a sum across hours, not the newest hour alone.
func TestFaultySumsAcrossTheWindow(t *testing.T) {
	ctx, pool, s := faultyStore(t)
	now := time.Now().UTC()
	h1 := store.TruncateHour(now).Add(-5 * time.Hour)
	h2 := store.TruncateHour(now).Add(-3 * time.Hour)
	seedSensor(t, ctx, pool, 1, 23.3, 42.7)
	seedMixed(t, ctx, pool, 1, "P2", 0, 4, h1)
	seedMixed(t, ctx, pool, 1, "P2", 4, 0, h2)
	seedReadingWithQuality(t, ctx, pool, 1, "P2", 10, "ok", now.Add(-time.Minute))
	rollupAndRefresh(t, ctx, s, now, h1, h2)

	if got := faultyOf(t, ctx, s, 1); len(got) != 1 {
		t.Errorf("Faulty = %v, want [P2]: 4 flagged of 8 over the window", got)
	}
}

// Hours older than the window do not count.
func TestFaultyIgnoresHoursOutsideTheWindow(t *testing.T) {
	ctx, pool, s := faultyStore(t)
	now := time.Now().UTC()
	old := store.TruncateHour(now).Add(-30 * time.Hour)
	seedSensor(t, ctx, pool, 1, 23.3, 42.7)
	seedMixed(t, ctx, pool, 1, "P2", 0, 10, old)
	seedReadingWithQuality(t, ctx, pool, 1, "P2", 10, "ok", now.Add(-time.Minute))
	rollupAndRefresh(t, ctx, s, now, old)

	if got := faultyOf(t, ctx, s, 1); len(got) != 0 {
		t.Errorf("Faulty = %v, want none: the flagged hour is 30h old", got)
	}
}

// Faulty for P2 still counts for temperature, and its ok P2 reading leaves the
// area median.
func TestFaultySensorLeavesTheAreaMedianForThatMetricOnly(t *testing.T) {
	ctx, pool, s := faultyStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	hour := store.TruncateHour(now).Add(-2 * time.Hour)
	seedArea(t, ctx, pool, "sofia", "oblast", 23.3, 42.7)
	for i, v := range []float64{10, 12, 14} {
		id := int64(i + 1)
		seedSensorReading(t, ctx, pool, id, 23.3+float64(i)*0.001, 42.7, "P2", v, "ok", now.Add(-time.Minute))
		seedSensorReading(t, ctx, pool, id, 23.3+float64(i)*0.001, 42.7, "temperature", 18+2*float64(i), "ok", now.Add(-time.Minute))
	}
	seedSensorReading(t, ctx, pool, 4, 23.31, 42.7, "P2", 900, "ok", now.Add(-time.Minute))
	seedReadingWithQuality(t, ctx, pool, 4, "temperature", 30, "ok", now.Add(-time.Minute))
	seedMixed(t, ctx, pool, 4, "P2", 0, 6, hour)
	assignAreas(t, ctx, pool)
	rollupAndRefresh(t, ctx, s, now, hour)

	aggs, err := s.AreaAggregates(ctx, []string{"oblast"})
	if err != nil {
		t.Fatalf("AreaAggregates: %v", err)
	}
	if len(aggs) != 1 {
		t.Fatalf("got %d areas, want 1", len(aggs))
	}
	if got := aggs[0].Values["P2"]; got != 12 {
		t.Errorf("P2 median = %v, want 12 — the faulty sensor's ok 900 must not count", got)
	}
	if got := aggs[0].Values["temperature"]; got != 21 {
		t.Errorf("temperature median = %v, want 21 (median of 18,20,22,30) — sensor 4 is not faulty for temperature", got)
	}

	points, err := s.AreaSeries(ctx, "sofia", "P2", now.Add(-10*time.Minute), nil, false, time.Hour)
	if err != nil {
		t.Fatalf("AreaSeries: %v", err)
	}
	for _, p := range points {
		if p.Value != 12 {
			t.Errorf("area series point %v = %v, want 12", p.Time, p.Value)
		}
	}
	bands, err := s.AreaSeriesBand(ctx, "sofia", "P2", now.Add(-10*time.Minute), nil, false, time.Hour)
	if err != nil {
		t.Fatalf("AreaSeriesBand: %v", err)
	}
	for _, b := range bands {
		if b.High != 14 {
			t.Errorf("band high = %v, want 14", b.High)
		}
	}
	all, err := s.AllAreaSeries(ctx, "P2", now.Add(-10*time.Minute), false, time.Hour)
	if err != nil {
		t.Fatalf("AllAreaSeries: %v", err)
	}
	for _, p := range all["sofia"] {
		if p.Value != 12 {
			t.Errorf("all-area series point = %v, want 12", p.Value)
		}
	}
	counts, err := s.AllAreaSeriesCounts(ctx, "P2", now.Add(-10*time.Minute), false, time.Hour)
	if err != nil {
		t.Fatalf("AllAreaSeriesCounts: %v", err)
	}
	for _, n := range counts["sofia"] {
		if n != 3 {
			t.Errorf("bucket sensor count = %d, want 3", n)
		}
	}
}

// The rollup leaves out a faulty pair's ok readings, and drops a row it wrote
// before the pair became faulty.
func TestRollupLeavesOutFaultySensors(t *testing.T) {
	ctx, pool, s := faultyStore(t)
	now := time.Now().UTC()
	bad := store.TruncateHour(now).Add(-3 * time.Hour)
	hour := store.TruncateHour(now).Add(-2 * time.Hour)
	seedSensor(t, ctx, pool, 1, 23.3, 42.7)
	seedMixed(t, ctx, pool, 1, "P2", 2, 0, hour)
	seedMixed(t, ctx, pool, 1, "temperature", 2, 0, hour)
	if _, err := s.RollupHour(ctx, hour); err != nil {
		t.Fatalf("RollupHour: %v", err)
	}
	if n := hourlyRows(t, ctx, pool, 1, "P2", hour); n != 1 {
		t.Fatalf("before the sensor went faulty: %d P2 rows, want 1", n)
	}

	seedMixed(t, ctx, pool, 1, "P2", 0, 10, bad)
	rollupAndRefresh(t, ctx, s, now, bad, hour)

	if n := hourlyRows(t, ctx, pool, 1, "P2", hour); n != 0 {
		t.Errorf("faulty P2 still has %d rollup rows for its ok hour, want 0", n)
	}
	if n := hourlyRows(t, ctx, pool, 1, "temperature", hour); n != 1 {
		t.Errorf("temperature has %d rollup rows, want 1 — only P2 is faulty", n)
	}
}

// A sensor stays faulty until the share drops below the threshold.
func TestFaultyRecoversWhenTheShareDrops(t *testing.T) {
	ctx, pool, s := faultyStore(t)
	now := time.Now().UTC()
	bad := store.TruncateHour(now).Add(-4 * time.Hour)
	good := store.TruncateHour(now).Add(-2 * time.Hour)
	seedSensor(t, ctx, pool, 1, 23.3, 42.7)
	seedMixed(t, ctx, pool, 1, "P2", 0, 6, bad)
	seedReadingWithQuality(t, ctx, pool, 1, "P2", 10, "ok", now.Add(-time.Minute))
	rollupAndRefresh(t, ctx, s, now, bad)
	if got := faultyOf(t, ctx, s, 1); len(got) != 1 {
		t.Fatalf("Faulty = %v, want [P2] before recovery", got)
	}

	// Six ok of twelve is still half.
	seedMixed(t, ctx, pool, 1, "P2", 6, 0, good)
	rollupAndRefresh(t, ctx, s, now, good)
	if got := faultyOf(t, ctx, s, 1); len(got) != 1 {
		t.Fatalf("Faulty = %v, want [P2] at exactly half", got)
	}

	seedMixed(t, ctx, pool, 1, "P2", 1, 0, good.Add(30*time.Minute))
	rollupAndRefresh(t, ctx, s, now, good)
	if got := faultyOf(t, ctx, s, 1); len(got) != 0 {
		t.Errorf("Faulty = %v, want none once the share is 6/13", got)
	}
}

func hourlyRows(t *testing.T, ctx contextT, pool poolT, id int64, metric string, bucket time.Time) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM reading_hourly WHERE sensor_id = $1 AND metric = $2 AND bucket = $3`,
		id, metric, bucket).Scan(&n); err != nil {
		t.Fatalf("count hourly rows: %v", err)
	}
	return n
}
