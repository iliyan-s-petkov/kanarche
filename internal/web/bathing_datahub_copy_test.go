package web_test

import (
	"strings"
	"testing"

	"kanarche.eu/internal/i18n"
)

const datahubItemURL = "https://www.eea.europa.eu/en/datahub/datahubitem-view/c3858959-90da-4c1b-b9ca-492db0e514df"

var datahubCopy = map[string]map[string]string{
	"en": {
		"sea.class_source_datahub":     "From the EEA {year} report",
		"sea.samples_pending":          "Lab samples for {season} are not published yet.",
		"sea.supplement_note":          "EEA bathing water status dataset, edition {edition}, published {published}.",
		"sea.info_link_datahub":        "Bathing Water Directive - Status of bathing water, 2025 v.1.0",
		"licences.eea_bathing_datahub": "Bathing Water Directive - Status of bathing water, 2025 v.1.0. European Environment Agency. CC BY 4.0.",
	},
	"bg": {
		"sea.class_source_datahub":     "По доклада на ЕАОС за {year}",
		"sea.samples_pending":          "Лабораторните проби за {season} още не са публикувани.",
		"sea.supplement_note":          "Набор данни на ЕАОС за състоянието на водите за къпане, издание {edition}, публикуван на {published}.",
		"sea.info_link_datahub":        "Bathing Water Directive - Status of bathing water, 2025 v.1.0",
		"licences.eea_bathing_datahub": "Bathing Water Directive - Status of bathing water, 2025 v.1.0. Европейска агенция по околна среда. CC BY 4.0.",
	},
}

// The approved Datahub copy exists in both locales, word for word, with no semicolon or en/em dash.
func TestDatahubCopyKeys(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	for lang, keys := range datahubCopy {
		for k, want := range keys {
			if got := cat.T(lang, k); got != want {
				t.Errorf("%s %s = %q, want %q", lang, k, got, want)
			}
			if strings.ContainsAny(want, ";–—") {
				t.Errorf("%s %s has a semicolon or dash", lang, k)
			}
		}
	}
}

// Every page that mounts the sea island hands the new copy to it.
func TestMapPagesCarryDatahubSeaCopy(t *testing.T) {
	rr := renderer(t, fixture(t))
	attrs := []string{
		"data-t-sea-class-source-datahub=", "data-t-sea-samples-pending=",
		"data-t-sea-supplement-note=", "data-t-sea-info-link-datahub=",
	}
	for _, path := range []string{"/en/", "/", "/embed?area=plovdiv-oblast", "/en/area/sofia"} {
		body := fetch(t, rr, path).Body.String()
		for _, a := range attrs {
			if !strings.Contains(body, a) {
				t.Errorf("%s lacks %s", path, a)
			}
		}
	}
}

// The Datahub dataset is a second EEA entry on /licences, under the same CC BY 4.0 credit.
func TestLicencesListsDatahubDataset(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path, lang := range map[string]string{"/licences": "bg", "/en/licences": "en"} {
		body := fetch(t, rr, path).Body.String()
		if !strings.Contains(body, `href="`+datahubItemURL+`"`) {
			t.Errorf("%s lacks the Datahub item link", path)
		}
		if !strings.Contains(body, datahubCopy[lang]["licences.eea_bathing_datahub"]) {
			t.Errorf("%s lacks the Datahub licence entry", path)
		}
	}
}
