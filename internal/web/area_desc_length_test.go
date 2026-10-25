package web

import (
	"encoding/json"
	"os"
	"testing"
	"unicode/utf8"

	"kanarche.eu/internal/i18n"
)

// maxDescRunes is the length past which search results truncate a meta description.
const maxDescRunes = 155

// Every description variant stays within maxDescRunes for every real area name.
// Names come from the committed boundary files, the same data the snapshot's
// AreaMeta is imported from, so the longest real name is always covered.
func TestAreaDescriptionsFitForEveryRealName(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := &Renderer{cat: cat}

	files := []struct{ file, kind string }{
		{"oblasti.geojson", "oblast"},
		{"cities.geojson", "city"},
		{"sofia-districts.geojson", "neighbourhood"},
	}
	total := 0
	for _, f := range files {
		raw, err := os.ReadFile("../../data/boundaries/" + f.file)
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
			t.Fatalf("%s: %v", f.file, err)
		}
		if len(fc.Features) == 0 {
			t.Fatalf("%s: no features", f.file)
		}
		for _, ft := range fc.Features {
			p := ft.Properties
			for _, lang := range cat.Languages() {
				// Same name choice as rowFrom: catalogue key, then name_en, then name_bg.
				name := p.NameBG
				if lang != i18n.DefaultLang && p.NameEN != "" {
					name = p.NameEN
				}
				if key := "area.name." + p.Slug; cat.Has(lang, key) {
					name = cat.T(lang, key)
				}
				// Both the citizen-only and the official-station (_eea) variants.
				for _, source := range []string{"", "eea"} {
					row := AreaRow{Slug: p.Slug, Name: name, Kind: f.kind, Covered: true, Source: source}
					_, desc := rr.areaSEO(row, lang)
					total++
					if n := utf8.RuneCountInString(desc); n > maxDescRunes {
						t.Errorf("%s %s %s source=%q: description is %d runes, max %d: %q", lang, f.kind, p.Slug, source, n, maxDescRunes, desc)
					}
				}
			}
		}
	}
	if total < 100 {
		t.Fatalf("only %d descriptions checked, the boundary files did not load", total)
	}
}
