package web_test

import (
	"strings"
	"testing"

	"kanarche.eu/internal/snapshot"
)

// Shumen's station is inside the boundary but has no fresh reading: the source split has no eea entry.
func TestFactsNameAStaleStationInsideTheBoundary(t *testing.T) {
	snap := factsSnapshot()
	m := snap.KnownSlugs["shumen"]
	m.SensorCount = 11
	m.BySource = map[string]snapshot.SourceEntry{"sensor.community": {N: 11}}
	m.FreshStations = 0
	snap.KnownSlugs["shumen"] = m

	text := factsText(t, fetch(t, renderer(t, snap), "/area/shumen").Body.String())
	if strings.Contains(text, "няма официална станция") {
		t.Errorf("page claims no official station although BG0038A is inside:\n%s", text)
	}
	want := "В Шумен има 11 граждански сензора и 1 официална станция на ИАОС: АИС Шумен. Тя е градска фонова станция."
	if !hasLine(text, want) {
		t.Errorf("missing line %q in:\n%s", want, text)
	}
}

// An uncovered area publishes no source split, so only the fresh stations are part of SensorCount.
func TestFactsUncoveredAreaDoesNotSubtractStaleStationsFromCitizens(t *testing.T) {
	snap := factsSnapshot()
	m := snap.KnownSlugs["shumen"]
	m.Covered = false
	m.SensorCount = 2
	m.BySource = nil
	m.Source = ""
	m.FreshStations = 0
	snap.KnownSlugs["shumen"] = m

	text := factsText(t, fetch(t, renderer(t, snap), "/area/shumen").Body.String())
	want := "В Шумен има 2 граждански сензора и 1 официална станция на ИАОС: АИС Шумен. Тя е градска фонова станция."
	if !hasLine(text, want) {
		t.Errorf("missing line %q in:\n%s", want, text)
	}
}
