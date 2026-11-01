package web

import (
	"testing"

	"kanarche.eu/internal/snapshot"
)

// officialArea has a citizen P2 headline with an official P2 beside it, and a
// P1 only the official stations report.
func officialArea() AreaRow {
	a := area("city", 5, map[string]float64{"P2": 12.4, "P1": 40})
	a.HeadlineSources = map[string]string{"P2": "community", "P1": "eea"}
	a.BySource = map[string]snapshot.SourceEntry{
		"eea":              {N: 1, Values: map[string]float64{"P2": 18, "P1": 40}},
		"sensor_community": {N: 4, Values: map[string]float64{"P2": 12.4}},
	}
	return a
}

func cardFor(t *testing.T, lang string, a AreaRow, metric string) Readout {
	t.Helper()
	label := areaPage(t, lang, a).T("metric." + metric)
	for _, c := range areaReadoutsFor(t, lang, a) {
		if c.Label == label {
			return c
		}
	}
	t.Fatalf("no card for %s", metric)
	return Readout{}
}

func TestOfficialRowUnderCitizenHeadline(t *testing.T) {
	c := cardFor(t, "en", officialArea(), "P2")
	if c.Official != "ИАОС stations: 18.0 µg/m³" || c.Source != "" {
		t.Errorf("Official=%q Source=%q", c.Official, c.Source)
	}
	if bg := cardFor(t, "bg", officialArea(), "P2"); bg.Official != "Станции на ИАОС: 18,0 µg/m³" {
		t.Errorf("bg Official=%q", bg.Official)
	}
}

func TestEEAFallbackLabelsTheCardAndDropsTheRow(t *testing.T) {
	c := cardFor(t, "en", officialArea(), "P1")
	if c.Source != "ИАОС stations" || c.Official != "" {
		t.Errorf("Source=%q Official=%q", c.Source, c.Official)
	}
}

func TestCitizenOnlyAreaHasNeitherLabelNorRow(t *testing.T) {
	a := area("city", 5, map[string]float64{"P2": 12.4})
	a.HeadlineSources = map[string]string{"P2": "community"}
	a.BySource = map[string]snapshot.SourceEntry{"sensor_community": {N: 5, Values: map[string]float64{"P2": 12.4}}}
	c := cardFor(t, "en", a, "P2")
	if c.Source != "" || c.Official != "" {
		t.Errorf("Source=%q Official=%q", c.Source, c.Official)
	}
}

func TestUnknownHeadlineSourceGetsNoOfficialRow(t *testing.T) {
	a := officialArea()
	a.HeadlineSources = nil
	c := cardFor(t, "en", a, "P2")
	if c.Official != "" || c.Source != "" {
		t.Errorf("Official=%q Source=%q", c.Official, c.Source)
	}
}
