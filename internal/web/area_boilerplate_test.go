package web_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/store"
)

// The area page keeps the disclaimer, the current level's health advice, a
// data-page link, the forecast note and the full CAMS licence credit. The generic text it used to
// repeat on every area lives on /about-the-data.
var dataAttr = regexp.MustCompile(` data-t-[a-z-]+="[^"]*"`)

func TestAreaPageKeepsShortBoilerplateOnly(t *testing.T) {
	snap := pollenSnap(t, []store.AreaPollenDay{{Slug: "sofia", Species: "ragweed", Day: "2026-08-09", Mean: 12, Max: 30}})
	snap.GeneratedAt = time.Now()
	meta := snap.KnownSlugs["sofia"]
	meta.Values = map[string]float64{"P2": 3}
	meta.WHODays = &snapshot.WHODays{DaysWithData: 100, DaysOver: 5, WorstDay: time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC), WorstMean: 41.26}
	snap.KnownSlugs["sofia"] = meta

	type lang struct {
		path, dataPath           string
		moved, kept              []string
		otherLevelAdvice         []string
		disclaimer, link, credit string
	}
	for _, l := range []lang{
		{
			path: "/area/sofia", dataPath: "/about-the-data",
			moved: []string{
				"Това са дневни средни на гражданските сензори.",
				"Нормите се проверяват с официални измервания.",
				"Подредбата се променя с всяко обновяване на данните.",
				"Общите въпроси за данните, цветовете и обновяването",
			},
			kept:             []string{"Какво означава това за дейностите на открито", "Качеството на въздуха е добро.", "За официални предупреждения следвайте официалните източници.", "Това е прогноза от модел, за ориентация."},
			otherLevelAdvice: []string{"Избягвайте физическите натоварвания на открито.", "Намалете физическите натоварвания на открито."},
			disclaimer:       "Стойността е ориентировъчна. Някои граждански станции може да са стари или неизправни, а ние не ги калибрираме.",
			link:             `href="/about-the-data"`,
			credit:           "Европейската комисия и ECMWF не носят отговорност",
		},
		{
			path: "/en/area/sofia", dataPath: "/en/about-the-data",
			moved: []string{
				"These are daily averages from citizen sensors.",
				"Limits are checked with official measurements.",
				"The ranking changes with every data update.",
				"The general questions about the data, the colours and the update times",
			},
			kept:             []string{"What this means for outdoor activity", "The air quality is good.", "For official alerts, follow official sources.", "This is a model forecast, for orientation."},
			otherLevelAdvice: []string{"Avoid physical activities outdoors.", "Reduce physical activities outdoors."},
			disclaimer:       "This figure is indicative. Some citizen stations may be old or faulty, and we do not calibrate them.",
			link:             `href="/en/about-the-data"`,
			credit:           "Neither the European Commission nor ECMWF is responsible",
		},
	} {
		rr := renderer(t, snap)
		// The map island's data-t-* attributes are its own labels, not page text.
		body := dataAttr.ReplaceAllString(fetch(t, rr, l.path).Body.String(), "")
		for _, s := range l.moved {
			if strings.Contains(body, s) {
				t.Errorf("%s: still carries moved text %q", l.path, s)
			}
		}
		for _, s := range l.otherLevelAdvice {
			if strings.Contains(body, s) {
				t.Errorf("%s: carries advice for another level %q", l.path, s)
			}
		}
		for _, s := range append(l.kept, l.disclaimer, l.link, l.credit, `href="https://open-meteo.com/"`) {
			if !strings.Contains(body, s) {
				t.Errorf("%s: missing %q", l.path, s)
			}
		}

		data := fetch(t, rr, l.dataPath).Body.String()
		for _, s := range l.moved {
			// The pointer to this page has nothing to move onto it.
			if strings.HasPrefix(s, "The general questions") || strings.HasPrefix(s, "Общите въпроси") {
				continue
			}
			if !strings.Contains(data, s) {
				t.Errorf("%s: moved text %q is not on the data page", l.dataPath, s)
			}
		}
	}
}
