package web

// In package web rather than web_test for the same reason readouts_test.go is:
// PageData.cat is unexported and AreaReadouts reads the catalogue.

import (
	"testing"

	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
)

// areaReadoutsFor builds the PageData an area page carries — the one area, the
// canonical metric order, and the catalogue — and asks for its strip.
func areaReadoutsFor(t *testing.T, lang string, area AreaRow) []Readout {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	p := PageData{
		Lang:          lang,
		Area:          &area,
		DefaultMetric: "P2",
		// The order the site publishes metrics in, which the strip must follow —
		// a map's iteration order would reshuffle the cells on every request.
		Metrics: []string{"P2", "P1", "temperature", "humidity", "pressure", "noise_LAeq", "noise_LA_max"},
		cat:     cat,
	}
	return p.AreaReadouts()
}

// areaPage is the PageData areaReadoutsFor builds, for the tests that ask it
// something besides the cards.
func areaPage(t *testing.T, lang string, a AreaRow) PageData {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	return PageData{
		Lang: lang, Area: &a, DefaultMetric: "P2", cat: cat,
		Metrics: []string{"P2", "P1", "temperature", "humidity", "pressure", "noise_LAeq", "noise_LA_max"},
	}
}

// area is the shorthand these cases are written in.
func area(kind string, sensors int, values map[string]float64) AreaRow {
	return AreaRow{Name: "Пловдив", Kind: kind, Covered: true, SensorCount: sensors, Values: values}
}

func TestAreaReadoutsGiveOneCellPerMeasuredMetric(t *testing.T) {
	got := areaReadoutsFor(t, "bg", area("oblast", 111, map[string]float64{
		"P2": 3.22, "P1": 7.7, "temperature": 14.0,
	}))
	// One card per measured metric. The sensor count lives in the section sub-line.
	if len(got) != 3 {
		t.Fatalf("got %d cells, want 3: one per measured metric", len(got))
	}
	if got[0].Value != "3,2" || got[0].Unit != "µg/m³" {
		t.Errorf("first cell = %q %q, want the default metric 3,2 µg/m³", got[0].Value, got[0].Unit)
	}
	if got[2].Unit != "°C" {
		t.Errorf("temperature unit = %q, want °C — each metric carries its own", got[2].Unit)
	}
}

// The cells follow the site's canonical metric order, not the order the values
// map happens to iterate in — otherwise the strip would rearrange itself
// between two requests for the same page.
func TestAreaReadoutsKeepTheCanonicalMetricOrder(t *testing.T) {
	got := areaReadoutsFor(t, "en", area("oblast", 4, map[string]float64{
		"humidity": 60, "P2": 3.2, "temperature": 14,
	}))
	want := []string{"PM2.5", "Temperature", "Humidity"}
	for i, label := range want {
		if got[i].Label != label {
			t.Errorf("cell %d = %q, want %q", i, got[i].Label, label)
		}
	}
}

// The default metric leads whatever the canonical order says. Canonical order
// is alphabetical, so P1 sorts above P2 — and a strip opening on PM10 while
// the chart, the map and the province list all show PM2.5 answers a question
// the page is not asking.
func TestAreaReadoutsOpenOnThePageDefaultMetric(t *testing.T) {
	got := areaReadoutsFor(t, "en", area("oblast", 4, map[string]float64{
		"P1": 7.7, "P2": 3.2, "temperature": 14,
	}))
	want := []string{"PM2.5", "PM10", "Temperature"}
	for i, label := range want {
		if got[i].Label != label {
			t.Errorf("cell %d = %q, want %q", i, got[i].Label, label)
		}
	}
}

// And it appears exactly once — leading it without excluding it from the loop
// prints the site's headline metric twice.
func TestAreaReadoutsDoNotRepeatTheDefaultMetric(t *testing.T) {
	got := areaReadoutsFor(t, "en", area("oblast", 4, map[string]float64{"P2": 3.2, "P1": 7.7}))
	if len(got) != 2 {
		t.Fatalf("got %d cells, want 2: two metrics", len(got))
	}
}

// A metric the area is not measuring gets no cell at all. An empty cell would
// be a claim that the site tried and found nothing, when in fact this area has
// no sensor carrying that instrument.
func TestAreaReadoutsSkipMetricsWithNoReading(t *testing.T) {
	got := areaReadoutsFor(t, "en", area("oblast", 4, map[string]float64{"P2": 3.2}))
	if len(got) != 1 {
		t.Fatalf("got %d cells, want 1: one measured metric", len(got))
	}
}

