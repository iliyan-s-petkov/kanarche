package web_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

// yearProfileFixture is the area page roster with the same profile on every area.
func yearProfileFixture(t *testing.T, y snapshot.YearProfile) *snapshot.Snapshot {
	t.Helper()
	snap := areaPageFixture(t)
	for slug, meta := range snap.KnownSlugs {
		copied := y
		meta.YearProfile = &copied
		snap.KnownSlugs[slug] = meta
	}
	return snap
}

// fullYear is twelve months from December 2025 with 25 days each, so 300 days of data.
func fullYear() snapshot.YearProfile {
	y := snapshot.YearProfile{
		DaysWithData: 300, Mean: 21.34,
		HasWinter: true, WinterMean: 38.26, HasSummer: true, SummerMean: 9.04,
		DaysOver: 120, WorstDay: time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC), WorstMean: 88.88,
	}
	for i := 0; i < 12; i++ {
		d := time.Date(2025, time.December+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		y.Months = append(y.Months, snapshot.MonthMean{Year: d.Year(), Month: d.Month(), Mean: float64(10 + i), Days: 25})
	}
	best, worst := y.Months[7], y.Months[1]
	y.Best, y.Worst = &best, &worst
	return y
}

// yearSection returns the year profile section of a rendered page, or "".
func yearSection(body string) string {
	i := strings.Index(body, `id="area-year"`)
	if i < 0 {
		return ""
	}
	rest := body[i:]
	return rest[:strings.Index(rest, "</section>")]
}

var yearTagRE = regexp.MustCompile(`<[^>]*>`)

func TestYearProfileRendersBothLanguages(t *testing.T) {
	rr := renderer(t, yearProfileFixture(t, fullYear()))
	bg := yearSection(fetch(t, rr, "/area/plovdiv").Body.String())
	en := yearSection(fetch(t, rr, "/en/area/plovdiv").Body.String())
	for _, want := range []string{
		"Въздухът в Пловдив през годината",
		"През последните 12 месеца средното ниво на ФПЧ2.5 в Пловдив е 21,3 µg/m³.",
		"Най-чистият месец е юли, средно 17,0 µg/m³. Най-замърсеният е януари, средно 11,0 µg/m³.",
		"Зимното средно е 38,3 µg/m³, а лятното е 9,0 µg/m³.",
		"От 300 дни с данни 120 са над дневната препоръка на СЗО за ФПЧ2.5 (15 µg/m³ средно за 24 часа). Най-лошият ден е 14 януари 2026 г., средно 88,9 µg/m³. Стойностите са дневни медиани от гражданските сензори. Не са официална проверка за нормите.",
		`<th scope="col">Месец</th>`, `Средно (µg/m³)`, `Дни с данни`,
	} {
		if !strings.Contains(bg, want) {
			t.Errorf("bg missing %q in %q", want, bg)
		}
	}
	for _, want := range []string{
		"Air in Plovdiv through the year",
		"Over the last 12 months the average PM2.5 in Plovdiv was 21.3 µg/m³.",
		"The cleanest month was July at 17.0 µg/m³. The most polluted was January at 11.0 µg/m³.",
		"The winter average was 38.3 µg/m³ and the summer average 9.0 µg/m³.",
		"Of 300 days with data, 120 were above the WHO daily guideline for PM2.5 (15 µg/m³ over 24 hours). The worst day was 14 January 2026 at 88.9 µg/m³. These are daily medians from citizen sensors. They are not an official check against legal limits.",
		`<th scope="col">Month</th>`, `Average (µg/m³)`, `Days with data`,
	} {
		if !strings.Contains(en, want) {
			t.Errorf("en missing %q in %q", want, en)
		}
	}
	for name, s := range map[string]string{"bg": bg, "en": en} {
		if n := strings.Count(s, `<th scope="row">`); n != 12 {
			t.Errorf("%s: %d month rows, want 12", name, n)
		}
	}
	if !strings.Contains(bg, "декември 2025") || !strings.Contains(en, "December 2025") {
		t.Errorf("rows must carry the year when the months span two: bg %q", bg)
	}
}

// A city whose name starts with В takes "във", through the same helper as the H1.
func TestYearProfileUsesVavForVarna(t *testing.T) {
	snap := yearProfileFixture(t, fullYear())
	meta := snap.KnownSlugs["plovdiv"]
	meta.NameBG, meta.NameEN = "Варна", "Varna"
	snap.KnownSlugs["plovdiv"] = meta
	got := yearSection(fetch(t, renderer(t, snap), "/area/plovdiv").Body.String())
	if !strings.Contains(got, "Въздухът във Варна през годината") || !strings.Contains(got, "ФПЧ2.5 във Варна") {
		t.Errorf("got %q", got)
	}
}

