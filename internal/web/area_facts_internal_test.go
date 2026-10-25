package web

import (
	"encoding/json"
	"os"
	"testing"

	"kanarche.eu/internal/upstream/eea"
)

// The 14 municipality-boundary cities must be real city slugs, or the place sentence would describe the wrong boundary.
func TestMunicipalityCitiesAreCommittedCitySlugs(t *testing.T) {
	raw, err := os.ReadFile("../../data/boundaries/cities.geojson")
	if err != nil {
		t.Fatal(err)
	}
	var fc struct {
		Features []struct {
			Properties struct {
				Slug string `json:"slug"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &fc); err != nil {
		t.Fatal(err)
	}
	cities := map[string]bool{}
	for _, f := range fc.Features {
		cities[f.Properties.Slug] = true
	}
	if len(municipalityCities) != 14 {
		t.Errorf("%d municipality cities, want 14", len(municipalityCities))
	}
	for slug := range municipalityCities {
		if !cities[slug] {
			t.Errorf("%q is not a slug in cities.geojson", slug)
		}
	}
	if municipalityCities["shumen"] || municipalityCities["sofia"] {
		t.Errorf("shumen and sofia use their own city boundary")
	}
}

// Every committed station must map to catalogue words, so no station prints a raw or empty classification.
func TestEveryCommittedStationHasKnownClassWords(t *testing.T) {
	codes := eea.StationCodes()
	if len(codes) != 32 {
		t.Fatalf("%d committed stations, want 32", len(codes))
	}
	for _, code := range codes {
		info, ok := eea.LookupStationInfo(code)
		if !ok {
			t.Fatalf("%s: no info", code)
		}
		typ, area := normStationType(info.Type), normStationArea(info.Area)
		if stationWordIDs[typ] == "" {
			t.Errorf("%s: type %q has no catalogue id", code, info.Type)
		}
		if stationWordIDs[area] == "" {
			t.Errorf("%s: area %q has no catalogue id", code, info.Area)
		}
	}
}
