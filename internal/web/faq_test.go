package web_test

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
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

// The area FAQ holds only area questions. The rank question needs the rank block, so
// the fixture's small peer groups leave two.
func TestAreaFAQHoldsOnlyAreaQuestions(t *testing.T) {
	rr := renderer(t, eeaFixture(t))
	for _, path := range []string{"/area/plovdiv", "/en/area/plovdiv", "/area/smolyan", "/en/area/smolyan-oblast", "/area/mladost"} {
		body := fetch(t, rr, path).Body.String()
		items := faqVisible(t, body)
		if len(items) < 2 || len(items) > 3 {
			t.Fatalf("%s: %d FAQ entries, want 2 or 3", path, len(items))
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
		"/en/area/mladost":      "Questions about the air in Mladost district",
		"/en/area/plovdiv":      "Questions about the air in Plovdiv",
		"/area/sofiyska-oblast": "Въпроси за въздуха в Софийска област",
	}
	for path, want := range cases {
		sec := faqSection(t, fetch(t, rr, path).Body.String())
		if got := html.UnescapeString(tagRE.ReplaceAllString(regexp.MustCompile(`(?s)<h2[^>]*>.*?</h2>`).FindString(sec), "")); got != want {
			t.Errorf("%s: heading = %q, want %q", path, got, want)
		}
	}
	// The BG questions take the same phrase.
	items := faqVisible(t, fetch(t, rr, "/area/veliko-tarnovo").Body.String())
	if want := "Има ли официална станция във Велико Търново?"; items[0].Q != want {
		t.Errorf("q = %q, want %q", items[0].Q, want)
	}
}

// The EAQI edges moved to the About page. They must still equal the scale.
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

	en := aboutFAQVisible(t, fetch(t, rr, "/en/about-the-data").Body.String())[3].A
	wantEN := []string{
		fmt.Sprintf("good is up to %s µg/m³, fair up to %s, moderate up to %s, poor up to %s, very poor up to %s,", p2[0], p2[1], p2[2], p2[3], p2[4]),
		fmt.Sprintf("the same bands end at %s, %s, %s, %s and %s.", p1[0], p1[1], p1[2], p1[3], p1[4]),
	}
	for _, w := range wantEN {
		if !strings.Contains(en, w) {
			t.Errorf("en a4 = %q, want it to contain %q", en, w)
		}
	}

	bg := aboutFAQVisible(t, fetch(t, rr, "/about-the-data").Body.String())[3].A
	wantBG := []string{
		fmt.Sprintf("доброто е до %s µg/m³, задоволителното до %s, умереното до %s, лошото до %s, много лошото до %s,", p2[0], p2[1], p2[2], p2[3], p2[4]),
		fmt.Sprintf("завършват при %s, %s, %s, %s и %s.", p1[0], p1[1], p1[2], p1[3], p1[4]),
	}
	for _, w := range wantBG {
		if !strings.Contains(bg, w) {
			t.Errorf("bg a4 = %q, want it to contain %q", bg, w)
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
	en := aboutFAQVisible(t, fetch(t, rr, "/en/about-the-data").Body.String())[1].A
	if want := fmt.Sprintf("every %d minutes.", minutes); !strings.Contains(en, want) {
		t.Errorf("en a2 = %q, want it to contain %q", en, want)
	}
	bg := aboutFAQVisible(t, fetch(t, rr, "/about-the-data").Body.String())[1].A
	if want := fmt.Sprintf("на всеки %d минути.", minutes); !strings.Contains(bg, want) {
		t.Errorf("bg a2 = %q, want it to contain %q", bg, want)
	}
}

// The pointers under the FAQ follow the page language and reach live pages.
func TestFAQPointersFollowLanguage(t *testing.T) {
	rr := renderer(t, eeaFixture(t))
	cases := []struct{ path, general, explainer string }{
		{"/area/plovdiv", `<a class="link" href="/about-the-data">За данните</a>`, `<a class="link" href="/air-pollution">Замърсяване на въздуха: обяснение</a>`},
		{"/en/area/plovdiv", `<a class="link" href="/en/about-the-data">About the data</a>`, `<a class="link" href="/en/air-pollution">Air pollution explained</a>`},
	}
	for _, c := range cases {
		body := fetch(t, rr, c.path).Body.String()
		for _, anchor := range []string{c.general, c.explainer} {
			if !strings.Contains(body, anchor) {
				t.Errorf("%s: page lacks %s", c.path, anchor)
			}
		}
		if strings.ContainsAny(regexp.MustCompile(`<p class="faq__more">.*?</p>`).FindString(body), "{}") {
			t.Errorf("%s: raw placeholder in a pointer line", c.path)
		}
	}
	for _, href := range []string{"/about-the-data", "/en/about-the-data", "/air-pollution", "/en/air-pollution"} {
		if rec := fetch(t, rr, href); rec.Code != 200 {
			t.Errorf("link target %s answers %d", href, rec.Code)
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
		type question struct {
			Type           string `json:"@type"`
			Name           string `json:"name"`
			AcceptedAnswer struct {
				Type string `json:"@type"`
				Text string `json:"text"`
			} `json:"acceptedAnswer"`
		}
		var faq []question
		for _, raw := range doc.Graph {
			var node struct {
				Type       string     `json:"@type"`
				MainEntity []question `json:"mainEntity"`
			}
			if err := json.Unmarshal(raw, &node); err != nil || node.Type != "FAQPage" {
				continue
			}
			faq = node.MainEntity
		}
		visible := faqVisible(t, body)
		if len(faq) == 0 || len(faq) != len(visible) {
			t.Fatalf("%s: %d JSON-LD entries and %d visible", path, len(faq), len(visible))
		}
		for i, q := range faq {
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

// The generic questions left the area catalogue for good.
func TestAreaFAQDropsTheGenericKeys(t *testing.T) {
	for _, lang := range []string{"en", "bg"} {
		raw, err := os.ReadFile(filepath.Join("..", "i18n", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var cat map[string]string
		if err := json.Unmarshal(raw, &cat); err != nil {
			t.Fatal(err)
		}
		for n := 4; n <= 11; n++ {
			for _, k := range []string{"faq.q" + strconv.Itoa(n), "faq.a" + strconv.Itoa(n)} {
				if _, ok := cat[k]; ok {
					t.Errorf("%s.json still has %s", lang, k)
				}
			}
		}
		for _, k := range []string{"faq.a5_official", "faq.air_pollution_link"} {
			if _, ok := cat[k]; ok {
				t.Errorf("%s.json still has %s", lang, k)
			}
		}
	}
}
