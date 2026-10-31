package web_test

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"testing"

	"kanarche.eu/internal/snapshot"
)

// h1Text is one real area h1 as the page renders it.
type h1Text struct {
	Slug string `json:"slug"`
	Lang string `json:"lang"`
	H1   string `json:"h1"`
}

// h1TextsFile is read by web/e2e/area-h1-budget.spec.js.
const h1TextsFile = "../../web/e2e/area-h1-texts.json"

// boundaryKinds maps each browsable boundary file to its area kind, as import-areas takes it.
var boundaryKinds = []struct{ file, kind string }{
	{"oblasti", "oblast"},
	{"cities", "city"},
	{"sofia-districts", "neighbourhood"},
}

// realAreaSnapshot holds every real oblast, city and Sofia district from data/boundaries.
func realAreaSnapshot(t *testing.T) (*snapshot.Snapshot, []string) {
	t.Helper()
	snap := areaPageFixture(t)
	snap.KnownSlugs = map[string]snapshot.AreaMeta{}
	var slugs []string
	for _, b := range boundaryKinds {
		raw, err := os.ReadFile(filepath.Join("..", "..", "data", "boundaries", b.file+".geojson"))
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
		for _, f := range fc.Features {
			p := f.Properties
			meta := snapshot.AreaMeta{Slug: p.Slug, Kind: b.kind, NameBG: p.NameBG, NameEN: p.NameEN,
				CentroidLon: 25, CentroidLat: 42.7, DefaultZoom: 11, Covered: true, SensorCount: 5}
			if b.kind == "neighbourhood" {
				meta.ParentSlug = "sofia"
			}
			snap.KnownSlugs[p.Slug] = meta
			slugs = append(slugs, p.Slug)
		}
	}
	return snap, slugs
}

// TestRealAreaH1Texts renders every real area page in both languages and pins the h1 texts the
// phone budget spec measures. Run with UPDATE_H1_TEXTS=1 to rewrite the file after a name or copy change.
func TestRealAreaH1Texts(t *testing.T) {
	snap, slugs := realAreaSnapshot(t)
	rr := renderer(t, snap)
	var got []h1Text
	for _, lang := range []string{"bg", "en"} {
		prefix := ""
		if lang == "en" {
			prefix = "/en"
		}
		for _, slug := range slugs {
			rec := fetch(t, rr, prefix+"/area/"+slug)
			if rec.Code != 200 {
				t.Fatalf("%s/area/%s = %d", prefix, slug, rec.Code)
			}
			h := html.UnescapeString(h1Of(rec.Body.String()))
			if h == "" {
				t.Fatalf("%s/area/%s has no h1", prefix, slug)
			}
			got = append(got, h1Text{slug, lang, h})
		}
	}
	out, err := json.MarshalIndent(got, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, '\n')
	if os.Getenv("UPDATE_H1_TEXTS") != "" {
		if err := os.WriteFile(h1TextsFile, out, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(h1TextsFile)
	if err != nil || string(want) != string(out) {
		t.Fatalf("%s is stale: run UPDATE_H1_TEXTS=1 go test ./internal/web -run TestRealAreaH1Texts", h1TextsFile)
	}
}
