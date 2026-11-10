package web_test

import (
	"html"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/web"
)

var openDataPaths = map[string]string{"/open-data": "bg", "/en/open-data": "en"}

// openDataRenderer builds a renderer whose rate limits are set by mutate.
func openDataRenderer(t *testing.T, mutate func(*config.RateLimit)) *web.Renderer {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t)
	if mutate != nil {
		mutate(&cfg.RateLimit)
	}
	h := snapshot.NewHolder(cfg.Series, config.Wind{})
	h.Store(fixture(t))
	rr, err := web.NewRenderer(cat, h, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return rr
}

func TestOpenDataPageRendersInBothLanguages(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := openDataRenderer(t, nil)
	for path, lang := range openDataPaths {
		rec := fetch(t, rr, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d, want 200", path, rec.Code)
		}
		body := rec.Body.String()
		title := cat.T(lang, "seo.opendata.title")
		desc := cat.T(lang, "seo.opendata.description")
		if title == "" || strings.HasPrefix(title, "!") || desc == "" || strings.HasPrefix(desc, "!") {
			t.Fatalf("%s: seo.opendata title or description missing", lang)
		}
		if n := utf8.RuneCountInString(desc); n >= 160 {
			t.Errorf("%s description is %d runes, want under 160", lang, n)
		}
		if !strings.Contains(body, "<title>"+html.EscapeString(title)) {
			t.Errorf("%s: <title> does not start with %q", path, title)
		}
		if !strings.Contains(body, `<meta name="description" content="`+html.EscapeString(desc)+`"`) {
			t.Errorf("%s: meta description missing", path)
		}
		if !strings.Contains(body, "<h1") || !strings.Contains(body, html.EscapeString(cat.T(lang, "opendata.title"))) {
			t.Errorf("%s: h1 missing", path)
		}
		for _, key := range []string{"opendata.data.body", "opendata.formats.body", "opendata.update.body"} {
			if !strings.Contains(body, html.EscapeString(cat.T(lang, key))) {
				t.Errorf("%s: %s not rendered", path, key)
			}
		}
	}
}

// Paragraph 5 is approved copy; with the committed config it must read exactly so.
func TestOpenDataFairUseReadsAsApprovedWithCommittedConfig(t *testing.T) {
	want := map[string]string{
		"/en/open-data": "Requests are rate limited per address. Currently the API allows about 10 requests per second in short bursts, history requests about 1 per second, and a client that reads more than 60 different areas or 40 different sensors in an hour is slowed down for 15 minutes. A limited request gets status 429 with a Retry-After header. Cache what you fetch. For regular use of the full dataset, write to privacy@kanarche.eu.",
		"/open-data":    "Заявките са ограничени за всеки адрес. В момента API допуска около 10 заявки в секунда на кратки серии, историята около 1 заявка в секунда, а клиент, който прочете над 60 различни района или 40 различни сензора за час, се забавя за 15 минути. Ограничената заявка получава статус 429 със заглавие Retry-After. Запазвайте получените данни. За редовно ползване на целия набор пишете на privacy@kanarche.eu.",
	}
	rr := openDataRenderer(t, nil)
	for path, text := range want {
		if body := fetch(t, rr, path).Body.String(); !strings.Contains(body, html.EscapeString(text)) {
			t.Errorf("%s: paragraph 5 differs from the approved text", path)
		}
	}
}

// The numbers come from config.RateLimit, so changing the config changes the page.
func TestOpenDataFairUseNumbersComeFromConfig(t *testing.T) {
	rr := openDataRenderer(t, func(r *config.RateLimit) {
		r.API.PerSecond = 7
		r.Series.PerSecond = 3
		r.Enumerate.AreasPerWindow = 33
		r.Enumerate.SensorsPerWindow = 22
		r.Enumerate.RetryAfter = 10 * time.Minute
	})
	for path, wants := range map[string][]string{
		"/en/open-data": {"about 7 requests per second", "about 3 per second", "more than 33 different areas or 22 different sensors", "slowed down for 10 minutes"},
		"/open-data":    {"около 7 заявки в секунда", "историята около 3 заявка в секунда", "над 33 различни района или 22 различни сензора", "се забавя за 10 минути"},
	} {
		body := fetch(t, rr, path).Body.String()
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %q after the config changed", path, want)
			}
		}
		if strings.Contains(body, "{api_rps}") || strings.Contains(body, "{areas}") {
			t.Errorf("%s leaks a placeholder", path)
		}
	}
}

