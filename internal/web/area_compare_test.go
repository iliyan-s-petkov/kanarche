package web_test

import (
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

// compareFixture is the area page roster with a fresh snapshot and a 24h mean on every covered area.
// silistra is an uncovered city that carries a mean too, so only the gate can keep its block off.
func compareFixture(t *testing.T, generatedAt time.Time) *snapshot.Snapshot {
	t.Helper()
	snap := areaPageFixture(t)
	snap.GeneratedAt = generatedAt
	for slug, meta := range snap.KnownSlugs {
		meta.Day24Mean = map[string]float64{"P2": 18.26, "P1": 31.04}
		snap.KnownSlugs[slug] = meta
	}
	return snap
}

var compareBG = []string{
	"Средната стойност за последните 24 часа е 18,3 µg/m³ за ФПЧ2.5 и 31,0 µg/m³ за ФПЧ10.",
	"Насоката на СЗО от 2021 г. за 24 часа е 15 µg/m³ за ФПЧ2.5 и 45 µg/m³ за ФПЧ10.",
	"Дневната норма на ЕС по Директива (ЕС) 2024/2881 се прилага от 2030 г. и е 25 µg/m³ за ФПЧ2.5 и 45 µg/m³ за ФПЧ10.",
	"Сравнението е ориентировъчно. Гражданските сензори не са калибрирани. Нормите се проверяват с официални измервания.",
}

var compareEN = []string{
	"The average for the last 24 hours is 18.3 µg/m³ for PM2.5 and 31.0 µg/m³ for PM10.",
	"The WHO 2021 guideline for 24 hours is 15 µg/m³ for PM2.5 and 45 µg/m³ for PM10.",
	"The EU daily limit in Directive (EU) 2024/2881 applies from 2030 and is 25 µg/m³ for PM2.5 and 45 µg/m³ for PM10.",
	"This comparison is indicative. Citizen sensors are not calibrated. Limits are checked with official measurements.",
}

func TestCityPageRendersLimitComparison(t *testing.T) {
	rr := renderer(t, compareFixture(t, time.Now()))
	for path, want := range map[string][]string{"/area/plovdiv": compareBG, "/en/area/plovdiv": compareEN} {
		body := fetch(t, rr, path).Body.String()
		for _, s := range want {
			if !strings.Contains(body, s) {
				t.Errorf("%s: missing %q", path, s)
			}
		}
	}
}

func TestComparisonOnlyOnCityPages(t *testing.T) {
	rr := renderer(t, compareFixture(t, time.Now()))
	for _, path := range []string{"/area/mladost", "/area/plovdiv-oblast", "/en/area/mladost", "/en/area/plovdiv-oblast"} {
		body := fetch(t, rr, path).Body.String()
		for _, s := range append(append([]string{}, compareBG...), compareEN...) {
			if strings.Contains(body, s) {
				t.Errorf("%s: renders comparison sentence %q", path, s)
			}
		}
		if strings.Contains(body, "area-season") {
			t.Errorf("%s: renders the season paragraph", path)
		}
	}
}

func TestComparisonNeedsFreshCoveredMean(t *testing.T) {
	fresh := compareFixture(t, time.Now())
	// No P10 mean means 24h coverage was too thin, so the whole block goes.
	thin := fresh.KnownSlugs["plovdiv"]
	thin.Day24Mean = map[string]float64{"P2": 18.26}
	fresh.KnownSlugs["plovdiv"] = thin

	cases := []struct {
		name string
		rr   func() string
	}{
		{"stale", func() string {
			return fetch(t, renderer(t, compareFixture(t, time.Now().Add(-3*time.Hour))), "/area/plovdiv").Body.String()
		}},
		{"uncovered", func() string { return fetch(t, renderer(t, fresh), "/area/silistra").Body.String() }},
		{"thin mean", func() string { return fetch(t, renderer(t, fresh), "/area/plovdiv").Body.String() }},
	}
	for _, c := range cases {
		body := c.rr()
		if strings.Contains(body, "area-compare") {
			t.Errorf("%s: renders the comparison block", c.name)
		}
		if !strings.Contains(body, "area-season") {
			t.Errorf("%s: season paragraph is missing", c.name)
		}
	}
}

func TestSeasonParagraphLink(t *testing.T) {
	rr := renderer(t, compareFixture(t, time.Now()))
	cases := []struct{ path, sentence, href, text string }{
		{"/area/plovdiv", "През зимата основен източник на ФПЧ е отоплението с дърва и въглища.", `href="/air-pollution"`, "Замърсяване на въздуха: причини, ФПЧ и AQI"},
		{"/en/area/plovdiv", "In winter the main source of particulate matter is home heating with wood and coal.", `href="/en/air-pollution"`, "Air pollution: causes, PM and AQI"},
	}
	for _, c := range cases {
		body := fetch(t, rr, c.path).Body.String()
		i := strings.Index(body, `class="area-season"`)
		if i < 0 {
			t.Fatalf("%s: no season paragraph", c.path)
		}
		block := body[i : i+strings.Index(body[i:], "</p>")]
		for _, want := range []string{c.sentence, c.href, ">" + c.text + "</a>"} {
			if !strings.Contains(block, want) {
				t.Errorf("%s: season paragraph missing %q in %q", c.path, want, block)
			}
		}
		if strings.Contains(block, "{air_link}") {
			t.Errorf("%s: placeholder left in %q", c.path, block)
		}
	}
}
