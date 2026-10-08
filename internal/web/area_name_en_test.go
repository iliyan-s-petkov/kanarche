package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"kanarche.eu/internal/snapshot"
)

// Some boundary name_en values are Cyrillic or nonstandard; en.json overrides them per slug.

// boundaryFeature is the subset of data/boundaries/*.geojson this test reads:
// just the properties every feature carries, ignoring geometry entirely so a
// large polygon never has to be parsed.
type boundaryFeature struct {
	Properties struct {
		Slug   string `json:"slug"`
		NameBG string `json:"name_bg"`
		NameEN string `json:"name_en"`
	} `json:"properties"`
}

type boundaryCollection struct {
	Features []boundaryFeature `json:"features"`
}

// loadBoundaryAreas reads every feature out of the given data/boundaries
// files, the same files internal/area.Import reads at import time, so this
// test walks the actual source of name_en rather than a hand-copied list that
// could drift from it.
func loadBoundaryAreas(t *testing.T, files ...string) []snapshot.AreaMeta {
	t.Helper()
	var out []snapshot.AreaMeta
	for _, f := range files {
		path := filepath.Join("..", "..", "data", "boundaries", f)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		var coll boundaryCollection
		if err := json.Unmarshal(raw, &coll); err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		for _, feat := range coll.Features {
			out = append(out, snapshot.AreaMeta{
				Slug:   feat.Properties.Slug,
				NameBG: feat.Properties.NameBG,
				NameEN: feat.Properties.NameEN,
			})
		}
	}
	return out
}

func containsCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}

// TestNoCyrillicInEnglishAreaNames is the general invariant: whatever
// name_en data/boundaries ships, and whatever area.name.<slug> overrides
// en.json carries, the name rowFrom picks for an English reader must be
// written in Latin script. It covers every Bulgarian oblast, city and Sofia
// district geojson feature, not just the ones already known to be broken, so
// a newly imported area with a Cyrillic or missing name_en fails this test
// before it ever reaches prod.
func TestNoCyrillicInEnglishAreaNames(t *testing.T) {
	rr := acRenderer(t)
	areas := loadBoundaryAreas(t, "bulgaria.geojson", "oblasti.geojson", "cities.geojson", "sofia-districts.geojson")
	if len(areas) == 0 {
		t.Fatal("loaded zero areas from data/boundaries — the fixture paths are wrong")
	}

	for _, meta := range areas {
		row := rr.rowFrom(meta, "en")
		if containsCyrillic(row.Name) {
			t.Errorf("slug %q: English name %q contains Cyrillic (name_en=%q); add an area.name.%s override to en.json",
				meta.Slug, row.Name, meta.NameEN, meta.Slug)
		}
	}
}

// TestAreaNameOverridesFixKnownBadTransliterations pins the exact overrides
// this fix adds: the raw name_en value is what data/boundaries and prod's
// /api/v1/areas serve today (verified live 2026-09-28), want is the
// Streamlined System transliteration from seo-copy.md. Unlike the invariant
// test above, this one also fails if an override regresses to some OTHER
// Latin spelling, not just back to Cyrillic.
func TestAreaNameOverridesFixKnownBadTransliterations(t *testing.T) {
	cases := []struct {
		slug, rawNameEN, want string
	}{
		{"gabrovo", "Габрово", "Gabrovo"},
		{"veliko-tarnovo", "Велико Търново", "Veliko Tarnovo"},
		{"pazardzhik", "Pazardzik", "Pazardzhik"},
		{"smolyan", "Smolian", "Smolyan"},
		{"krasna-polyana", "Krasna poliana", "Krasna Polyana"},
		{"krasno-selo", "Krasno selo", "Krasno Selo"},
		{"kremikovtsi", "Kremikovci", "Kremikovtsi"},
		{"nadezhda", "Nadejda", "Nadezhda"},
		{"ovcha-kupel", "Ovcha kupel", "Ovcha Kupel"},
		{"sredets", "Sredec", "Sredets"},
		{"triaditsa", "Triadica", "Triaditsa"},
		{"vazrazhdane", "Vazrajdane", "Vazrazhdane"},
	}

	rr := acRenderer(t)
	for _, c := range cases {
		meta := snapshot.AreaMeta{Slug: c.slug, NameBG: "irrelevant", NameEN: c.rawNameEN}
		got := rr.rowFrom(meta, "en").Name
		if got != c.want {
			t.Errorf("slug %q: rowFrom(en).Name = %q, want %q (raw name_en was %q)",
				c.slug, got, c.want, c.rawNameEN)
		}
		if strings.Contains(got, c.rawNameEN) {
			t.Errorf("slug %q: rowFrom(en).Name still contains the broken raw value %q", c.slug, c.rawNameEN)
		}
	}
}
