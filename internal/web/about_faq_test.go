package web_test

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	aboutFAQSectionRE = regexp.MustCompile(`(?s)<section class="about-faq"[^>]*>.*?</section>`)
	aboutFAQItemRE    = regexp.MustCompile(`(?s)<h3>(.*?)</h3>\s*<p>(.*?)</p>`)
	placeholderRE     = regexp.MustCompile(`\{[a-z0-9_]+\}`)
)

var aboutFAQQuestions = map[string][]string{
	"/en/about-the-data": {
		"Where does the air quality data come from?",
		"How often is the data updated?",
		"What are PM2.5 and PM10?",
		"What do the colours on the map mean?",
		"Why is a sensor hidden or marked as faulty?",
		"Why does an area sometimes have no value?",
		"How far back does the history go?",
		"Can I reuse the data?",
		"How accurate are low-cost sensors?",
		"Where does the data on bathing water cleanliness come from?",
		"What do the ratings excellent, good, sufficient and poor mean?",
		"Why does the rating not show the water quality for today?",
		"How do I find beaches on the map?",
		"Is the water in the Black Sea clean?",
	},
	"/about-the-data": {
		"Откъде идват данните за въздуха?",
		"Колко често се обновяват данните?",
		"Какво са ФПЧ2.5 и ФПЧ10?",
		"Какво означават цветовете на картата?",
		"Защо някой сензор е скрит или отбелязан като неизправен?",
		"Защо понякога за даден район няма стойност?",
		"Колко назад във времето стига историята?",
		"Мога ли да използвам данните?",
		"Колко точни са нискобюджетните сензори?",
		"Откъде идват данните за чистотата на водата за къпане?",
		"Какво значат оценките отлично, добро, задоволително и лошо?",
		"Защо оценката не показва качеството на водата за днешния ден?",
		"Как да открия плажовете на картата?",
		"Чиста ли е водата в Черно море?",
	},
}

// aboutFAQVisible returns the visible question and answer text of the About page FAQ.
func aboutFAQVisible(t *testing.T, body string) []faqEntry {
	t.Helper()
	sec := aboutFAQSectionRE.FindString(body)
	if sec == "" {
		t.Fatalf("no about FAQ section in page")
	}
	var out []faqEntry
	for _, m := range aboutFAQItemRE.FindAllStringSubmatch(sec, -1) {
		out = append(out, faqEntry{
			Q: html.UnescapeString(tagRE.ReplaceAllString(m[1], "")),
			A: html.UnescapeString(tagRE.ReplaceAllString(m[2], "")),
		})
	}
	return out
}