func TestOpenDataLicenceParagraphLinksAndStatesDerivedLicence(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := openDataRenderer(t, nil)
	for path, c := range map[string]struct{ lang, href, warranty string }{
		"/open-data":    {"bg", `href="/licences"`, "без гаранции"},
		"/en/open-data": {"en", `href="/en/licences"`, "no warranty"},
	} {
		body := fetch(t, rr, path).Body.String()
		if !strings.Contains(body, c.href) {
			t.Errorf("%s does not link to the licences page", path)
		}
		if strings.Contains(body, "{licences}") {
			t.Errorf("%s leaks {licences}", path)
		}
		derived := html.EscapeString(cat.T(c.lang, "licences.derived.body"))
		d, w := strings.Index(body, derived), strings.Index(body, c.warranty)
		if d < 0 || w < d {
			t.Errorf("%s: derived-values sentence must come before the warranty sentence (d=%d w=%d)", path, d, w)
		}
	}
	if got := cat.T("en", "licences.derived.body"); got != "Values computed by Kanarche, such as area medians and 24 hour means, are under ODbL 1.0 because they come from ODbL data. Credit Kanarche and Sensor.Community." {
		t.Errorf("en derived sentence changed: %q", got)
	}
	if got := cat.T("bg", "licences.derived.body"); got != "Стойностите, изчислени от Канарче, като медианите за районите и средните стойности за 24 часа, са под ODbL 1.0, защото идват от данни под ODbL. Посочвайте Канарче и Sensor.Community." {
		t.Errorf("bg derived sentence changed: %q", got)
	}
}

func TestLicencesPageStatesDerivedValuesLicence(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := openDataRenderer(t, nil)
	for path, lang := range map[string]string{"/licences": "bg", "/en/licences": "en"} {
		body := fetch(t, rr, path).Body.String()
		if !strings.Contains(body, html.EscapeString(cat.T(lang, "licences.derived.body"))) {
			t.Errorf("%s lacks the derived-values sentence", path)
		}
	}
}

func TestOpenDataIsInSitemapAndLLMsTxt(t *testing.T) {
	rr := openDataRenderer(t, nil)
	sm := fetch(t, rr, "/sitemap.xml").Body.String()
	for _, loc := range []string{"<loc>" + testBaseURL + "/open-data</loc>", "<loc>" + testBaseURL + "/en/open-data</loc>"} {
		if !strings.Contains(sm, loc) {
			t.Errorf("sitemap lacks %s", loc)
		}
	}
	if llms := fetch(t, rr, "/llms.txt").Body.String(); !strings.Contains(llms, "("+testBaseURL+"/open-data)") {
		t.Error("llms.txt lacks the open-data page")
	}
}

func TestFooterLinksToOpenData(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := openDataRenderer(t, nil)
	for path, c := range map[string]struct{ lang, href string }{
		"/en/about-the-data": {"en", `href="/en/open-data"`},
		"/about-the-data":    {"bg", `href="/open-data"`},
	} {
		foot := footerOf(t, fetch(t, rr, path).Body.String())
		if !strings.Contains(foot, c.href+">"+cat.T(c.lang, "footer.opendata")+"</a>") {
			t.Errorf("%s: footer lacks the open data link", path)
		}
	}
	if cat.T("en", "footer.opendata") != "Open data" || cat.T("bg", "footer.opendata") != "Отворени данни" {
		t.Error("footer.opendata copy changed")
	}
}

func TestOpenDataIsLinkedFromLicencesAndAbout(t *testing.T) {
	rr := openDataRenderer(t, nil)
	for path, href := range map[string]string{
		"/en/licences": `href="/en/open-data"`, "/licences": `href="/open-data"`,
		"/en/about": `href="/en/open-data"`, "/about": `href="/open-data"`,
	} {
		body := fetch(t, rr, path).Body.String()
		body = body[:strings.Index(body, "<footer")]
		if !strings.Contains(body, href) {
			t.Errorf("%s main content lacks a link to the open data page", path)
		}
	}
}
