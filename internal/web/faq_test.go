package web_test

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/api"
)

var (
	faqSectionRE = regexp.MustCompile(`(?s)<section class="faq"[^>]*>.*?</section>`)
	faqItemRE    = regexp.MustCompile(`(?s)<details class="faq__item">\s*<summary[^>]*>(.*?)</summary>\s*<div class="faq__answer">\s*<p>(.*?)</p>\s*</div>\s*</details>`)
	tagRE        = regexp.MustCompile(`<[^>]*>`)
)

type faqEntry struct{ Q, A string }

// faqSection returns the FAQ markup of one rendered area page.
func faqSection(t *testing.T, body string) string {
	t.Helper()
	sec := faqSectionRE.FindString(body)
	if sec == "" {
		t.Fatalf("no FAQ section in page")
	}
	return sec
}

// faqVisible returns the visible question and answer text, tags stripped and entities decoded.
func faqVisible(t *testing.T, body string) []faqEntry {
	t.Helper()
	var out []faqEntry
	for _, m := range faqItemRE.FindAllStringSubmatch(faqSection(t, body), -1) {
		out = append(out, faqEntry{
			Q: html.UnescapeString(tagRE.ReplaceAllString(m[1], "")),
			A: html.UnescapeString(tagRE.ReplaceAllString(m[2], "")),
		})
	}
	return out
}

func TestFAQRendersSevenQuestionsInBothLanguages(t *testing.T) {
	rr := renderer(t, eeaFixture(t))
	for _, path := range []string{"/area/plovdiv", "/en/area/plovdiv", "/area/smolyan", "/en/area/smolyan-oblast", "/area/mladost"} {
		body := fetch(t, rr, path).Body.String()
		items := faqVisible(t, body)
		if len(items) != 7 {
			t.Fatalf("%s: %d FAQ entries, want 7", path, len(items))
		}
		for i, it := range items {
			if strings.TrimSpace(it.Q) == "" || strings.TrimSpace(it.A) == "" {
				t.Errorf("%s: entry %d is empty: %+v", path, i+1, it)
			}
		}
		if n := strings.Count(faqSection(t, body), "<h2"); n != 1 {
			t.Errorf("%s: %d h2 in the FAQ section, want 1", path, n)
		}
	}
}

func TestFAQLeavesNoRawPlaceholder(t *testing.T) {
	snap := eeaFixture(t)
	rr := renderer(t, snap)
	for slug := range snap.KnownSlugs {
		for _, prefix := range []string{"", "/en"} {
			path := prefix + "/area/" + slug
			sec := faqSection(t, fetch(t, rr, path).Body.String())
			if strings.ContainsAny(sec, "{}") {
				t.Errorf("%s: FAQ section still holds a raw placeholder: %q", path, regexp.MustCompile(`\{[^}]*\}?`).FindString(sec))
			}
		}
	}
}

func TestFAQHeadingUsesBulgarianPreposition(t *testing.T) {
	rr := renderer(t, eeaFixture(t))
	cases := map[string]string{
		"/area/smolyan":         "Въпроси за въздуха в Смолян",
		"/area/veliko-tarnovo":  "Въпроси за въздуха във Велико Търново",
		"/area/mladost":         "Въпроси за въздуха в район Младост",
		"/area/smolyan-oblast":  "Въпроси за въздуха в област Смолян",
		"/en/area/smolyan":      "Questions about the air in Smolyan",
		"/en/area/mladost":      "Questions about the air in Mladost",
		"/en/area/plovdiv":      "Questions about the air in Plovdiv",
		"/area/sofiyska-oblast": "Въпроси за въздуха в Софийска област",
	}
	for path, want := range cases {
		sec := faqSection(t, fetch(t, rr, path).Body.String())
		if got := html.UnescapeString(tagRE.ReplaceAllString(regexp.MustCompile(`(?s)<h2[^>]*>.*?</h2>`).FindString(sec), "")); got != want {
			t.Errorf("%s: heading = %q, want %q", path, got, want)
		}
	}
	// The BG questions that carry {area_in} take the same phrase.
	items := faqVisible(t, fetch(t, rr, "/area/veliko-tarnovo").Body.String())
	if want := "Какво е качеството на въздуха във Велико Търново сега?"; items[0].Q != want {
		t.Errorf("q1 = %q, want %q", items[0].Q, want)
	}
}

