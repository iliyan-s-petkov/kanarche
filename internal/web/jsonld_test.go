package web_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

const ldOpen = `<script type="application/ld+json">`

// ldNode is the union of the fields the tests read from any @graph node.
type ldNode struct {
	Type                string   `json:"@type"`
	ID                  string   `json:"@id"`
	Name                string   `json:"name"`
	AlternateName       string   `json:"alternateName"`
	Description         string   `json:"description"`
	URL                 string   `json:"url"`
	InLanguage          string   `json:"inLanguage"`
	License             string   `json:"license"`
	IsAccessibleForFree bool     `json:"isAccessibleForFree"`
	SameAs              []string `json:"sameAs"`
	Creator             *struct {
		Type string `json:"@type"`
		Name string `json:"name"`
	} `json:"creator"`
	Distribution []struct {
		Type           string `json:"@type"`
		EncodingFormat string `json:"encodingFormat"`
		ContentURL     string `json:"contentUrl"`
	} `json:"distribution"`
	VariableMeasured []struct {
		Name string `json:"name"`
	} `json:"variableMeasured"`
	SpatialCoverage *struct {
		Name string `json:"name"`
	} `json:"spatialCoverage"`
	ItemListElement []struct {
		Type     string `json:"@type"`
		Position int    `json:"position"`
		Name     string `json:"name"`
		Item     string `json:"item"`
	} `json:"itemListElement"`
}

type ldDoc struct {
	Context string   `json:"@context"`
	Graph   []ldNode `json:"@graph"`
}

// jsonLD returns the one JSON-LD block on the page, parsed.
func jsonLD(t *testing.T, path, body string) ldDoc {
	t.Helper()
	if n := strings.Count(body, ldOpen); n != 1 {
		t.Fatalf("%s: %d JSON-LD blocks, want 1", path, n)
	}
	start := strings.Index(body, ldOpen) + len(ldOpen)
	end := strings.Index(body[start:], "</script>")
	var doc ldDoc
	if err := json.Unmarshal([]byte(body[start:start+end]), &doc); err != nil {
		t.Fatalf("%s: JSON-LD does not parse: %v\n%s", path, err, body[start:start+end])
	}
	if doc.Context != "https://schema.org" {
		t.Errorf("%s: @context = %q", path, doc.Context)
	}
	return doc
}

func nodeOfType(doc ldDoc, typ string) *ldNode {
	for i := range doc.Graph {
		if doc.Graph[i].Type == typ {
			return &doc.Graph[i]
		}
	}
	return nil
}

func jsonLDFixture(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	snap := seoFixture(t)
	snap.KnownSlugs["sofiya"] = snapshot.AreaMeta{Slug: "sofiya", Kind: "city", NameBG: "София", NameEN: "Sofia",
		CentroidLon: 23.32, CentroidLat: 42.69, DefaultZoom: 11, Covered: true, SensorCount: 300}
	snap.KnownSlugs["plovdiv-oblast"] = snapshot.AreaMeta{Slug: "plovdiv-oblast", Kind: "oblast", NameBG: "Пловдив", NameEN: "Plovdiv",
		DefaultZoom: 9, Covered: true, SensorCount: 150}
	for slug, parent := range map[string]string{"plovdiv": "plovdiv-oblast", "mladost": "sofiya"} {
		m := snap.KnownSlugs[slug]
		m.ParentSlug = parent
		snap.KnownSlugs[slug] = m
	}
	return snap
}

func TestJSONLDHomeHasWebSiteAndOrganization(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	for path, lang := range map[string]string{"/": "bg", "/en/": "en"} {
		brand := testBrand(map[string]string{"bg": "", "en": "/en"}[lang])
		doc := jsonLD(t, path, fetch(t, rr, path).Body.String())

		site := nodeOfType(doc, "WebSite")
		if site == nil || site.Name == "" || site.InLanguage != lang || !strings.HasPrefix(site.URL, "https://airbg.org/") {
			t.Errorf("%s: WebSite = %+v", path, site)
		}
		org := nodeOfType(doc, "Organization")
		if org == nil || org.Name != brand || org.URL != "https://airbg.org/" {
			t.Fatalf("%s: Organization = %+v", path, org)
		}
		if len(org.SameAs) != 1 || org.SameAs[0] != "https://github.com/iliyan-s-petkov/kanarche" {
			t.Errorf("%s: Organization.sameAs = %v", path, org.SameAs)
		}
	}
}

