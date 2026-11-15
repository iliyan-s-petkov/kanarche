package web_test

import (
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

// whoDaysFixture is the area page roster with a year summary on every area.
func whoDaysFixture(t *testing.T, w snapshot.WHODays) *snapshot.Snapshot {
	t.Helper()
	snap := areaPageFixture(t)
	for slug, meta := range snap.KnownSlugs {
		copied := w
		meta.WHODays = &copied
		snap.KnownSlugs[slug] = meta
	}
	return snap
}

func whoDays(withData, over int) snapshot.WHODays {
	return snapshot.WHODays{
		DaysWithData: withData, DaysOver: over,
		WorstDay: time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC), WorstMean: 41.26,
	}
}

// whoDaysBlock returns the text of the area-who-days paragraph, or "" when the page has none.
func whoDaysBlock(body string) string {
	i := strings.Index(body, `class="area-who-days">`)
	if i < 0 {
		return ""
	}
	rest := body[i+len(`class="area-who-days">`):]
	return rest[:strings.Index(rest, "</p>")]
}

func TestWHODaysPicksKeyByCount(t *testing.T) {
	cases := []struct {
		name         string
		over         int
		bg, en       string
		notBG, notEN string
	}{
		{
			"many", 12,
			"От 100 дни с данни през последната година 12 дни в Пловдив са над препоръчителната стойност на СЗО за ФПЧ2.5, която е 15 µg/m³ за 24 часа. Най-лошият ден е 14 януари 2026 г. със средно 41,3 µg/m³.",
			"Of 100 days with data in the past year, 12 days in Plovdiv were above the WHO guideline for PM2.5 of 15 µg/m³ over 24 hours. The worst day was 14 January 2026, averaging 41.3 µg/m³.",
			"1 ден", "1 day in",
		},
		{
			"one", 1,
			"От 100 дни с данни през последната година 1 ден в Пловдив е над препоръчителната стойност на СЗО за ФПЧ2.5, която е 15 µg/m³ за 24 часа. Най-лошият ден е 14 януари 2026 г. със средно 41,3 µg/m³.",
			"Of 100 days with data in the past year, 1 day in Plovdiv was above the WHO guideline for PM2.5 of 15 µg/m³ over 24 hours. The worst day was 14 January 2026, averaging 41.3 µg/m³.",
			"1 дни", "1 days",
		},
		{
			"zero", 0,
			"През нито един от 100 дни с данни в Пловдив не е имало дневна средна над препоръчителната стойност на СЗО за ФПЧ2.5, която е 15 µg/m³ за 24 часа. Най-високата дневна средна е 41,3 µg/m³ на 14 януари 2026 г.",
			"On none of the 100 days with data in Plovdiv was the daily average above the WHO guideline for PM2.5 of 15 µg/m³ over 24 hours. The highest daily average was 41.3 µg/m³ on 14 January 2026.",
			"са над", "were above",
		},
	}
	for _, c := range cases {
		rr := renderer(t, whoDaysFixture(t, whoDays(100, c.over)))
		bg := whoDaysBlock(fetch(t, rr, "/area/plovdiv").Body.String())
		en := whoDaysBlock(fetch(t, rr, "/en/area/plovdiv").Body.String())
		if !strings.Contains(bg, c.bg) || !strings.Contains(en, c.en) {
			t.Errorf("%s: bg %q en %q", c.name, bg, en)
		}
		for _, tail := range []string{"Нормите се проверяват с официални измервания.", "Limits are checked with official measurements."} {
			if strings.Contains(bg+en, tail) {
				t.Errorf("%s: still carries %q, which lives on the data page", c.name, tail)
			}
		}
		if c.name != "many" && (strings.Contains(bg, c.notBG) || strings.Contains(en, c.notEN)) {
			t.Errorf("%s: wrong key wording in bg %q en %q", c.name, bg, en)
		}
	}
}

func TestWHODaysGate(t *testing.T) {
	for _, c := range []struct {
		days int
		want bool
	}{{59, false}, {60, true}} {
		rr := renderer(t, whoDaysFixture(t, whoDays(c.days, 5)))
		for _, path := range []string{"/area/plovdiv", "/en/area/plovdiv"} {
			if got := whoDaysBlock(fetch(t, rr, path).Body.String()) != ""; got != c.want {
				t.Errorf("%s with %d days: rendered = %v, want %v", path, c.days, got, c.want)
			}
		}
	}
}

func TestWHODaysOnEveryKind(t *testing.T) {
	rr := renderer(t, whoDaysFixture(t, whoDays(100, 12)))
	cases := map[string]string{
		"/area/plovdiv":           "12 дни в Пловдив са над",
		"/area/mladost":           "12 дни в район Младост са над",
		"/area/plovdiv-oblast":    "12 дни в област Пловдив са над",
		"/en/area/plovdiv":        "12 days in Plovdiv were above",
		"/en/area/mladost":        "12 days in Mladost district were above",
		"/en/area/plovdiv-oblast": "12 days in Plovdiv Province were above",
	}
	for path, want := range cases {
		block := whoDaysBlock(fetch(t, rr, path).Body.String())
		if !strings.Contains(block, want) {
			t.Errorf("%s: %q does not contain %q", path, block, want)
		}
	}
}

func TestWHODaysAfterSeasonOnCityPages(t *testing.T) {
	rr := renderer(t, whoDaysFixture(t, whoDays(100, 12)))
	body := fetch(t, rr, "/area/plovdiv").Body.String()
	season, who := strings.Index(body, `class="area-season"`), strings.Index(body, `class="area-who-days"`)
	if season < 0 || who < season {
		t.Errorf("season at %d, who days at %d", season, who)
	}
}

func TestWHODaysAbsentWithoutSummary(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))
	if whoDaysBlock(fetch(t, rr, "/area/plovdiv").Body.String()) != "" {
		t.Error("block rendered without a summary")
	}
}
