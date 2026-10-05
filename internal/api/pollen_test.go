package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"airbg.org/internal/api"
	"airbg.org/internal/snapshot"
	"airbg.org/internal/store"
)

func withPollen(t *testing.T, snap *snapshot.Snapshot) *snapshot.Snapshot {
	t.Helper()
	cfg := testConfig(t).Pollen
	days := []string{"2026-10-04", "2026-10-05", "2026-10-06"}
	v := snapshot.PollenViewForTesting([]store.AreaPollenDay{
		{Slug: "sofia", Species: "ragweed", Day: "2026-10-04", Mean: 12, Max: 30},
	}, cfg, days)["sofia"]
	if err := snap.SetPollenForTesting("sofia", v, cfg, time.Date(2026, 10, 4, 9, 20, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestAreaPollenServesTheTable(t *testing.T) {
	rec := serve(t, deps(t, withPollen(t, fixture(t))), get("/api/v1/area/sofia/pollen", "203.0.113.9"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var got struct {
		Area        string `json:"area"`
		Forecast    bool   `json:"forecast"`
		Attribution struct {
			URL string `json:"url"`
		} `json:"attribution"`
		Summary struct {
			Level   string `json:"level"`
			Species string `json:"species"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body: %v", err)
	}
	if got.Area != "sofia" || !got.Forecast || got.Attribution.URL == "" || got.Summary.Level != "moderate" || got.Summary.Species != "ragweed" {
		t.Errorf("body = %+v", got)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.HasPrefix(cc, "public") {
		t.Errorf("Cache-Control = %q, want public: the forecast is not per-sensor data", cc)
	}
}

func TestAreaPollenUnknownSlugIs404(t *testing.T) {
	rec := serve(t, deps(t, withPollen(t, fixture(t))), get("/api/v1/area/nope/pollen", "203.0.113.9"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// A known area without a forecast is unavailable, not empty.
func TestAreaPollenWithoutAForecastIs503(t *testing.T) {
	rec := serve(t, deps(t, fixture(t)), get("/api/v1/area/sofia/pollen", "203.0.113.9"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestAttributionsCreditCAMSForPollen(t *testing.T) {
	for _, a := range api.Attributions() {
		if a.Source == "open-meteo" {
			if !strings.Contains(a.Text, "Copernicus") || !strings.Contains(a.Text, "Open-Meteo") {
				t.Errorf("open-meteo attribution = %q, want it to name Copernicus CAMS for pollen", a.Text)
			}
			return
		}
	}
	t.Error("no open-meteo attribution")
}

func TestPollenMapServesTodaysLevelPerProvince(t *testing.T) {
	rec := serve(t, deps(t, withPollen(t, fixture(t))), get("/api/v1/pollen", "203.0.113.9"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var got struct {
		Date  string `json:"date"`
		Areas []struct {
			Slug  string `json:"slug"`
			Level string `json:"level"`
		} `json:"areas"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body: %v", err)
	}
	if got.Date != "2026-10-04" || len(got.Areas) != 1 || got.Areas[0].Slug != "sofia" || got.Areas[0].Level != "moderate" {
		t.Errorf("body = %+v", got)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.HasPrefix(cc, "public") {
		t.Errorf("Cache-Control = %q, want public", cc)
	}
}

func TestPollenMapWithoutAForecastIs503(t *testing.T) {
	rec := serve(t, deps(t, fixture(t)), get("/api/v1/pollen", "203.0.113.9"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}
