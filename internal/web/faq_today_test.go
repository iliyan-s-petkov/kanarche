package web_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/store"
)

// Question openers, one per language, that identify the two daily questions.
const (
	todayQBG  = "Замърсен ли е въздухът"
	todayQEN  = "Is the air in"
	pollenQBG = "Какъв е прашецът"
	pollenQEN = "What is the pollen"
)

// faqToday returns the visible area FAQ entries of one rendered page and the FAQPage JSON-LD entries.
func faqToday(t *testing.T, snap *snapshot.Snapshot, path string) (visible, ld []faqEntry) {
	t.Helper()
	body := fetch(t, renderer(t, snap), path).Body.String()
	visible = faqVisible(t, body)
	start := strings.Index(body, ldOpen) + len(ldOpen)
	end := strings.Index(body[start:], "</script>")
	var doc struct {
		Graph []struct {
			Type       string `json:"@type"`
			MainEntity []struct {
				Name           string `json:"name"`
				AcceptedAnswer struct {
					Text string `json:"text"`
				} `json:"acceptedAnswer"`
			} `json:"mainEntity"`
		} `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(body[start:start+end]), &doc); err != nil {
		t.Fatal(err)
	}
	for _, n := range doc.Graph {
		if n.Type == "FAQPage" {
			for _, e := range n.MainEntity {
				ld = append(ld, faqEntry{Q: e.Name, A: e.AcceptedAnswer.Text})
			}
		}
	}
	return visible, ld
}

// faqFind returns the entry whose question starts with prefix.
func faqFind(items []faqEntry, prefix string) (faqEntry, bool) {
	for _, it := range items {
		if strings.HasPrefix(it.Q, prefix) {
			return it, true
		}
	}
	return faqEntry{}, false
}

func pick(path, bg, en string) string {
	if strings.HasPrefix(path, "/en/") {
		return en
	}
	return bg
}

func withValues(snap *snapshot.Snapshot, slug string, values map[string]float64) {
	m := snap.KnownSlugs[slug]
	m.Values = values
	snap.KnownSlugs[slug] = m
}

// withPollen installs a pollen table for slug, as Build would. Today is 2026-08-09.
func withPollen(t *testing.T, snap *snapshot.Snapshot, slug string, rows []store.AreaPollenDay) {
	t.Helper()
	cfg := testConfig(t).Pollen
	days := []string{"2026-08-09", "2026-08-10", "2026-08-11"}
	v := snapshot.PollenViewForTesting(rows, cfg, days)[slug]
	if v == nil {
		t.Fatal("no pollen view")
	}
	if err := snap.SetPollenForTesting(slug, v, cfg, time.Date(2026, 8, 9, 9, 20, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
}

func TestFAQTodayAnswerWithValues(t *testing.T) {
	snap := eeaFixture(t)
	withValues(snap, "plovdiv", map[string]float64{"P2": 4, "P1": 60})
	want := map[string]faqEntry{
		"/area/plovdiv": {
			Q: "Замърсен ли е въздухът в Пловдив днес?",
			A: "Към 09.08.2026, 15:00 ч. медианата на ФПЧ2.5 в Пловдив е 4,0 µg/m³, а на ФПЧ10 е 60,0 µg/m³. По Европейския индекс за качество на въздуха това е клас „умерено“. Можете да се занимавате с обичайните си дейности на открито.",
		},
		"/en/area/plovdiv": {
			Q: "Is the air in Plovdiv polluted today?",
			A: "As of Aug 9, 2026, 15:00 the median PM2.5 in Plovdiv is 4.0 µg/m³ and PM10 is 60.0 µg/m³. On the European Air Quality Index that is moderate. Enjoy your usual outdoor activities.",
		},
	}
	for path, w := range want {
		visible, ld := faqToday(t, snap, path)
		prefix := pick(path, todayQBG, todayQEN)
		if got, ok := faqFind(visible, prefix); !ok || got != w {
			t.Errorf("%s:\n got %+v\nwant %+v", path, got, w)
		}
		if got, ok := faqFind(ld, prefix); !ok || got != w {
			t.Errorf("%s: JSON-LD\n got %+v\nwant %+v", path, got, w)
		}
	}
}

func TestFAQTodayAnswerWithoutValue(t *testing.T) {
	snap := eeaFixture(t)
	for path, a := range map[string]string{
		"/area/plovdiv":    "В момента в Пловдив няма достатъчно сензори с данни за обща стойност. Показваме стойност, когато поне три места подават измервания.",
		"/en/area/plovdiv": "There are not enough sensors with data in Plovdiv for an overall value right now. We show a value when at least three places report measurements.",
	} {
		visible, ld := faqToday(t, snap, path)
		prefix := pick(path, todayQBG, todayQEN)
		if got, ok := faqFind(visible, prefix); !ok || got.A != a {
			t.Errorf("%s: got %+v, want answer %q", path, got, a)
		}
		if got, ok := faqFind(ld, prefix); !ok || got.A != a {
			t.Errorf("%s: JSON-LD %+v, want answer %q", path, got, a)
		}
	}
}

func TestFAQPollenNamesTheHighestSpecies(t *testing.T) {
	snap := eeaFixture(t)
	withPollen(t, snap, "plovdiv", []store.AreaPollenDay{
		{Slug: "plovdiv", Species: "ragweed", Day: "2026-08-09", Mean: 12, Max: 30},
		{Slug: "plovdiv", Species: "grass", Day: "2026-08-09", Mean: 0.5, Max: 1},
	})
	want := map[string]faqEntry{
		"/area/plovdiv":    {Q: "Какъв е прашецът в Пловдив днес?", A: "По прогнозата за днес най-силен е прашецът от амброзия, с умерено ниво. Прогнозата е от модел и не е измерване."},
		"/en/area/plovdiv": {Q: "What is the pollen level in Plovdiv today?", A: "Today's forecast has the highest level for ragweed pollen, moderate. The forecast comes from a model and is not a measurement."},
	}
	for path, w := range want {
		visible, ld := faqToday(t, snap, path)
		prefix := pick(path, pollenQBG, pollenQEN)
		if got, ok := faqFind(visible, prefix); !ok || got != w {
			t.Errorf("%s: got %+v, want %+v", path, got, w)
		}
		if got, ok := faqFind(ld, prefix); !ok || got != w {
			t.Errorf("%s: JSON-LD %+v, want %+v", path, got, w)
		}
	}
}

// Two species at the same level: the earlier one in the catalogue order wins, as the page chip does.
func TestFAQPollenTieTakesTheEarlierSpecies(t *testing.T) {
	snap := eeaFixture(t)
	withPollen(t, snap, "plovdiv", []store.AreaPollenDay{
		{Slug: "plovdiv", Species: "grass", Day: "2026-08-09", Mean: 1000, Max: 2000},
		{Slug: "plovdiv", Species: "ragweed", Day: "2026-08-09", Mean: 1000, Max: 2000},
	})
	visible, _ := faqToday(t, snap, "/en/area/plovdiv")
	got, _ := faqFind(visible, pollenQEN)
	if !strings.Contains(got.A, "ragweed pollen, high") {
		t.Errorf("tie answer %q, want ragweed", got.A)
	}
}

func TestFAQPollenAllLow(t *testing.T) {
	snap := eeaFixture(t)
	withPollen(t, snap, "plovdiv", []store.AreaPollenDay{
		{Slug: "plovdiv", Species: "birch", Day: "2026-08-09", Mean: 0.01, Max: 0.1},
	})
	for path, a := range map[string]string{
		"/area/plovdiv":    "По прогнозата за днес прашецът в Пловдив е на ниско ниво при всички видове. Прогнозата е от модел и не е измерване.",
		"/en/area/plovdiv": "Today's forecast has low levels for every pollen type in Plovdiv. The forecast comes from a model and is not a measurement.",
	} {
		visible, ld := faqToday(t, snap, path)
		prefix := pick(path, pollenQBG, pollenQEN)
		if got, ok := faqFind(visible, prefix); !ok || got.A != a {
			t.Errorf("%s: got %+v, want answer %q", path, got, a)
		}
		if got, ok := faqFind(ld, prefix); !ok || got.A != a {
			t.Errorf("%s: JSON-LD %+v, want answer %q", path, got, a)
		}
	}
}

func TestFAQPollenOmittedWithoutForecast(t *testing.T) {
	snap := eeaFixture(t)
	for _, path := range []string{"/area/plovdiv", "/en/area/plovdiv"} {
		visible, ld := faqToday(t, snap, path)
		prefix := pick(path, pollenQBG, pollenQEN)
		for name, set := range map[string][]faqEntry{"visible": visible, "json-ld": ld} {
			if _, ok := faqFind(set, prefix); ok {
				t.Errorf("%s: %s holds the pollen question without a forecast", path, name)
			}
		}
	}
}

// An oblast drops the station and territory answers but still gets the two daily questions.
func TestOblastFAQHasTodayAndPollenButNotStationOrTerritory(t *testing.T) {
	snap := eeaFixture(t)
	withValues(snap, "smolyan-oblast", map[string]float64{"P2": 4, "P1": 60})
	withPollen(t, snap, "smolyan-oblast", []store.AreaPollenDay{
		{Slug: "smolyan-oblast", Species: "ragweed", Day: "2026-08-09", Mean: 12, Max: 30},
	})
	for _, path := range []string{"/area/smolyan-oblast", "/en/area/smolyan-oblast"} {
		visible, _ := faqToday(t, snap, path)
		if len(visible) != 2 {
			t.Errorf("%s: %d entries, want 2: %+v", path, len(visible), visible)
		}
		for _, q := range []string{pick(path, todayQBG, todayQEN), pick(path, pollenQBG, pollenQEN)} {
			if _, ok := faqFind(visible, q); !ok {
				t.Errorf("%s: missing %q", path, q)
			}
		}
	}
}

func TestFAQTodayCopyHasNoSemicolonsOrDashes(t *testing.T) {
	snap := eeaFixture(t)
	withValues(snap, "plovdiv", map[string]float64{"P2": 4, "P1": 60})
	withPollen(t, snap, "plovdiv", []store.AreaPollenDay{
		{Slug: "plovdiv", Species: "ragweed", Day: "2026-08-09", Mean: 12, Max: 30},
	})
	for _, path := range []string{"/area/plovdiv", "/en/area/plovdiv"} {
		visible, _ := faqToday(t, snap, path)
		for _, it := range visible {
			if strings.ContainsAny(it.Q+it.A, ";\u2013\u2014") {
				t.Errorf("%s: %q holds a semicolon or dash", path, it.A)
			}
		}
	}
}
