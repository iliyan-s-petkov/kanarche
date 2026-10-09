package web_test

import (
	"regexp"
	"strings"
	"testing"
)

const (
	footerRefEN = "Indicative data, with no guarantee. Do not use it for health or safety decisions."
	footerRefBG = "Данните са ориентировъчни и без гаранция. Не ги използвайте за решения за здраве или безопасност."
	aboutRefEN  = "This map is for reference, not medical advice"
	aboutRefBG  = "Картата е за информация, не е медицински съвет"
)

func TestFooterReferenceLine(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path, want := range map[string]string{"/": footerRefBG, "/en/": footerRefEN} {
		body := fetch(t, rr, path).Body.String()
		if !strings.Contains(footerOf(t, body), want) {
			t.Errorf("%s footer lacks %q", path, want)
		}
	}
}

// The reference heading must be the first h2 on the About page.
func TestAboutReferenceSectionFirst(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path, want := range map[string]string{"/about-the-data": aboutRefBG, "/en/about-the-data": aboutRefEN} {
		body := fetch(t, rr, path).Body.String()
		loc := h2Re.FindStringIndex(body)
		if loc == nil || !strings.HasPrefix(body[loc[1]:], want+"</h2>") {
			t.Errorf("%s: first h2 is not %q: %.60q", path, want, h2Re.FindString(body))
		}
	}
}

var h2Re = regexp.MustCompile(`<h2(?: [^>]*)?>`)
