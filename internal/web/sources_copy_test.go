package web_test

import (
	"strings"
	"testing"

	"airbg.org/internal/i18n"
)

const (
	camsChart     = "https://atmosphere.copernicus.eu/charts/packages/cams_air_quality/products/europe-air-quality-forecast-pollens"
	climateAdapt  = "https://climate-adapt.eea.europa.eu/en/observatory/publications-data/analysis-data/cams-ground-level-pollen-forecast"
	openMeteoDocs = "https://open-meteo.com/en/docs/air-quality-api"
	eeaBathingMap = "https://www.eea.europa.eu/en/analysis/maps-and-charts/state-of-bathing-waters-in-2025"
	eeaBathingTop = "https://www.eea.europa.eu/en/topics/in-depth/bathing-water"
)

var sourcesKeys = []string{
	"pollen.info.label", "pollen.info.title", "pollen.info.body", "pollen.info.hourly",
	"pollen.info.link_thresholds", "pollen.info.link_chart",
	"sea.info.label", "sea.info.title", "sea.info.body", "sea.info.link_map", "sea.info.link_eea",
	"footer.src.pollen",
	"about.pollen.heading", "about.pollen.body", "about.bathing.heading", "about.bathing.body",
}

// Every new key exists in both languages and has no semicolon or dash.
func TestSourcesCopyKeys(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"bg", "en"} {
		for _, k := range sourcesKeys {
			if !cat.Has(lang, k) {
				t.Errorf("%s lacks %s", lang, k)
				continue
			}
			if strings.ContainsAny(cat.T(lang, k), ";–—") {
				t.Errorf("%s %s has a semicolon or dash", lang, k)
			}
		}
	}
}

func TestFooterListsPollenSource(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path, label := range map[string]string{"/en/": "CAMS pollen forecast", "/": "Прогноза за прашец CAMS"} {
		foot := footerOf(t, fetch(t, rr, path).Body.String())
		if !strings.Contains(foot, `href="`+camsChart+`"`) || !strings.Contains(foot, label) {
			t.Errorf("%s footer lacks the CAMS pollen entry %q", path, label)
		}
	}
}

func TestAboutDataPollenAndBathingSections(t *testing.T) {
	rr := renderer(t, fixture(t))
	type want struct{ heading, pollenBody, bathingBody, sources string }
	cases := map[string]want{
		"/en/about-the-data": {"Pollen is a model forecast, not a count", "We do not measure pollen.", "Bathing sites and classes come from the EEA WISE", "Pollen forecast from CAMS (Copernicus Atmosphere Monitoring Service) via Open-Meteo.com (CC BY 4.0)."},
		"/about-the-data":    {"Прашецът е прогноза от модел, а не преброяване", "Не измерваме прашец.", "Местата за къпане и техните класове идват", "Прогноза за прашец от CAMS"},
	}
	for path, w := range cases {
		body := fetch(t, rr, path).Body.String()
		for _, s := range []string{w.heading, w.pollenBody, w.bathingBody, w.sources, climateAdapt, openMeteoDocs, camsChart, eeaBathingMap, eeaBathingTop} {
			if !strings.Contains(body, s) {
				t.Errorf("%s lacks %q", path, s)
			}
		}
	}
}

// html/template URL-escapes any attribute whose name contains "url", and "hourly" does,
// so the daily-mean note rides on data-t-pollen-info-mean. It must arrive as plain text.
func TestPollenMeanNoteIsNotURLEscaped(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := renderer(t, fixture(t))
	for path, lang := range map[string]string{"/": "bg", "/en/": "en"} {
		body := fetch(t, rr, path).Body.String()
		want := `data-t-pollen-info-mean="` + cat.T(lang, "pollen.info.hourly") + `"`
		if !strings.Contains(body, want) {
			t.Errorf("%s does not carry the plain daily-mean note", path)
		}
	}
}

// The map islands read the popup copy from data-t-* attributes on every page that mounts a map.
func TestMapPagesCarryLegendInfoCopy(t *testing.T) {
	rr := renderer(t, fixture(t))
	attrs := []string{
		"data-t-pollen-info-label=", "data-t-pollen-info-title=", "data-t-pollen-info-body=", "data-t-pollen-info-mean=",
		"data-t-pollen-info-link-thresholds=", "data-t-pollen-info-link-chart=",
		"data-t-sea-info-label=", "data-t-sea-info-title=", "data-t-sea-info-body=",
		"data-t-sea-info-link-map=", "data-t-sea-info-link-eea=",
	}
	for _, path := range []string{"/en/", "/embed?area=plovdiv-oblast"} {
		body := fetch(t, rr, path).Body.String()
		for _, a := range attrs {
			if !strings.Contains(body, a) {
				t.Errorf("%s lacks %s", path, a)
			}
		}
	}
}
