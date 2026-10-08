package cloudflare_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kanarche.eu/internal/db"
	"kanarche.eu/internal/store"
	"kanarche.eu/internal/testsupport"
	"kanarche.eu/internal/upstream/cloudflare"
)

func newStoreForCollector(t *testing.T) (context.Context, *store.Store) {
	t.Helper()
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return ctx, store.New(pool, testsupport.StoreConfig(), 5*time.Second)
}

// cfServer answers the GraphQL query with one row per day in the requested
// range, echoing the since/until it was sent so tests can assert on the
// window a run actually requested.
type cfServer struct {
	srv       *httptest.Server
	gotSince  string
	gotUntil  string
	gotLimit  float64
	callCount int
}

func newCFServer(t *testing.T) *cfServer {
	t.Helper()
	s := &cfServer{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.callCount++
		var body struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		s.gotSince, _ = body.Variables["since"].(string)
		s.gotUntil, _ = body.Variables["until"].(string)
		s.gotLimit, _ = body.Variables["limit"].(float64)

		since, _ := time.Parse("2006-01-02", s.gotSince)
		until, _ := time.Parse("2006-01-02", s.gotUntil)

		type row struct {
			Dimensions struct {
				Date string `json:"date"`
			} `json:"dimensions"`
			Uniq struct {
				Uniques int `json:"uniques"`
			} `json:"uniq"`
			Sum struct {
				Requests  int64 `json:"requests"`
				PageViews int64 `json:"pageViews"`
			} `json:"sum"`
		}
		var rows []row
		for d := since; !d.After(until); d = d.AddDate(0, 0, 1) {
			var rr row
			rr.Dimensions.Date = d.Format("2006-01-02")
			rr.Uniq.Uniques = 10
			rr.Sum.Requests = 100
			rr.Sum.PageViews = 50
			rows = append(rows, rr)
		}

		resp := struct {
			Data struct {
				Viewer struct {
					Zones []struct {
						HTTPRequests1dGroups []row `json:"httpRequests1dGroups"`
					} `json:"zones"`
				} `json:"viewer"`
			} `json:"data"`
		}{}
		resp.Data.Viewer.Zones = []struct {
			HTTPRequests1dGroups []row `json:"httpRequests1dGroups"`
		}{{HTTPRequests1dGroups: rows}}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func TestRunOnceBackfillsThirtyDaysOnAnEmptyTable(t *testing.T) {
	ctx, s := newStoreForCollector(t)
	srv := newCFServer(t)

	c := cloudflare.NewCollector(testConfig(srv.srv.URL), "test-token", s)
	now := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	c.SetClockForTesting(func() time.Time { return now })

	stats, err := c.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !stats.Backfill {
		t.Error("Backfill = false, want true on an empty table")
	}
	if stats.Points != 30 || stats.Written != 30 {
		t.Errorf("Points=%d Written=%d, want 30/30", stats.Points, stats.Written)
	}
	if srv.gotSince != "2026-08-30" || srv.gotUntil != "2026-09-28" {
		t.Errorf("requested window [%s, %s], want [2026-08-30, 2026-09-28]", srv.gotSince, srv.gotUntil)
	}
}

// A gap of zero days (the collector already has today's row) still re-checks
// the minimum lookback window, to catch a correction to a day already closed.
func TestRunOnceUsesMinimumLookbackWhenDataIsCurrent(t *testing.T) {
	ctx, s := newStoreForCollector(t)
	now := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	if _, err := s.UpsertVisitorDaily(ctx, []store.VisitorDaily{
		{Day: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), Uniques: 1, Requests: 1, PageViews: 1, FetchedAt: now},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	srv := newCFServer(t)
	c := cloudflare.NewCollector(testConfig(srv.srv.URL), "test-token", s)
	c.SetClockForTesting(func() time.Time { return now })

	stats, err := c.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if stats.Backfill {
		t.Error("Backfill = true, want false when the table already has data")
	}
	if stats.Points != 3 {
		t.Errorf("Points = %d, want 3", stats.Points)
	}
	if srv.gotSince != "2026-09-26" || srv.gotUntil != "2026-09-28" {
		t.Errorf("requested window [%s, %s], want [2026-09-26, 2026-09-28]", srv.gotSince, srv.gotUntil)
	}
}

// A snapshot seeded well in the past must not be read as "empty" (which
// would only change the Backfill flag, not the window here since both cases
// clamp to the same 30-day ceiling) — it must still size the window off the
// actual gap, clamped to the retention ceiling, so the days between the
// snapshot and today get pulled.
func TestRunOnceFillsTheGapAfterAnOldSeededSnapshot(t *testing.T) {
	ctx, s := newStoreForCollector(t)

	oldSeed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snapshotEnd := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	if _, err := s.SeedVisitorDaily(ctx, []store.VisitorDaily{
		{Day: oldSeed, Uniques: 999, Requests: 999, PageViews: 999, FetchedAt: oldSeed},
		{Day: snapshotEnd, Uniques: 999, Requests: 999, PageViews: 999, FetchedAt: snapshotEnd},
	}); err != nil {
		t.Fatalf("SeedVisitorDaily: %v", err)
	}

	srv := newCFServer(t)
	c := cloudflare.NewCollector(testConfig(srv.srv.URL), "test-token", s)
	now := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	c.SetClockForTesting(func() time.Time { return now })

	stats, err := c.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if stats.Backfill {
		t.Error("Backfill = true, want false: the table already had a seeded row")
	}
	// gap = 39 days (Aug 20 -> Sep 28) + minLookbackDays, clamped to 30.
	if srv.gotSince != "2026-08-30" || srv.gotUntil != "2026-09-28" {
		t.Errorf("requested window [%s, %s], want [2026-08-30, 2026-09-28]", srv.gotSince, srv.gotUntil)
	}
	if stats.Points != 30 || stats.Written != 30 {
		t.Errorf("Points=%d Written=%d, want 30/30", stats.Points, stats.Written)
	}

	// Both seeded rows predate the fetched window and must be untouched.
	maxDay, ok, err := s.VisitorDailyMaxDay(ctx)
	if err != nil {
		t.Fatalf("VisitorDailyMaxDay: %v", err)
	}
	if !ok || !maxDay.Equal(time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("max day = %v (ok=%v), want 2026-09-28", maxDay, ok)
	}
}

func TestLoopDoesNothingWithoutAToken(t *testing.T) {
	ctx, s := newStoreForCollector(t)
	srv := newCFServer(t)

	c := cloudflare.NewCollector(testConfig(srv.srv.URL), "", s)
	runCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	c.Loop(runCtx) // must return promptly, not block until the ticker fires

	if srv.callCount != 0 {
		t.Errorf("callCount = %d, want 0: an empty token must never reach the API", srv.callCount)
	}
	_, ok, err := s.VisitorDailyMaxDay(ctx)
	if err != nil {
		t.Fatalf("VisitorDailyMaxDay: %v", err)
	}
	if ok {
		t.Error("table is not empty, but Loop should have done nothing")
	}
}
