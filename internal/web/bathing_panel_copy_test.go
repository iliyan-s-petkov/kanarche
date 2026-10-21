package web_test

import (
	"strings"
	"testing"

	"kanarche.eu/internal/i18n"
)

// The copy the redesigned bathing-water card and details dialog read. Each key reaches the island as an attribute.
var panelCopyKeys = map[string]string{
	"sea.initial.excellent":  "data-t-sea-initial-excellent=",
	"sea.initial.good":       "data-t-sea-initial-good=",
	"sea.initial.sufficient": "data-t-sea-initial-sufficient=",
	"sea.initial.poor":       "data-t-sea-initial-poor=",
	"sea.official_class":     "data-t-sea-official-class=",
	"sea.chips.heading":      "data-t-sea-chips-heading=",
	"sea.chips.key_within":   "data-t-sea-chips-key-within=",
	"sea.chips.none":         "data-t-sea-chips-none=",
	"sea.chips.info_label":   "data-t-sea-chips-info-label=",
	"sea.seasons_since":      "data-t-sea-seasons-since=",
	"sea.details":            "data-t-sea-details=",
	"sea.back":               "data-t-sea-back=",
	"sea.tabs.label":         "data-t-sea-tabs-label=",
	"sea.tabs.samples":       "data-t-sea-tabs-samples=",
	"sea.tabs.seasons":       "data-t-sea-tabs-seasons=",
	"sea.tabs.about":         "data-t-sea-tabs-about=",
	"sea.metric.label":       "data-t-sea-metric-label=",
	"sea.season_picker":      "data-t-sea-season-picker=",
	"sea.chart.summary":      "data-t-sea-chart-summary=",
	"sea.chart.caption":      "data-t-sea-chart-caption=",
	"sea.chart.show_table":   "data-t-sea-chart-show-table=",
	"sea.chart.show_chart":   "data-t-sea-chart-show-chart=",
	"sea.chart.band_within":  "data-t-sea-chart-band-within=",
	"sea.chart.band_between": "data-t-sea-chart-band-between=",
	"sea.chart.band_above":   "data-t-sea-chart-band-above=",
	"sea.chart.hollow":       "data-t-sea-chart-hollow=",
	"sea.credit.source":      "data-t-sea-credit-source=",
	"sea.rzi.remark.label":   "data-t-sea-remark-label=",
	"sea.rzi.remark.credit":  "data-t-sea-remark-credit=",
	"sea.sources.eea":        "data-t-sea-sources-eea=",
	"sea.sources.info_label": "data-t-sea-sources-info-label=",
}

// Every page that mounts the sea island hands the panel copy to it, and the copy is non-empty in both languages.
func TestMapPagesCarryPanelCopy(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := renderer(t, fixture(t))
	for _, path := range []string{"/en/", "/", "/embed?area=plovdiv-oblast", "/en/area/sofia"} {
		body := fetch(t, rr, path).Body.String()
		for key, attr := range panelCopyKeys {
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

// The four class initials differ within a language, so the letter can tell the classes apart.
func TestClassInitialsAreDistinct(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"en", "bg"} {
		seen := map[string]string{}
		for _, class := range []string{"excellent", "good", "sufficient", "poor"} {
			key := "sea.initial." + class
			v := cat.T(lang, key)
			if other, dup := seen[v]; dup {
				t.Errorf("%s: %s and %s share the initial %q", lang, key, other, v)
			}
			seen[v] = key
		}
	}
}
