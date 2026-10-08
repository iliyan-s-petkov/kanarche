package datahub

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

// pinnedURL is the format of sea.datahub.url in airbg.yaml.
const pinnedURL = "https://sdi.eea.europa.eu/datastore/public/eea_t_bathing-water-status_p_1990-2025_v01_r00/bw_assessment_eea_datahub_1990_2025.xlsx"

const pinnedPath = "/eea_t_bathing-water-status_p_1990-2025_v01_r00/bw_assessment_eea_datahub_1990_2025.xlsx"

// watchServer answers every request with status and counts them.
func watchServer(t *testing.T, status int, hits *atomic.Int32, method *atomic.Value) (FetchConfig, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if method != nil {
			method.Store(r.Method)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return FetchConfig{
		URL:          srv.URL + pinnedPath,
		AllowedHosts: []string{u.Hostname()},
		Timeout:      5 * time.Second,
		Client:       &http.Client{Transport: srv.Client().Transport},
	}, srv
}

func TestWatchHeadOn200SetsGauge(t *testing.T) {
	var hits atomic.Int32
	var method atomic.Value
	cfg, _ := watchServer(t, http.StatusOK, &hits, &method)
	newEditionFound.Set(0)
	found, err := Watch(context.Background(), cfg)
	if err != nil || !found {
		t.Fatalf("Watch = %v, %v, want true, nil", found, err)
	}
	if method.Load() != http.MethodHead {
		t.Errorf("method = %v, want HEAD", method.Load())
	}
	if newEditionFound.Value() != 1 {
		t.Errorf("gauge = %v, want 1", newEditionFound.Value())
	}
}

func TestWatch404ClearsGauge(t *testing.T) {
	var hits atomic.Int32
	cfg, _ := watchServer(t, http.StatusNotFound, &hits, nil)
	newEditionFound.Set(1)
	found, err := Watch(context.Background(), cfg)
	if err != nil || found {
		t.Fatalf("Watch = %v, %v, want false, nil", found, err)
	}
	if newEditionFound.Value() != 0 {
		t.Errorf("gauge = %v, want 0 after 404", newEditionFound.Value())
	}
}

func TestWatchOtherStatusIsErrorAndKeepsGauge(t *testing.T) {
	for _, code := range []int{http.StatusForbidden, http.StatusInternalServerError} {
		var hits atomic.Int32
		cfg, _ := watchServer(t, code, &hits, nil)
		newEditionFound.Set(1)
		found, err := Watch(context.Background(), cfg)
		if found || err == nil {
			t.Errorf("status %d: Watch = %v, %v, want false, error", code, found, err)
		}
		if newEditionFound.Value() != 1 {
			t.Errorf("status %d: gauge changed to %v", code, newEditionFound.Value())
		}
	}
}

func TestWatchRejectsOffHostRedirect(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, "https://evil.example.com/file.xlsx", http.StatusMovedPermanently)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	cfg := FetchConfig{
		URL: srv.URL + pinnedPath, AllowedHosts: []string{u.Hostname()}, Timeout: 5 * time.Second,
		Client: &http.Client{Transport: srv.Client().Transport},
	}
	_, err := Watch(context.Background(), cfg)
	if !errors.Is(err, ErrRedirect) {
		t.Errorf("err = %v, want ErrRedirect", err)
	}
}

// The allowlist comes from config, so a host outside it is refused before any request.
func TestWatchHostOutsideAllowlistMakesNoRequest(t *testing.T) {
	var hits atomic.Int32
	cfg, _ := watchServer(t, http.StatusOK, &hits, nil)
	cfg.AllowedHosts = []string{"sdi.eea.europa.eu"}
	_, err := Watch(context.Background(), cfg)
	if !errors.Is(err, ErrHostNotAllowed) {
		t.Errorf("err = %v, want ErrHostNotAllowed", err)
	}
	if hits.Load() != 0 {
		t.Errorf("%d requests made", hits.Load())
	}
}

func TestWatchPlainHTTPRefused(t *testing.T) {
	_, err := Watch(context.Background(), FetchConfig{
		URL: "http://sdi.eea.europa.eu" + pinnedPath, AllowedHosts: []string{"sdi.eea.europa.eu"}, Timeout: time.Second,
	})
	if !errors.Is(err, ErrNotHTTPS) {
		t.Errorf("err = %v, want ErrNotHTTPS", err)
	}
}

// A URL with no year derives to itself, so probing it would only re-find the pinned file.
func TestWatchUnchangedURLMakesNoRequest(t *testing.T) {
	var hits atomic.Int32
	cfg, srv := watchServer(t, http.StatusOK, &hits, nil)
	cfg.URL = srv.URL + "/latest/bw_assessment.xlsx"
	newEditionFound.Set(0)
	found, err := Watch(context.Background(), cfg)
	if found || err != nil {
		t.Errorf("Watch = %v, %v, want false, nil", found, err)
	}
	if hits.Load() != 0 {
		t.Errorf("%d requests made", hits.Load())
	}
	if newEditionFound.Value() != 0 {
		t.Errorf("gauge = %v, want 0", newEditionFound.Value())
	}
}

func TestDeriveNextEditionURL(t *testing.T) {
	tests := []struct{ name, current, want string }{
		{"pinned format", pinnedURL,
			"https://sdi.eea.europa.eu/datastore/public/eea_t_bathing-water-status_p_1990-2026_v01_r01/bw_assessment_eea_datahub_1990_2026.xlsx"},
		{"single year", "https://sdi.eea.europa.eu/eea_t_bathing_p_2025_v01_r00/data_2025.xlsx",
			"https://sdi.eea.europa.eu/eea_t_bathing_p_2026_v01_r01/data_2026.xlsx"},
		{"no year", "https://sdi.eea.europa.eu/latest/data.xlsx", "https://sdi.eea.europa.eu/latest/data.xlsx"},
		// Port and host digits are not edition years, so only the path may change.
		{"port with 20xx, no year in path", "https://127.0.0.1:42010/latest/bw_assessment.xlsx",
			"https://127.0.0.1:42010/latest/bw_assessment.xlsx"},
		{"port with 20xx and year in path", "https://127.0.0.1:42025/eea_t_p_2025_v01_r00/data_2025.xlsx",
			"https://127.0.0.1:42025/eea_t_p_2026_v01_r01/data_2026.xlsx"},
		{"host with 20xx, no year in path", "https://data2024.example.eu/latest/data.xlsx",
			"https://data2024.example.eu/latest/data.xlsx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveNextEditionURL(tt.current); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
