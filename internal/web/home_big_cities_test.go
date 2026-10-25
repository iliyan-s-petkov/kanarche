package web_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

var bigCitySlugs = []string{"sofia", "plovdiv", "varna", "burgas", "ruse", "stara-zagora", "pleven"}

// bigCitiesFixture holds the seven cities and six Sofia districts with
// 10..60 sensors. Areas in noValue have no reading.
func bigCitiesFixture(noValue map[string]bool) *snapshot.Snapshot {
	known := map[string]snapshot.AreaMeta{}
	add := func(slug, kind, parent, bg, en string, sensors int, pm float64) {
		m := snapshot.AreaMeta{Slug: slug, Kind: kind, ParentSlug: parent, NameBG: bg, NameEN: en,
			Covered: true, SensorCount: sensors}
		if !noValue[slug] {
			m.Values = map[string]float64{"P2": pm}
		}
		known[slug] = m
	}
	for i, s := range bigCitySlugs {
		add(s, "city", "", "Град-"+s, "City-"+s, 100+i, 12.5+float64(i))
	}
	for i := 1; i <= 6; i++ {
		add(fmt.Sprintf("d%d", i), "neighbourhood", "sofia", fmt.Sprintf("Район%d", i), fmt.Sprintf("District%d", i), i*10, 20)
	}
	return &snapshot.Snapshot{GeneratedAt: time.Now(), KnownSlugs: known}
}

func TestHomeBigCitiesLinksAndValues(t *testing.T) {
	rr := renderer(t, bigCitiesFixture(nil))
	for _, c := range []struct{ path, pre, heading, name, value string }{
		{"/", "", "Големите градове сега", "Град-", "13,5"},
		{"/en/", "/en", "Big cities now", "City-", "13.5"},
	} {
		sec := section(t, fetch(t, rr, c.path).Body.String(), "home-big-cities")
		if !strings.Contains(sec, ">"+c.heading+"</h2>") {
			t.Errorf("%s: heading %q missing", c.path, c.heading)
		}
		for _, s := range bigCitySlugs {
			if !strings.Contains(sec, `href="`+c.pre+`/area/`+s+`"`) {
				t.Errorf("%s: no link to %s", c.path, s)
			}
		}
		// The four busiest districts are d6..d3, the quiet d2 and d1 are left out.
		for _, d := range []string{"d6", "d5", "d4", "d3"} {
			if !strings.Contains(sec, `href="`+c.pre+`/area/`+d+`"`) {
				t.Errorf("%s: no link to district %s", c.path, d)
			}
		}
		for _, d := range []string{"d2", "d1"} {
			if strings.Contains(sec, `/area/`+d+`"`) {
				t.Errorf("%s: district %s should not be listed", c.path, d)
			}
		}
		if !strings.Contains(sec, c.name+"plovdiv</a>") {
			t.Errorf("%s: city name missing", c.path)
		}
		if !strings.Contains(sec, c.value) || !strings.Contains(sec, "µg/m³") {
			t.Errorf("%s: value %s or unit missing", c.path, c.value)
		}
		if got := strings.Count(sec, "<li>"); got != 11 {
			t.Errorf("%s: %d rows, want 11", c.path, got)
		}
	}
}

func TestHomeBigCitiesNoData(t *testing.T) {
	rr := renderer(t, bigCitiesFixture(map[string]bool{"varna": true}))
	for _, c := range []struct{ path, pre, text string }{
		{"/", "", "Недостатъчно данни"},
		{"/en/", "/en", "Not enough data"},
	} {
		sec := section(t, fetch(t, rr, c.path).Body.String(), "home-big-cities")
		if !strings.Contains(sec, `href="`+c.pre+`/area/varna"`) {
			t.Errorf("%s: link dropped for a city without data", c.path)
		}
		if strings.Count(sec, c.text) != 1 {
			t.Errorf("%s: want exactly one %q", c.path, c.text)
		}
	}
}
