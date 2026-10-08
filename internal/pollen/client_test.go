package pollen_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/pollen"
	"kanarche.eu/internal/store"
)

func testConfig() config.Pollen {
	return config.Pollen{
		Enabled:         true,
		URL:             "https://air-quality.invalid/v1/air-quality",
		UserAgent:       "test collector (+https://test.invalid)",
		Domain:          "cams_europe",
		Country:         "BG",
		LatticeDeg:      0.2,
		LatticeMarginKm: 10,
		CellReachKm:     25,
		RunAtUTC:        []string{"09:15", "21:15"},
		StaleAfter:      13 * time.Hour,
		PastDays:        1,
		ForecastDays:    4,
		DaysShown:       3,
		MinHours:        12,
		RequestTimeout:  5 * time.Second,
		PointsPerReq:    100,
		MaxPayloadBytes: 1 << 20,
		Species: []config.PollenSpecies{
			{Name: "ragweed", Levels: []float64{3, 50}},
			{Name: "birch", Levels: []float64{10, 100}},
		},
	}
}

// series renders one location's response for ragweed and birch.
func series(times []string, ragweed, birch []string) string {
	q := func(xs []string) string {
		out := make([]string, len(xs))
		for i, x := range xs {
			out[i] = `"` + x + `"`
		}
		return strings.Join(out, ",")
	}
	return fmt.Sprintf(`{"latitude":42.7,"longitude":23.3,
		"hourly_units":{"time":"iso8601","ragweed_pollen":"grains/m³","birch_pollen":"grains/m³"},
		"hourly":{"time":[%s],"ragweed_pollen":[%s],"birch_pollen":[%s]}}`,
		q(times), strings.Join(ragweed, ","), strings.Join(birch, ","))
}

func TestRequestURLAsksForTheConfiguredSpeciesAndDomain(t *testing.T) {
	cells := []store.PollenCell{{LonC: 2330, LatC: 4270}, {LonC: -5, LatC: 4200}}
	u, err := url.Parse(pollen.RequestURLForTesting(testConfig(), cells))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	for key, want := range map[string]string{
		"latitude":      "42.70,42.00",
		"longitude":     "23.30,-0.05",
		"hourly":        "ragweed_pollen,birch_pollen",
		"domains":       "cams_europe",
		"timezone":      "UTC",
		"past_days":     "1",
		"forecast_days": "4",
	} {
		if got := q.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if u.Host != "air-quality.invalid" || u.Path != "/v1/air-quality" {
		t.Errorf("url = %s, want the configured endpoint", u)
	}
}

func TestParseMapsByPositionAndDropsNulls(t *testing.T) {
	cells := []store.PollenCell{{LonC: 2320, LatC: 4260}, {LonC: 2340, LatC: 4280}}
	times := []string{"2026-10-04T00:00", "2026-10-04T01:00"}
	body := "[" + series(times, []string{"1.5", "null"}, []string{"0", "2"}) + "," +
		series(times, []string{"7", "8"}, []string{"null", "null"}) + "]"

	got, err := pollen.Parse([]byte(body), cells, []string{"ragweed", "birch"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d rows, want 5 (three nulls dropped): %+v", len(got), got)
	}
	want := store.PollenForecast{LonC: 2340, LatC: 4280, Species: "ragweed",
		ValidAt: time.Date(2026, 10, 4, 1, 0, 0, 0, time.UTC), Grains: 8}
	found := false
	for _, r := range got {
		if r == want {
			found = true
		}
		if r.Species == "ragweed" && r.LonC == 2320 && r.ValidAt.Hour() == 1 {
			t.Errorf("null stored as %+v", r)
		}
	}
	if !found {
		t.Errorf("rows %+v lack %+v", got, want)
	}
}

func TestParseRejectsALocationCountMismatch(t *testing.T) {
	cells := []store.PollenCell{{LonC: 2320, LatC: 4260}, {LonC: 2340, LatC: 4280}}
	body := "[" + series([]string{"2026-10-04T00:00"}, []string{"1"}, []string{"1"}) + "]"
	if _, err := pollen.Parse([]byte(body), cells, []string{"ragweed", "birch"}); err == nil {
		t.Error("Parse accepted one location for two cells")
	}
}

func TestParseRejectsAValueCountMismatch(t *testing.T) {
	cells := []store.PollenCell{{LonC: 2320, LatC: 4260}}
	body := "[" + series([]string{"2026-10-04T00:00", "2026-10-04T01:00"}, []string{"1"}, []string{"1", "2"}) + "]"
	if _, err := pollen.Parse([]byte(body), cells, []string{"ragweed", "birch"}); err == nil {
		t.Error("Parse accepted one ragweed value for two timestamps")
	}
}

func TestParseRejectsAnotherUnit(t *testing.T) {
	cells := []store.PollenCell{{LonC: 2320, LatC: 4260}}
	body := strings.Replace("["+series([]string{"2026-10-04T00:00"}, []string{"1"}, []string{"1"})+"]",
		`"birch_pollen":"grains/m³"`, `"birch_pollen":"µg/m³"`, 1)
	if _, err := pollen.Parse([]byte(body), cells, []string{"ragweed", "birch"}); err == nil {
		t.Error("Parse accepted birch in µg/m³")
	}
}

func TestParseRejectsAMissingSpecies(t *testing.T) {
	cells := []store.PollenCell{{LonC: 2320, LatC: 4260}}
	body := "[" + series([]string{"2026-10-04T00:00"}, []string{"1"}, []string{"1"}) + "]"
	if _, err := pollen.Parse([]byte(body), cells, []string{"ragweed", "grass"}); err == nil {
		t.Error("Parse accepted a response without grass")
	}
}

// fakeAPI answers every request with one series per requested latitude.
func fakeAPI(t *testing.T, requests *atomic.Int32, agent *atomic.Value) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if agent != nil {
			agent.Store(r.Header.Get("User-Agent"))
		}
		n := len(strings.Split(r.URL.Query().Get("latitude"), ","))
		parts := make([]string, n)
		for i := range parts {
			parts[i] = series([]string{"2026-10-04T00:00"}, []string{"3"}, []string{"4"})
		}
		_, _ = w.Write([]byte("[" + strings.Join(parts, ",") + "]"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchBatchesAndSendsTheUserAgent(t *testing.T) {
	var requests atomic.Int32
	var agent atomic.Value
	srv := fakeAPI(t, &requests, &agent)
	cfg := testConfig()
	cfg.URL = srv.URL
	cells := make([]store.PollenCell, 250)
	for i := range cells {
		cells[i] = store.PollenCell{LonC: 2200 + 20*(i%50), LatC: 4100 + 20*(i/50)}
	}
	got, err := pollen.New(cfg).Fetch(context.Background(), cells)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if n := requests.Load(); n != 3 {
		t.Errorf("requests = %d, want 3 batches of at most 100", n)
	}
	if len(got) != 500 {
		t.Errorf("rows = %d, want 250 cells x 2 species", len(got))
	}
	if a, _ := agent.Load().(string); a != cfg.UserAgent {
		t.Errorf("User-Agent = %q, want %q", a, cfg.UserAgent)
	}
}

func TestFetchFailsOnAnErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "limit", http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	cfg := testConfig()
	cfg.URL = srv.URL
	if _, err := pollen.New(cfg).Fetch(context.Background(), []store.PollenCell{{LonC: 2320, LatC: 4260}}); err == nil {
		t.Error("Fetch accepted a 429")
	}
}
