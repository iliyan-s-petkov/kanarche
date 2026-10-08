package snapshot

import (
	"encoding/json"
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

func faultyPair() []store.SensorReading {
	return []store.SensorReading{
		{SensorID: 1, SensorType: "SDS011", Lon: 23.3, Lat: 42.7,
			Values: map[string]float64{"P2": 10}},
		{SensorID: 2, SensorType: "SDS011", Lon: 23.3001, Lat: 42.7,
			Values: map[string]float64{"P2": 900, "temperature": 20}, Faulty: []string{"P2"}},
	}
}

func TestSensorPayloadCarriesTheFaultyColumn(t *testing.T) {
	raw, err := json.Marshal(sensorPayloadFrom(time.Now(), faultyPair()))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var body struct {
		Sensors struct {
			ID     []int64    `json:"id"`
			Faulty [][]string `json:"faulty"`
		} `json:"sensors"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := body.Sensors.Faulty
	if len(got) != len(body.Sensors.ID) {
		t.Fatalf("faulty has %d entries for %d sensors", len(got), len(body.Sensors.ID))
	}
	if got[0] == nil || len(got[0]) != 0 {
		t.Errorf("healthy sensor faulty = %#v, want []", got[0])
	}
	if len(got[1]) != 1 || got[1][0] != "P2" {
		t.Errorf("faulty sensor faulty = %v, want [P2]", got[1])
	}
}

// A faulty metric leaves the hex medians and coverage; the sensor's other
// metrics and its own point stay.
func TestFaultyMetricLeavesTheHexesOnly(t *testing.T) {
	sensors := faultyPair()
	snap := &Snapshot{}
	if err := buildHexes(snap, sensors, time.Now()); err != nil {
		t.Fatalf("buildHexes: %v", err)
	}
	var p2, temp []float64
	for _, h := range snap.hexTiers[HexResolutionKM].Hexes {
		if v, ok := h.Values["P2"]; ok {
			p2 = append(p2, v)
		}
		if v, ok := h.Values["temperature"]; ok {
			temp = append(temp, v)
		}
	}
	if len(p2) != 1 || p2[0] != 10 {
		t.Errorf("hex P2 values = %v, want [10]", p2)
	}
	if len(temp) != 1 || temp[0] != 20 {
		t.Errorf("hex temperature values = %v, want [20]", temp)
	}
	for src, per := range snap.coverage {
		if per["P2"] != 1 {
			t.Errorf("coverage[%s][P2] = %d, want 1", src, per["P2"])
		}
	}
	if _, ok := sensors[1].Values["P2"]; !ok {
		t.Error("buildHexes removed P2 from the shared sensor row")
	}
}
