package web_test

import (
	"strings"
	"testing"
)

// The browser builds aria-labels too, so it reads the same spoken words from
// the document instead of carrying a second copy.
func TestPagesCarryTheSpokenUnitTable(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, path := range []string{"/", "/en/", "/areas", "/en/area/sofia"} {
		body := fetch(t, rr, path).Body.String()
		if !strings.Contains(body, `<meta name="spoken-units" content="`) {
			t.Errorf("%s: no spoken-units table in the head", path)
		}
	}
	en := fetch(t, rr, "/en/").Body.String()
	if !strings.Contains(en, "micrograms per cubic metre") {
		t.Errorf("/en/ never names the spoken unit")
	}
}

// The home summary is the map page's text, not the areas table's.
func TestHomeSummaryIsOnTheMapPageOnly(t *testing.T) {
	rr := renderer(t, fixture(t))
	if body := fetch(t, rr, "/en/").Body.String(); !strings.Contains(body, `id="home-summary"`) {
		t.Errorf("/en/ has no home summary")
	}
	if body := fetch(t, rr, "/en/areas").Body.String(); strings.Contains(body, `id="home-summary"`) {
		t.Errorf("/en/areas repeats the home summary above its own table")
	}
}
