package snapshot_test

import (
	"encoding/json"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/store"
)

// The overlay's whole risk is being read as measurement, so the fields that say
// otherwise are part of the contract, not decoration. See docs/wind-overlay.md.
func TestWindPayloadNamesItselfAForecast(t *testing.T) {
	now := time.Date(2026, 9, 5, 14, 20, 0, 0, time.UTC)
	validAt := time.Date(2026, 9, 5, 14, 0, 0, 0, time.UTC)

	body := snapshot.WindPayloadJSONForTesting(now, validAt, "ecmwf_ifs025", 0.25,
		[]store.WindVector{{Q: 0, R: 0, SpeedMS: 3.46, Direction: 270.4}})

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got["forecast"] != true {
		t.Errorf("forecast = %v, want true", got["forecast"])
	}
	if got["model"] != "ecmwf_ifs025" {
		t.Errorf("model = %v, want the model name", got["model"])
	}
	if got["model_resolution_deg"] != 0.25 {
		t.Errorf("model_resolution_deg = %v, want 0.25", got["model_resolution_deg"])
	}
	// The forecast hour is not the build time. A client showing generated_at as
	// the validity would claim the forecast is twenty minutes fresher than it is.
	if got["valid_at"] != "2026-09-05T14:00:00Z" {
		t.Errorf("valid_at = %v, want the forecast hour, not the build time", got["valid_at"])
	}
	if got["generated_at"] != "2026-09-05T14:20:00Z" {
		t.Errorf("generated_at = %v, want the build time", got["generated_at"])
	}
}

func TestWindVectorsCarryTheLatticeCentre(t *testing.T) {
	now := time.Now().UTC()
	body := snapshot.WindPayloadJSONForTesting(now, now, "m", 0.25,
		[]store.WindVector{{Q: 87, R: 171, SpeedMS: 3.46, Direction: 270.44}})

	var got struct {
		Vectors []struct {
			Lon, Lat, SpeedMS float64
			DirectionDeg      float64 `json:"direction_deg"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Vectors) != 1 {
		t.Fatalf("got %d vectors, want 1", len(got.Vectors))
	}
	// Lattice index (87, 171) is 21.75E 42.75N: the index is in 0.25 degree units.
	if got.Vectors[0].Lon != 21.75 || got.Vectors[0].Lat != 42.75 {
		t.Errorf("vector at index (87,171) = %v, %v; want 21.75, 42.75",
			got.Vectors[0].Lon, got.Vectors[0].Lat)
	}
	if got.Vectors[0].DirectionDeg != 270.4 {
		t.Errorf("direction_deg = %v, want 270.4 — one decimal is finer than the model",
			got.Vectors[0].DirectionDeg)
	}
}
