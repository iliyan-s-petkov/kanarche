package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/api"
	"kanarche.eu/internal/store"
)

type visitorsBody struct {
	GeneratedAt time.Time        `json:"generated_at"`
	Days        []map[string]any `json:"days"`
}

// visitorRows returns n consecutive days ending today (UTC), oldest first.
func visitorRows(n int) []store.VisitorDaily {
	end := utcToday()
	out := make([]store.VisitorDaily, 0, n)
	for i := n - 1; i >= 0; i-- {
		day := end.AddDate(0, 0, -i)
		out = append(out, store.VisitorDaily{Day: day, Uniques: 100 + i, Requests: int64(1000 + i), PageViews: int64(500 + i), FetchedAt: day})
	}
	return out
}

func utcToday() time.Time {
	n := time.Now().UTC()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// days is a calendar window: a gap must not let the window reach further back.
func TestVisitorsDaysIsACalendarWindowNotARowCount(t *testing.T) {
	old, recent := utcToday().AddDate(0, 0, -40), utcToday().AddDate(0, 0, -2)
	rows := []store.VisitorDaily{{Day: old, Uniques: 1}, {Day: recent, Uniques: 2}}
	b, _, _ := visitorsGet(t, &stubSource{visitors: rows}, "?days=30")
	if len(b.Days) != 1 || b.Days[0]["day"] != recent.Format("2006-01-02") {
		t.Errorf("days = %v, want only %s", b.Days, recent.Format("2006-01-02"))
	}
	// The window includes today: days=3 reaches back to today-2.
	b, _, _ = visitorsGet(t, &stubSource{visitors: rows}, "?days=3")
	if len(b.Days) != 1 {
		t.Errorf("days=3: %v, want today-2 included", b.Days)
	}
	b, _, _ = visitorsGet(t, &stubSource{visitors: rows}, "?days=2")
	if len(b.Days) != 0 {
		t.Errorf("days=2: %v, want today-2 excluded", b.Days)
	}
}

func visitorsDeps(t *testing.T, src *stubSource) api.Deps {
	t.Helper()
	d := deps(t, fixture(t))
	d.Store = src
	return d
}

// serveOn sends one request through an already-built router, so per-router
// state (the visitors cache) persists across calls.
func serveOn(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, get(path, "203.0.113.7"))
	return rec
}

func visitorsGet(t *testing.T, src *stubSource, query string) (visitorsBody, int, http.Header) {
	t.Helper()
	rec := serve(t, visitorsDeps(t, src), get("/api/v1/visitors"+query, "203.0.113.7"))
	var b visitorsBody
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
			t.Fatalf("body is not JSON: %v\n%s", err, rec.Body.String())
		}
	}
	return b, rec.Code, rec.Header()
}

func TestVisitorsShape(t *testing.T) {
	b, code, h := visitorsGet(t, &stubSource{visitors: visitorRows(2)}, "")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if len(b.Days) != 2 {
		t.Fatalf("days = %d, want 2", len(b.Days))
	}
	first := b.Days[0]
	if first["day"] != utcToday().AddDate(0, 0, -1).Format("2006-01-02") {
		t.Errorf("day = %v, want yesterday", first["day"])
	}
	for _, k := range []string{"uniques", "requests", "page_views"} {
		if _, ok := first[k].(float64); !ok {
			t.Errorf("%s = %v, want a number", k, first[k])
		}
	}
	if len(first) != 4 {
		t.Errorf("day has keys %v, want exactly day, uniques, requests, page_views", first)
	}
	if b.GeneratedAt.IsZero() {
		t.Errorf("generated_at is zero")
	}
	if ct := h.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
}

// Cloudflare uniques are daily unique IPs; summing them across days overcounts.
func TestVisitorsCarriesNoSummedUniques(t *testing.T) {
	rec := serve(t, visitorsDeps(t, &stubSource{visitors: visitorRows(3)}), get("/api/v1/visitors", "203.0.113.7"))
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for k := range raw {
		if k != "days" && k != "generated_at" {
			t.Errorf("unexpected top-level field %q", k)
		}
	}
}

