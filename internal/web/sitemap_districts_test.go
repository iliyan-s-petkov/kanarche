package web_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

// The 11 Plovdiv and Varna districts of the committed files are in the sitemap in both languages
// with an hreflang pair, and none of the pages is noindexed, covered or not.
func TestPlovdivVarnaDistrictsAreInTheSitemap(t *testing.T) {
	areas := map[string]snapshot.AreaMeta{}
	add := func(m snapshot.AreaMeta) { areas[m.Slug] = m }
	for _, c := range []struct{ slug, bg, en string }{{"plovdiv", "Пловдив", "Plovdiv"}, {"varna", "Варна", "Varna"}} {
		add(snapshot.AreaMeta{Slug: c.slug + "-oblast", Kind: "oblast", NameBG: c.bg, NameEN: c.en, CentroidLon: 25, CentroidLat: 42,
			DefaultZoom: 9, Covered: true, SensorCount: 30, Values: map[string]float64{"P2": 10, "P1": 15}, Km2: 5000, Source: "sensor.community"})
		add(snapshot.AreaMeta{Slug: c.slug, Kind: "city", NameBG: c.bg, NameEN: c.en, ParentSlug: c.slug + "-oblast", CentroidLon: 25,
			CentroidLat: 42, DefaultZoom: 11, Covered: true, SensorCount: 30, Values: map[string]float64{"P2": 10, "P1": 15}, Km2: 100, Source: "sensor.community"})
	}
	var slugs []string
	for _, f := range []string{"plovdiv", "varna"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "data", "boundaries", f+"-districts.geojson"))
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
		for i, ft := range fc.Features {
			p := ft.Properties
			m := snapshot.AreaMeta{Slug: p.Slug, Kind: "neighbourhood", NameBG: p.NameBG, NameEN: p.NameEN, ParentSlug: f, CentroidLon: 25 + float64(i)*0.01,
				CentroidLat: 42, DefaultZoom: 13, Km2: 8, SensorCount: 1}
			if i%2 == 0 {
				m.Covered, m.SensorCount, m.Source = true, 4, "sensor.community"
				m.Values = map[string]float64{"P2": 10, "P1": 15}
			}
			add(m)
			slugs = append(slugs, p.Slug)
		}
	}
	if len(slugs) != 11 {
		t.Fatalf("read %d district slugs, want 11", len(slugs))
	}
	doc, get := guardSitemap(t, &snapshot.Snapshot{GeneratedAt: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC), KnownSlugs: areas})

	type entry struct {
		links map[string]string
	}
	byLoc := map[string]entry{}
	for _, u := range doc.URLs {
		e := entry{links: map[string]string{}}
		for _, l := range u.Links {
			e.links[l.Hreflang] = l.Href
		}
		byLoc[u.Loc] = e
	}
	found := 0
	for _, slug := range slugs {
		var bgLoc, enLoc string
		for loc := range byLoc {
			switch {
			case strings.HasSuffix(loc, "/en/area/"+slug):
				enLoc = loc
			case strings.HasSuffix(loc, "/area/"+slug):
				bgLoc = loc
			}
		}
		if bgLoc == "" || enLoc == "" {
			t.Errorf("%s: sitemap has bg %q en %q, want both", slug, bgLoc, enLoc)
			continue
		}
		found += 2
		for _, loc := range []string{bgLoc, enLoc} {
			if byLoc[loc].links["bg"] != bgLoc || byLoc[loc].links["en"] != enLoc {
				t.Errorf("%s: hreflang links %v, want bg %s and en %s", loc, byLoc[loc].links, bgLoc, enLoc)
			}
		}
		for _, path := range []string{"/area/" + slug, "/en/area/" + slug} {
			body, header, code := get(path)
			if code != 200 || noindexed(body, header) {
				t.Errorf("%s: status %d, noindex %v", path, code, noindexed(body, header))
			}
		}
	}
	if found != 22 {
		t.Errorf("found %d district URLs in the sitemap, want 22", found)
	}
}
