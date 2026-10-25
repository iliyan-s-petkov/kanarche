package web_test

import (
	"encoding/xml"
	"net/url"
	"strings"
	"testing"

	"kanarche.eu/internal/snapshot"
)

// noindexed reports whether a response asks not to be indexed, by meta robots or X-Robots-Tag.
func noindexed(body, header string) bool {
	return strings.Contains(body, `<meta name="robots" content="noindex"`) ||
		strings.Contains(strings.ToLower(header), "noindex")
}

// guardFixture adds an uncovered city and an uncovered district to the h1 fixture,
// so covered and uncovered areas of every kind are in the sitemap.
func guardFixture(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	snap := h1Fixture(t)
	snap.KnownSlugs["vidin-city"] = snapshot.AreaMeta{Slug: "vidin-city", Kind: "city", NameBG: "Видин", NameEN: "Vidin",
		ParentSlug: "vidin", CentroidLon: 22.87, CentroidLat: 43.99, DefaultZoom: 11}
	snap.KnownSlugs["mladost"] = snapshot.AreaMeta{Slug: "mladost", Kind: "neighbourhood", NameBG: "Младост", NameEN: "Mladost",
		ParentSlug: "sofia", CentroidLon: 23.37, CentroidLat: 42.65, DefaultZoom: 13}
	return snap
}

func guardSitemap(t *testing.T, snap *snapshot.Snapshot) (sitemapXML, func(path string) (string, string, int)) {
	t.Helper()
	rr := renderer(t, snap)
	var doc sitemapXML
	if err := xml.Unmarshal(fetch(t, rr, "/sitemap.xml").Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	get := func(path string) (string, string, int) {
		rec := fetch(t, rr, path)
		return rec.Body.String(), rec.Header().Get("X-Robots-Tag"), rec.Code
	}
	return doc, get
}

// Every sitemap URL renders indexable, in both languages, for areas of every kind.
func TestEverySitemapURLRendersWithoutNoindex(t *testing.T) {
	doc, get := guardSitemap(t, guardFixture(t))
	if len(doc.URLs) < 40 {
		t.Fatalf("sitemap has %d URLs, want static pages and areas in both languages", len(doc.URLs))
	}
	for _, u := range doc.URLs {
		parsed, err := url.Parse(u.Loc)
		if err != nil {
			t.Fatal(err)
		}
		body, header, code := get(parsed.RequestURI())
		if code != 200 {
			t.Errorf("%s = %d, want 200", u.Loc, code)
			continue
		}
		if noindexed(body, header) {
			t.Errorf("%s is in the sitemap but renders noindex", u.Loc)
		}
	}
}

// Pages that render noindex are not listed in the sitemap.
func TestNoindexPagesAreAbsentFromSitemap(t *testing.T) {
	doc, get := guardSitemap(t, guardFixture(t))
	listed := map[string]bool{}
	for _, u := range doc.URLs {
		listed[u.Loc] = true
	}
	for _, p := range []string{"/embed", "/en/embed", "/no-such-page", "/en/no-such-page", "/area/no-such-area"} {
		body, header, _ := get(p)
		if !noindexed(body, header) {
			t.Errorf("%s: want noindex", p)
		}
		if listed["https://airbg.org"+p] {
			t.Errorf("%s renders noindex but is in the sitemap", p)
		}
	}
}
