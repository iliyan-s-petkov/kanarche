package web_test

import (
	"net/http"
	"strings"
	"testing"
)

const (
	termsLineEN = "Indicative data, with no guarantee. Do not use it for health or safety decisions."
	termsLineBG = "Данните са ориентировъчни и без гаранция. Не ги използвайте за решения за здраве или безопасност."
	embedLineEN = "Indicative data, not for health decisions."
	embedLineBG = "Ориентировъчни данни, не за решения за здравето."
	areaHintEN  = "This figure is indicative."
	areaHintBG  = "Стойността е ориентировъчна."
)

// Every page type that renders the footer carries the disclaimer and the terms link.
func TestFooterCarriesDisclaimerAndTermsLink(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))
	for _, tc := range []struct{ path, line, href string }{
		{"/", termsLineBG, `href="/terms"`},
		{"/en/", termsLineEN, `href="/en/terms"`},
		{"/areas", termsLineBG, `href="/terms"`},
		{"/area/sofiya", termsLineBG, `href="/terms"`},
		{"/en/area/sofiya", termsLineEN, `href="/en/terms"`},
		{"/about", termsLineBG, `href="/terms"`},
		{"/about-the-data", termsLineBG, `href="/terms"`},
		{"/privacy", termsLineBG, `href="/terms"`},
		{"/licences", termsLineBG, `href="/terms"`},
		{"/terms", termsLineBG, `href="/terms"`},
		{"/en/terms", termsLineEN, `href="/en/terms"`},
		{"/no-such-page", termsLineBG, `href="/terms"`},
	} {
		foot := footerOf(t, fetch(t, rr, tc.path).Body.String())
		if !strings.Contains(foot, tc.line) {
			t.Errorf("%s footer lacks the disclaimer line", tc.path)
		}
		if !strings.Contains(foot, tc.href) {
			t.Errorf("%s footer lacks %s", tc.path, tc.href)
		}
	}
}

// The embed drops the site footer, so its footline carries the short line and an absolute link.
func TestEmbedFootlineCarriesDisclaimerAndTermsLink(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, tc := range []struct{ path, line, href string }{
		{"/embed", embedLineBG, `href="https://airbg.org/terms"`},
		{"/en/embed", embedLineEN, `href="https://airbg.org/en/terms"`},
	} {
		body := fetch(t, rr, tc.path).Body.String()
		for _, want := range []string{tc.line, tc.href} {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %q", tc.path, want)
			}
		}
	}
}

func TestTermsPages(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, tc := range []struct{ path, lang, title, canonical, h1, liability string }{
		{"/terms", "bg", "<title>Условия за ползване и отказ от отговорност | Канарче</title>",
			`<link rel="canonical" href="https://airbg.org/terms">`, "Условия за ползване", "Без гаранции"},
		{"/en/terms", "en", "<title>Terms of use and data disclaimer | Kanarche</title>",
			`<link rel="canonical" href="https://airbg.org/en/terms">`, "Terms of use", "No warranty"},
	} {
		rec := fetch(t, rr, tc.path)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", tc.path, rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{`<html lang="` + tc.lang + `">`, tc.title, tc.canonical,
			`<meta name="description"`, `hreflang="bg"`, `hreflang="en"`, `hreflang="x-default"`,
			tc.h1, tc.liability, "/issues"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %q", tc.path, want)
			}
		}
		// The footer brand line sits outside main; the terms copy must not use the retired BG word.
		main := body[:strings.Index(body, "<footer")]
		for _, bad := range []string{"mailto:", "индикативн", "!terms.", "{licences}", "{privacy}", "{about}", "{issues}"} {
			if strings.Contains(main, bad) {
				t.Errorf("%s contains %q", tc.path, bad)
			}
		}
	}
}

func TestSitemapListsTerms(t *testing.T) {
	doc := fetchSitemap(t)
	locs := map[string]bool{}
	for _, u := range doc.URLs {
		locs[u.Loc] = true
	}
	for _, want := range []string{"https://airbg.org/terms", "https://airbg.org/en/terms"} {
		if !locs[want] {
			t.Errorf("sitemap is missing %q", want)
		}
	}
}

func TestAreaSummaryShowsIndicativeHint(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))
	for path, want := range map[string]string{"/area/sofiya": areaHintBG, "/en/area/sofiya": areaHintEN} {
		body := fetch(t, rr, path).Body.String()
		start := strings.Index(body, `class="area-summary"`)
		if start < 0 {
			t.Fatalf("%s has no area summary", path)
		}
		end := strings.Index(body[start:], "</div>")
		if end < 0 || !strings.Contains(body[start:start+end], want) {
			t.Errorf("%s area summary lacks %q", path, want)
		}
	}
}

// The About note names the two projects it is not tied to and links the terms.
func TestAboutNonAffiliationNote(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path, want := range map[string]string{
		"/about":    "Канарче е независим проект. Не е свързано със Sensor.Community или с airbg.info (Код България).",
		"/en/about": "Kanarche is an independent project. It is not affiliated with Sensor.Community or with airbg.info (Код България).",
	} {
		body := fetch(t, rr, path).Body.String()
		if !strings.Contains(body, want) {
			t.Errorf("%s lacks %q", path, want)
		}
	}
	if !strings.Contains(fetch(t, rr, "/en/about").Body.String(), `href="/en/terms"`) {
		t.Error("/en/about note does not link the terms")
	}
}
