package web_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// A district page links every other district, not just the nearest few, under the siblings heading.
func TestDistrictPageLinksAllSiblingDistricts(t *testing.T) {
	snap := guardFixture(t)
	for i := 0; i < 10; i++ {
		slug := fmt.Sprintf("district-%d", i)
		m := snap.KnownSlugs["mladost"]
		m.Slug, m.NameBG, m.NameEN = slug, "Район "+slug, "District "+slug
		m.CentroidLon += float64(i) * 0.01
		snap.KnownSlugs[slug] = m
	}
	var districts []string
	for slug, m := range snap.KnownSlugs {
		if m.Kind == "neighbourhood" && slug != "mladost" {
			districts = append(districts, slug)
		}
	}
	if len(districts) < 10 {
		t.Fatalf("fixture has %d sibling districts, want at least 10", len(districts))
	}

	rr := renderer(t, snap)
	for prefix, heading := range map[string]string{"": "Други райони", "/en": "Other districts"} {
		body := fetch(t, rr, prefix+"/area/mladost").Body.String()
		for _, slug := range districts {
			if !regexp.MustCompile(`href="` + prefix + `/area/` + slug + `"`).MatchString(body) {
				t.Errorf("%s/area/mladost does not link %s", prefix, slug)
			}
		}
		if !strings.Contains(body, "<h2>"+heading+"</h2>") {
			t.Errorf("%s/area/mladost: want heading %q", prefix, heading)
		}
		if strings.Contains(body, "Най-близки райони") || strings.Contains(body, "Nearest districts") {
			t.Errorf("%s/area/mladost still has the nearest-districts heading", prefix)
		}
	}
}

// Other kinds keep their nearest lists and headings.
func TestNonDistrictPagesKeepNearestHeading(t *testing.T) {
	rr := renderer(t, guardFixture(t))
	body := fetch(t, rr, "/en/area/varna").Body.String()
	if !strings.Contains(body, "<h2>Nearest provincial capitals</h2>") {
		t.Error("city page lost its nearest heading")
	}
}
