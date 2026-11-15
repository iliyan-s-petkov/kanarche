package web_test

import (
	"strings"
	"testing"

	"kanarche.eu/internal/snapshot"
)

// TestAreasDirectoryHTMLNamesDistrictCity: the rendered /areas page carries the
// per-city districts heading, so a template that hardcodes Sofia fails.
func TestAreasDirectoryHTMLNamesDistrictCity(t *testing.T) {
	snap := areaPageFixture(t)
	snap.KnownSlugs["zapaden"] = snapshot.AreaMeta{Slug: "zapaden", Kind: "neighbourhood", NameBG: "Западен", NameEN: "Zapaden",
		ParentSlug: "plovdiv", CentroidLon: 24.70, CentroidLat: 42.15, DefaultZoom: 13}
	rr := renderer(t, snap)
	for path, want := range map[string][]string{
		"/areas":    {"<h4>Райони на Пловдив</h4>", "<h4>Райони на София</h4>"},
		"/en/areas": {"<h4>Plovdiv districts</h4>", "<h4>Sofia districts</h4>"},
	} {
		body := fetch(t, rr, path).Body.String()
		for _, w := range want {
			if n := strings.Count(body, w); n != 1 {
				t.Errorf("%s: %q appears %d times, want 1", path, w, n)
			}
		}
	}
}
