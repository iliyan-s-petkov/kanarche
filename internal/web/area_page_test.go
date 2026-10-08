package web_test

import (
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

// areaPageFixture is seoFixture's kind roster plus the parent/centroid data
// SEO6's breadcrumb, link blocks and directory need — a plain city/oblast
// pair is not enough to exercise ParentSlug chains or the nearest-list.
func areaPageFixture(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	return &snapshot.Snapshot{
		GeneratedAt: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC),
		KnownSlugs: map[string]snapshot.AreaMeta{
			"plovdiv-oblast": {Slug: "plovdiv-oblast", Kind: "oblast", NameBG: "Пловдив", NameEN: "Plovdiv",
				CentroidLon: 24.75, CentroidLat: 42.14, DefaultZoom: 9, Covered: true, SensorCount: 137},
			"plovdiv": {Slug: "plovdiv", Kind: "city", NameBG: "Пловдив", NameEN: "Plovdiv",
				ParentSlug: "plovdiv-oblast", CentroidLon: 24.75, CentroidLat: 42.14, DefaultZoom: 11,
				Covered: true, SensorCount: 5, Values: map[string]float64{"P2": 12.3}},
			"sofiya-grad-oblast": {Slug: "sofiya-grad-oblast", Kind: "oblast", NameBG: "София-град", NameEN: "Sofia-City",
				CentroidLon: 23.32, CentroidLat: 42.69, DefaultZoom: 9, Covered: true, SensorCount: 412},
			"sofiya": {Slug: "sofiya", Kind: "city", NameBG: "София", NameEN: "Sofia",
				ParentSlug: "sofiya-grad-oblast", CentroidLon: 23.32, CentroidLat: 42.69, DefaultZoom: 11,
				Covered: true, SensorCount: 30, Values: map[string]float64{"P2": 8.0}},
			"mladost": {Slug: "mladost", Kind: "neighbourhood", NameBG: "Младост", NameEN: "Mladost",
				ParentSlug: "sofiya", CentroidLon: 23.37, CentroidLat: 42.65, DefaultZoom: 13,
				Covered: true, SensorCount: 4, Values: map[string]float64{"P2": 6.0}},
			"lozenets": {Slug: "lozenets", Kind: "neighbourhood", NameBG: "Лозенец", NameEN: "Lozenets",
				ParentSlug: "sofiya", CentroidLon: 23.32, CentroidLat: 42.68, DefaultZoom: 13,
				Covered: true, SensorCount: 6, Values: map[string]float64{"P2": 9.5}},
			"sofiyska-oblast": {Slug: "sofiyska-oblast", Kind: "oblast", NameBG: "Софийска", NameEN: "Sofia",
				CentroidLon: 23.7, CentroidLat: 42.5, DefaultZoom: 9, Covered: true, SensorCount: 40},
			"silistra-oblast": {Slug: "silistra-oblast", Kind: "oblast", NameBG: "Силистра", NameEN: "Silistra",
				CentroidLon: 27.26, CentroidLat: 44.12, DefaultZoom: 9, Covered: false, SensorCount: 0},
			"silistra": {Slug: "silistra", Kind: "city", NameBG: "Силистра", NameEN: "Silistra",
				ParentSlug: "silistra-oblast", CentroidLon: 27.26, CentroidLat: 44.12, DefaultZoom: 11,
				Covered: false, SensorCount: 0},
		},
	}
}

// TestAreaSentenceNoJS: the air-now sentence, the breadcrumb trail and the
// area-links blocks are all plain server HTML — a crawler or a no-JS reader
// gets the same content a scripted browser would build client-side.
func TestAreaSentenceNoJS(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))

	body := fetch(t, rr, "/area/mladost").Body.String()
	if !strings.Contains(body, `<p class="area-now">`) {
		t.Error("the area-now sentence is missing from the no-JS render")
	}
	if !strings.Contains(body, `<nav class="breadcrumb"`) {
		t.Error("the breadcrumb nav is missing")
	}
	if !strings.Contains(body, `>Област София-град<`) || !strings.Contains(body, `>София<`) {
		t.Error("the breadcrumb does not carry the oblast and city ancestors")
	}
	if !strings.Contains(body, `<nav class="area-links"`) {
		t.Error("the area-links block (parent/children/nearest) is missing")
	}
	if !strings.Contains(body, `href="/area/sofiya"`) {
		t.Error("the parent link to the city is missing")
	}
}

// TestAreasDirectoryLinksEveryAreaHTTP: /areas' server HTML links every area
// in the snapshot, in both languages — the crawlable list the plan calls for.
func TestAreasDirectoryLinksEveryAreaHTTP(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))

	for _, path := range []string{"/areas", "/en/areas"} {
		body := fetch(t, rr, path).Body.String()
		for slug := range areaPageFixture(t).KnownSlugs {
			want := `href="/area/` + slug + `"`
			if path == "/en/areas" {
				want = `href="/en/area/` + slug + `"`
			}
			if !strings.Contains(body, want) {
				t.Errorf("%s does not link area %q (want %s)", path, slug, want)
			}
		}
	}
}

// TestAreaPageNoDataExample: an uncovered city (Silistra, no recent sensors)
// states its absence in words rather than rendering a blank or a stale value.
func TestAreaPageNoDataExample(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))
	body := fetch(t, rr, "/area/silistra").Body.String()
	if !strings.Contains(body, "няма сензор със скорошни данни") {
		t.Errorf("the no-data city page does not state the absence in words:\n%s", body)
	}
}

// U06: an oblast page H1 carries the kind prefix; a city keeps the bare name.
func TestAreaH1CarriesOblastKindPrefix(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))
	cases := map[string]string{
		"/area/plovdiv-oblast":    `<h1 class="t-title">Област Пловдив</h1>`,
		"/en/area/plovdiv-oblast": `<h1 class="t-title">Plovdiv Province</h1>`,
		"/area/sofiya":            `<h1 class="t-title">София</h1>`,
	}
	for path, want := range cases {
		if body := fetch(t, rr, path).Body.String(); !strings.Contains(body, want) {
			t.Errorf("%s: missing %s", path, want)
		}
	}
}
