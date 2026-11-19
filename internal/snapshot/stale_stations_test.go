package snapshot

import (
	"reflect"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/store"
)

// A station whose readings are past the freshness window is still inside the
// boundary, so the area keeps listing it and the nearest-station lookup keeps its position.
func TestBuildSensorsListsRegistryStationsWhoseReadingsAreStale(t *testing.T) {
	now := time.Date(2026, 11, 19, 13, 0, 0, 0, time.UTC)
	h := NewHolder(config.Series{DefaultMetric: "P2", DefaultWindow: time.Hour}, config.Wind{})
	snap := &Snapshot{
		KnownSlugs:  map[string]AreaMeta{"sofia": {Slug: "sofia", Covered: true, SensorCount: 1}},
		AreaSensors: map[string]Body{},
		AreaSeries:  map[string]Body{},
	}
	fresh := []store.SensorReading{{SensorID: 7, Source: "sensor.community", Lon: 23.3, Lat: 42.7, AreaSlugs: []string{"sofia"}}}
	registry := []store.SensorReading{officialSensor(1001, "BG0050A", 23.31, 42.68, "sofia")}

	if _, err := buildSensors(snap, h, fresh, registry, nil, nil, 0, nil, now); err != nil {
		t.Fatal(err)
	}
	meta := snap.KnownSlugs["sofia"]
	if want := []string{"BG0050A"}; !reflect.DeepEqual(meta.Stations, want) {
		t.Errorf("Stations = %v, want %v", meta.Stations, want)
	}
	if meta.FreshStations != 0 {
		t.Errorf("FreshStations = %d, want 0", meta.FreshStations)
	}
	if _, ok := snap.Stations["BG0050A"]; !ok {
		t.Errorf("snap.Stations = %v, want BG0050A positioned", snap.Stations)
	}
}

func TestBuildSensorsCountsFreshStationsSeparately(t *testing.T) {
	now := time.Date(2026, 11, 19, 13, 0, 0, 0, time.UTC)
	h := NewHolder(config.Series{DefaultMetric: "P2", DefaultWindow: time.Hour}, config.Wind{})
	snap := &Snapshot{
		KnownSlugs:  map[string]AreaMeta{"sofia": {Slug: "sofia", SensorCount: 1}},
		AreaSensors: map[string]Body{},
		AreaSeries:  map[string]Body{},
	}
	fresh := []store.SensorReading{officialSensor(1001, "BG0050A", 23.31, 42.68, "sofia")}
	registry := []store.SensorReading{
		officialSensor(1001, "BG0050A", 23.31, 42.68, "sofia"),
		officialSensor(1002, "BG0052A", 23.40, 42.66, "sofia"),
	}

	if _, err := buildSensors(snap, h, fresh, registry, nil, nil, 0, nil, now); err != nil {
		t.Fatal(err)
	}
	meta := snap.KnownSlugs["sofia"]
	if want := []string{"BG0050A", "BG0052A"}; !reflect.DeepEqual(meta.Stations, want) {
		t.Errorf("Stations = %v, want %v", meta.Stations, want)
	}
	if meta.FreshStations != 1 {
		t.Errorf("FreshStations = %d, want 1", meta.FreshStations)
	}
}