func TestAboutFAQKeysExistInBothCatalogues(t *testing.T) {
	for _, lang := range []string{"en", "bg"} {
		raw, err := os.ReadFile(filepath.Join("..", "i18n", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var cat map[string]string
		if err := json.Unmarshal(raw, &cat); err != nil {
			t.Fatal(err)
		}
		keys := []string{"about_faq.heading"}
		for n := 1; n <= 14; n++ {
			if n == 3 || n == 4 {
				continue // reused from faq.q2/a2 and faq.q3/a3
			}
			keys = append(keys, "about_faq.q"+strconv.Itoa(n), "about_faq.a"+strconv.Itoa(n))
		}
		for _, k := range keys {
			if strings.TrimSpace(cat[k]) == "" {
				t.Errorf("%s.json is missing %s", lang, k)
			}
		}
	}
}

func TestAboutFAQRendersAllQuestionsInOrder(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path, want := range aboutFAQQuestions {
		items := aboutFAQVisible(t, fetch(t, rr, path).Body.String())
		if len(items) != len(want) {
			t.Fatalf("%s: %d FAQ entries, want %d", path, len(items), len(want))
		}
		for i, it := range items {
			if it.Q != want[i] {
				t.Errorf("%s: question %d = %q, want %q", path, i+1, it.Q, want[i])
			}
			if strings.TrimSpace(it.A) == "" {
				t.Errorf("%s: answer %d is empty", path, i+1)
			}
		}
	}
}

func TestAboutFAQHasNoUnreplacedPlaceholders(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path := range aboutFAQQuestions {
		body := fetch(t, rr, path).Body.String()
		if m := placeholderRE.FindString(aboutFAQSectionRE.FindString(body)); m != "" {
			t.Errorf("%s: unreplaced placeholder %s", path, m)
		}
		items := aboutFAQVisible(t, body)
		if !strings.Contains(items[1].A, " 5 ") {
			t.Errorf("%s: answer 2 does not carry the poll interval: %q", path, items[1].A)
		}
		if !strings.Contains(items[3].A, "µg/m³") || strings.Contains(items[3].A, "{") {
			t.Errorf("%s: answer 4 has no filled EAQI edges: %q", path, items[3].A)
		}
	}
}

func TestAboutFAQLinksSourcesAndLicences(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range []struct{ path, licences string }{
		{"/en/about-the-data", `href="/en/licences"`},
		{"/about-the-data", `href="/licences"`},
	} {
		body := fetch(t, rr, c.path).Body.String()
		sec := aboutFAQSectionRE.FindString(body)
		for _, want := range []string{`href="#sources"`, c.licences} {
			if !strings.Contains(sec, want) {
				t.Errorf("%s: FAQ has no %s", c.path, want)
			}
		}
		if !strings.Contains(body, `id="sources"`) {
			t.Errorf("%s: the sources heading has no id to link to", c.path)
		}
	}
}

func TestAboutFAQPageJSONLDMatchesVisibleText(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path := range aboutFAQQuestions {
		body := fetch(t, rr, path).Body.String()
		start := strings.Index(body, ldOpen) + len(ldOpen)
		end := strings.Index(body[start:], "</script>")
		var doc struct {
			Graph []json.RawMessage `json:"@graph"`
		}
		if err := json.Unmarshal([]byte(body[start:start+end]), &doc); err != nil {
			t.Fatalf("%s: JSON-LD does not parse: %v", path, err)
		}
		type node struct {
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
		var faq *node
		types := map[string]bool{}
		for _, raw := range doc.Graph {
			var n node
			if err := json.Unmarshal(raw, &n); err != nil {
				t.Fatal(err)
			}
			types[n.Type] = true
			if n.Type == "FAQPage" {
				faq = &n
			}
		}
		if !types["Dataset"] {
			t.Errorf("%s: Dataset node missing from the graph", path)
		}
		if faq == nil {
			t.Fatalf("%s: no FAQPage node in the graph", path)
		}
		visible := aboutFAQVisible(t, body)
		if len(faq.MainEntity) != 14 || len(visible) != 14 {
			t.Fatalf("%s: %d JSON-LD entries and %d visible, want 14 each", path, len(faq.MainEntity), len(visible))
		}
		for i, q := range faq.MainEntity {
			if q.Type != "Question" || q.AcceptedAnswer.Type != "Answer" {
				t.Errorf("%s: entry %d types = %q/%q", path, i+1, q.Type, q.AcceptedAnswer.Type)
			}
			if i+1 == 10 || i+1 == 14 {
				// Answers 10 and 14 end in links: the page shows labels, JSON-LD the plain URLs.
				if q.Name != visible[i].Q {
					t.Errorf("%s: entry %d name differs", path, i+1)
				}
				continue
			}
			if q.Name != visible[i].Q || q.AcceptedAnswer.Text != visible[i].A {
				t.Errorf("%s: entry %d JSON-LD differs from visible text:\n ld  %q / %q\n vis %q / %q",
					path, i+1, q.Name, q.AcceptedAnswer.Text, visible[i].Q, visible[i].A)
			}
		}
	}
}

func TestAboutPageDropsSensorsSection(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path, heading := range map[string]string{
		"/en/about-the-data": "The sensors are not reference instruments",
		"/about-the-data":    "Сензорите не са референтни уреди",
	} {
		if strings.Contains(fetch(t, rr, path).Body.String(), heading) {
			t.Errorf("%s still renders the old sensors heading", path)
		}
	}
}

func TestAboutFAQSitsBetweenReferenceAndCities(t *testing.T) {
	rr := renderer(t, fixture(t))
	body := fetch(t, rr, "/en/about-the-data").Body.String()
	ref := strings.Index(body, "This map is for reference, not medical advice")
	faq := strings.Index(body, `class="about-faq"`)
	cities := strings.Index(body, "City areas are not all the same kind of thing")
	if ref < 0 || faq < ref || cities < faq {
		t.Errorf("want reference < FAQ < cities, got %d, %d, %d", ref, faq, cities)
	}
}

var rziFAQURLs = []string{"https://rzi-varna.example/results", "https://rzi-burgas.example/results", "https://rzi-dobrich.example/results"}

// Answers 10 and 14 end with the three RHI pages from .RZIPages, labelled and separated like /licences.
func TestAboutFAQBathingAnswersLinkRZIPages(t *testing.T) {
	rr := rziRenderer(t, true)
	for path, labels := range map[string]string{
		"/about-the-data":    `РЗИ Варна</a> · <a href="https://rzi-burgas.example/results" rel="noopener noreferrer">РЗИ Бургас</a> · <a href="https://rzi-dobrich.example/results" rel="noopener noreferrer">РЗИ Добрич</a>`,
		"/en/about-the-data": `RHI Varna</a> · <a href="https://rzi-burgas.example/results" rel="noopener noreferrer">RHI Burgas</a> · <a href="https://rzi-dobrich.example/results" rel="noopener noreferrer">RHI Dobrich</a>`,
	} {
		sec := aboutFAQSectionRE.FindString(fetch(t, rr, path).Body.String())
		for _, u := range rziFAQURLs {
			if got := strings.Count(sec, `<a href="`+u+`" rel="noopener noreferrer">`); got != 2 {
				t.Errorf("%s: %s linked %d times in the FAQ, want 2 (a10 and a14)", path, u, got)
			}
		}
		if got := strings.Count(sec, labels); got != 2 {
			t.Errorf("%s: labelled link run appears %d times, want 2", path, got)
		}
	}
}

// The FAQPage text for answers 10 and 14 is plain: the sentence, then the three URLs.
func TestAboutFAQPageBathingAnswersCarryPlainURLs(t *testing.T) {
	rr := rziRenderer(t, true)
	for path := range aboutFAQQuestions {
		body := fetch(t, rr, path).Body.String()
		start := strings.Index(body, ldOpen) + len(ldOpen)
		end := strings.Index(body[start:], "</script>")
		var doc struct {
			Graph []struct {
				Type       string `json:"@type"`
				MainEntity []struct {
					AcceptedAnswer struct {
						Text string `json:"text"`
					} `json:"acceptedAnswer"`
				} `json:"mainEntity"`
			} `json:"@graph"`
		}
		if err := json.Unmarshal([]byte(body[start:start+end]), &doc); err != nil {
			t.Fatal(err)
		}
		for _, n := range doc.Graph {
			if n.Type != "FAQPage" {
				continue
			}
			if len(n.MainEntity) != 14 {
				t.Fatalf("%s: %d FAQ entries, want 14", path, len(n.MainEntity))
			}
			for _, i := range []int{9, 13} {
				text := n.MainEntity[i].AcceptedAnswer.Text
				if strings.Contains(text, "<") || strings.Contains(text, "[links]") {
					t.Errorf("%s: answer %d is not plain text: %q", path, i+1, text)
				}
				if !strings.HasSuffix(text, ": "+strings.Join(rziFAQURLs, " ")) {
					t.Errorf("%s: answer %d does not end with the three URLs: %q", path, i+1, text)
				}
			}
		}
	}
}
