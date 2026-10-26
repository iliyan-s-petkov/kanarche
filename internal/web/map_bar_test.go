package web_test

import (
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
