package web

import (
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
)

func loadCat(t *testing.T) *i18n.Catalogue {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	return cat
}

// The sentence keeps the visible symbol for the eye and hides it from the
// speech layer, with the spoken words beside it for a screen reader.
func TestAreaNowSentenceSpeaksTheUnit(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()
	p := acPageData(rr, "en", snap.GeneratedAt, snap.GeneratedAt)
	row := rr.rowFrom(snap.KnownSlugs["plovdiv"], "en")

	got := string(p.areaNowHTML(row))
	want := `<span aria-hidden="true">µg/m³</span><span class="visually-hidden"> micrograms per cubic metre</span>`
	if !strings.Contains(got, want) {
		t.Errorf("areaNowHTML() = %q, want the unit marked up as %q", got, want)
	}
}

func TestAreaDaySentenceSpeaksTheUnit(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()
	p := acPageData(rr, "en", snap.GeneratedAt, snap.GeneratedAt)
	row := rr.rowFrom(snap.KnownSlugs["plovdiv"], "en")
	row.Day = &snapshot.DayRange{Min: 2, Max: 20, MinAt: snap.GeneratedAt, MaxAt: snap.GeneratedAt, Buckets: 12}

	got := string(p.areaDayHTML(row))
	if !strings.Contains(got, "micrograms per cubic metre") {
		t.Errorf("areaDayHTML() = %q, want the spoken unit", got)
	}
}

// Every unit the site prints has spoken words in both languages, or a screen
// reader would stay on the symbol.
func TestEveryPublishedUnitHasSpokenWords(t *testing.T) {
	cat := loadCat(t)
	for _, lang := range []string{"en", "bg"} {
		p := PageData{Lang: lang, DefaultMetric: "P2", cat: cat}
		for _, m := range []string{"P1", "P2", "temperature", "humidity", "pressure", "noise_LAeq", "noise_LA_max", "O3", "NO2"} {
			if p.SpokenUnit(m) == p.T("unit."+m) {
				t.Errorf("%s %s: no spoken form for %q", lang, m, p.T("unit."+m))
			}
		}
	}
}

func TestSpokenUnitsAttrMapsSymbolToWords(t *testing.T) {
	p := PageData{Lang: "en", DefaultMetric: "P2", cat: loadCat(t)}
	got := p.SpokenUnitsAttr()
	for _, want := range []string{`"µg/m³":"micrograms per cubic metre"`, `"°C":"degrees Celsius"`, `"dB(A)":"decibels"`} {
		if !strings.Contains(got, want) {
			t.Errorf("SpokenUnitsAttr() = %s, want %s", got, want)
		}
	}
}

func TestReadoutCellCarriesTheSpokenUnit(t *testing.T) {
	got := areaReadoutsFor(t, "en", area("oblast", 111, map[string]float64{"P2": 3.2, "temperature": 14.0}))
	if got[0].UnitSpoken != "micrograms per cubic metre" {
		t.Errorf("PM2.5 cell spoken unit = %q", got[0].UnitSpoken)
	}
	if got[1].UnitSpoken != "degrees Celsius" {
		t.Errorf("temperature cell spoken unit = %q", got[1].UnitSpoken)
	}
}

func homePageData(t *testing.T, lang string) PageData {
	t.Helper()
	at := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	return PageData{
		Lang: lang, DefaultMetric: "P2", GeneratedAt: at, Now: at,
		cat: loadCat(t), sofiaLoc: time.UTC,
		Areas: []AreaRow{
			{Name: "Plovdiv", Kind: "oblast", Covered: true, SensorCount: 100, Value: 20, HasValue: true},
			{Name: "Varna", Kind: "oblast", Covered: true, SensorCount: 50, Value: 3, HasValue: true},
			{Name: "Ruse", Kind: "oblast", Covered: true, SensorCount: 30, Value: 4, HasValue: true},
			{Name: "Silistra", Kind: "oblast", Covered: false},
		},
	}
}

// Sentence one is the country's air-now line, sentence two names the worst
// province. Both carry the spoken unit.
func TestHomeSummaryStatesTheCountryAndTheWorstProvince(t *testing.T) {
	p := homePageData(t, "en")
	got := string(p.HomeNowHTML())

	for _, want := range []string{
		"Bulgaria right now: PM2.5 4",
		"that is 'good' (indicative)",
		"Median of 180 sensors",
		"Highest province: Plovdiv, 20",
		"micrograms per cubic metre",
		`data-home-second`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("HomeNowHTML() = %q, want substring %q", got, want)
		}
	}
	if !strings.Contains(got, "visually-hidden") {
		t.Errorf("the summary is for the speech layer, it must not paint: %q", got)
	}
}

func TestHomeSummaryWithNoReadingsSaysSo(t *testing.T) {
	p := homePageData(t, "en")
	p.Areas = []AreaRow{{Name: "Silistra", Kind: "oblast"}}
	got := string(p.HomeNowHTML())
	if !strings.Contains(got, "Bulgaria: no sensor with recent data") {
		t.Errorf("HomeNowHTML() = %q, want the none sentence", got)
	}
	if strings.Contains(got, "Highest province") {
		t.Errorf("no province has a value, yet %q names the highest", got)
	}
}

func TestHomeSummaryIsBulgarianInBulgarian(t *testing.T) {
	got := string(homePageData(t, "bg").HomeNowHTML())
	for _, want := range []string{"България", "Най-висока област: Plovdiv", "микрограма на кубичен метър"} {
		if !strings.Contains(got, want) {
			t.Errorf("HomeNowHTML() = %q, want substring %q", got, want)
		}
	}
}

// The near-you strings and direction words ride on the summary node itself, so
// the browser has nothing else to look up.
func TestHomeSummaryNodeCarriesTheNearYouCopy(t *testing.T) {
	got := string(homePageData(t, "en").HomeNowHTML())
	for _, want := range []string{`data-t-near-km="Near you:`, `data-t-near-m="Near you:`, `data-t-dir-nw="northwest"`} {
		if !strings.Contains(got, want) {
			t.Errorf("HomeNowHTML() = %q, want attribute %q", got, want)
		}
	}
}

func TestHomeReadoutsCarryTheSpokenUnit(t *testing.T) {
	got := readoutsFor(t, "en", []AreaRow{row("Plovdiv", 9.0, true, true, 30), row("Vidin", 3.1, true, true, 2)})
	for i := 0; i < 2; i++ {
		if got[i].UnitSpoken != "micrograms per cubic metre" {
			t.Errorf("home readout %d spoken unit = %q", i, got[i].UnitSpoken)
		}
	}
}
