package wind_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/db"
	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/store"
	"kanarche.eu/internal/testsupport"
	"kanarche.eu/internal/wind"
)

const collectorSeriesTimeout = 5 * time.Second

// testStoreConfig mirrors airbg.yaml's store: block, including the official
// window testsupport.StoreConfig omits — LatestSensors requires it to admit
// any reading at all (see internal/store/aggregate.go's freshnessPredicate).
func testStoreConfig() config.Store {
	return config.Store{
		CoverageThreshold:       3,
		FreshnessWindow:         2 * time.Hour,
		OfficialFreshnessWindow: 6 * time.Hour,
	}
}

func migratedStore(t *testing.T) (context.Context, *pgxpool.Pool, *store.Store) {
	t.Helper()
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return ctx, pool, store.New(pool, testStoreConfig(), collectorSeriesTimeout)
}

func testWindConfig(url string, pointsPerReq int) config.Wind {
	return config.Wind{
		URL:             url,
		Model:           "ecmwf_ifs025",
		ResolutionDeg:   0.25,
		RequestTimeout:  5 * time.Second,
		PollInterval:    time.Hour,
		ForecastHours:   24,
		PointsPerReq:    pointsPerReq,
		MaxPayloadBytes: 1 << 20,
		Retention:       48 * time.Hour,
	}
}

func countForecastRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM wind_forecast`).Scan(&n); err != nil {
		t.Fatalf("count wind_forecast: %v", err)
	}
	return n
}

// locationsInRequest reports how many comma-separated latitudes a batch
// request asked about — the request's own record of its batch size.
func locationsInRequest(r *http.Request) int {
	lat := r.URL.Query().Get("latitude")
	if lat == "" {
		return 0
	}
	return len(strings.Split(lat, ","))
}

// TestRunOnceStoresOneRowPerLatticePointPerHour is the success path: every
// lattice point, at every forecast hour the upstream returns, must land in
// wind_forecast. The database holds no sensors: the lattice does not need any.
func TestRunOnceStoresOneRowPerLatticePointPerHour(t *testing.T) {
	ctx, pool, s := migratedStore(t)
	points := len(snapshot.WindLattice())

	var requests int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		n := locationsInRequest(r)
		var b strings.Builder
		b.WriteByte('[')
		for i := 0; i < n; i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, `{"hourly":{"time":["2026-09-05T00:00","2026-09-05T01:00"],`+
				`"wind_speed_10m":[%d,%d],"wind_direction_10m":[10,20]}}`, 3+i, 4+i)
		}
		b.WriteByte(']')
		w.Write([]byte(b.String()))
	}))
	defer srv.Close()

	c := wind.NewCollector(testWindConfig(srv.URL, 100), s)
	n, err := c.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if want := int64(points * 2); n != want {
		t.Fatalf("RunOnce returned %d rows written, want %d", n, want)
	}
	if got := countForecastRows(t, ctx, pool); got != points*2 {
		t.Fatalf("wind_forecast has %d rows, want %d", got, points*2)
	}
	if want := (points + 99) / 100; requests != want {
		t.Fatalf("upstream got %d requests, want %d (batches of 100)", requests, want)
	}

	vs, _, model, err := s.CurrentWind(ctx, time.Date(2026, 9, 5, 0, 30, 0, 0, time.UTC), snapshot.WindGridKM)
	if err != nil {
		t.Fatalf("CurrentWind: %v", err)
	}
	if len(vs) != points {
		t.Fatalf("CurrentWind returned %d vectors for the first hour, want %d", len(vs), points)
	}
	if model != "ecmwf_ifs025" {
		t.Errorf("model = %q, want ecmwf_ifs025", model)
	}
}

// TestRunOnceUpstream5xxStoresNothing covers a failing upstream: a batch that
// fails must abort the whole cycle, not write whatever batches happened to
// succeed first.
func TestRunOnceUpstream5xxStoresNothing(t *testing.T) {
	ctx, pool, s := migratedStore(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := wind.NewCollector(testWindConfig(srv.URL, 100), s)
	n, err := c.RunOnce(ctx)
	if err == nil {
		t.Fatal("RunOnce succeeded against a 500 upstream, want an error")
	}
	if n != 0 {
		t.Errorf("RunOnce reported %d rows written on failure, want 0", n)
	}
	if got := countForecastRows(t, ctx, pool); got != 0 {
		t.Errorf("wind_forecast has %d rows after a failed cycle, want 0", got)
	}
}

// TestRunOnceBatchesAtThePointsPerReqBoundary pins the batch split itself:
// the lattice is cut into full batches and one short one, and every point's
// row is stored, not merely the first batch's.
func TestRunOnceBatchesAtThePointsPerReqBoundary(t *testing.T) {
	ctx, pool, s := migratedStore(t)
	points := len(snapshot.WindLattice())
	perReq := 200
	var want []int
	for left := points; left > 0; left -= perReq {
		want = append(want, min(perReq, left))
	}

	var mu sync.Mutex
	var batchSizes []int
	var nextSpeed float64 = 100 // increments across every point in every batch, never resets

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := locationsInRequest(r)

		mu.Lock()
		batchSizes = append(batchSizes, n)
		mu.Unlock()

		var b strings.Builder
		b.WriteByte('[')
		for i := 0; i < n; i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			mu.Lock()
			speed := nextSpeed
			nextSpeed++
			mu.Unlock()
			fmt.Fprintf(&b, `{"hourly":{"time":["2026-09-05T00:00"],`+
				`"wind_speed_10m":[%v],"wind_direction_10m":[0]}}`, speed)
		}
		b.WriteByte(']')
		w.Write([]byte(b.String()))
	}))
	defer srv.Close()

	c := wind.NewCollector(testWindConfig(srv.URL, perReq), s)
	n, err := c.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if n != int64(points) {
		t.Fatalf("RunOnce returned %d rows written, want %d", n, points)
	}
	if got := countForecastRows(t, ctx, pool); got != points {
		t.Fatalf("wind_forecast has %d rows, want %d", got, points)
	}

	mu.Lock()
	got := append([]int(nil), batchSizes...)
	mu.Unlock()
	if len(got) != len(want) {
		t.Fatalf("batch sizes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("batch sizes = %v, want %v", got, want)
		}
	}

	// Every point-index-derived speed must be stored exactly once: proof the
	// last, short batch was not dropped.
	var distinct int
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT speed_ms) FROM wind_forecast`).Scan(&distinct); err != nil {
		t.Fatalf("count distinct speeds: %v", err)
	}
	if distinct != points {
		t.Fatalf("stored %d distinct speeds, want %d", distinct, points)
	}
}

