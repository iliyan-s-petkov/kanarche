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
			if len(g.DistrictBlocks) == 0 {
				continue
			}
			seen++
			if got := g.DistrictBlocks[0].Label; got != want[lang][g.Oblast.Slug] {
				t.Errorf("%s: %s districts label = %q, want %q", lang, g.Oblast.Slug, got, want[lang][g.Oblast.Slug])
			}
		}
		if seen != 2 {
			t.Errorf("%s: groups with districts = %d, want 2", lang, seen)
		}
	}
}

// TestAreasDirectoryKeepsEveryCitysDistricts: an oblast with two cities that
// both have districts lists both blocks, in city name order.
func TestAreasDirectoryKeepsEveryCitysDistricts(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()
	snap.KnownSlugs["burgas"] = snapshot.AreaMeta{Slug: "burgas", Kind: "city", NameBG: "Бургас", NameEN: "Burgas",
		ParentSlug: "sofia-grad-oblast", CentroidLon: 27.46, CentroidLat: 42.50, DefaultZoom: 11}
	snap.KnownSlugs["slaveykov"] = snapshot.AreaMeta{Slug: "slaveykov", Kind: "neighbourhood", NameBG: "Славейков", NameEN: "Slaveykov",
		ParentSlug: "burgas", CentroidLon: 27.46, CentroidLat: 42.51, DefaultZoom: 13}

	for _, g := range rr.buildDirectory(snap, i18n.DefaultLang) {
		if g.Oblast.Slug != "sofia-grad-oblast" {
			continue
		}
		if len(g.DistrictBlocks) != 2 {
			t.Fatalf("sofia-grad-oblast blocks = %d, want 2", len(g.DistrictBlocks))
		}
		if got := g.DistrictBlocks[0].Label; got != "Райони на Бургас" {
			t.Errorf("first block label = %q, want Райони на Бургас", got)
		}
		if got := g.DistrictBlocks[1].Label; got != "Райони на София" {
			t.Errorf("second block label = %q, want Райони на София", got)
		}
		if len(g.DistrictBlocks[0].Rows) != 1 || g.DistrictBlocks[0].Rows[0].Slug != "slaveykov" {
			t.Errorf("first block rows = %+v, want slaveykov", g.DistrictBlocks[0].Rows)
		}
		return
	}
	t.Fatal("sofia-grad-oblast group missing")
}