func TestFAQThresholdsEqualScaleEdges(t *testing.T) {
	edges := map[string][]string{}
	for _, s := range api.Scales() {
		if s.Name != "eaqi" || (s.Metric != "P2" && s.Metric != "P1") {
			continue
		}
		for _, b := range s.Bands {
			if b.Upper != nil {
				edges[s.Metric] = append(edges[s.Metric], strconv.FormatFloat(*b.Upper, 'f', -1, 64))
			}
		}
	}
	if len(edges["P2"]) != 5 || len(edges["P1"]) != 5 {
		t.Fatalf("scale edges = %v, want five per metric", edges)
	}
	p2, p1 := edges["P2"], edges["P1"]
	rr := renderer(t, eeaFixture(t))

	en := faqVisible(t, fetch(t, rr, "/en/area/plovdiv").Body.String())[2].A
	wantEN := []string{
		fmt.Sprintf("good is up to %s µg/m³, fair up to %s, moderate up to %s, poor up to %s, very poor up to %s,", p2[0], p2[1], p2[2], p2[3], p2[4]),
		fmt.Sprintf("the same bands end at %s, %s, %s, %s and %s.", p1[0], p1[1], p1[2], p1[3], p1[4]),
	}
	for _, w := range wantEN {
		if !strings.Contains(en, w) {
			t.Errorf("en a3 = %q, want it to contain %q", en, w)
		}
	}

	bg := faqVisible(t, fetch(t, rr, "/area/plovdiv").Body.String())[2].A
	wantBG := []string{
		fmt.Sprintf("доброто е до %s µg/m³, задоволителното до %s, умереното до %s, лошото до %s, много лошото до %s,", p2[0], p2[1], p2[2], p2[3], p2[4]),
		fmt.Sprintf("завършват при %s, %s, %s, %s и %s.", p1[0], p1[1], p1[2], p1[3], p1[4]),
	}
	for _, w := range wantBG {
		if !strings.Contains(bg, w) {
			t.Errorf("bg a3 = %q, want it to contain %q", bg, w)
		}
	}
}

func TestFAQMinutesMatchUpstreamPollInterval(t *testing.T) {
	cfg := testConfig(t)
	if cfg.Upstream.PollInterval%time.Minute != 0 {
		t.Fatalf("upstream.poll_interval = %v is not whole minutes", cfg.Upstream.PollInterval)
	}
	minutes := int(cfg.Upstream.PollInterval / time.Minute)
	rr := renderer(t, eeaFixture(t))
	en := faqVisible(t, fetch(t, rr, "/en/area/plovdiv").Body.String())[3].A
	if want := fmt.Sprintf("every %d minutes.", minutes); !strings.Contains(en, want) {
		t.Errorf("en a4 = %q, want it to contain %q", en, want)
	}
	bg := faqVisible(t, fetch(t, rr, "/area/plovdiv").Body.String())[3].A
	if want := fmt.Sprintf("на всеки %d минути.", minutes); !strings.Contains(bg, want) {
		t.Errorf("bg a4 = %q, want it to contain %q", bg, want)
	}
}

// The official-stations sentence follows the same hasEEA rule as the area description.
func TestFAQOfficialSentenceOnlyWithEEAData(t *testing.T) {
	rr := renderer(t, eeaFixture(t))
	cases := []struct {
		path string
		want bool
	}{
		{"/en/area/plovdiv", true},
		{"/en/area/smolyan-oblast", true},
		{"/en/area/mladost", true},
		{"/en/area/smolyan", false},
		{"/en/area/veliko-tarnovo", false},
		{"/en/area/krasna-polyana", false},
		{"/area/plovdiv", true},
		{"/area/smolyan", false},
	}
	for _, c := range cases {
		a5 := faqVisible(t, fetch(t, rr, c.path).Body.String())[4].A
		marker := "Official stations of the Executive Environment Agency"
		if strings.HasPrefix(c.path, "/area") {
			marker = "Показани са и официалните станции"
		}
		if got := strings.Contains(a5, marker); got != c.want {
			t.Errorf("%s: official sentence present = %v, want %v (a5 = %q)", c.path, got, c.want, a5)
		}
	}
}

