package web

import (
	"os"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/i18n"
)

type healthCase struct {
	name, key         string
	value             float64
	enGeneral, enSens string
	bgGeneral, bgSens string
}

var healthCases = []healthCase{
	{"good", "good", 3,
		"The air quality is good. Enjoy your usual outdoor activities.",
		"The air quality is good. Enjoy your usual outdoor activities.",
		"Качеството на въздуха е добро. Можете да се занимавате с обичайните си дейности на открито.",
		"Качеството на въздуха е добро. Можете да се занимавате с обичайните си дейности на открито."},
	{"fair", "fair", 10,
		"Enjoy your usual outdoor activities.",
		"Enjoy your usual outdoor activities.",
		"Можете да се занимавате с обичайните си дейности на открито.",
		"Можете да се занимавате с обичайните си дейности на открито."},
	{"moderate", "moderate", 30,
		"Enjoy your usual outdoor activities.",
		"Consider reducing intense outdoor activities, if you experience symptoms.",
		"Можете да се занимавате с обичайните си дейности на открито.",
		"Ако усещате симптоми, помислете дали да не намалите интензивните дейности на открито."},
	{"poor", "poor", 70,
		"Consider reducing intense activities outdoors, if you experience symptoms such as sore eyes, a cough or sore throat.",
		"Consider reducing physical activities, particularly outdoors, especially if you experience symptoms.",
		"Ако усещате симптоми като парене в очите, кашлица или възпалено гърло, помислете дали да не намалите интензивните дейности на открито.",
		"Помислете дали да не намалите физическите натоварвания, особено на открито, най-вече ако усещате симптоми."},
	{"very poor", "very_poor", 100,
		"Consider reducing intense activities outdoors, if you experience symptoms such as sore eyes, a cough or sore throat.",
		"Reduce physical activities, particularly outdoors, especially if you experience symptoms.",
		"Ако усещате симптоми като парене в очите, кашлица или възпалено гърло, помислете дали да не намалите интензивните дейности на открито.",
		"Намалете физическите натоварвания, особено на открито, най-вече ако усещате симптоми."},
	{"extremely poor", "extremely_poor", 200,
		"Reduce physical activities outdoors.",
		"Avoid physical activities outdoors.",
		"Намалете физическите натоварвания на открито.",
		"Избягвайте физическите натоварвания на открито."},
}

func healthRow(v float64) AreaRow { return AreaRow{Covered: true, HasValue: true, Value: v} }

func TestAreaHealthPerBand(t *testing.T) {
	rr := acRenderer(t)
	t0 := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	heads := map[string]string{
		"en": "What this means for outdoor activity",
		"bg": "Какво означава това за дейностите на открито",
	}
	for _, c := range healthCases {
		for _, lang := range []string{"en", "bg"} {
			p := acPageData(rr, lang, t0, t0)
			got := string(p.areaHealthHTML(healthRow(c.value)))
			gen, sens := c.enGeneral, c.enSens
			if lang == "bg" {
				gen, sens = c.bgGeneral, c.bgSens
			}
			for _, want := range []string{heads[lang], gen, sens, `href="https://airindex.eea.europa.eu/AQI/index.html"`, `rel="noopener noreferrer"`} {
				if !strings.Contains(got, want) {
					t.Errorf("%s %s: missing %q in %q", c.name, lang, want, got)
				}
			}
		}
	}
}

func TestAreaHealthGate(t *testing.T) {
	rr := acRenderer(t)
	t0 := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	fresh := acPageData(rr, "en", t0, t0)
	stale := acPageData(rr, "en", t0, t0.Add(31*time.Minute))
	noScale := acPageData(rr, "en", t0, t0)
	noScale.DefaultMetric = "no_such_metric"

	cases := []struct {
		name string
		p    PageData
		row  AreaRow
	}{
		{"stale", stale, healthRow(30)},
		{"uncovered", fresh, AreaRow{Covered: false, HasValue: true, Value: 30}},
		{"no value", fresh, AreaRow{Covered: true, HasValue: false}},
		{"unknown scale", noScale, healthRow(30)},
	}
	for _, c := range cases {
		if got := c.p.areaHealthHTML(c.row); got != "" {
			t.Errorf("%s: got %q, want empty", c.name, got)
		}
	}
	if got := fresh.areaHealthHTML(healthRow(30)); got == "" {
		t.Error("fresh covered row rendered nothing")
	}
}

func TestAreaHealthKeysAndCopyRules(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{"health.heading", "health.general", "health.sensitive", "health.source", "health.link"}
	for _, c := range healthCases {
		keys = append(keys, "health."+c.key+".general", "health."+c.key+".sensitive")
	}
	for _, k := range keys {
		for _, lang := range cat.Languages() {
			if !cat.Has(lang, k) {
				t.Errorf("%s missing from %s", k, lang)
				continue
			}
			v := cat.T(lang, k)
			if strings.ContainsAny(v, ";—–") {
				t.Errorf("%s/%s has a semicolon or dash: %q", lang, k, v)
			}
		}
	}
}

func TestAreaTemplateRendersHealth(t *testing.T) {
	src, err := os.ReadFile("templates/area.gohtml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), ".AreaHealthHTML") {
		t.Error("area.gohtml does not render .AreaHealthHTML")
	}
}
