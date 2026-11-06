package web

import (
	"strings"
	"testing"

	"kanarche.eu/internal/snapshot"
)

// 10 stations of which 3 are official: the median covers 7 citizen sensors.
func citizenRow() AreaRow {
	a := area("city", 10, map[string]float64{"P2": 12})
	a.Official = 3
	return a
}

func TestSubtitleCountsCitizenSensorsOnly(t *testing.T) {
	cases := map[string]string{
		"bg": "медиана за града · 7 сензора",
		"en": "city median · 7 sensors",
	}
	for lang, want := range cases {
		if got := areaPage(t, lang, citizenRow()).AreaReadoutsSub(); got != want {
			t.Errorf("%s sub-line = %q, want %q", lang, got, want)
		}
	}
}

func TestMedianSentenceCountsCitizenSensorsOnly(t *testing.T) {
	a := citizenRow()
	a.HasValue, a.Value = true, 12
	for lang, want := range map[string]string{"bg": "Медиана от 7 сензора", "en": "Median of 7 sensors"} {
		if got := areaPage(t, lang, a).areaNowText(a); !strings.Contains(got, want) {
			t.Errorf("%s now sentence = %q, want it to contain %q", lang, got, want)
		}
	}
}

// The few-sensors sentence is not a median, so it keeps the full station count.
func TestFewSentenceKeepsStationCount(t *testing.T) {
	a := citizenRow()
	a.Covered = false
	if got := areaPage(t, "en", a).areaNowText(a); !strings.Contains(got, "10 sensors") {
		t.Errorf("few sentence = %q, want 10 sensors", got)
	}
}

// An all-fallback area has no citizen sensors; the count renders as 0 like any other.
func TestAllOfficialAreaRendersZeroCitizens(t *testing.T) {
	a := area("city", 2, map[string]float64{"P2": 9})
	a.Official = 2
	if got := areaPage(t, "en", a).AreaReadoutsSub(); got != "city median · 0 sensors" {
		t.Errorf("sub-line = %q", got)
	}
}

// The home sentence sums citizen sensors across provinces, not every station.
func TestHomeMedianSentenceCountsCitizenSensors(t *testing.T) {
	p := homePageData(t, "en")
	p.Areas[0].Official = 10
	if got := string(p.HomeNowHTML()); !strings.Contains(got, "Median of 170 sensors") {
		t.Errorf("HomeNowHTML() = %q, want Median of 170 sensors", got)
	}
}

// rowFrom must carry the official split, or every sentence above counts all stations.
func TestRowFromCarriesOfficialCount(t *testing.T) {
	rr := acRenderer(t)
	meta := snapshot.AreaMeta{
		Slug: "x", Kind: "city", Covered: true, SensorCount: 9,
		BySource: map[string]snapshot.SourceEntry{"eea": {N: 2}},
	}
	if got := rr.rowFrom(meta, "en").Citizens(); got != 7 {
		t.Errorf("Citizens() = %d, want 7", got)
	}
}

func TestOfficialCountFromMeta(t *testing.T) {
	meta := snapshot.AreaMeta{
		Covered: true, SensorCount: 9,
		BySource: map[string]snapshot.SourceEntry{"eea": {N: 2}},
	}
	if got := officialCount(meta); got != 2 {
		t.Errorf("officialCount = %d, want 2", got)
	}
}