// TestRunOnceContextCancelledMidBatchStoresNothing covers cancellation while
// the fetch loop is partway through its batches: the first batch's rows must
// not be written just because it finished before the cycle was cancelled.
func TestRunOnceContextCancelledMidBatchStoresNothing(t *testing.T) {
	ctx, pool, s := migratedStore(t)

	runCtx, cancel := context.WithCancel(ctx)

	var requestCount int
	var mu sync.Mutex
	secondRequestArrived := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		this := requestCount
		mu.Unlock()

		if this == 2 {
			// Tell the test this batch is in flight, then wait for the
			// client's context to be cancelled before answering: the
			// in-flight request must be aborted by ctx, not completed.
			close(secondRequestArrived)
			<-runCtx.Done()
			return
		}

		n := locationsInRequest(r)
		var b strings.Builder
		b.WriteByte('[')
		for i := 0; i < n; i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"hourly":{"time":["2026-09-05T00:00"],"wind_speed_10m":[5],"wind_direction_10m":[0]}}`)
		}
		b.WriteByte(']')
		w.Write([]byte(b.String()))
	}))
	defer srv.Close()

	go func() {
		<-secondRequestArrived
		cancel()
	}()

	c := wind.NewCollector(testWindConfig(srv.URL, 1), s)
	n, err := c.RunOnce(runCtx)
	if err == nil {
		t.Fatal("RunOnce succeeded despite the context being cancelled mid-batch, want an error")
	}
	if n != 0 {
		t.Errorf("RunOnce reported %d rows written after cancellation, want 0", n)
	}
	if got := countForecastRows(t, context.Background(), pool); got != 0 {
		t.Errorf("wind_forecast has %d rows after a cancelled cycle, want 0 — the first batch's rows leaked", got)
	}
}
