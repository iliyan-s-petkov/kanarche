package store_test

import (
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

// Quality is the worst flag across metrics; Flags says which metric carries
// which unusable flag, so a dead climate chip does not condemn the PM readings.
func TestLatestSensorsReportsUnusableFlagsPerMetric(t *testing.T) {
	ctx, pool := migrated(t)
	s := store.New(pool, testStoreConfig(), testSeriesTimeout)

	now := time.Now().UTC().Truncate(time.Minute)
	seedSensorReading(t, ctx, pool, 71, 23.0, 42.0, "P2", 10, "ok", now)
	seedReadingWithQuality(t, ctx, pool, 71, "temperature", 400, "out_of_range", now)
	seedReadingWithQuality(t, ctx, pool, 71, "humidity", 55, "no_neighbours", now)
	seedSensorReading(t, ctx, pool, 72, 23.1, 42.1, "P2", 12, "ok", now)

	sensors, err := s.LatestSensors(ctx)
	if err != nil {
		t.Fatalf("LatestSensors: %v", err)
	}
	flags := map[int64]map[string]string{}
	for _, sr := range sensors {
		flags[sr.SensorID] = sr.Flags
	}
	got := flags[71]
	if len(got) != 1 || got["temperature"] != "out_of_range" {
		t.Errorf("Flags[71] = %v, want only temperature=out_of_range (ok and no_neighbours are usable)", got)
	}
	if len(flags[72]) != 0 {
		t.Errorf("Flags[72] = %v, want none for a healthy sensor", flags[72])
	}
}
