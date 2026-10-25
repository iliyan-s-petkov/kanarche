package web_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

func TestHomeBigCitiesDistrictSuffix(t *testing.T) {
	rr := renderer(t, bigCitiesFixture(nil, time.Now()))
	for _, c := range []struct{ path, name, suffix string }{
		{"/", "Район6", ", София"},
		{"/en/", "District6", ", Sofia"},
	} {
		for _, r := range bigRows(t, rr, c.path) {
			isDistrict := strings.Contains(r, `/area/d`)
			if isDistrict && !strings.Contains(r, c.suffix+"</a>") {
				t.Errorf("%s: district row lacks suffix: %s", c.path, r)
			}
			if !isDistrict && strings.Contains(r, c.suffix+"</a>") {
				t.Errorf("%s: city row has suffix: %s", c.path, r)
			}
			if strings.Contains(r, `/area/d6"`) && !strings.Contains(r, ">"+c.name+c.suffix+"</a>") {
				t.Errorf("%s: d6 row text: %s", c.path, r)
			}
		}
	}
}

// geoNames reads slug to names from a boundary file, the prod name source.
func geoNames(t *testing.T, file string) map[string][2]string {
	t.Helper()
	raw, err := os.ReadFile("../../data/boundaries/" + file)
	if err != nil {
		t.Fatal(err)
	}
	var fc struct {
		Features []struct {
			Properties struct {
				Slug   string `json:"slug"`
				NameBG string `json:"name_bg"`
				NameEN string `json:"name_en"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &fc); err != nil {
		t.Fatal(err)
	}
	out := map[string][2]string{}
	for _, f := range fc.Features {
		out[f.Properties.Slug] = [2]string{f.Properties.NameBG, f.Properties.NameEN}
	}
	return out
}

func TestHomeBigCitiesUsesProdNames(t *testing.T) {
	known := map[string]snapshot.AreaMeta{}
	for slug, n := range geoNames(t, "cities.geojson") {
		known[slug] = snapshot.AreaMeta{Slug: slug, Kind: "city", NameBG: n[0], NameEN: n[1],
			Covered: true, SensorCount: 5, Values: map[string]float64{"P2": 10}}
	}
	for slug, n := range geoNames(t, "sofia-districts.geojson") {
		known[slug] = snapshot.AreaMeta{Slug: slug, Kind: "neighbourhood", ParentSlug: "sofia", NameBG: n[0], NameEN: n[1],
			Covered: true, SensorCount: 5, Values: map[string]float64{"P2": 10}}
	}
	rr := renderer(t, &snapshot.Snapshot{GeneratedAt: time.Now(), KnownSlugs: known})
	rowsBG, rowsEN := bigRows(t, rr, "/"), bigRows(t, rr, "/en/")
	if !strings.Contains(rowsBG[0], ">София</a>") || strings.Contains(rowsBG[0], "Sofia") {
		t.Errorf("bg sofia row: %s", rowsBG[0])
	}
	if !strings.Contains(rowsEN[0], ">Sofia</a>") || strings.Contains(rowsEN[0], "София") {
		t.Errorf("en sofia row: %s", rowsEN[0])
	}
	// A real district gets the city suffix in its own language.
	last := len(rowsBG) - 1
	if !strings.Contains(rowsBG[last], ", София</a>") || !strings.Contains(rowsEN[last], ", Sofia</a>") {
		t.Errorf("district rows: %s / %s", rowsBG[last], rowsEN[last])
	}
}
