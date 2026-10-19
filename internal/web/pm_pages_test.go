package web_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"kanarche.eu/internal/api"
)

var pmPages = []struct{ path, metric, other string }{
	{"/pm25", "P2", "/pm10"},
	{"/pm10", "P1", "/pm25"},
	{"/en/pm25", "P2", "/en/pm10"},
	{"/en/pm10", "P1", "/en/pm25"},
}

func TestPMPagesRenderInBothLanguages(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range pmPages {
		rec := fetch(t, rr, c.path)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", c.path, rec.Code)
		}
		body := rec.Body.String()
		prefix := strings.TrimSuffix(strings.TrimSuffix(c.path, "/pm25"), "/pm10")
		wants := []string{
			"<h1", "<title>", `<meta name="description"`,
			`<link rel="canonical" href="https://airbg.org` + c.path + `"`,
			`hreflang="en"`, `hreflang="bg"`,
			`href="` + c.other + `"`,
			`href="` + prefix + `/areas"`,
			`href="` + prefix + `/about-the-data"`,
			`href="https://www.eea.europa.eu/en/topics/in-depth/air-pollution" rel="noopener`,
		}
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Errorf("%s: missing %q", c.path, want)
			}
		}
	}
}

func TestPMPagesTitlesDiffer(t *testing.T) {
	rr := renderer(t, fixture(t))
	seen := map[string]string{}
	for _, c := range pmPages {
		body := fetch(t, rr, c.path).Body.String()
		i, j := strings.Index(body, "<title>"), strings.Index(body, "</title>")
		if i < 0 || j < i {
			t.Fatalf("%s: no title", c.path)
		}
		title := body[i:j]
		if prev, dup := seen[title]; dup {
			t.Errorf("%s and %s share title %q", c.path, prev, title)
		}
		seen[title] = c.path
	}
}

func TestPMPagesBandsAreFilled(t *testing.T) {
	rr := renderer(t, fixture(t))
	edges := map[string][]string{}
	for _, s := range api.Scales() {
		if s.Name != "eaqi" {
			continue
		}
		for _, b := range s.Bands {
			if b.Upper != nil {
				edges[s.Metric] = append(edges[s.Metric], strconv.FormatFloat(*b.Upper, 'f', -1, 64))
			}
		}
	}
	for _, c := range pmPages {
		body := fetch(t, rr, c.path).Body.String()
		start := strings.Index(body, "<main")
		if start < 0 {
			t.Fatalf("%s: no <main>", c.path)
		}
		if strings.Contains(body[start:], "{p") {
			t.Errorf("%s: raw placeholder left", c.path)
		}
		if len(edges[c.metric]) != 5 {
			t.Fatalf("%s: %d edges for %s", c.path, len(edges[c.metric]), c.metric)
		}
		for _, e := range edges[c.metric] {
			if !strings.Contains(body[start:], e+" µg/m³") && !strings.Contains(body[start:], e+",") && !strings.Contains(body[start:], e+" и") {
				t.Errorf("%s: band edge %s not rendered", c.path, e)
			}
		}
	}
}

func TestPMPagesCarryJSONLDAndSitemap(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range pmPages {
		body := fetch(t, rr, c.path).Body.String()
		for _, want := range []string{`"@type":"Article"`, `"@type":"BreadcrumbList"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: JSON-LD lacks %s", c.path, want)
			}
		}
	}
	doc := fetchSitemap(t)
	for _, c := range pmPages {
		found := false
		for _, u := range doc.URLs {
			if u.Loc == "https://airbg.org"+c.path {
				found = true
			}
		}
		if !found {
			t.Errorf("sitemap lacks %s", c.path)
		}
	}
}

func TestFooterLinksToPMPages(t *testing.T) {
	rr := renderer(t, fixture(t))
	for prefix, wants := range map[string][]string{"": {`href="/pm25"`, `href="/pm10"`}, "/en": {`href="/en/pm25"`, `href="/en/pm10"`}} {
		body := fetch(t, rr, prefix+"/about-the-data").Body.String()
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Errorf("%s/about-the-data lacks link %s", prefix, want)
			}
		}
	}
}
