package web_test

import (
	"encoding/json"
	"html"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

var airPollutionPages = []struct{ prefix, path, h1 string }{
	{"", "/air-pollution", "Замърсяване на въздуха: какво дишаме и откъде идва"},
	{"/en", "/en/air-pollution", "Air pollution: what we breathe and where it comes from"},
}

func TestAirPollutionRendersInBothLanguages(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range airPollutionPages {
		rec := fetch(t, rr, c.path)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", c.path, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `<h1 class="t-title">`+c.h1+`</h1>`) {
			t.Errorf("%s: h1 %q missing", c.path, c.h1)
		}
		for _, bad := range []string{"{p2_", "{p1_", "{minutes}", "_link}", "{link}"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s: unfilled placeholder %s", c.path, bad)
			}
		}
		for _, want := range []string{
			`href="` + c.prefix + `/pm25"`, `href="` + c.prefix + `/pm10"`,
			`href="` + c.prefix + `/areas"`, `href="` + c.prefix + `/about-the-data"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: missing body link %s", c.path, want)
			}
		}
	}
}

func TestAirPollutionHeadTags(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range airPollutionPages {
		body := fetch(t, rr, c.path).Body.String()
		for _, want := range []string{
			`<link rel="canonical" href="https://airbg.org` + c.path + `"`,
			`hreflang="bg" href="https://airbg.org/air-pollution"`,
			`hreflang="en" href="https://airbg.org/en/air-pollution"`,
			`hreflang="x-default" href="https://airbg.org/air-pollution"`,
			`<meta property="og:title"`, `<meta property="og:description"`, `<meta name="twitter:`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: head lacks %q", c.path, want)
			}
		}
	}
}

// ldGraph returns the @graph nodes of the page's JSON-LD.
func ldGraph(t *testing.T, body string) []map[string]any {
	t.Helper()
	start := strings.Index(body, ldOpen) + len(ldOpen)
	end := strings.Index(body[start:], "</script>")
	var doc struct {
		Graph []map[string]any `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(body[start:start+end]), &doc); err != nil {
		t.Fatalf("json-ld: %v", err)
	}
	return doc.Graph
}

var summaryRe = regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)

func TestAirPollutionFAQPageMatchesVisibleDetails(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range airPollutionPages {
		body := fetch(t, rr, c.path).Body.String()
		var visible []string
		for _, m := range summaryRe.FindAllStringSubmatch(body, -1) {
			visible = append(visible, html.UnescapeString(m[1]))
		}
		var ld []string
		for _, n := range ldGraph(t, body) {
			if n["@type"] != "FAQPage" {
				continue
			}
			for _, q := range n["mainEntity"].([]any) {
				ld = append(ld, q.(map[string]any)["name"].(string))
			}
		}
		if len(ld) != 8 {
			t.Fatalf("%s: FAQPage has %d questions, want 8", c.path, len(ld))
		}
		if strings.Join(ld, "|") != strings.Join(visible, "|") {
			t.Errorf("%s: FAQPage questions %q differ from visible details %q", c.path, ld, visible)
		}
	}
}

func TestAirPollutionBreadcrumb(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range airPollutionPages {
		var crumbs []any
		for _, n := range ldGraph(t, fetch(t, rr, c.path).Body.String()) {
			if n["@type"] == "BreadcrumbList" {
				crumbs = n["itemListElement"].([]any)
			}
		}
		if len(crumbs) != 2 {
			t.Fatalf("%s: breadcrumb has %d items, want 2", c.path, len(crumbs))
		}
		last := crumbs[1].(map[string]any)
		if last["item"] != "https://airbg.org"+c.path {
			t.Errorf("%s: last crumb item = %v", c.path, last["item"])
		}
	}
}

func TestAirPollutionInSitemap(t *testing.T) {
	doc := fetchSitemap(t)
	for _, c := range airPollutionPages {
		found := false
		for _, u := range doc.URLs {
			found = found || u.Loc == "https://airbg.org"+c.path
		}
		if !found {
			t.Errorf("sitemap lacks %s", c.path)
		}
	}
}

func TestFooterLinksToFAQAndExplainer(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, prefix := range []string{"", "/en"} {
		for _, path := range []string{"/", "/area/sofia", "/about"} {
			target := prefix + path
			if path == "/" && prefix != "" {
				target = prefix + "/"
			}
			body := fetch(t, rr, target).Body.String()
			for _, want := range []string{
				`<a href="` + prefix + `/about-the-data#faq">`,
				`<a href="` + prefix + `/air-pollution">`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s: footer lacks %s", target, want)
				}
			}
		}
	}
}

func TestAboutDataFAQHasAnchorID(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, p := range []string{"/about-the-data", "/en/about-the-data"} {
		body := fetch(t, rr, p).Body.String()
		if !strings.Contains(body, `id="faq"`) || !strings.Contains(body, `id="about-faq-heading"`) {
			t.Errorf("%s: faq anchors missing", p)
		}
	}
}

func TestExplainerLinesOnHomeAndArea(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range []struct{ path, prefix string }{{"/", ""}, {"/en/", "/en"}} {
		body := fetch(t, rr, c.path).Body.String()
		for _, want := range []string{`href="` + c.prefix + `/air-pollution"`, `href="` + c.prefix + `/about-the-data#faq"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: home line lacks %s", c.path, want)
			}
		}
	}
	rr = renderer(t, eeaFixture(t))
	for _, c := range []struct{ path, prefix string }{{"/area/plovdiv", ""}, {"/en/area/plovdiv", "/en"}} {
		body := fetch(t, rr, c.path).Body.String()
		if i := strings.Index(body, `<p class="faq__more">`); i < 0 || !strings.Contains(body[i:], `<a class="link" href="`+c.prefix+`/air-pollution">`) {
			t.Errorf("%s: area FAQ lacks the explainer line", c.path)
		}
	}
}

func TestAboutBGHeadingsRenamed(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/about").Body.String()
	for _, want := range []string{"Какво показва картата", "Какво означават цветовете", "Областите като таблица"} {
		if !strings.Contains(body, want) {
			t.Errorf("/about lacks heading %q", want)
		}
	}
}