func TestFAQAboutLinkFollowsLanguage(t *testing.T) {
	rr := renderer(t, eeaFixture(t))
	cases := []struct{ path, href, label string }{
		{"/area/plovdiv", "/about-the-data", "За данните"},
		{"/en/area/plovdiv", "/en/about-the-data", "About the data"},
	}
	for _, c := range cases {
		sec := faqSection(t, fetch(t, rr, c.path).Body.String())
		links := regexp.MustCompile(`<a [^>]*href="([^"]*)"[^>]*>([^<]*)</a>`).FindAllStringSubmatch(sec, -1)
		if len(links) != 1 || links[0][1] != c.href || links[0][2] != c.label {
			t.Errorf("%s: FAQ links = %v, want one link %s labelled %q", c.path, links, c.href, c.label)
		}
		if rec := fetch(t, rr, c.href); rec.Code != 200 {
			t.Errorf("%s: link target %s answers %d", c.path, c.href, rec.Code)
		}
	}
}

func TestFAQPageJSONLDMatchesVisibleText(t *testing.T) {
	rr := renderer(t, eeaFixture(t))
	for _, path := range []string{"/area/plovdiv", "/en/area/plovdiv", "/area/smolyan", "/en/area/smolyan"} {
		body := fetch(t, rr, path).Body.String()
		start := strings.Index(body, ldOpen) + len(ldOpen)
		end := strings.Index(body[start:], "</script>")
		var doc struct {
			Graph []json.RawMessage `json:"@graph"`
		}
		if err := json.Unmarshal([]byte(body[start:start+end]), &doc); err != nil {
			t.Fatalf("%s: JSON-LD does not parse: %v", path, err)
		}
		var faq *struct {
			Type       string `json:"@type"`
			MainEntity []struct {
				Type           string `json:"@type"`
				Name           string `json:"name"`
				AcceptedAnswer struct {
					Type string `json:"@type"`
					Text string `json:"text"`
				} `json:"acceptedAnswer"`
			} `json:"mainEntity"`
		}
		for _, raw := range doc.Graph {
			var probe struct {
				Type string `json:"@type"`
			}
			if err := json.Unmarshal(raw, &probe); err != nil || probe.Type != "FAQPage" {
				continue
			}
			faq = new(struct {
				Type       string `json:"@type"`
				MainEntity []struct {
					Type           string `json:"@type"`
					Name           string `json:"name"`
					AcceptedAnswer struct {
						Type string `json:"@type"`
						Text string `json:"text"`
					} `json:"acceptedAnswer"`
				} `json:"mainEntity"`
			})
			if err := json.Unmarshal(raw, faq); err != nil {
				t.Fatalf("%s: FAQPage node: %v", path, err)
			}
		}
		if faq == nil {
			t.Fatalf("%s: no FAQPage node in the graph", path)
		}
		visible := faqVisible(t, body)
		if len(faq.MainEntity) != 7 || len(visible) != 7 {
			t.Fatalf("%s: %d JSON-LD entries and %d visible, want 7 each", path, len(faq.MainEntity), len(visible))
		}
		for i, q := range faq.MainEntity {
			if q.Type != "Question" || q.AcceptedAnswer.Type != "Answer" {
				t.Errorf("%s: entry %d types = %q/%q", path, i+1, q.Type, q.AcceptedAnswer.Type)
			}
			if q.Name != visible[i].Q || q.AcceptedAnswer.Text != visible[i].A {
				t.Errorf("%s: entry %d JSON-LD differs from visible text:\n ld  %q / %q\n vis %q / %q",
					path, i+1, q.Name, q.AcceptedAnswer.Text, visible[i].Q, visible[i].A)
			}
		}
	}
}

func TestFAQAreaMidSentenceIsLowercaseOblast(t *testing.T) {
	rr := renderer(t, eeaFixture(t))
	items := faqVisible(t, fetch(t, rr, "/area/smolyan-oblast").Body.String())
	if want := "Защо понякога няма стойност за област Смолян?"; items[6].Q != want {
		t.Errorf("q7 = %q, want %q", items[6].Q, want)
	}
	items = faqVisible(t, fetch(t, rr, "/area/sofiyska-oblast").Body.String())
	if want := "Защо понякога няма стойност за Софийска област?"; items[6].Q != want {
		t.Errorf("q7 = %q, want %q", items[6].Q, want)
	}
}
