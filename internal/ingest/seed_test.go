package ingest_test

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/area"
	"kanarche.eu/internal/db"
	"kanarche.eu/internal/ingest"
	"kanarche.eu/internal/metrics"
	"kanarche.eu/internal/quality"
	"kanarche.eu/internal/store"
	"kanarche.eu/internal/testsupport"
	"kanarche.eu/internal/upstream"
)

// A frozen sensor must be flagged on the first poll after a restart, not after
// another 12 polls: the history is seeded from the reading table.
func TestSeededHistoryFlagsFrozenSensorOnFirstPoll(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := area.Import(ctx, pool, "../area/testdata/bulgaria.geojson", area.NationalBoundaryKind); err != nil {
		t.Fatalf("area.Import(bulgaria): %v", err)
	}
	st := store.New(pool, testStoreConfig(), testSeriesTimeout)

	now := time.Now().UTC().Truncate(time.Second)
	var prior []quality.Scored
	for i := 1; i <= 11; i++ {
		prior = append(prior, quality.Scored{
			Reading: reading(1, "P1", 42, 0, now.Add(-time.Duration(i)*5*time.Minute)),
			Flag:    quality.FlagOK,
		})
	}
	if err := st.UpsertSensors(ctx, prior, map[int64]string{1: "BG"}); err != nil {
		t.Fatalf("UpsertSensors: %v", err)
	}
	if _, err := st.WriteReadings(ctx, prior); err != nil {
		t.Fatalf("WriteReadings: %v", err)
	}

	run := func(seed bool) ingest.Stats {
		hist := quality.NewHistory(12)
		if seed {
			if _, err := st.SeedHistory(ctx, hist, 3*time.Hour, 12); err != nil {
				t.Fatalf("SeedHistory: %v", err)
			}
		}
		f := stubFetcher{readings: []upstream.Reading{reading(1, "P1", 42, 0, now)}}
		stats, err := ingest.New(f, st, hist, testScorer(), testAssignTimeout, testCountries).RunOnce(ctx)
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		return stats
	}

	if got := run(false).Flagged[quality.FlagStuck]; got != 0 {
		t.Fatalf("unseeded stuck = %d, want 0 (the restart gap this test closes)", got)
	}
	before := flaggedCount(t, "stuck")
	if got := run(true).Flagged[quality.FlagStuck]; got != 1 {
		t.Errorf("seeded stuck = %d, want 1 on the first poll", got)
	}

	// The counter is process-global and other tests flag stuck readings too, so
	// assert the delta over the seeded run, not an absolute value.
	if got := flaggedCount(t, "stuck") - before; got != 1 {
		t.Errorf("airbg_readings_flagged_total{flag=\"stuck\"} grew by %d over the seeded run, want 1", got)
	}
}

// flaggedCount scrapes the counter; a label not yet incremented has no line and reads 0.
func flaggedCount(t *testing.T, flag string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	prefix := `airbg_readings_flagged_total{flag="` + flag + `"} `
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if v, ok := strings.CutPrefix(line, prefix); ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				t.Fatalf("parse %q: %v", line, err)
			}
			return n
		}
	}
	return 0
}
