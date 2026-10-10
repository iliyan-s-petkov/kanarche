package web_test

import (
	"strings"
	"testing"
)

// Every page that carries the masthead opens with a skip link to <main>.
func TestEveryPageOpensWithASkipLink(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))
	bg := `<a class="skip" href="#main">Към съдържанието</a>`
	en := `<a class="skip" href="#main">Skip to content</a>`
	for _, tc := range []struct{ path, link string }{
		{"/", bg},
		{"/en/", en},
		{"/area/sofiya", bg},
		{"/en/area/sofiya", en},
		{"/areas", bg},
		{"/about", bg},
		{"/en/about", en},
		{"/terms", bg},
		{"/en/terms", en},
		{"/privacy", bg},
		{"/licences", bg},
		{"/no-such-page", bg},
	} {
		body := fetch(t, rr, tc.path).Body.String()
		_, afterBody, ok := strings.Cut(body, "<body>")
		if !ok {
			t.Fatalf("%s: no <body>", tc.path)
		}
		if first := strings.Index(afterBody, "<a "); first < 0 || !strings.HasPrefix(afterBody[first:], tc.link) {
			t.Errorf("%s: the first link in <body> is not %s", tc.path, tc.link)
		}
		if !strings.Contains(body, `<main class="page" id="main" tabindex="-1">`) {
			t.Errorf("%s: <main> is not a focusable #main target", tc.path)
		}
	}
}

// The embed has no masthead to skip, so it carries no skip link.
func TestEmbedHasNoSkipLink(t *testing.T) {
	body := framed(t, renderer(t, areaPageFixture(t)), "/embed").Body.String()
	if strings.Contains(body, `class="skip"`) {
		t.Error("the embed carries a skip link with nothing to skip")
	}
}