func TestYearProfileShortDataRendersOnlyCoverage(t *testing.T) {
	y := fullYear()
	y.DaysWithData = 179
	rr := renderer(t, yearProfileFixture(t, y))
	bg := yearSection(fetch(t, rr, "/area/plovdiv").Body.String())
	en := yearSection(fetch(t, rr, "/en/area/plovdiv").Body.String())
	if !strings.Contains(bg, "Данните за Пловдив обхващат само 179 дни. Годишен профил още няма.") {
		t.Errorf("bg %q", bg)
	}
	if !strings.Contains(en, "Data for Plovdiv covers only 179 days. There is no year profile yet.") {
		t.Errorf("en %q", en)
	}
	for _, s := range []string{bg, en} {
		if strings.Contains(s, "<table") || strings.Contains(s, "µg/m³") {
			t.Errorf("short-data section carries more than the coverage line: %q", s)
		}
	}
}

func TestYearProfileExactly180DaysRendersProfile(t *testing.T) {
	y := fullYear()
	y.DaysWithData = 180
	got := yearSection(fetch(t, renderer(t, yearProfileFixture(t, y)), "/area/plovdiv").Body.String())
	if !strings.Contains(got, "<table") {
		t.Errorf("180 days should render the profile: %q", got)
	}
}

func TestYearProfileNoProfileNoSection(t *testing.T) {
	body := fetch(t, renderer(t, areaPageFixture(t)), "/area/plovdiv").Body.String()
	if strings.Contains(body, `id="area-year"`) {
		t.Error("area without a profile still renders the section")
	}
}

// A missing winter or summer drops the whole sentence, never leaves blanks.
func TestYearProfileOmitsSentencesWithoutData(t *testing.T) {
	y := fullYear()
	y.HasWinter = false
	y.Best, y.Worst = nil, nil
	rr := renderer(t, yearProfileFixture(t, y))
	bg := yearSection(fetch(t, rr, "/area/plovdiv").Body.String())
	en := yearSection(fetch(t, rr, "/en/area/plovdiv").Body.String())
	for _, gone := range []string{"Зимното", "Най-чистият", "Най-замърсеният", "{", "}"} {
		if strings.Contains(bg, gone) {
			t.Errorf("bg still has %q: %q", gone, bg)
		}
	}
	for _, gone := range []string{"winter average", "cleanest month", "most polluted", "{", "}"} {
		if strings.Contains(en, gone) {
			t.Errorf("en still has %q: %q", gone, en)
		}
	}
	if !strings.Contains(bg, "средното ниво на ФПЧ2.5") || !strings.Contains(bg, "<table") {
		t.Errorf("rest of the section must stay: %q", bg)
	}
}

func TestYearProfileOneDayOverUsesSingularVerb(t *testing.T) {
	y := fullYear()
	y.DaysOver = 1
	rr := renderer(t, yearProfileFixture(t, y))
	bg := yearSection(fetch(t, rr, "/area/plovdiv").Body.String())
	en := yearSection(fetch(t, rr, "/en/area/plovdiv").Body.String())
	if !strings.Contains(bg, "300 дни с данни 1 е над") || !strings.Contains(en, "300 days with data, 1 was above") {
		t.Errorf("bg %q en %q", bg, en)
	}
}

// The profile replaces the older WHO paragraph on the page, so the same sentence is not printed twice.
func TestYearProfileSupersedesWHODaysParagraph(t *testing.T) {
	snap := yearProfileFixture(t, fullYear())
	for slug, meta := range snap.KnownSlugs {
		meta.WHODays = &snapshot.WHODays{DaysWithData: 300, DaysOver: 120, WorstDay: time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC), WorstMean: 88.88}
		snap.KnownSlugs[slug] = meta
	}
	body := fetch(t, renderer(t, snap), "/area/plovdiv").Body.String()
	if strings.Contains(body, `class="area-who-days"`) {
		t.Error("WHO days paragraph still rendered next to the year profile")
	}
}

func TestYearProfileCopyHasNoSemicolonsOrDashes(t *testing.T) {
	rr := renderer(t, yearProfileFixture(t, fullYear()))
	for _, path := range []string{"/area/plovdiv", "/en/area/plovdiv"} {
		text := yearTagRE.ReplaceAllString(yearSection(fetch(t, rr, path).Body.String()), " ")
		for _, bad := range []string{";", "—", "–"} {
			// Entities from the HTML escaper count as the characters they print.
			if strings.Contains(text, bad) {
				t.Errorf("%s: section text contains %q: %q", path, bad, text)
			}
		}
	}
}
