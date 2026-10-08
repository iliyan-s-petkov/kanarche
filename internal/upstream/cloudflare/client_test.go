package cloudflare_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/upstream/cloudflare"
)

func testConfig(url string) config.Cloudflare {
	return config.Cloudflare{
		Enabled:        true,
		URL:            url,
		ZoneID:         "b9c4f3ae6570006c5b2d62f114dd4fd9",
		RequestTimeout: 5 * time.Second,
		PollInterval:   24 * time.Hour,
	}
}

const sampleBody = `{
  "data": {
    "viewer": {
      "zones": [
        {
          "httpRequests1dGroups": [
            {"dimensions": {"date": "2026-09-27"}, "uniq": {"uniques": 181}, "sum": {"requests": 900, "pageViews": 878}},
            {"dimensions": {"date": "2026-09-28"}, "uniq": {"uniques": 104}, "sum": {"requests": 300, "pageViews": 162}}
          ]
        }
      ]
    }
  }
}`

func TestFetchDailyParsesRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleBody))
	}))
	defer srv.Close()

	c := cloudflare.New(testConfig(srv.URL), "test-token")
	since := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	points, err := c.FetchDaily(context.Background(), since, until)
	if err != nil {
		t.Fatalf("FetchDaily: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("got %d points, want 2", len(points))
	}
	if points[0].Uniques != 181 || points[0].Requests != 900 || points[0].PageViews != 878 {
		t.Errorf("points[0] = %+v", points[0])
	}
	if !points[0].Date.Equal(since) {
		t.Errorf("points[0].Date = %v, want %v", points[0].Date, since)
	}
	if points[1].Uniques != 104 {
		t.Errorf("points[1] = %+v", points[1])
	}
}

// The token is a credential and must never appear in a log line or an error.
// The one thing this test can actually prove is that it reaches the server as
// the Authorization header the API expects, not as a query parameter or a
// logged value.
func TestFetchDailySendsBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if strings.Contains(r.URL.RawQuery, "s3cr3t") {
			t.Errorf("token leaked into the query string: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleBody))
	}))
	defer srv.Close()

	c := cloudflare.New(testConfig(srv.URL), "s3cr3t")
	if _, err := c.FetchDaily(context.Background(), time.Now(), time.Now()); err != nil {
		t.Fatalf("FetchDaily: %v", err)
	}
	if gotAuth != "Bearer s3cr3t" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer s3cr3t")
	}
}

func TestFetchDailyReturnsGraphQLErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": null, "errors": [{"message": "zone not found"}]}`))
	}))
	defer srv.Close()

	c := cloudflare.New(testConfig(srv.URL), "test-token")
	_, err := c.FetchDaily(context.Background(), time.Now(), time.Now())
	if err == nil || !strings.Contains(err.Error(), "zone not found") {
		t.Fatalf("err = %v, want one mentioning %q", err, "zone not found")
	}
}

func TestFetchDailyRejectsNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := cloudflare.New(testConfig(srv.URL), "test-token")
	_, err := c.FetchDaily(context.Background(), time.Now(), time.Now())
	if err == nil {
		t.Fatal("err = nil, want an error for a 401")
	}
}

// An empty zones list means the configured zone_id matched nothing. The
// error must name the zone id, since that is what an operator needs to fix,
// and must never mention the token.
func TestFetchDailyEmptyZonesErrorNamesZoneNotToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"viewer": {"zones": []}}}`))
	}))
	defer srv.Close()

	c := cloudflare.New(testConfig(srv.URL), "s3cr3t-token")
	_, err := c.FetchDaily(context.Background(), time.Now(), time.Now())
	if err == nil {
		t.Fatal("err = nil, want an error for an empty zones list")
	}
	if !strings.Contains(err.Error(), "b9c4f3ae6570006c5b2d62f114dd4fd9") {
		t.Errorf("err = %q, want it to name the zone id", err.Error())
	}
	if strings.Contains(err.Error(), "s3cr3t-token") {
		t.Errorf("err = %q, must never contain the token", err.Error())
	}
}

func TestFetchDailySendsZoneAndDateRange(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleBody))
	}))
	defer srv.Close()

	c := cloudflare.New(testConfig(srv.URL), "test-token")
	since := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	if _, err := c.FetchDaily(context.Background(), since, until); err != nil {
		t.Fatalf("FetchDaily: %v", err)
	}
	vars, ok := gotBody["variables"].(map[string]any)
	if !ok {
		t.Fatalf("request body has no variables map: %+v", gotBody)
	}
	if vars["zoneTag"] != "b9c4f3ae6570006c5b2d62f114dd4fd9" {
		t.Errorf("zoneTag = %v, want the configured zone id", vars["zoneTag"])
	}
	if vars["since"] != "2026-08-30" {
		t.Errorf("since = %v, want 2026-08-30", vars["since"])
	}
	if vars["until"] != "2026-09-28" {
		t.Errorf("until = %v, want 2026-09-28", vars["until"])
	}
}