func TestVisitorsEmptyTableIsAnEmptyArray(t *testing.T) {
	rec := serve(t, visitorsDeps(t, &stubSource{}), get("/api/v1/visitors", "203.0.113.7"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"days":[]`) {
		t.Errorf("body = %s, want an empty days array, not null", rec.Body.String())
	}
}

func TestVisitorsKeepsAscendingOrderAndGaps(t *testing.T) {
	t1, t2 := utcToday().AddDate(0, 0, -9), utcToday().AddDate(0, 0, -6)
	rows := []store.VisitorDaily{{Day: t1, Uniques: 1}, {Day: t2, Uniques: 4}}
	b, _, _ := visitorsGet(t, &stubSource{visitors: rows}, "")
	if len(b.Days) != 2 || b.Days[0]["day"] != t1.Format("2006-01-02") || b.Days[1]["day"] != t2.Format("2006-01-02") {
		t.Errorf("days = %v, want the two stored days only, oldest first", b.Days)
	}
}

func TestVisitorsDaysParameter(t *testing.T) {
	cases := []struct {
		query string
		want  int
	}{
		{"", 30},
		{"?days=7", 7},
		{"?days=0", 1},
		{"?days=-5", 1},
		{"?days=365", 365},
		{"?days=9999", 365},
	}
	for _, c := range cases {
		b, code, _ := visitorsGet(t, &stubSource{visitors: visitorRows(400)}, c.query)
		if code != http.StatusOK {
			t.Errorf("%q: status = %d", c.query, code)
			continue
		}
		if len(b.Days) != c.want {
			t.Errorf("%q: %d days, want %d", c.query, len(b.Days), c.want)
		}
		// The newest day is always kept; clamping trims the old end.
		if last := b.Days[len(b.Days)-1]["day"]; last != utcToday().Format("2006-01-02") {
			t.Errorf("%q: last day = %v, want today", c.query, last)
		}
	}
}

func TestVisitorsRejectsNonIntegerDays(t *testing.T) {
	for _, q := range []string{"?days=abc", "?days=1.5", "?days=", "?days=7x", "?days=0x10"} {
		_, code, h := visitorsGet(t, &stubSource{visitors: visitorRows(3)}, q)
		if code != http.StatusBadRequest {
			t.Errorf("%q: status = %d, want 400", q, code)
		}
		if h.Get("Cache-Control") != "no-store" {
			t.Errorf("%q: error response Cache-Control = %q", q, h.Get("Cache-Control"))
		}
	}
}

func TestVisitorsIsPubliclyCacheable(t *testing.T) {
	rec := serve(t, visitorsDeps(t, &stubSource{visitors: visitorRows(1)}), get("/api/v1/visitors", "203.0.113.7"))
	vis, maxAge := cacheVisibility(t, rec, "/api/v1/visitors")
	if vis != "public" {
		t.Errorf("visibility = %q, want public: one aggregate, no per-entity key", vis)
	}
	if n, err := strconv.Atoi(maxAge); err != nil || n < 300 {
		t.Errorf("max-age = %q, want at least 300s for a once-a-day dataset", maxAge)
	}
}

func TestVisitorsQueriesTheDatabaseOncePerTTL(t *testing.T) {
	src := &stubSource{visitors: visitorRows(5)}
	r := router(t, visitorsDeps(t, src))
	for _, q := range []string{"", "?days=3", "?days=90", ""} {
		if code := serveOn(t, r, "/api/v1/visitors"+q).Code; code != http.StatusOK {
			t.Fatalf("%q: status = %d", q, code)
		}
	}
	if src.visitorCalls != 1 {
		t.Errorf("store called %d times, want 1: the data changes once a day", src.visitorCalls)
	}
}

func TestVisitorsStoreErrorIs500AndNotCached(t *testing.T) {
	src := &stubSource{visitorsErr: errors.New("boom: select from visitor_daily")}
	r := router(t, visitorsDeps(t, src))
	rec := serveOn(t, r, "/api/v1/visitors")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "visitor_daily") {
		t.Errorf("error body leaks the database error: %s", rec.Body.String())
	}
	src.visitorsErr = nil
	if code := serveOn(t, r, "/api/v1/visitors").Code; code != http.StatusOK {
		t.Errorf("after recovery status = %d, want 200: an error must not be cached", code)
	}
}
