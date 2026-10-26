package web_test

import (
	"os"
	"regexp"
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

// The bar height is one token: the attribution lift, the embed footline and the
// landscape legend cap all read --map-bar-h, so a taller bar moves all three.
func TestMapBarHeightIsOneToken(t *testing.T) {
	css, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatalf("ReadFile app.css error = %v", err)
	}
	if n := strings.Count(string(css), "var(--map-bar-h)"); n < 4 {
		t.Errorf("app.css reads --map-bar-h %d times, want at least 4 (attribution, embed footline, two landscape caps)", n)
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

// The rail's table link: the area page jumps to its own sensors section,
// the home page to the areas table.
func TestMapTableLinkTargets(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	cases := map[string]string{"/en/area/high": "#sensors-h", "/en/": "/en/areas"}
	for path, want := range cases {
		body := fetch(t, rr, path).Body.String()
		m := regexp.MustCompile(`data-table-href="([^"]*)"`).FindStringSubmatch(body)
		if m == nil || m[1] != want {
			t.Errorf("%s: data-table-href = %v, want %q", path, m, want)
		}
		if !strings.Contains(body, `data-t-table-link="Data table"`) {
			t.Errorf("%s: the table link has no label", path)
		}
	}
}
