package web_test

import (
	"strings"
	"testing"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/web"
)

// rziRenderer builds a renderer with sea.rzi.enabled set as given. The page
// URLs are marked so the test can tell they come from the config.
func rziRenderer(t *testing.T, enabled bool) *web.Renderer {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t)
	cfg.Sea.RZI.Enabled = enabled
	cfg.Sea.RZI.Varna.PageURL = "https://rzi-varna.example/results"
	cfg.Sea.RZI.Burgas.PageURL = "https://rzi-burgas.example/results"
	cfg.Sea.RZI.Dobrich.PageURL = "https://rzi-dobrich.example/results"
	h := snapshot.NewHolder(cfg.Series, config.Wind{})
	h.Store(fixture(t))
	rr, err := web.NewRenderer(cat, h, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return rr
}

var rziLangs = map[string]string{"/licences": "bg", "/en/licences": "en"}

func TestLicencesShowsRZIItemWhenEnabled(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := rziRenderer(t, true)
	for path, lang := range rziLangs {
		body := fetch(t, rr, path).Body.String()
		for _, key := range []string{"licences.rzi.name", "licences.rzi.what", "licences.rzi.unofficial", "licences.rzi.derived", "licences.rzi.changes", "sea.source.rzi_varna", "sea.source.rzi_burgas", "sea.source.rzi_dobrich"} {
			if !strings.Contains(body, cat.T(lang, key)) {
				t.Errorf("%s lacks %s", path, key)
			}
		}
		for _, u := range []string{"https://rzi-varna.example/results", "https://rzi-burgas.example/results", "https://rzi-dobrich.example/results"} {
			if !strings.Contains(body, `href="`+u+`"`) {
				t.Errorf("%s lacks link %s", path, u)
			}
		}
		// The item follows the EEA bathing entries.
		if strings.Index(body, cat.T(lang, "licences.rzi.name")) < strings.Index(body, cat.T(lang, "licences.bathing.name")) {
			t.Errorf("%s: RZI item comes before the EEA bathing item", path)
		}
	}
}

func TestLicencesHasNoRZIItemWhenDisabled(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := rziRenderer(t, false)
	for path, lang := range rziLangs {
		body := fetch(t, rr, path).Body.String()
		for _, key := range []string{"licences.rzi.name", "licences.rzi.what", "sea.source.rzi_varna"} {
			if strings.Contains(body, cat.T(lang, key)) {
				t.Errorf("%s shows %s while RZI is disabled", path, key)
			}
		}
		if strings.Contains(body, "rzi-varna.example") {
			t.Errorf("%s links an RZI page while RZI is disabled", path)
		}
	}
}

func TestLicencesIntroHasNoNumber(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"en": "The data comes from several sources, each with its own licence. The Kanarche code is free software.",
		"bg": "Данните идват от няколко източника, всеки със свой лиценз. Кодът на Канарче е свободен.",
	}
	for lang, w := range want {
		if got := cat.T(lang, "licences.intro"); got != w {
			t.Errorf("%s licences.intro = %q, want %q", lang, got, w)
		}
	}
	for path := range rziLangs {
		for _, enabled := range []bool{true, false} {
			body := fetch(t, rziRenderer(t, enabled), path).Body.String()
			if strings.Contains(body, "four sources") || strings.Contains(body, "четири източника") {
				t.Errorf("%s (rzi=%v) still counts the sources", path, enabled)
			}
		}
	}
}

// The legend line rides on data-t-sea-info-rzi: filled when enabled, empty when not.
// The new-tab text is not gated.
func TestMapPagesGateTheRZILegendLine(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{true, false} {
		rr := rziRenderer(t, enabled)
		for path, lang := range map[string]string{"/": "bg", "/en/": "en", "/embed?area=plovdiv-oblast": "bg", "/en/area/sofia": "en"} {
			body := fetch(t, rr, path).Body.String()
			wantRZI := `data-t-sea-info-rzi="` + cat.T(lang, "sea.info.rzi") + `"`
			if enabled && !strings.Contains(body, wantRZI) {
				t.Errorf("%s: enabled but lacks %s", path, wantRZI)
			}
			if !enabled && !strings.Contains(body, `data-t-sea-info-rzi=""`) {
				t.Errorf("%s: disabled but the attribute is not empty", path)
			}
			if !strings.Contains(body, `data-t-sea-new-tab="`+cat.T(lang, "common.new_tab")+`"`) {
				t.Errorf("%s lacks the new-tab text (rzi=%v)", path, enabled)
			}
		}
	}
}
