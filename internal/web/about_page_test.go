package web_test

import (
	"html"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// tagRe strips markup so a text assertion sees what a reader sees, and an
// attribute such as a script URL cannot trip it.
var tagRe = regexp.MustCompile(`(?s)<script.*?</script>|<[^>]*>`)

var bannedWordRe = regexp.MustCompile(`(?i)\b(API|AQI)\b`)

func visibleText(body string) string { return tagRe.ReplaceAllString(body, " ") }

func aboutPaths() []string { return []string{"/about", "/en/about"} }

// The page is static prose and must render with no snapshot, like the data page.
func TestAboutPageRoutes(t *testing.T) {
	for _, withSnapshot := range []bool{true, false} {
		rr := renderer(t, nil)
		if withSnapshot {
			rr = renderer(t, fixture(t))
		}
		for _, p := range aboutPaths() {
			rec := fetch(t, rr, p)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s (snapshot=%v) = %d, want 200", p, withSnapshot, rec.Code)
			}
			if strings.Contains(rec.Body.String(), "!about.") {
				t.Errorf("%s has an untranslated key marker", p)
			}
		}
	}
}

func TestAboutPageTitleAndDescription(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, p := range aboutPaths() {
		body := fetch(t, rr, p).Body.String()
		title, desc := pageTitle(t, body), pageDescription(t, body)
		if n := utf8.RuneCountInString(title); n == 0 || n > 60 {
			t.Errorf("%s title %q is %d runes, want 1..60", p, title, n)
		}
		if !strings.HasSuffix(title, " | "+testBrand(map[bool]string{true: "/en", false: ""}[strings.HasPrefix(p, "/en")])) {
			t.Errorf("%s title %q lacks the brand suffix", p, title)
		}
		if n := utf8.RuneCountInString(desc); n == 0 || n > 155 {
			t.Errorf("%s description is %d runes, want 1..155", p, n)
		}
		other := pageTitle(t, fetch(t, rr, strings.Replace(p, "about", "about-the-data", 1)).Body.String())
		if title == other {
			t.Errorf("%s shares a title with the data page: %q", p, title)
		}
	}
	if got := pageTitle(t, fetch(t, rr, "/en/about").Body.String()); got != "About: free, open air-quality map for Bulgaria | Kanarche" {
		t.Errorf("en title = %q", got)
	}
}

func TestAboutPageNamesNeitherTheAPINorAQI(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, p := range aboutPaths() {
		text := visibleText(fetch(t, rr, p).Body.String())
		// Whole words, case-insensitive: "capital" contains "api".
		if m := bannedWordRe.FindString(text); m != "" {
			t.Errorf("%s mentions %q in visible text", p, m)
		}
	}
}

