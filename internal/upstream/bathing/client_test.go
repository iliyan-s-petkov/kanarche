package bathing_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/upstream/bathing"
)

const testUA = "kanarche.test collector (+https://kanarche.test/)"

func testConfig(url string) config.Sea {
	return config.Sea{
		Enabled:         true,
		URL:             url,
		Country:         "BG",
		UserAgent:       testUA,
		RequestTimeout:  5 * time.Second,
		RefreshInterval: 168 * time.Hour,
		MaxPayloadBytes: 1 << 20,
		MaxRows:         1000,
	}
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// discodata answers each of the three queries with its fixture, by table name.
func discodata(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	sites, status, samples := fixture(t, "sites.json"), fixture(t, "status.json"), fixture(t, "samples.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if got := r.Header.Get("User-Agent"); got != testUA {
			t.Errorf("User-Agent = %q, want %q", got, testUA)
		}
		q := r.URL.Query().Get("query")
		if !strings.Contains(q, "countryCode='BG'") {
			t.Errorf("query %q is not scoped to BG", q)
		}
		if n := r.URL.Query().Get("nrOfHits"); n != "1000" {
			t.Errorf("nrOfHits = %q, want max_rows", r.URL.Query().Get("nrOfHits"))
		}
		switch {
		case strings.Contains(q, "spatial_ProtectedArea"):
			w.Write(sites)
		case strings.Contains(q, "assessment_BathingWaterStatus"):
			w.Write(status)
		case strings.Contains(q, "assessment_MonitoringResult"):
			w.Write(samples)
		default:
			t.Errorf("unexpected query %q", q)
			http.Error(w, "no", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestFetchReadsAllThreeTables(t *testing.T) {
	srv, calls := discodata(t)
	raw, err := bathing.New(testConfig(srv.URL)).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(raw.Sites) != 4 || len(raw.Status) != 12 || len(raw.Samples) != 25 {
		t.Errorf("got %d sites, %d status, %d samples; want 4, 12, 25", len(raw.Sites), len(raw.Status), len(raw.Samples))
	}
	if calls.Load() != 3 {
		t.Errorf("%d requests, want 3", calls.Load())
	}
}

func TestFetchSurfacesTheErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errors":[{"error":"Invalid object name 'x'.","errorcode":10003}]}`))
	}))
	defer srv.Close()
	_, err := bathing.New(testConfig(srv.URL)).Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Invalid object name") {
		t.Errorf("Fetch = %v, want the Discodata error text", err)
	}
}

// A page as long as nrOfHits may have more behind it; a partial import would drop sites silently.
func TestFetchRejectsAFullPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"inspireIdLocalId":"BG1"},{"inspireIdLocalId":"BG2"}]}`))
	}))
	defer srv.Close()
	cfg := testConfig(srv.URL)
	cfg.MaxRows = 2
	_, err := bathing.New(cfg).Fetch(context.Background())
	if !errors.Is(err, bathing.ErrTruncated) {
		t.Errorf("Fetch with max_rows 2 over a 2-row page = %v, want ErrTruncated", err)
	}
}

func TestFetchRejectsAnOversizedBody(t *testing.T) {
	srv, _ := discodata(t)
	cfg := testConfig(srv.URL)
	cfg.MaxPayloadBytes = 1024
	_, err := bathing.New(cfg).Fetch(context.Background())
	if !errors.Is(err, bathing.ErrPayloadTooLarge) {
		t.Errorf("Fetch = %v, want ErrPayloadTooLarge", err)
	}
}

func TestFetchRejectsANon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if _, err := bathing.New(testConfig(srv.URL)).Fetch(context.Background()); err == nil {
		t.Error("Fetch accepted a 503")
	}
}

func TestFetchRefusesAnOffOriginRedirect(t *testing.T) {
	var hit atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.Store(true)
		w.Write([]byte(`{"results":[]}`))
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/sql", http.StatusFound)
	}))
	defer srv.Close()
	if _, err := bathing.New(testConfig(srv.URL)).Fetch(context.Background()); err == nil {
		t.Error("Fetch followed a redirect to another origin")
	}
	if hit.Load() {
		t.Error("the other origin was contacted")
	}
}

// Country is spliced into SQL; the client refuses anything config validation would.
func TestFetchRefusesAnInvalidCountry(t *testing.T) {
	srv, calls := discodata(t)
	cfg := testConfig(srv.URL)
	cfg.Country = "BG' OR '1'='1"
	if _, err := bathing.New(cfg).Fetch(context.Background()); err == nil {
		t.Error("Fetch ran with a non-ISO country")
	}
	if calls.Load() != 0 {
		t.Errorf("%d requests sent, want none", calls.Load())
	}
}
