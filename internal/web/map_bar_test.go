package web_test

import (
	"os"
	"strings"
	"testing"
)

// The bottom bar is server-rendered around the freshness overlay so its
// position is known at first paint; the legend joins it from JS.
func TestMapBarWrapsTheFreshnessOverlay(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	for _, path := range []string{"/en/area/high", "/en/"} {
		body := fetch(t, rr, path).Body.String()
		if !strings.Contains(body, `<div class="map-bar"><div class="map-freshness">`) {
			t.Errorf("%s: the freshness overlay is not inside .map-bar", path)
		}
	}
}

// The bar height is one token: the embed footline and the landscape legend caps
// read --map-bar-h, so a taller bar moves all of them.
func TestMapBarHeightIsOneToken(t *testing.T) {
	css, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatalf("ReadFile app.css error = %v", err)
	}
	if n := strings.Count(string(css), "var(--map-bar-h)"); n < 3 {
		t.Errorf("app.css reads --map-bar-h %d times, want at least 3 (embed footline, two landscape caps)", n)
	}
	if strings.Contains(string(css), "52px") {
		t.Error("app.css still hard-codes the 52px bar height; use var(--map-bar-h)")
	}
	if strings.Contains(string(css), "28px - 128px") {
		t.Error("the landscape legend cap still folds the bar into a 128px literal")
	}
	for _, p := range []string{"static/theme.css", "../../web/src/styles/theme.css"} {
		theme, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("ReadFile %s error = %v", p, err)
		}
		if !strings.Contains(string(theme), "--map-bar-h:") {
			t.Errorf("%s does not define --map-bar-h", p)
		}
	}
}

// The rail carries reset view, not a table link: the pages hand the map no table target.
func TestMapRailHasNoTableLink(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	for _, path := range []string{"/en/area/high", "/en/"} {
		body := fetch(t, rr, path).Body.String()
		if strings.Contains(body, "data-table-href") || strings.Contains(body, "data-t-table-link") {
			t.Errorf("%s: the map still gets a table link for the rail", path)
		}
		if !strings.Contains(body, `data-t-zoom-reset="`) {
			t.Errorf("%s: the reset view label is missing", path)
		}
	}
}
