package web

import (
	"testing"

	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
)

// TestAreasDirectoryDistrictsNameTheirCity: the /areas directory labels each
// city's districts with that city, not with Sofia for every group.
func TestAreasDirectoryDistrictsNameTheirCity(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()
	snap.KnownSlugs["zapaden"] = snapshot.AreaMeta{Slug: "zapaden", Kind: "neighbourhood", NameBG: "Западен", NameEN: "Zapaden",
		ParentSlug: "plovdiv", CentroidLon: 24.70, CentroidLat: 42.15, DefaultZoom: 13}

	want := map[string]map[string]string{
		"bg": {"plovdiv-oblast": "Райони на Пловдив", "sofia-grad-oblast": "Райони на София"},
		"en": {"plovdiv-oblast": "Plovdiv districts", "sofia-grad-oblast": "Sofia districts"},
	}
	for _, lang := range []string{i18n.DefaultLang, "en"} {
		seen := 0
		for _, g := range rr.buildDirectory(snap, lang) {
			if len(g.Districts) == 0 {
				continue
			}
			seen++
			if got := g.DistrictsLabel; got != want[lang][g.Oblast.Slug] {
				t.Errorf("%s: %s districts label = %q, want %q", lang, g.Oblast.Slug, got, want[lang][g.Oblast.Slug])
			}
		}
		if seen != 2 {
			t.Errorf("%s: groups with districts = %d, want 2", lang, seen)
		}
	}
}
