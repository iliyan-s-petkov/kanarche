package web_test

import (
	"regexp"
	"strings"
	"testing"
)

var (
	scrollWrapRE = regexp.MustCompile(`<div class="area-sensors__scroll"([^>]*)>`)
	labelledByRE = regexp.MustCompile(`aria-labelledby="([^"]+)"`)
)

// Both horizontal scroll wrappers must be keyboard focusable and named, or axe reports scrollable-region-focusable.
func TestAreaScrollWrappersAreFocusableNamedRegions(t *testing.T) {
	snap := stationListFixture(t)
	off := officialSnapshot()
	m := snap.KnownSlugs["sofia"]
	m.Stations = []string{"BG0038A"}
	snap.KnownSlugs["sofia"] = m
	snap.StationReadings = off.StationReadings
	body := fetch(t, renderer(t, snap), "/area/sofia").Body.String()
	wraps := scrollWrapRE.FindAllStringSubmatch(body, -1)
	if len(wraps) != 2 {
		t.Fatalf("scroll wrappers = %d, want 2 (official and static)", len(wraps))
	}
	for _, w := range wraps {
		attrs := w[1]
		if !strings.Contains(attrs, `tabindex="0"`) || !strings.Contains(attrs, `role="region"`) {
			t.Errorf("wrapper %q lacks tabindex=0 or role=region", attrs)
		}
		m := labelledByRE.FindStringSubmatch(attrs)
		if m == nil {
			t.Errorf("wrapper %q has no aria-labelledby", attrs)
			continue
		}
		if n := strings.Count(body, `id="`+m[1]+`"`); n != 1 {
			t.Errorf("aria-labelledby %q matches %d ids, want 1", m[1], n)
		}
	}
}
