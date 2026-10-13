package api_test

import (
	"strings"
	"testing"

	"kanarche.eu/internal/api"
)

func TestAttributionsNameBothNetworksAndTheirMaps(t *testing.T) {
	got := api.Attributions(true)

	want := map[string]string{
		"sensor.community": "https://maps.sensor.community/",
		"eea":              "https://eea.government.bg/kav/",
		"open-meteo":       "https://open-meteo.com/",
		"eea-bathing":      "https://www.eea.europa.eu/en/topics/in-depth/water/bathing-water",
		"rzi":              "https://kanarche.eu/licences",
		"cams-pollen":      "https://atmosphere.copernicus.eu/charts/packages/cams_air_quality/products/europe-air-quality-forecast-pollens",
	}
	seen := map[string]bool{}
	for _, a := range got {
		if url, ok := want[a.Source]; ok {
			if a.URL != url {
				t.Errorf("%s links to %q, want %q", a.Source, a.URL, url)
			}
			seen[a.Source] = true
		}
		if a.Text == "" {
			t.Errorf("attribution %q has no text", a.Source)
		}
	}
	for s := range want {
		if !seen[s] {
			t.Errorf("no attribution for %s", s)
		}
	}
}

// ODbL 1.0 and the EEA reuse terms both require attribution.
func TestEveryIngestedSourceIsCredited(t *testing.T) {
	credited := map[string]bool{}
	for _, a := range api.Attributions(true) {
		credited[a.Source] = true
	}
	for _, s := range []string{"sensor.community", "eea", "openstreetmap", "open-meteo", "eea-bathing", "rzi"} {
		if !credited[s] {
			t.Errorf("%s is used and not credited", s)
		}
	}
}

func TestRZIAttributionFollowsTheEnabledFlag(t *testing.T) {
	has := func(on bool) bool {
		for _, a := range api.Attributions(on) {
			if a.Source == "rzi" {
				return true
			}
		}
		return false
	}
	if !has(true) {
		t.Error("rzi enabled: attribution missing")
	}
	if has(false) {
		t.Error("rzi disabled: attribution must not be listed")
	}
}

func TestMetaListsRZIOnlyWhenEnabled(t *testing.T) {
	for _, on := range []bool{true, false} {
		d := deps(t, fixture(t))
		d.Config.Sea.RZI.Enabled = on
		body := serve(t, d, get("/api/v1/meta", "203.0.113.95")).Body.String()
		if got := strings.Contains(body, `"source":"rzi"`); got != on {
			t.Errorf("rzi enabled=%v: /meta lists rzi = %v", on, got)
		}
	}
}

func TestRZIAttributionText(t *testing.T) {
	const want = "Bathing water lab results: regional health inspectorates (RHI) Varna, Burgas and Dobrich. Derived data, CC BY 4.0"
	for _, a := range api.Attributions(true) {
		if a.Source == "rzi" && a.Text != want {
			t.Errorf("rzi text = %q, want %q", a.Text, want)
		}
	}
}
