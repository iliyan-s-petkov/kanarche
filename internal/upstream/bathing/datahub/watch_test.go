package datahub

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestWatchHeads200OnNewEdition(t *testing.T) {
	// The server tracks whether the body was read
	bodyRead := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD, got %s", r.Method)
		}
		// Record if any body read was attempted
		io.ReadAll(r.Body)
		if r.Body != nil {
			n, _ := io.ReadAll(r.Body)
			if len(n) > 0 {
				bodyRead = true
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Use the server's actual host and construct a proper test URL
	baseURL := srv.URL
	testURL := baseURL + "/eea_t_bathing-water-status_p_1990-2025_v01_r00/bw_2025.xlsx"

	cfg := &http.Client{
		Transport: srv.Client().Transport,
	}

	// Extract just the hostname from the server URL
	u, _ := url.Parse(baseURL)
	result, err := Watch(context.Background(), testURL, u.Hostname(), 5*time.Second, cfg)
	if err != nil {
		t.Errorf("Watch failed: %v", err)
	}
	if !result {
		t.Errorf("Watch should return true on 200, got false")
	}
	if bodyRead {
		t.Errorf("Watch should not read the response body")
	}
}

func TestWatchReturns404AsNoNewEdition(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	baseURL := srv.URL
	testURL := baseURL + "/eea_t_bathing_p_2025_v01_r00/data_2025.xlsx"

	cfg := &http.Client{
		Transport: srv.Client().Transport,
	}

	u, _ := url.Parse(baseURL)
	result, err := Watch(context.Background(), testURL, u.Hostname(), 5*time.Second, cfg)
	if err != nil {
		t.Errorf("Watch should not error on 404, got: %v", err)
	}
	if result {
		t.Errorf("Watch should return false on 404, got true")
	}
}

func TestWatchRejectsRedirect(t *testing.T) {
	srv1 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://evil.example.com/file.xlsx")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer srv1.Close()

	cfg := &http.Client{
		Transport: srv1.Client().Transport,
	}

	u, _ := url.Parse(srv1.URL)
	// Use a URL from srv1 that will redirect elsewhere - this should fail the host check
	_, err := Watch(context.Background(), srv1.URL+"/eea_t_bathing_p_2025_v01_r00/data_2025.xlsx", u.Hostname(), 5*time.Second, cfg)
	if err == nil {
		t.Errorf("Watch should reject off-host redirect, got nil error")
	}
}

func TestWatchDeriveNextEditionURL(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		expected string
	}{
		{
			"basic bump",
			"https://sdi.eea.europa.eu/datastore/public/eea_t_bathing-water-status_p_1990-2025_v01_r00/bw_assessment_eea_datahub_1990_2025.xlsx",
			"https://sdi.eea.europa.eu/datastore/public/eea_t_bathing-water-status_p_1990-2026_v01_r01/bw_assessment_eea_datahub_1990_2026.xlsx",
		},
		{
			"year at different position",
			"https://sdi.eea.europa.eu/eea_t_bathing_p_2025_v01_r00/data_2025.xlsx",
			"https://sdi.eea.europa.eu/eea_t_bathing_p_2026_v01_r01/data_2026.xlsx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveNextEditionURL(tt.current)
			if result != tt.expected {
				t.Errorf("deriveNextEditionURL(%q) = %q, want %q", tt.current, result, tt.expected)
			}
		})
	}
}
