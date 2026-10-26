package web_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

func cardsSnapshot() *snapshot.Snapshot {
	return &snapshot.Snapshot{
		GeneratedAt: time.Now(),
		KnownSlugs: map[string]snapshot.AreaMeta{
			"pleven": {Slug: "pleven", Kind: "oblast", NameBG: "Плевен", NameEN: "Pleven", Covered: true, SensorCount: 10,
				Values: map[string]float64{"P2": 19.6, "P1": 29.6, "C6H6": 1.4, "CO": 2190, "temperature": 11.6, "humidity": 100, "pressure": 1003.5}},
		},
	}
}

// cardsOf returns the markup of every .readout card in body, in order.
func cardsOf(body string) []string {
	return regexp.MustCompile(`(?s)<div class="readout card">.*?\n  </div>`).FindAllString(body, -1)
}

func TestAreaCardsAreNumberPlusBandBarWithNoRing(t *testing.T) {
	body := fetch(t, renderer(t, cardsSnapshot()), "/area/pleven").Body.String()
	cards := cardsOf(body)
	if len(cards) != 7 {
		t.Fatalf("got %d cards, want 7", len(cards))
	}
	if strings.Contains(body, `class="gauge`) || strings.Contains(body, "readout__tier") {
		t.Error("the area page still draws a ring or a per-card tier line")
	}
	// Presentation attributes, not a style attribute: style-src has no unsafe-inline.
	bar := `<svg class="readout__bar"`
	if !strings.Contains(cards[0], bar) || !strings.Contains(cards[0], `fill="#f0e641"`) || !strings.Contains(cards[0], `<span class="readout__band">умерено</span>`) {
		t.Errorf("PM2.5 card lacks its band bar and word:\n%s", cards[0])
	}
	for _, c := range cards {
		if strings.Contains(c, "ФПЧ") {
			continue
		}
		if strings.Contains(c, bar) || strings.Contains(c, "readout__band") {
			t.Errorf("a card without a health band draws a bar or word:\n%s", c)
		}
	}
}

func TestAreaCardsSplitUnderTwoH3Groups(t *testing.T) {
	body := fetch(t, renderer(t, cardsSnapshot()), "/area/pleven").Body.String()
	poll := strings.Index(body, `<h3 class="readouts__heading">Замърсители</h3>`)
	weather := strings.Index(body, `<h3 class="readouts__heading">Времето</h3>`)
	pm := poll + strings.Index(body[poll:], "ФПЧ2.5")
	temp := weather + strings.Index(body[weather:], "Температура")
	if poll < 0 || weather < poll {
		t.Fatalf("headings missing or out of order: pollutants=%d weather=%d", poll, weather)
	}
	if pm < poll || pm > weather || temp < weather || temp > len(body) {
		t.Errorf("a card sits under the wrong heading: pollutants=%d pm=%d weather=%d temp=%d", poll, pm, weather, temp)
	}
	if strings.Count(body, `<div class="readouts">`) != 2 {
		t.Errorf("want one readouts grid per group, got %d", strings.Count(body, `<div class="readouts">`))
	}
}

func TestAreaCardsSayTheirTierOnceInTheSubLine(t *testing.T) {
	body := fetch(t, renderer(t, cardsSnapshot()), "/area/pleven").Body.String()
	if !strings.Contains(body, `<p class="readouts__sub">медиана за областта · 10 сензора</p>`) {
		t.Error("no sub-line with the tier and the sensor count")
	}
	if strings.Count(body, "readout__tier") != 0 {
		t.Error("a card still carries its own tier line")
	}
}

func TestAreaHeaderChipRowShowsDefaultMetricValueAndBand(t *testing.T) {
	body := fetch(t, renderer(t, cardsSnapshot()), "/area/pleven").Body.String()
	head := body[strings.Index(body, `<div class="page-head">`):strings.Index(body, `<div class="area-summary"`)]
	for _, want := range []string{
		`<h1 class="t-title">`,
		`<div class="area-chips">`,
		`<span class="area-chips__value">19,6 <span class="area-chips__unit" aria-hidden="true">µg/m³</span>`,
		`<span class="chip"><svg class="chip__dot"`,
		`fill="#f0e641"`,
		`умерено</span>`,
	} {
		if !strings.Contains(head, want) {
			t.Errorf("header lacks %q:\n%s", want, head)
		}
	}
}

func TestAreaHeaderChipRowIsAbsentWithoutAValue(t *testing.T) {
	snap := cardsSnapshot()
	m := snap.KnownSlugs["pleven"]
	m.Covered = false
	snap.KnownSlugs["pleven"] = m
	body := fetch(t, renderer(t, snap), "/area/pleven").Body.String()
	if strings.Contains(body, "area-chips") {
		t.Error("an uncovered area draws a chip row")
	}
}

func TestHomeCardsHaveNoRing(t *testing.T) {
	body := fetch(t, renderer(t, rankingSnapshot()), "/").Body.String()
	if strings.Contains(body, `class="gauge`) {
		t.Error("the home strip still draws a ring")
	}
	if !strings.Contains(body, `class="readout__bar"`) {
		t.Error("the home PM cards lost their band bar")
	}
}
