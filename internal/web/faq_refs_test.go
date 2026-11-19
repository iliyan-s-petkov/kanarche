package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// officialHosts is the allowlist of primary official domains an FAQ source may link to.
var officialHosts = map[string]bool{
	"www.who.int":              true,
	"eur-lex.europa.eu":        true,
	"lex.bg":                   true,
	"airindex.eea.europa.eu":   true,
	"www.eea.europa.eu":        true,
	"eea.government.bg":        true,
	"atmosphere.copernicus.eu": true,
	"www.ecmwf.int":            true,
}

var (
	faqSourceRE = regexp.MustCompile(`(?s)<p class="faq__source">(.*?)</p>`)
	faqHrefRE   = regexp.MustCompile(`<a [^>]*href="([^"]+)"([^>]*)>`)
	// An About item is a heading, the answer paragraph and an optional source paragraph.
	aboutItemWithSourceRE = regexp.MustCompile(`(?s)<h3>.*?</h3>\s*<p>.*?</p>(?:\s*<p class="faq__source">(.*?)</p>)?`)
)

// sourceLinks returns the hrefs of one source paragraph and fails when a link lacks rel="noopener noreferrer".
func sourceLinks(t *testing.T, where, para string) []string {
	t.Helper()
	var out []string
	for _, m := range faqHrefRE.FindAllStringSubmatch(para, -1) {
		if !strings.Contains(m[0], `rel="noopener noreferrer"`) {
			t.Errorf("%s: source link %s has no rel=\"noopener noreferrer\"", where, m[1])
		}
		out = append(out, m[1])
	}
	return out
}

// aboutSourceLinks maps the 1-based About question number to its source hrefs.
func aboutSourceLinks(t *testing.T, path, body string) map[int][]string {
	t.Helper()
	sec := aboutFAQSectionRE.FindString(body)
	if sec == "" {
		t.Fatalf("%s: no About FAQ section", path)
	}
	out := map[int][]string{}
	for i, m := range aboutItemWithSourceRE.FindAllStringSubmatch(sec, -1) {
		if m[1] != "" {
			out[i+1] = sourceLinks(t, path, m[1])
		}
	}
	return out
}

// explainerSourceLinks maps the 1-based air pollution question number to its source hrefs.
func explainerSourceLinks(t *testing.T, path, body string) map[int][]string {
	t.Helper()
	out := map[int][]string{}
	parts := strings.Split(body, `<details class="faq__item">`)
	for i, part := range parts[1:] {
		if end := strings.Index(part, "</details>"); end >= 0 {
			part = part[:end]
		}
		if m := faqSourceRE.FindStringSubmatch(part); m != nil {
			out[i+1] = sourceLinks(t, path, m[1])
		}
	}
	return out
}

// The answers that rest on an official document each link to it, in both languages, and no other answer does.
func TestFAQAnswersRenderTheirOfficialRefs(t *testing.T) {
	rr := renderer(t, fixture(t))
	wantAbout := map[int]int{1: 1, 4: 1, 10: 2, 11: 2, 12: 1, 14: 1, 15: 1, 16: 1}
	wantExplainer := map[int]int{2: 1, 3: 4, 6: 1, 7: 1, 10: 1}
	for _, path := range []string{"/about-the-data", "/en/about-the-data"} {
		got := aboutSourceLinks(t, path, fetch(t, rr, path).Body.String())
		for n, want := range wantAbout {
			if len(got[n]) != want {
				t.Errorf("%s: answer %d has %d source links %v, want %d", path, n, len(got[n]), got[n], want)
			}
		}
		if len(got) != len(wantAbout) {
			t.Errorf("%s: %d answers carry sources, want %d", path, len(got), len(wantAbout))
		}
	}
	for _, c := range airPollutionPages {
		got := explainerSourceLinks(t, c.path, fetch(t, rr, c.path).Body.String())
		for n, want := range wantExplainer {
			if len(got[n]) != want {
				t.Errorf("%s: answer %d has %d source links %v, want %d", c.path, n, len(got[n]), got[n], want)
			}
		}
		if len(got) != len(wantExplainer) {
			t.Errorf("%s: %d answers carry sources, want %d", c.path, len(got), len(wantExplainer))
		}
	}
}

// The area FAQ links the EAQI under the daily answer, the Executive Environment Agency under the station answer and CAMS under pollen.
func TestAreaFAQRendersOfficialRefs(t *testing.T) {
	rr := renderer(t, eeaFixture(t))
	for _, path := range []string{"/area/plovdiv", "/en/area/plovdiv"} {
		body := fetch(t, rr, path).Body.String()
		var n int
		for _, m := range faqSourceRE.FindAllStringSubmatch(faqSection(t, body), -1) {
			n += len(sourceLinks(t, path, m[1]))
		}
		if n == 0 {
			t.Errorf("%s: no FAQ answer carries a source link", path)
		}
	}
}

// Every source link is https on an allowlisted official host, and the Bulgarian pages use the Bulgarian EUR-Lex text.
func TestFAQRefsAreHTTPSOnOfficialHosts(t *testing.T) {
	rr := renderer(t, eeaFixture(t))
	pages := []string{"/about-the-data", "/en/about-the-data", "/air-pollution", "/en/air-pollution", "/area/plovdiv", "/en/area/plovdiv"}
	for _, path := range pages {
		body := fetch(t, rr, path).Body.String()
		seen := 0
		for _, m := range faqSourceRE.FindAllStringSubmatch(body, -1) {
			for _, href := range sourceLinks(t, path, m[1]) {
				seen++
				u, err := url.Parse(href)
				if err != nil || u.Scheme != "https" || !officialHosts[u.Host] {
					t.Errorf("%s: source %q is not https on an official host", path, href)
					continue
				}
				if u.Host == "eur-lex.europa.eu" {
					want := "/legal-content/EN/"
					if !strings.HasPrefix(path, "/en/") {
						want = "/legal-content/BG/"
					}
					if !strings.HasPrefix(u.Path, want) {
						t.Errorf("%s: EUR-Lex link %q should start %s", path, href, want)
					}
				}
			}
		}
		if seen == 0 {
			t.Errorf("%s: no source links at all", path)
		}
	}
}

// The FAQPage JSON-LD keeps the plain answer, with no source line or link in it.
func TestFAQSourceLineStaysOutOfJSONLD(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range airPollutionPages {
		for _, n := range ldGraph(t, fetch(t, rr, c.path).Body.String()) {
			for _, q := range n.MainEntity {
				if strings.Contains(q.AcceptedAnswer.Text, "http") || strings.Contains(q.AcceptedAnswer.Text, "Source") || strings.Contains(q.AcceptedAnswer.Text, "Източник") {
					t.Errorf("%s: JSON-LD answer carries a source: %q", c.path, q.AcceptedAnswer.Text)
				}
			}
		}
	}
}
