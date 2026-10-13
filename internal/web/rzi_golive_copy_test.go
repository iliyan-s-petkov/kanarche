package web_test

import (
	"html"
	"strings"
	"testing"

	"kanarche.eu/internal/i18n"
)

var rziCopyPages = []struct{ lang, about, licences string }{
	{"bg", "/about-the-data", "/licences"},
	{"en", "/en/about-the-data", "/en/licences"},
}

func metaDescription(body string) string {
	const open = `<meta name="description" content="`
	i := strings.Index(body, open)
	if i < 0 {
		return ""
	}
	rest := body[i+len(open):]
	return html.UnescapeString(rest[:strings.Index(rest, `"`)])
}

// Flag off: the about paragraph and the licences meta description are the
// existing keys and nothing else.
func TestRZIGoLiveCopyOffKeepsTodaysText(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := rziRenderer(t, false)
	for _, p := range rziCopyPages {
		about := fetch(t, rr, p.about).Body.String()
		if want := "<p>" + html.EscapeString(cat.T(p.lang, "about.sources.body")) + "</p>"; !strings.Contains(about, want) {
			t.Errorf("%s: sources paragraph is not exactly the existing body", p.about)
		}
		if strings.Contains(about, html.EscapeString(cat.T(p.lang, "about.sources.rzi"))) {
			t.Errorf("%s: RZI sentence shown while disabled", p.about)
		}
		lic := fetch(t, rr, p.licences).Body.String()
		if got, want := metaDescription(lic), cat.T(p.lang, "seo.licences.description"); got != want {
			t.Errorf("%s: description = %q, want %q", p.licences, got, want)
		}
	}
}

// Flag on: the RHI sentence follows the body in the same paragraph and the
// licences description is the RHI version.
func TestRZIGoLiveCopyOnAddsRHI(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := rziRenderer(t, true)
	for _, p := range rziCopyPages {
		about := fetch(t, rr, p.about).Body.String()
		want := "<p>" + html.EscapeString(cat.T(p.lang, "about.sources.body")) + " " + html.EscapeString(cat.T(p.lang, "about.sources.rzi")) + "</p>"
		if !strings.Contains(about, want) {
			t.Errorf("%s: sources paragraph lacks the RHI sentence", p.about)
		}
		lic := fetch(t, rr, p.licences).Body.String()
		got := metaDescription(lic)
		if want := cat.T(p.lang, "seo.licences.description_rzi"); got != want || got == "" {
			t.Errorf("%s: description = %q, want %q", p.licences, got, want)
		}
	}
	if !strings.Contains(cat.T("en", "about.sources.rzi"), "RHI") || !strings.Contains(cat.T("bg", "about.sources.rzi"), "РЗИ") {
		t.Error("sentence does not name the RHI")
	}
}
