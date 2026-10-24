package web_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kanarche.eu/internal/snapshot"
)

// cardImage returns the og:image and twitter:image URLs of body, failing when they differ.
func cardImage(t *testing.T, path, body string) string {
	t.Helper()
	og := tagAttr(t, body, `<meta property="og:image" content="`)
	if tw := tagAttr(t, body, `<meta name="twitter:image" content="`); tw != og {
		t.Errorf("%s: twitter:image = %q, want it to follow og:image %q", path, tw, og)
	}
	return og
}

// TestAreaPageUsesItsOwnCard: an area page points og:image at the card for its
// slug in the page language, at the card size.
func TestAreaPageUsesItsOwnCard(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))
	for _, c := range []struct{ path, want string }{
		{"/area/plovdiv", "https://airbg.org/static/og/bg/plovdiv.png"},
		{"/en/area/plovdiv", "https://airbg.org/static/og/en/plovdiv.png"},
		{"/area/plovdiv-oblast", "https://airbg.org/static/og/bg/plovdiv-oblast.png"},
		{"/en/area/mladost", "https://airbg.org/static/og/en/mladost.png"},
	} {
		body := fetch(t, rr, c.path).Body.String()
		got := cardImage(t, c.path, body)
		if !strings.HasPrefix(got, c.want+"?") {
			t.Errorf("%s: og:image = %q, want the hashed %s", c.path, got, c.want)
		}
		if !hasMarker(body, `<meta property="og:image:width" content="1200">`) ||
			!hasMarker(body, `<meta property="og:image:height" content="630">`) {
			t.Errorf("%s: og:image size is not 1200x630", c.path)
		}
		if img := fetch(t, rr, strings.TrimPrefix(got, "https://airbg.org")); img.Code != http.StatusOK {
			t.Errorf("%s: card served status %d, want 200", c.path, img.Code)
		}
	}
}

// TestAreaPageWithoutCardFallsBack: a slug with no card file keeps the shared
// preview and its 1280x640 size.
func TestAreaPageWithoutCardFallsBack(t *testing.T) {
	snap := areaPageFixture(t)
	snap.KnownSlugs["no-card-yet"] = snapshot.AreaMeta{Slug: "no-card-yet", Kind: "city", NameBG: "Тест", NameEN: "Test",
		CentroidLon: 24, CentroidLat: 42, DefaultZoom: 11, Covered: true, SensorCount: 1}
	rr := renderer(t, snap)
	for _, path := range []string{"/area/no-card-yet", "/en/area/no-card-yet"} {
		body := fetch(t, rr, path).Body.String()
		if got := cardImage(t, path, body); !strings.HasPrefix(got, "https://airbg.org/static/social-preview.png") {
			t.Errorf("%s: og:image = %q, want the social-preview fallback", path, got)
		}
		if !hasMarker(body, `<meta property="og:image:width" content="1280">`) ||
			!hasMarker(body, `<meta property="og:image:height" content="640">`) {
			t.Errorf("%s: fallback size is not 1280x640", path)
		}
	}
}

// TestEveryBoundaryAreaHasACardInBothLanguages: a new area in the boundary
// files cannot ship without a card, because its page would silently fall back.
func TestEveryBoundaryAreaHasACardInBothLanguages(t *testing.T) {
	areas := ogBoundaryAreas(t, "oblasti.geojson", "cities.geojson", "sofia-districts.geojson")
	snap := areaPageFixture(t)
	snap.KnownSlugs = map[string]snapshot.AreaMeta{}
	for _, a := range areas {
		a.Kind, a.Covered, a.SensorCount = "city", true, 1
		snap.KnownSlugs[a.Slug] = a
	}
	rr := renderer(t, snap)
	for _, a := range areas {
		for _, lang := range []string{"bg", "en"} {
			path := "/area/" + a.Slug
			if lang == "en" {
				path = "/en" + path
			}
			body := fetch(t, rr, path).Body.String()
			if got := cardImage(t, path, body); !strings.Contains(got, "/static/og/"+lang+"/"+a.Slug+".png?") {
				t.Errorf("%s: og:image = %q, has no %s card", path, got, lang)
			}
		}
	}
}

// ogBoundaryAreas reads every feature of the given data/boundaries files.
func ogBoundaryAreas(t *testing.T, files ...string) []snapshot.AreaMeta {
	t.Helper()
	var out []snapshot.AreaMeta
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join("..", "..", "data", "boundaries", f))
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		var coll struct {
			Features []struct {
				Properties struct {
					Slug   string `json:"slug"`
					NameBG string `json:"name_bg"`
					NameEN string `json:"name_en"`
				} `json:"properties"`
			} `json:"features"`
		}
		if err := json.Unmarshal(raw, &coll); err != nil {
			t.Fatalf("parsing %s: %v", f, err)
		}
		for _, feat := range coll.Features {
			p := feat.Properties
			out = append(out, snapshot.AreaMeta{Slug: p.Slug, NameBG: p.NameBG, NameEN: p.NameEN,
				CentroidLon: 24, CentroidLat: 42, DefaultZoom: 11})
		}
	}
	return out
}
