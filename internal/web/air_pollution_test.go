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

type apLDNode struct {
	Type       string `json:"@type"`
	MainEntity []struct {
		Name           string `json:"name"`
		AcceptedAnswer struct {
			Text string `json:"text"`
		} `json:"acceptedAnswer"`
	} `json:"mainEntity"`
	ItemListElement []struct {
		Item string `json:"item"`
	} `json:"itemListElement"`
}

// ldGraph returns the @graph nodes of the page's JSON-LD.
func ldGraph(t *testing.T, body string) []apLDNode {
	t.Helper()
	start := strings.Index(body, ldOpen) + len(ldOpen)
	end := strings.Index(body[start:], "</script>")
	var doc struct {
		Graph []apLDNode `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(body[start:start+end]), &doc); err != nil {
		t.Fatalf("json-ld: %v", err)
	}
	return doc.Graph
}

var (
	summaryRe = regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)
	answerRe  = regexp.MustCompile(`(?s)<div class="faq__answer"><p>(.*?)</p></div>`)
)

func TestAirPollutionFAQPageMatchesVisibleDetails(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range airPollutionPages {
		body := fetch(t, rr, c.path).Body.String()
		var visibleQ, visibleA []string
		for _, m := range summaryRe.FindAllStringSubmatch(body, -1) {
			visibleQ = append(visibleQ, html.UnescapeString(m[1]))
		}
		for _, m := range answerRe.FindAllStringSubmatch(body, -1) {
			visibleA = append(visibleA, html.UnescapeString(m[1]))
		}
		var ldQ, ldA []string
		for _, n := range ldGraph(t, body) {
			if n.Type != "FAQPage" {
				continue
			}
			for _, q := range n.MainEntity {
				ldQ = append(ldQ, q.Name)
				ldA = append(ldA, q.AcceptedAnswer.Text)
			}
		}
		if len(ldQ) != 14 {
			t.Fatalf("%s: FAQPage has %d questions, want 14", c.path, len(ldQ))
		}
		if strings.Join(ldQ, "|") != strings.Join(visibleQ, "|") {
			t.Errorf("%s: FAQPage questions %q differ from visible details %q", c.path, ldQ, visibleQ)
		}
		if strings.Join(ldA, "|") != strings.Join(visibleA, "|") {
			t.Errorf("%s: FAQPage answers %q differ from visible answers %q", c.path, ldA, visibleA)
		}
	}
}

func TestAirPollutionBreadcrumb(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range airPollutionPages {
		var items []string
		for _, n := range ldGraph(t, fetch(t, rr, c.path).Body.String()) {
			if n.Type == "BreadcrumbList" {
				for _, it := range n.ItemListElement {
					items = append(items, it.Item)
				}
			}
		}
		if len(items) != 2 {
			t.Fatalf("%s: breadcrumb has %d items, want 2", c.path, len(items))
		}
		if items[1] != "https://airbg.org"+c.path {
			t.Errorf("%s: last crumb item = %v", c.path, items[1])
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
	headingRe := regexp.MustCompile(`(?s)<h[1-6][^>]*>\s*([^<]*?)\s*</h[1-6]>`)
	var headings []string
	for _, m := range headingRe.FindAllStringSubmatch(body, -1) {
		headings = append(headings, html.UnescapeString(m[1]))
	}
	joined := "\n" + strings.Join(headings, "\n") + "\n"
	for _, want := range []string{"Какво показва картата", "Какво означават цветовете", "Областите като таблица"} {
		if !strings.Contains(joined, "\n"+want+"\n") {
			t.Errorf("/about has no heading %q", want)
		}
	}
	for _, old := range []string{"Прочетете картата", "Прочетете цветовете", "Прочетете областите като таблица"} {
		if strings.Contains(body, old) {
			t.Errorf("/about still carries %q", old)
		}
	}
}
