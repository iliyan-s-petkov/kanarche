package web_test

import (
	"strings"
	"testing"

	"kanarche.eu/internal/snapshot"
)

const (
	officialStaleBG = "Официалните станции не са подали данни през последните 12 часа."
	officialStaleEN = "The official stations have not reported in the last 12 hours."
)

func shumenWith(fresh int, stations ...string) *snapshot.Snapshot {
	snap := networkSnapshot(11, stations...)
	m := snap.KnownSlugs["shumen"]
	m.FreshStations = fresh
	snap.KnownSlugs["shumen"] = m
	return snap
}

func TestFactsShowTheStaleLineWhenNoOfficialStationIsFresh(t *testing.T) {
	rr := renderer(t, shumenWith(0, "BG0038A"))
	for path, want := range map[string]string{"/area/shumen": officialStaleBG, "/en/area/shumen": officialStaleEN} {
		text := factsText(t, fetch(t, rr, path).Body.String())
		if !strings.Contains(text, want) {
			t.Errorf("%s: missing %q in:\n%s", path, want, text)
		}
	}
}

func TestFactsOmitTheStaleLineWhenOneStationIsFresh(t *testing.T) {
	rr := renderer(t, shumenWith(1, "BG0038A", "BG0050A"))
	for _, path := range []string{"/area/shumen", "/en/area/shumen"} {
		text := factsText(t, fetch(t, rr, path).Body.String())
		if strings.Contains(text, officialStaleBG) || strings.Contains(text, officialStaleEN) {
			t.Errorf("%s: stale line shown with a fresh station:\n%s", path, text)
		}
	}
}

func TestFactsOmitTheStaleLineWithoutStations(t *testing.T) {
	rr := renderer(t, shumenWith(0))
	for _, path := range []string{"/area/shumen", "/en/area/shumen"} {
		text := factsText(t, fetch(t, rr, path).Body.String())
		if strings.Contains(text, officialStaleBG) || strings.Contains(text, officialStaleEN) {
			t.Errorf("%s: stale line shown without stations:\n%s", path, text)
		}
	}
}
