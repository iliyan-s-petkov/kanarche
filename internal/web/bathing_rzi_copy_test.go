package web_test

import (
	"strings"
	"testing"

	"kanarche.eu/internal/i18n"
)

var rziCopyKeys = map[string]string{
	"sea.latest.heading":            "data-t-sea-latest-heading=",
	"sea.latest.single_sample_note": "data-t-sea-latest-single-sample-note=",
	"sea.latest.stale":              "data-t-sea-latest-stale=",
	"sea.latest.off_season":         "data-t-sea-latest-off-season=",
	"sea.credit.rzi":                "data-t-sea-credit-rzi=",
	"sea.notice.unofficial":         "data-t-sea-notice-unofficial=",
	"sea.source.rzi_varna":          "data-t-sea-source-rzi-varna=",
	"sea.source.rzi_burgas":         "data-t-sea-source-rzi-burgas=",
	"sea.source.rzi_dobrich":        "data-t-sea-source-rzi-dobrich=",
	"sea.source.link":               "data-t-sea-source-link=",
	"sea.unit.mpn":                  "data-t-sea-unit-mpn=",
}

// The RZI credit keeps the approved wording, built from a region and a date.
func TestRZICreditTemplate(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"en": "Source: {region}, sample of {date}",
		"bg": "Източник: {region}, проба от {date}",
	}
	for lang, w := range want {
		if got := cat.T(lang, "sea.credit.rzi"); got != w {
			t.Errorf("%s sea.credit.rzi = %q, want %q", lang, got, w)
		}
	}
}

// Every page that mounts the sea island hands the RZI copy to it, and the copy is non-empty.
func TestMapPagesCarryRZISeaCopy(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := renderer(t, fixture(t))
	for _, path := range []string{"/en/", "/", "/embed?area=plovdiv-oblast", "/en/area/sofia"} {
		body := fetch(t, rr, path).Body.String()
		for key, attr := range rziCopyKeys {
			if !strings.Contains(body, attr) {
				t.Errorf("%s lacks %s", path, attr)
			}
			for _, lang := range []string{"en", "bg"} {
				if strings.TrimSpace(cat.T(lang, key)) == "" {
					t.Errorf("%s %s is empty", lang, key)
				}
			}
		}
	}
}
