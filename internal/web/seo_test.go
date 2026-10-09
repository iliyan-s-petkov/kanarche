package web_test

import (
	"encoding/xml"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestRobotsTxt pins the body byte for byte, including the Sitemap line built
// from testConfig's BaseURL (https://airbg.org) — not a hardcoded host.
func TestRobotsTxt(t *testing.T) {
	rec := fetch(t, renderer(t, fixture(t)), "/robots.txt")

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /robots.txt = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}

	want := "User-agent: *\n" +
		"Allow: /\n" +
		"Disallow: /api/\n" +
		"Sitemap: https://airbg.org/sitemap.xml\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("robots.txt = %q, want %q", got, want)
	}
}

// sitemapXML is the shape this package's tests decode /sitemap.xml into.
type sitemapXML struct {
	XMLName xml.Name `xml:"urlset"`
	Xmlns   string   `xml:"xmlns,attr"`
	URLs    []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
		Links   []struct {
			Rel      string `xml:"rel,attr"`
			Hreflang string `xml:"hreflang,attr"`
			Href     string `xml:"href,attr"`
		} `xml:"link"`
	} `xml:"url"`
}

func fetchSitemap(t *testing.T) sitemapXML {
	t.Helper()
	rec := fetch(t, renderer(t, fixture(t)), "/sitemap.xml")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /sitemap.xml = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/xml") {
		t.Errorf("Content-Type = %q, want application/xml", ct)
	}

	var doc sitemapXML
	if err := xml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("sitemap.xml did not parse as XML: %v\n%s", err, rec.Body.String())
	}
	return doc
}

// TestSitemapIsWellFormedXML: encoding/xml.Unmarshal above already proves this
// for every other test in this file; this one also pins the namespace, which a
// well-formedness check alone would not catch.
func TestSitemapIsWellFormedXML(t *testing.T) {
	doc := fetchSitemap(t)
	if doc.Xmlns != "http://www.sitemaps.org/schemas/sitemap/0.9" {
		t.Errorf("urlset xmlns = %q, want the sitemap protocol namespace", doc.Xmlns)
	}
}

// TestSitemapContainsEveryArea: covered fixture areas (sofia) must appear,
// plus the six static pages. A dropped area is invisible in the rendered
// site and only shows up here.
func TestSitemapContainsEveryArea(t *testing.T) {
	doc := fetchSitemap(t)

	locs := make(map[string]bool, len(doc.URLs))
	for _, u := range doc.URLs {
		locs[u.Loc] = true
	}

	for _, want := range []string{
		"https://airbg.org/",
		"https://airbg.org/areas",
		"https://airbg.org/about",
		"https://airbg.org/about-the-data",
		"https://airbg.org/area/sofia",
	} {
		if !locs[want] {
			t.Errorf("sitemap is missing %q", want)
		}
	}
	for _, want := range []string{
		"https://airbg.org/en/",
		"https://airbg.org/en/areas",
		"https://airbg.org/en/about",
		"https://airbg.org/en/about-the-data",
		"https://airbg.org/en/area/sofia",
	} {
		if !locs[want] {
			t.Errorf("sitemap is missing the English variant %q", want)
		}
	}
	// Eight pages in each of the two fixture languages; uncovered vidin is left out.
	if len(doc.URLs) != 16 {
		t.Errorf("sitemap has %d <url> entries, want 16: %+v", len(doc.URLs), doc.URLs)
	}
}

// TestSitemapAlternatesMatchPageHead: the hreflang set on a sitemap entry is
// the set the page itself emits in <head>, for the bg and the en variant.
func TestSitemapAlternatesMatchPageHead(t *testing.T) {
	doc := fetchSitemap(t)
	rr := renderer(t, fixture(t))
	linkRe := regexp.MustCompile(`<link rel="alternate" hreflang="([^"]+)" href="([^"]+)">`)

	for _, path := range []string{"/about", "/en/about", "/area/sofia", "/en/area/sofia"} {
		head := map[string]string{}
		for _, m := range linkRe.FindAllStringSubmatch(fetch(t, rr, path).Body.String(), -1) {
			head[m[1]] = m[2]
		}
		if len(head) == 0 {
			t.Fatalf("%s: no hreflang alternates in <head>", path)
		}

		loc := "https://airbg.org" + path
		found := false
		for _, u := range doc.URLs {
			if u.Loc != loc {
				continue
			}
			found = true
			got := map[string]string{}
			for _, l := range u.Links {
				got[l.Hreflang] = l.Href
			}
			if got["x-default"] != head["bg"] {
				t.Errorf("%s: x-default = %q, want the bg alternate %q", path, got["x-default"], head["bg"])
			}
			// The head names x-default too, at the same URL.
			if head["x-default"] != got["x-default"] {
				t.Errorf("%s: head x-default = %q, sitemap %q", path, head["x-default"], got["x-default"])
			}
			delete(got, "x-default")
			delete(head, "x-default")
			if !reflect.DeepEqual(got, head) {
				t.Errorf("%s: sitemap alternates %v != head alternates %v", path, got, head)
			}
		}
		if !found {
			t.Errorf("sitemap has no entry for %s", loc)
		}
	}
}

// TestSitemapHasReciprocalAlternatesAndXDefault: every URL must carry an
// hreflang link for every served language, plus x-default, and the bg/en
// hrefs must be the SAME page in each language (reciprocal) — not just present
// somewhere on the page.
func TestSitemapHasReciprocalAlternatesAndXDefault(t *testing.T) {
	doc := fetchSitemap(t)

	for _, u := range doc.URLs {
		byLang := map[string]string{}
		for _, l := range u.Links {
			if l.Rel != "alternate" {
				t.Errorf("%s: link rel = %q, want alternate", u.Loc, l.Rel)
			}
			byLang[l.Hreflang] = l.Href
		}

		bg, ok := byLang["bg"]
		if !ok {
			t.Errorf("%s: no bg alternate", u.Loc)
		}
		en, ok := byLang["en"]
		if !ok {
			t.Errorf("%s: no en alternate", u.Loc)
		}
		xdefault, ok := byLang["x-default"]
		if !ok {
			t.Errorf("%s: no x-default alternate", u.Loc)
		}
		if xdefault != bg {
			t.Errorf("%s: x-default = %q, want it to match the bg alternate %q", u.Loc, xdefault, bg)
		}
		if u.Loc != bg && u.Loc != en {
			t.Errorf("%s: loc is neither the bg alternate %q nor the en alternate %q", u.Loc, bg, en)
		}
		if !strings.HasPrefix(en, "https://airbg.org/en/") && en != "https://airbg.org/en/" {
			t.Errorf("%s: en alternate = %q, want it under /en/", u.Loc, en)
		}
	}
}

// TestSitemapExcludesEmbedAndAPI: neither surface is meant for search results.
func TestSitemapExcludesEmbedAndAPI(t *testing.T) {
	rec := fetch(t, renderer(t, fixture(t)), "/sitemap.xml")
	body := rec.Body.String()
	if strings.Contains(body, "/embed") {
		t.Error("sitemap.xml references /embed, which is noindex")
	}
	if strings.Contains(body, "/api/") {
		t.Error("sitemap.xml references /api/, which robots.txt disallows")
	}
}