// 0 µg/m³ is a reading. Treating a zero as "not measured" would silently drop
// the cell on the cleanest night of the year.
func TestAreaReadoutsTreatZeroAsAReading(t *testing.T) {
	got := areaReadoutsFor(t, "bg", area("oblast", 4, map[string]float64{"P2": 0}))
	if len(got) != 1 || got[0].Value != "0,0" {
		t.Fatalf("got %d cells, first %q — want a cell reading 0,0", len(got), got[0].Value)
	}
}

// The card carries no tier line: the section sub-line says once what every
// figure covers and how many sensors it comes from.
func TestAreaReadoutsCarryNoTierLine(t *testing.T) {
	got := areaReadoutsFor(t, "en", area("oblast", 4, map[string]float64{"P2": 3.2, "temperature": 14}))
	for i, r := range got {
		if r.Tier != "" {
			t.Errorf("cell %d tier = %q, want none", i, r.Tier)
		}
	}
}

// What the figures cover, said once. A province page reports a province and a
// city page a city, and "province median" over Пловдив-град would be false.
func TestAreaReadoutsSubLineNamesTierAndSensorCount(t *testing.T) {
	cases := []struct {
		lang, kind string
		sensors    int
		want       string
	}{
		{"en", "oblast", 4, "province median · 4 sensors"},
		{"en", "city", 1, "city median · 1 sensor"},
		{"bg", "oblast", 10, "медиана за областта · 10 сензора"},
	}
	for _, c := range cases {
		p := areaPage(t, c.lang, area(c.kind, c.sensors, map[string]float64{"P2": 3.2}))
		if got := p.AreaReadoutsSub(); got != c.want {
			t.Errorf("%s %s sub-line = %q, want %q", c.lang, c.kind, got, c.want)
		}
	}
}

// An area below the coverage floor publishes no average, so it gets no strip —
// the page already states the absence in its own words. Four cells of nothing
// would contradict that notice.
func TestAreaReadoutsAreAbsentWithoutCoverage(t *testing.T) {
	a := area("oblast", 2, map[string]float64{"P2": 3.2})
	a.Covered = false
	if got := areaReadoutsFor(t, "bg", a); got != nil {
		t.Errorf("AreaReadouts() = %v, want nil for an uncovered area", got)
	}
}

// A covered area with every sensor silent has no cards at all. The sentence
// above already says how many sensors there are and that none reports.
func TestAreaReadoutsAreEmptyWithNoReadings(t *testing.T) {
	if got := areaReadoutsFor(t, "bg", area("oblast", 9, nil)); len(got) != 0 {
		t.Fatalf("got %v, want no cards", got)
	}
}

// The strip only exists on a page that has an area. Home and about carry none.
func TestAreaReadoutsAreAbsentWithoutAnArea(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	p := PageData{Lang: "bg", cat: cat}
	if got := p.AreaReadouts(); got != nil {
		t.Errorf("AreaReadouts() = %v, want nil with no area", got)
	}
}

// The same rule the province rows and the country strip follow: the decimal
// separator belongs to the language.
func TestAreaReadoutsWriteTheDecimalTheLanguageWrites(t *testing.T) {
	values := map[string]float64{"P2": 18.16}
	if got := areaReadoutsFor(t, "en", area("oblast", 4, values)); got[0].Value != "18.2" {
		t.Errorf("English = %q, want 18.2", got[0].Value)
	}
	if got := areaReadoutsFor(t, "bg", area("oblast", 4, values)); got[0].Value != "18,2" {
		t.Errorf("Bulgarian = %q, want 18,2", got[0].Value)
	}
}

// Every cell is labelled and valued in both languages. An empty label is what
// an invented catalogue key looks like, which the startup key check cannot see.
func TestEveryAreaReadoutIsLabelledAndValued(t *testing.T) {
	for _, lang := range []string{"bg", "en"} {
		got := areaReadoutsFor(t, lang, area("oblast", 4, map[string]float64{"P2": 3.2, "noise_LAeq": 51}))
		if len(got) != 2 {
			t.Fatalf("%s: got %d cells, want 2", lang, len(got))
		}
		for i, r := range got {
			if r.Label == "" || r.Value == "" {
				t.Errorf("%s: cell %d is incomplete: %+v", lang, i, r)
			}
		}
	}
}

