package web_test

import (
	"html"
	"strings"
	"testing"
	"unicode/utf8"

	"kanarche.eu/internal/snapshot"
)

func titleOf(t *testing.T, body string) string {
	t.Helper()
	m := titleRE.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no title in page")
	}
	return html.UnescapeString(m[1])
}

// A district title over 60 runes drops " (AQI)" in BG and " today" in EN, and only that title.
func TestLongDistrictTitleDropsItsSuffix(t *testing.T) {
	snap := districtFixture()
	snap.KnownSlugs["dalga"] = snapshot.AreaMeta{Slug: "dalga", Kind: "neighbourhood", NameBG: "Дългоимеросан", NameEN: "Longnamedistrictxxxx",
		ParentSlug: "testgrad", CentroidLon: 25.1, CentroidLat: 42.1, DefaultZoom: 13, Covered: true, SensorCount: 5,
		Values: map[string]float64{"P2": 10, "P1": 15}, Km2: 9, Source: "sensor.community"}
	rr := renderer(t, snap)
	cases := []struct {
		path, wantTitle, absent string
	}{
		{"/area/dalga", "Дългоимеросан, Тестоград: индекс за качество на въздуха", "(AQI)"},
		{"/en/area/dalga", "Longnamedistrictxxxx, Testgrad: air quality index (AQI)", "today"},
	}
	for _, c := range cases {
		title := titleOf(t, fetch(t, rr, c.path).Body.String())
		if !strings.HasPrefix(title, c.wantTitle) || strings.Contains(title, c.absent) {
			t.Errorf("%s: title %q, want prefix %q without %q", c.path, title, c.wantTitle, c.absent)
		}
		if n := utf8.RuneCountInString(title); n > 60 {
			t.Errorf("%s: title %q is %d runes, want <= 60", c.path, title, n)
		}
	}
	// A short name keeps the full title.
	for path, want := range map[string]string{
		"/area/severen":    "Северен, Тестоград: индекс за качество на въздуха (AQI)",
		"/en/area/severen": "Severen, Testgrad: air quality index (AQI) today",
	} {
		if title := titleOf(t, fetch(t, rr, path).Body.String()); !strings.HasPrefix(title, want) {
			t.Errorf("%s: title %q, want prefix %q", path, title, want)
		}
	}
}

// An uncovered district of another city names its city, so two uncovered Mladosts have different titles.
func TestUncoveredDistrictTitleNamesItsCity(t *testing.T) {
	snap := districtFixture()
	for _, s := range []string{"mladost", "mladost-testgrad"} {
		m := snap.KnownSlugs[s]
		if s == "mladost-testgrad" {
			m = snapshot.AreaMeta{Slug: s, Kind: "neighbourhood", NameBG: "Младост", NameEN: "Mladost", ParentSlug: "testgrad",
				CentroidLon: 25.1, CentroidLat: 42.1, DefaultZoom: 13, SensorCount: 1, Km2: 9, Source: "sensor.community"}
		}
		m.Covered = false
		m.Values = nil
		snap.KnownSlugs[s] = m
	}
	rr := renderer(t, snap)
	for _, lang := range []string{"", "/en"} {
		sofia := titleOf(t, fetch(t, rr, lang+"/area/mladost").Body.String())
		other := titleOf(t, fetch(t, rr, lang+"/area/mladost-testgrad").Body.String())
		if sofia == other {
			t.Errorf("%q: both uncovered Mladost pages share the title %q", lang, sofia)
		}
		if !strings.Contains(other, "Testgrad") && !strings.Contains(other, "Тестоград") {
			t.Errorf("%q: uncovered title %q does not name its city", lang, other)
		}
		if strings.Contains(sofia, "Testgrad") || strings.Contains(sofia, "Тестоград") {
			t.Errorf("%q: Sofia title %q names another city", lang, sofia)
		}
	}
}
