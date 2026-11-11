package web_test

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// og:site_name is the localized product name.
func TestOGSiteNameIsTheProductName(t *testing.T) {
	rr := renderer(t, seoFixture(t))
	for _, path := range []string{"/", "/en/", "/area/plovdiv", "/en/about-the-data"} {
		body := fetch(t, rr, path).Body.String()
		brand := testBrand(map[bool]string{true: "/en", false: ""}[strings.HasPrefix(path, "/en")])
		if got := tagAttr(t, body, `<meta property="og:site_name" content="`); got != brand {
			t.Errorf("%s: og:site_name = %q, want %q", path, got, brand)
		}
		if alt := tagAttr(t, body, `<meta property="og:image:alt" content="`); strings.Contains(alt, "Моят въздух") || strings.Contains(alt, "My Air") {
			t.Errorf("%s: og:image:alt %q carries the old brand", path, alt)
		}
	}
}

// The Dataset JSON-LD describes citizen sensors only: prod has no official
// data, so no ИАОС/ЕАОС/official-station claim.
func TestDatasetJSONLDNamesCitizenSensorsOnly(t *testing.T) {
	rr := renderer(t, seoFixture(t))
	official := regexp.MustCompile(`ИАОС|ЕАОС|ExEA|EEA|official|станци`)
	for _, path := range []string{"/about-the-data", "/en/about-the-data"} {
		doc := jsonLD(t, path, fetch(t, rr, path).Body.String())
		ds := nodeOfType(doc, "Dataset")
		if ds == nil {
			t.Fatalf("%s: no Dataset node", path)
		}
		for _, text := range []string{ds.Name, ds.Description} {
			if official.MatchString(text) {
				t.Errorf("%s: Dataset text %q names official stations", path, text)
			}
		}
		if !strings.Contains(ds.Description, "sensor.community") {
			t.Errorf("%s: Dataset description %q does not name sensor.community", path, ds.Description)
		}
	}
}

// The /areas description states 27 cities and 35 districts; pin those
// numbers to the boundary files so adding a boundary forces a copy update.
func TestAreasDescriptionCountsMatchBoundaryFiles(t *testing.T) {
	count := func(file string) int {
		data, err := os.ReadFile(filepath.Join("..", "..", "data", "boundaries", file))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", file, err)
		}
		var fc struct {
			Features []json.RawMessage `json:"features"`
		}
		if err := json.Unmarshal(data, &fc); err != nil {
			t.Fatalf("Unmarshal(%s): %v", file, err)
		}
		return len(fc.Features)
	}
	cities := count("cities.geojson")
	districts := count("sofia-districts.geojson") + count("plovdiv-districts.geojson") + count("varna-districts.geojson")

	rr := renderer(t, seoFixture(t))
	bg := pageDescription(t, fetch(t, rr, "/areas").Body.String())
	en := html.UnescapeString(pageDescription(t, fetch(t, rr, "/en/areas").Body.String()))
	if want := fmt.Sprintf("%d-те областни града и %d-те района на София, Пловдив и Варна", cities, districts); !strings.Contains(bg, want) {
		t.Errorf("bg /areas description %q does not contain %q", bg, want)
	}
	if want := fmt.Sprintf("all %d provincial capitals and %d districts in Sofia, Plovdiv and Varna", cities, districts); !strings.Contains(en, want) {
		t.Errorf("en /areas description %q does not contain %q", en, want)
	}
}
