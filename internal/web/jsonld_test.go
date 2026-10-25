package web_test

import (
	"encoding/json"
	"regexp"
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
	Email               string   `json:"email"`
	Publisher           *struct {
		ID string `json:"@id"`
	} `json:"publisher"`
	Creator *struct {
		ID string `json:"@id"`
	} `json:"creator"`
	Logo *struct {
		Type   string `json:"@type"`
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"logo"`
	DateModified     string `json:"dateModified"`
	DatePublished    string `json:"datePublished"`
	Headline         string `json:"headline"`
	MainEntityOfPage string `json:"mainEntityOfPage"`
	Image            string `json:"image"`
	TemporalCoverage string `json:"temporalCoverage"`
	Distribution     []struct {
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
	ApplicationCategory string `json:"applicationCategory"`
	OperatingSystem     string `json:"operatingSystem"`
	IsPartOf            *struct {
		ID string `json:"@id"`
	} `json:"isPartOf"`
	Author *struct {
		ID string `json:"@id"`
	} `json:"author"`
	IsBasedOn *struct {
		ID string `json:"@id"`
	} `json:"isBasedOn"`
	ContainedInPlace *ldPlaceNode `json:"containedInPlace"`
	ItemListElement  []struct {
		Type     string `json:"@type"`
		Position int    `json:"position"`
		Name     string `json:"name"`
		Item     string `json:"item"`
	} `json:"itemListElement"`
}

// ldPlaceNode is a Place as nested under containedInPlace.
type ldPlaceNode struct {
	Type             string       `json:"@type"`
	Name             string       `json:"name"`
	URL              string       `json:"url"`
	ContainedInPlace *ldPlaceNode `json:"containedInPlace"`
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
	snap.KnownSlugs["sofia"] = snapshot.AreaMeta{Slug: "sofia", Kind: "city", NameBG: "София", NameEN: "Sofia",
		CentroidLon: 23.32, CentroidLat: 42.69, DefaultZoom: 11, Covered: true, SensorCount: 300}
	snap.KnownSlugs["plovdiv-oblast"] = snapshot.AreaMeta{Slug: "plovdiv-oblast", Kind: "oblast", NameBG: "Пловдив", NameEN: "Plovdiv",
		DefaultZoom: 9, Covered: true, SensorCount: 150}
	for slug, parent := range map[string]string{"plovdiv": "plovdiv-oblast", "mladost": "sofia", "sofia": "sofia-grad-oblast"} {
		m := snap.KnownSlugs[slug]
		m.ParentSlug = parent
		m.CentroidLon, m.CentroidLat = 24.75, 42.15
		snap.KnownSlugs[slug] = m
	}
	return snap
}

func TestJSONLDHomeHasOrganization(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	names := map[string][2]string{"/": {"Канарче", "Kanarche"}, "/en/": {"Kanarche", "Канарче"}}
	for path, want := range names {
		doc := jsonLD(t, path, fetch(t, rr, path).Body.String())
		org := nodeOfType(doc, "Organization")
		if org == nil {
			t.Fatalf("%s: no Organization node", path)
		}
		if org.ID != "https://airbg.org/#org" || org.Name != want[0] || org.AlternateName != want[1] || org.URL != "https://airbg.org/" {
			t.Errorf("%s: Organization = %+v", path, org)
		}
		if org.Logo == nil || org.Logo.Type != "ImageObject" || !strings.HasPrefix(org.Logo.URL, "https://airbg.org/") || !strings.Contains(org.Logo.URL, ".png") || org.Logo.Width < 112 || org.Logo.Height < 112 {
			t.Errorf("%s: logo = %+v", path, org.Logo)
		}
		if len(org.SameAs) != 1 || org.SameAs[0] != "https://github.com/iliyan-s-petkov/kanarche" {
			t.Errorf("%s: Organization.sameAs = %v", path, org.SameAs)
		}
		if org.Email != "" {
			t.Errorf("%s: Organization exposes email %q", path, org.Email)
		}
	}
}

func TestJSONLDHomeHasWebSiteAndPerson(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	for path, lang := range map[string]string{"/": "bg", "/en/": "en"} {
		doc := jsonLD(t, path, fetch(t, rr, path).Body.String())

		site := nodeOfType(doc, "WebSite")
		if site == nil || site.Name == "" || site.InLanguage != lang || !strings.HasPrefix(site.URL, "https://airbg.org/") {
			t.Errorf("%s: WebSite = %+v", path, site)
		}
		if len(site.SameAs) != 1 || site.SameAs[0] != "https://github.com/iliyan-s-petkov/kanarche" {
			t.Errorf("%s: WebSite.sameAs = %v", path, site.SameAs)
		}
		person := nodeOfType(doc, "Person")
		if person == nil || person.Name != "Iliyan Petkov" || person.Email != "privacy@kanarche.eu" {
			t.Fatalf("%s: Person = %+v", path, person)
		}
		if person.ID != "https://airbg.org/#controller" {
			t.Errorf("%s: Person @id = %q", path, person.ID)
		}
		if site.Publisher == nil || site.Publisher.ID != "https://airbg.org/#org" {
			t.Errorf("%s: WebSite.publisher = %+v, want #org", path, site.Publisher)
		}
	}
}

func TestJSONLDHomeHasWebApplication(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	for path, lang := range map[string]string{"/": "bg", "/en/": "en"} {
		doc := jsonLD(t, path, fetch(t, rr, path).Body.String())
		site, app := nodeOfType(doc, "WebSite"), nodeOfType(doc, "WebApplication")
		if app == nil {
			t.Fatalf("%s: no WebApplication node", path)
		}
		if app.ID != "https://airbg.org/#webapp" || app.Name == "" || len(app.Description) < 50 || app.InLanguage != lang {
			t.Errorf("%s: WebApplication = %+v", path, app)
		}
		if app.ApplicationCategory == "" || app.OperatingSystem == "" || !app.IsAccessibleForFree {
			t.Errorf("%s: WebApplication category/os/free = %q/%q/%v", path, app.ApplicationCategory, app.OperatingSystem, app.IsAccessibleForFree)
		}
		if app.IsPartOf == nil || app.IsPartOf.ID != site.ID {
			t.Errorf("%s: isPartOf = %+v, want %q", path, app.IsPartOf, site.ID)
		}
		if app.Publisher == nil || app.Publisher.ID != "https://airbg.org/#org" {
			t.Errorf("%s: publisher = %+v, want #org", path, app.Publisher)
		}
		if app.Creator == nil || app.Creator.ID != "https://airbg.org/#org" {
			t.Errorf("%s: creator = %+v, want #org", path, app.Creator)
		}
		if app.License != "https://opensource.org/license/mit" {
			t.Errorf("%s: license = %q", path, app.License)
		}
		if app.IsBasedOn == nil || app.IsBasedOn.ID != "https://airbg.org/about-the-data#dataset" {
			t.Errorf("%s: isBasedOn = %+v, want the about-the-data Dataset @id", path, app.IsBasedOn)
		}
	}
}

func TestJSONLDAreaContainedInPlace(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	cases := map[string][]string{
		"/area/plovdiv":    {"/area/plovdiv-oblast"},
		"/area/mladost":    {"/area/sofia", "/area/sofia-grad-oblast"},
		"/en/area/plovdiv": {"/en/area/plovdiv-oblast"},
	}
	for path, want := range cases {
		place := nodeOfType(jsonLD(t, path, fetch(t, rr, path).Body.String()), "Place")
		if place == nil {
			t.Fatalf("%s: no Place", path)
		}
		got := place.ContainedInPlace
		for i, u := range want {
			if got == nil || got.Type != "Place" || got.Name == "" || got.URL != "https://airbg.org"+u {
				t.Fatalf("%s: containedInPlace[%d] = %+v, want url %s", path, i, got, u)
			}
			got = got.ContainedInPlace
		}
		if got != nil {
			t.Errorf("%s: containedInPlace chain longer than %d", path, len(want))
		}
	}
	if place := nodeOfType(jsonLD(t, "/area/plovdiv-oblast", fetch(t, rr, "/area/plovdiv-oblast").Body.String()), "Place"); place != nil && place.ContainedInPlace != nil {
		t.Errorf("oblast has containedInPlace %+v", place.ContainedInPlace)
	}
}

func TestJSONLDAboutHasDataset(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	for _, path := range []string{"/about-the-data", "/en/about-the-data"} {
		ds := nodeOfType(jsonLD(t, path, fetch(t, rr, path).Body.String()), "Dataset")
		if ds == nil {
			t.Fatalf("%s: no Dataset node", path)
		}
		if ds.ID != "https://airbg.org/about-the-data#dataset" {
			t.Errorf("%s: Dataset @id = %q", path, ds.ID)
		}
		if ds.Name == "" || len(ds.Description) < 50 {
			t.Errorf("%s: Dataset name %q / description %q (Google needs both, description 50+ chars)", path, ds.Name, ds.Description)
		}
		if ds.License != "https://opendatacommons.org/licenses/odbl/1-0/" {
			t.Errorf("%s: license = %q, want the ODbL URL", path, ds.License)
		}
		if ds.Creator == nil || ds.Creator.ID != "https://airbg.org/#org" {
			t.Errorf("%s: creator = %+v, want #org", path, ds.Creator)
		}
		if ds.Publisher == nil || ds.Publisher.ID != "https://airbg.org/#org" {
			t.Errorf("%s: publisher = %+v, want #org", path, ds.Publisher)
		}
		if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}/\.\.$`).MatchString(ds.TemporalCoverage) {
			t.Errorf("%s: temporalCoverage = %q", path, ds.TemporalCoverage)
		}
		if _, err := time.Parse(time.RFC3339, ds.DateModified); err != nil {
			t.Errorf("%s: dateModified = %q: %v", path, ds.DateModified, err)
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
		"/area/mladost":           {"/", "/areas", "/area/sofia-grad-oblast", "/area/sofia", "/area/mladost"},
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

// collectRefs gathers every object that is only an {"@id": ...} reference.
func collectRefs(v any, refs *[]string) {
	switch x := v.(type) {
	case map[string]any:
		if id, ok := x["@id"].(string); ok && len(x) == 1 {
			*refs = append(*refs, id)
		}
		for _, c := range x {
			collectRefs(c, refs)
		}
	case []any:
		for _, c := range x {
			collectRefs(c, refs)
		}
	}
}

func TestJSONLDIDReferencesResolveInGraph(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	for _, path := range []string{"/", "/en/", "/about-the-data", "/en/about-the-data", "/area/plovdiv", "/pm25", "/pm10", "/en/pm25", "/en/pm10", "/air-pollution", "/en/air-pollution"} {
		body := fetch(t, rr, path).Body.String()
		start := strings.Index(body, ldOpen) + len(ldOpen)
		end := strings.Index(body[start:], "</script>")
		var doc struct {
			Graph []map[string]any `json:"@graph"`
		}
		if err := json.Unmarshal([]byte(body[start:start+end]), &doc); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		ids := map[string]bool{}
		var refs []string
		for _, n := range doc.Graph {
			if id, ok := n["@id"].(string); ok && len(n) > 1 {
				ids[id] = true
			}
			for k, c := range n {
				if k != "@id" {
					collectRefs(c, &refs)
				}
			}
		}
		if path == "/" && len(refs) == 0 {
			t.Errorf("%s: found no @id references, the check is inert", path)
		}
		// The home WebApplication points at the Dataset that lives on about-the-data.
		if path == "/" || path == "/en/" {
			ids["https://airbg.org/about-the-data#dataset"] = true
		}
		for _, r := range refs {
			if !ids[r] {
				t.Errorf("%s: reference %q has no node in the graph", path, r)
			}
		}
	}
}

func TestJSONLDDatasetTemporalCoverage(t *testing.T) {
	cases := map[time.Time]string{
		time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC): "2026-08-31/..",
		time.Date(2029, 3, 15, 9, 0, 0, 0, time.UTC):  "2027-03-15/..",
	}
	for at, want := range cases {
		snap := jsonLDFixture(t)
		snap.GeneratedAt = at
		for _, path := range []string{"/about-the-data", "/en/about-the-data"} {
			ds := nodeOfType(jsonLD(t, path, fetch(t, renderer(t, snap), path).Body.String()), "Dataset")
			if ds == nil || ds.TemporalCoverage != want {
				t.Errorf("%s at %s: temporalCoverage = %+v, want %q", path, at.Format("2006-01-02"), ds, want)
			}
		}
	}
}

func TestJSONLDOrganizationSameAsIncludesConfiguredSocial(t *testing.T) {
	fb, li := "https://www.facebook.com/airbg", "https://www.linkedin.com/company/airbg"
	for path, want := range map[string][]string{"/": {"https://github.com/iliyan-s-petkov/kanarche", fb, li}} {
		org := nodeOfType(jsonLD(t, path, fetch(t, rendererWithSocial(t, fb, li), path).Body.String()), "Organization")
		if org == nil || strings.Join(org.SameAs, " ") != strings.Join(want, " ") {
			t.Errorf("%s: Organization = %+v, want sameAs %v", path, org, want)
		}
	}
}

func TestJSONLDPMArticlePublishedByOrganization(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	for _, path := range []string{"/pm25", "/pm10", "/en/pm25", "/en/pm10"} {
		doc := jsonLD(t, path, fetch(t, rr, path).Body.String())
		art, org := nodeOfType(doc, "Article"), nodeOfType(doc, "Organization")
		if art == nil || org == nil || art.Publisher == nil || art.Publisher.ID != org.ID || org.ID != "https://airbg.org/#org" {
			t.Errorf("%s: Article = %+v, Organization = %+v", path, art, org)
		}
		if art != nil && (art.Author == nil || art.Author.ID != "https://airbg.org/#controller") {
			t.Errorf("%s: Article.author = %+v, want the controller", path, art.Author)
		}
	}
}