// Health metrics carry the band colour and word. Benzene, CO and the weather
// metrics have no health band, so they get neither: a bar there would claim a class.
func TestAreaReadoutsCarryBandOnlyWhereAHealthBandExists(t *testing.T) {
	got := areaReadoutsFor(t, "bg", area("oblast", 4, map[string]float64{
		"P2": 19.6, "P1": 29.6, "temperature": 11.6, "humidity": 100, "pressure": 1003.5,
	}))
	if got[0].Band != "умерено" || got[0].BandColour != "#f0e641" {
		t.Errorf("PM2.5 band = %q %q, want умерено #f0e641", got[0].Band, got[0].BandColour)
	}
	if got[1].Band != "задоволително" || got[1].BandColour == "" {
		t.Errorf("PM10 band = %q %q, want задоволително with a colour", got[1].Band, got[1].BandColour)
	}
	for _, r := range got[2:] {
		if r.Band != "" || r.BandColour != "" {
			t.Errorf("%s has band %q %q, want none", r.Label, r.Band, r.BandColour)
		}
	}
}

// Benzene and CO are pollutants without a health band: they sit with the
// pollutants and show no bar. In the card order the canonical list gives them.
func TestAreaReadoutsAxisOnlyPollutantsHaveNoBand(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	p := PageData{
		Lang: "bg", Area: &AreaRow{Covered: true, Kind: "oblast", SensorCount: 3, Values: map[string]float64{"P2": 3, "C6H6": 1.4, "CO": 2190}},
		DefaultMetric: "P2", Metrics: []string{"P2", "C6H6", "CO"}, cat: cat,
	}
	got := p.AreaReadouts()
	if len(got) != 3 {
		t.Fatalf("got %d cells, want 3", len(got))
	}
	for _, r := range got[1:] {
		if r.BandColour != "" || r.Band != "" || r.Weather {
			t.Errorf("%s = %+v, want a pollutant card with no band", r.Label, r)
		}
	}
}

// The cards split into two groups under an H3 each. The heading rides on the
// first card of its group, and weather cards are the three meteo metrics.
func TestAreaReadoutsSplitIntoPollutantsAndWeather(t *testing.T) {
	got := areaReadoutsFor(t, "en", area("oblast", 4, map[string]float64{
		"P2": 3.2, "P1": 7.7, "temperature": 14, "humidity": 60, "pressure": 1003.5, "noise_LAeq": 51,
	}))
	var headings []string
	for _, r := range got {
		if r.Heading != "" {
			headings = append(headings, r.Heading)
		}
	}
	if len(headings) != 2 || headings[0] != "Pollutants" || headings[1] != "Weather" {
		t.Fatalf("headings = %v, want [Pollutants Weather]", headings)
	}
	wantWeather := map[string]bool{"Temperature": true, "Humidity": true, "Pressure": true}
	seenWeather := false
	for _, r := range got {
		if r.Weather {
			seenWeather = true
		}
		if seenWeather && !r.Weather {
			t.Errorf("%s follows a weather card but is not weather: the groups must be contiguous", r.Label)
		}
		if r.Weather != wantWeather[r.Label] {
			t.Errorf("%s weather = %v", r.Label, r.Weather)
		}
	}
}

// With no weather metric there is no weather heading, and vice versa.
func TestAreaReadoutsOmitAnEmptyGroupHeading(t *testing.T) {
	got := areaReadoutsFor(t, "en", area("oblast", 4, map[string]float64{"P2": 3.2}))
	if len(got) != 1 || got[0].Heading != "Pollutants" {
		t.Fatalf("got %+v, want one pollutant card carrying the heading", got)
	}
	got = areaReadoutsFor(t, "en", area("oblast", 4, map[string]float64{"temperature": 14}))
	if len(got) != 1 || got[0].Heading != "Weather" {
		t.Fatalf("got %+v, want one weather card carrying the heading", got)
	}
}

// When per-network rows are removed, all cells should have empty Group fields
// and the count should be exactly N+1 (N metrics + sensor count, no breakdown).
func TestAreaReadoutsHaveNoPerNetworkRows(t *testing.T) {
	// Area with two networks reporting P2 and temperature.
	a := area("oblast", 4, map[string]float64{"P2": 25, "temperature": 20})
	a.BySource = map[string]snapshot.SourceEntry{
		"sensor.community": {N: 3, Values: map[string]float64{"P2": 20, "temperature": 19}},
		"eea":              {N: 1, Values: map[string]float64{"P2": 100, "temperature": 25}},
	}
	got := areaReadoutsFor(t, "en", a)

	// One cell per metric, no per-network rows.
	if len(got) != 2 {
		t.Fatalf("got %d readouts, want 2", len(got))
	}
}
