package snapshot

import (
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/store"
)

// A fresh station carries its latest values and last time. A stale one has no values and no time: the
// registry's last_seen is an ingest stamp, so it must not show as a measurement time.
func TestBuildSensorsKeepsOfficialReadingsPerStation(t *testing.T) {
	now := time.Date(2026, 11, 19, 13, 0, 0, 0, time.UTC)
	seen := now.Add(-10 * time.Minute)
	old := now.Add(-5 * 24 * time.Hour)
	h := NewHolder(config.Series{DefaultMetric: "P2", DefaultWindow: time.Hour}, config.Wind{})
	snap := &Snapshot{
		KnownSlugs:  map[string]AreaMeta{"sofia": {Slug: "sofia", Covered: true, SensorCount: 1}},
		AreaSensors: map[string]Body{},
		AreaSeries:  map[string]Body{},
	}
	live := officialSensor(1001, "BG0050A", 23.31, 42.68, "sofia")
	live.Values = map[string]float64{"P2": 12, "P1": 20, "NO2": 31}
	live.LastSeen = seen
	staleReg := officialSensor(1002, "BG0052A", 23.40, 42.66, "sofia")
	staleReg.LastSeen = old
	registry := []store.SensorReading{officialSensor(1001, "BG0050A", 23.31, 42.68, "sofia"), staleReg}

	if _, err := buildSensors(snap, h, []store.SensorReading{live}, registry, nil, nil, 0, nil, now); err != nil {
		t.Fatal(err)
	}
	got := snap.StationReadings["BG0050A"]
	if got.Values["P2"] != 12 || got.Values["NO2"] != 31 || !got.LastSeen.Equal(seen) {
		t.Errorf("fresh station = %+v, want P2 12, NO2 31, seen %v", got, seen)
	}
	if _, has := got.Values["O3"]; has {
		t.Errorf("fresh station invented an O3 value: %+v", got)
	}
	stale, ok := snap.StationReadings["BG0052A"]
	if !ok || len(stale.Values) != 0 || !stale.LastSeen.IsZero() {
		t.Errorf("stale station = %+v (present %v), want no values and no time", stale, ok)
	}
}

// A station the registry knows but no fresh reading covers gets an entry with no values and a zero time,
// even when the registry row carries a last_seen.
func TestStationReadingsFromRegistryOnlyStationHasNoTime(t *testing.T) {
	regSeen := time.Date(2026, 11, 19, 17, 19, 0, 0, time.UTC)
	reg := officialSensor(1, "BG0052A", 23.4, 42.66)
	reg.LastSeen = regSeen
	got, ok := stationReadingsFrom([]store.SensorReading{reg}, nil)["BG0052A"]
	if !ok {
		t.Fatalf("registry-only station has no entry")
	}
	if len(got.Values) != 0 || !got.LastSeen.IsZero() {
		t.Errorf("registry-only station = %+v, want no values and no time", got)
	}
}

// Two devices of one station: the newest time wins and a metric is taken from the first device that has it.
func TestStationReadingsFromMergesDevicesOfOneStation(t *testing.T) {
	t1 := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	a := officialSensor(1, "BG0050A", 23.3, 42.6)
	a.Values, a.LastSeen = map[string]float64{"P2": 5}, t1
	b := officialSensor(2, "BG0050A", 23.3, 42.6)
	b.Values, b.LastSeen = map[string]float64{"P2": 9, "O3": 70}, t2
	got := stationReadingsFrom(nil, []store.SensorReading{a, b})["BG0050A"]
	if got.Values["P2"] != 5 || got.Values["O3"] != 70 || !got.LastSeen.Equal(t2) {
		t.Errorf("merged = %+v", got)
	}
}