func TestJSONLDAboutHasDataset(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	for _, path := range []string{"/about-the-data", "/en/about-the-data"} {
		ds := nodeOfType(jsonLD(t, path, fetch(t, rr, path).Body.String()), "Dataset")
		if ds == nil {
			t.Fatalf("%s: no Dataset node", path)
		}
		if ds.Name == "" || len(ds.Description) < 50 {
			t.Errorf("%s: Dataset name %q / description %q (Google needs both, description 50+ chars)", path, ds.Name, ds.Description)
		}
		if !strings.Contains(ds.License, "opendatacommons.org/licenses/odbl") {
			t.Errorf("%s: license = %q, want ODbL", path, ds.License)
		}
		if ds.Creator == nil || ds.Creator.Type != "Organization" || ds.Creator.Name == "" {
			t.Errorf("%s: creator = %+v", path, ds.Creator)
		}
		if !ds.IsAccessibleForFree {
			t.Errorf("%s: isAccessibleForFree is false", path)
		}
		if ds.SpatialCoverage == nil || ds.SpatialCoverage.Name == "" {
			t.Errorf("%s: spatialCoverage = %+v", path, ds.SpatialCoverage)
		}
		if len(ds.Distribution) == 0 {
			t.Errorf("%s: no distribution", path)
		}
		for _, d := range ds.Distribution {
			if d.Type != "DataDownload" || d.EncodingFormat != "application/json" || !strings.HasPrefix(d.ContentURL, "https://airbg.org/api/v1/") {
				t.Errorf("%s: distribution = %+v", path, d)
			}
		}
		names := map[string]bool{}
		for _, v := range ds.VariableMeasured {
			names[v.Name] = true
		}
		pm25, pm10 := "PM2.5", "PM10"
		if !strings.HasPrefix(path, "/en/") {
			pm25, pm10 = "ФПЧ2.5", "ФПЧ10"
		}
		if !names[pm25] || !names[pm10] {
			t.Errorf("%s: variableMeasured %v lacks %s or %s", path, names, pm25, pm10)
		}
	}
}

func TestJSONLDAreaBreadcrumbs(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	cases := map[string][]string{
		"/area/plovdiv-oblast":    {"/", "/areas", "/area/plovdiv-oblast"},
		"/area/plovdiv":           {"/", "/areas", "/area/plovdiv-oblast", "/area/plovdiv"},
		"/area/mladost":           {"/", "/areas", "/area/sofiya", "/area/mladost"},
		"/en/area/veliko-tarnovo": {"/en/", "/en/areas", "/en/area/veliko-tarnovo"},
	}
	for path, want := range cases {
		list := nodeOfType(jsonLD(t, path, fetch(t, rr, path).Body.String()), "BreadcrumbList")
		if list == nil {
			t.Fatalf("%s: no BreadcrumbList", path)
		}
		if len(list.ItemListElement) != len(want) {
			t.Fatalf("%s: %d crumbs, want %d: %+v", path, len(list.ItemListElement), len(want), list.ItemListElement)
		}
		for i, item := range list.ItemListElement {
			if item.Type != "ListItem" || item.Position != i+1 || item.Name == "" {
				t.Errorf("%s: crumb %d = %+v, want ListItem at position %d", path, i, item, i+1)
			}
			if item.Item != "https://airbg.org"+want[i] {
				t.Errorf("%s: crumb %d item = %q, want %q", path, i, item.Item, "https://airbg.org"+want[i])
			}
		}
	}
}

func TestJSONLDAbsentOnErrorAndEmbed(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	if body := fetch(t, rr, "/area/does-not-exist").Body.String(); strings.Contains(body, ldOpen) {
		t.Error("404 page carries JSON-LD")
	}
	if body := framed(t, rr, "/embed").Body.String(); strings.Contains(body, ldOpen) {
		t.Error("/embed carries JSON-LD")
	}
}

func TestJSONLDHostileAreaNameStaysInsideScript(t *testing.T) {
	hostile := `Evil</script><script>alert(1)</script>`
	snap := &snapshot.Snapshot{
		GeneratedAt: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
		KnownSlugs: map[string]snapshot.AreaMeta{
			"evil": {Slug: "evil", Kind: "city", NameBG: hostile, NameEN: hostile,
				CentroidLon: 23, CentroidLat: 42, DefaultZoom: 11, Covered: true, SensorCount: 1},
		},
	}
	body := fetch(t, renderer(t, snap), "/area/evil").Body.String()
	if strings.Contains(body, "<script>alert(1)") {
		t.Fatal("hostile area name broke out of the JSON-LD script")
	}
	list := nodeOfType(jsonLD(t, "/area/evil", body), "BreadcrumbList")
	if list == nil || list.ItemListElement[len(list.ItemListElement)-1].Name != hostile {
		t.Errorf("hostile name did not round-trip inside the JSON: %+v", list)
	}
}