func TestAboutPageLinksTheRepositoryAndEmbedDocs(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, p := range aboutPaths() {
		body := fetch(t, rr, p).Body.String()
		for _, want := range []string{
			sourceRepoHref,
			`href="` + sourceRepoURL + `/blob/master/CONTRIBUTING.md"`,
			`href="` + sourceRepoURL + `/blob/master/docs/embedding.md"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s is missing %s", p, want)
			}
		}
		if !strings.Contains(body, "https://airbg.org/embed?area=") {
			t.Errorf("%s embed snippet is not the /embed?area= form", p)
		}
		if strings.Contains(body, "/embed/plovdiv") {
			t.Errorf("%s carries the mockup's wrong /embed/<slug> form", p)
		}
	}
}

func TestAboutPagePrivacyWording(t *testing.T) {
	rr := renderer(t, fixture(t))
	en := strings.Join(strings.Fields(visibleText(fetch(t, rr, "/en/about").Body.String())), " ")
	const wantEN = "We keep no IP address or identifier: none in logs, none in the database. Your IP is used for a moment in memory to limit abusive traffic, then forgotten. Cloudflare, our network provider, sees it too and gives us only daily totals."
	if !strings.Contains(en, wantEN) {
		t.Errorf("en page lacks the approved privacy wording")
	}
	if strings.Contains(en, "We record nothing about you") {
		t.Errorf("the no-personal-data tile contradicts the privacy section")
	}
	bg := strings.Join(strings.Fields(visibleText(fetch(t, rr, "/about").Body.String())), " ")
	if !strings.Contains(bg, "Cloudflare") || !strings.Contains(bg, "дневни суми") {
		t.Errorf("bg privacy section is missing the Cloudflare daily-totals statement")
	}
}

func TestAboutPageBulgarianUsesFPCNotPM(t *testing.T) {
	text := visibleText(fetch(t, renderer(t, fixture(t)), "/about").Body.String())
	if strings.Contains(text, "ПМ") {
		t.Errorf("bg about page contains ПМ; the site term is ФПЧ")
	}
}

func TestAboutPageSharesThePrivacyControls(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, p := range aboutPaths() {
		body := fetch(t, rr, p).Body.String()
		for _, key := range storageAllowListKeys(t) {
			if !strings.Contains(body, key) {
				t.Errorf("%s does not list %q", p, key)
			}
		}
		if !strings.Contains(body, `data-island="clearsettings"`) {
			t.Errorf("%s has no clearsettings island", p)
		}
	}
}

func TestAboutPageHasTheVisitorsIsland(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, p := range aboutPaths() {
		body := fetch(t, rr, p).Body.String()
		for _, want := range []string{`data-island="visitors"`, `data-t-empty="`, `data-t-title="`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s is missing %s", p, want)
			}
		}
	}
}

func TestMastheadHasAnAboutTab(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, tc := range []struct{ page, href string }{
		{"/", `href="/about"`}, {"/areas", `href="/about"`},
		{"/en/", `href="/en/about"`}, {"/en/about", `href="/en/about" aria-current="page"`},
	} {
		if body := fetch(t, rr, tc.page).Body.String(); !strings.Contains(body, tc.href) {
			t.Errorf("%s masthead lacks %s", tc.page, tc.href)
		}
	}
}

func TestSitemapListsTheAboutPage(t *testing.T) {
	doc := fetchSitemap(t)
	for _, u := range doc.URLs {
		if strings.HasSuffix(u.Loc, "/about") {
			return
		}
	}
	t.Errorf("sitemap has no /about entry")
}

var (
	imgTagRe = regexp.MustCompile(`<img\b[^>]*>`)
	attrRe   = regexp.MustCompile(`([a-z-]+)="([^"]*)"`)
)

// Each Getting started step carries a light and a dark screenshot, with
// alt text, fixed dimensions and a file the embedded static FS really serves.
func TestAboutStartScreenshots(t *testing.T) {
	rr := renderer(t, nil)
	alts := map[string]string{}
	for _, p := range aboutPaths() {
		body := fetch(t, rr, p).Body.String()
		start := body[strings.Index(body, `id="start"`):]
		start = start[:strings.Index(start, `id="privacy"`)]
		imgs := imgTagRe.FindAllString(start, -1)
		if len(imgs) != 10 {
			t.Fatalf("%s: %d screenshots in #start, want 10 (5 steps x light and dark)", p, len(imgs))
		}
		for _, tag := range imgs {
			a := map[string]string{}
			for _, m := range attrRe.FindAllStringSubmatch(tag, -1) {
				a[m[1]] = m[2]
			}
			if strings.TrimSpace(a["alt"]) == "" || strings.Contains(a["alt"], "!about.") {
				t.Errorf("%s: %s has no usable alt", p, tag)
			}
			if a["width"] == "" || a["height"] == "" {
				t.Errorf("%s: %s lacks width or height", p, tag)
			}
			if a["loading"] != "lazy" || a["decoding"] != "async" {
				t.Errorf("%s: %s must be lazy and async-decoded", p, tag)
			}
			img := fetch(t, rr, html.UnescapeString(a["src"]))
			if img.Code != http.StatusOK || img.Body.Len() == 0 || !strings.HasPrefix(img.Header().Get("Content-Type"), "image/webp") {
				t.Errorf("%s: %s serves %d %q, %d bytes", p, a["src"], img.Code, img.Header().Get("Content-Type"), img.Body.Len())
			}
			if img.Body.Len() > 100*1024 {
				t.Errorf("%s is %d bytes, want under 100 KB", a["src"], img.Body.Len())
			}
			if strings.Contains(a["src"], "-bg-") != (p == "/about") {
				t.Errorf("%s points at the wrong language: %s", p, a["src"])
			}
			alts[p] = a["alt"]
		}
	}
	if alts["/about"] == alts["/en/about"] {
		t.Errorf("bg and en alt text are identical")
	}
}
