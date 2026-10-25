package snapshot

import (
	"reflect"
	"testing"

	"kanarche.eu/internal/store"
)

func officialSensor(id int64, code string, lon, lat float64, areas ...string) store.SensorReading {
	return store.SensorReading{SensorID: id, Source: "eea", StationCode: code, Lon: lon, Lat: lat, AreaSlugs: areas}
}

func TestOfficialStationsFromKeepsOnlyEEAStationsWithACode(t *testing.T) {
	sensors := []store.SensorReading{
		officialSensor(1, "BG0038A", 26.9, 43.27, "shumen"),
		{SensorID: 2, Source: "sensor.community", Lon: 26.9, Lat: 43.2},
		officialSensor(3, "", 27, 43),
		{SensorID: 4, Source: "sensor.community", StationCode: "BG0001A"},
	}
	got := officialStationsFrom(sensors)
	want := map[string]OfficialStation{"BG0038A": {Code: "BG0038A", Lon: 26.9, Lat: 43.27}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestStationCodesByAreaIsSortedAndDeduplicated(t *testing.T) {
	sensors := []store.SensorReading{
		officialSensor(1, "BG0050A", 23.3, 42.7, "sofia", "sofia-grad-oblast"),
		officialSensor(2, "BG0040A", 23.3, 42.7, "sofia", "sofia-grad-oblast"),
		officialSensor(3, "BG0040A", 23.3, 42.7, "sofia"),
		officialSensor(4, "BG0038A", 26.9, 43.27, "shumen"),
		{SensorID: 5, Source: "sensor.community", Lon: 23.3, Lat: 42.7, AreaSlugs: []string{"sofia"}},
	}
	got := stationCodesByArea(sensors)
	want := map[string][]string{
		"sofia":             {"BG0040A", "BG0050A"},
		"sofia-grad-oblast": {"BG0040A", "BG0050A"},
		"shumen":            {"BG0038A"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestStationCodesByAreaEmptyWhenNoOfficialStation(t *testing.T) {
	got := stationCodesByArea([]store.SensorReading{{SensorID: 1, Source: "sensor.community", AreaSlugs: []string{"a"}}})
	if len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}
