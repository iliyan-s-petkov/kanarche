package snapshot

import (
	"encoding/json"
	"testing"
	"time"

	"airbg.org/internal/config"
	"airbg.org/internal/store"
)

func pollenTestConfig() config.Pollen {
	return config.Pollen{
		Enabled: true, Domain: "cams_europe", DaysShown: 3,
		Species: []config.PollenSpecies{
			{Name: "ragweed", Levels: []float64{0.2, 10, 30, 100}},
			{Name: "grass", Levels: []float64{0.2, 10, 30, 100}},
			{Name: "birch", Levels: []float64{0.2, 10, 100, 500}},
		},
	}
}

func TestPollenLevelCountsTheBoundsReached(t *testing.T) {
	bounds := []float64{0.2, 10, 30, 100}
	for _, tc := range []struct {
		v    float64
		want string
	}{
		{0, "none"}, {0.19, "none"}, {0.2, "low"}, {9.9, "low"}, {10, "moderate"},
		{29.9, "moderate"}, {30, "high"}, {100, "very_high"}, {5000, "very_high"},
	} {
		if got := pollenLevel(tc.v, bounds); got != tc.want {
			t.Errorf("pollenLevel(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestPollenViewsBuildsTheTableAndSummary(t *testing.T) {
	days := []string{"2026-10-04", "2026-10-05", "2026-10-06"}
	rows := []store.AreaPollenDay{
		{Slug: "sofia", Species: "ragweed", Day: "2026-10-04", Mean: 12.34, Max: 40},
		{Slug: "sofia", Species: "grass", Day: "2026-10-04", Mean: 45, Max: 80},
		{Slug: "sofia", Species: "ragweed", Day: "2026-10-05", Mean: 0.1, Max: 0.3},
		// Outside the days shown: ignored.
		{Slug: "sofia", Species: "ragweed", Day: "2026-10-07", Mean: 900, Max: 900},
		// Not configured: ignored.
		{Slug: "sofia", Species: "olive", Day: "2026-10-04", Mean: 900, Max: 900},
	}
	views := pollenViews(rows, pollenTestConfig(), days)
	v := views["sofia"]
	if v == nil {
		t.Fatal("no view for sofia")
	}
	if len(v.Species) != 3 || v.Species[0].Name != "ragweed" || v.Species[2].Name != "birch" {
		t.Fatalf("species = %+v, want config order ragweed, grass, birch", v.Species)
	}
	rw := v.Species[0].Days
	if len(rw) != 3 || rw[0].Level != "moderate" || rw[0].Mean != 12.3 || rw[1].Level != "none" || rw[2].Has {
		t.Errorf("ragweed days = %+v", rw)
	}
	if v.Species[2].Days[0].Has {
		t.Errorf("birch has no rows but its day 0 = %+v", v.Species[2].Days[0])
	}
	if v.Summary == nil || v.Summary.Level != "high" || v.Summary.Species != "grass" || v.Summary.Date != "2026-10-04" {
		t.Errorf("summary = %+v, want high from grass on 2026-10-04", v.Summary)
	}
}

// A summary names a species only when something is in the air; an all-none
// day is a summary of "none", and a day with no data at all is no summary.
func TestPollenSummaryEdges(t *testing.T) {
	days := []string{"2026-10-04", "2026-10-05", "2026-10-06"}
	quiet := pollenViews([]store.AreaPollenDay{
		{Slug: "a", Species: "ragweed", Day: "2026-10-04", Mean: 0.01},
		{Slug: "a", Species: "grass", Day: "2026-10-04", Mean: 0},
	}, pollenTestConfig(), days)["a"]
	if quiet.Summary == nil || quiet.Summary.Level != "none" || quiet.Summary.Species != "" {
		t.Errorf("quiet summary = %+v, want none with no species", quiet.Summary)
	}
	later := pollenViews([]store.AreaPollenDay{
		{Slug: "b", Species: "ragweed", Day: "2026-10-05", Mean: 50},
	}, pollenTestConfig(), days)["b"]
	if later.Summary != nil {
		t.Errorf("summary = %+v, want nil when today has no data", later.Summary)
	}
}

// Ties go to the first species in config order, so the chip does not flicker.
func TestPollenSummaryTieGoesToConfigOrder(t *testing.T) {
	days := []string{"2026-10-04"}
	v := pollenViews([]store.AreaPollenDay{
		{Slug: "a", Species: "grass", Day: "2026-10-04", Mean: 15},
		{Slug: "a", Species: "ragweed", Day: "2026-10-04", Mean: 12},
	}, pollenTestConfig(), days)["a"]
	if v.Summary.Species != "ragweed" {
		t.Errorf("summary species = %q, want ragweed (first in config at the same level)", v.Summary.Species)
	}
}

func TestPollenPayloadCarriesAttributionAndNulls(t *testing.T) {
	days := []string{"2026-10-04", "2026-10-05"}
	cfg := pollenTestConfig()
	v := pollenViews([]store.AreaPollenDay{{Slug: "sofia", Species: "ragweed", Day: "2026-10-04", Mean: 3, Max: 9}}, cfg, days)["sofia"]
	fetched := time.Date(2026, 10, 4, 9, 20, 0, 0, time.UTC)
	body, err := encode(pollenPayloadFrom(time.Now(), "sofia", fetched, cfg, days, v))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Forecast    bool   `json:"forecast"`
		Unit        string `json:"unit"`
		Domain      string `json:"domain"`
		Attribution struct {
			Text string `json:"text"`
			URL  string `json:"url"`
		} `json:"attribution"`
		Species []struct {
			Name string `json:"name"`
			Days []struct {
				Level *string  `json:"level"`
				Mean  *float64 `json:"mean"`
			} `json:"days"`
		} `json:"species"`
	}
	if err := json.Unmarshal(body.JSON, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Forecast || got.Unit != "grains/m³" || got.Domain != "cams_europe" {
		t.Errorf("payload = %+v, want a forecast in grains/m³ from cams_europe", got)
	}
	if got.Attribution.URL != "https://open-meteo.com/" || got.Attribution.Text == "" {
		t.Errorf("attribution = %+v, want text and the Open-Meteo URL", got.Attribution)
	}
	d := got.Species[0].Days
	if d[0].Level == nil || *d[0].Level != "low" || d[1].Level != nil || d[1].Mean != nil {
		t.Errorf("ragweed days = %+v, want low then nulls", d)
	}
}

func TestPollenDaysAreLocal(t *testing.T) {
	sofia, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Skip("no tzdata")
	}
	// 22:30 UTC on the 3rd is 01:30 on the 4th in Sofia.
	now := time.Date(2026, 10, 3, 22, 30, 0, 0, time.UTC)
	days, from, to := pollenDays(now, sofia, 3)
	if len(days) != 3 || days[0] != "2026-10-04" || days[2] != "2026-10-06" {
		t.Errorf("days = %v", days)
	}
	if !from.Equal(time.Date(2026, 10, 3, 21, 0, 0, 0, time.UTC)) {
		t.Errorf("from = %v, want local midnight of the 4th", from)
	}
	if !to.Equal(time.Date(2026, 10, 6, 21, 0, 0, 0, time.UTC)) {
		t.Errorf("to = %v, want local midnight of the 7th", to)
	}
	// The 25th of October 2026 has 25 hours in Sofia: the span is by calendar.
	_, from, to = pollenDays(time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC), sofia, 2)
	if to.Sub(from) != 49*time.Hour {
		t.Errorf("span over the DST change = %v, want 49h", to.Sub(from))
	}
}

func TestNewHolderTakesPollenAsAnOption(t *testing.T) {
	series := config.Series{DefaultMetric: "P2", DefaultWindow: time.Hour}
	if h := NewHolder(series, config.Wind{}); h.pollen.Enabled {
		t.Error("pollen enabled without the option")
	}
	if h := NewHolder(series, config.Wind{}, WithPollen(pollenTestConfig())); !h.pollen.Enabled || h.pollenZone == nil {
		t.Errorf("WithPollen not applied: %+v", h.pollen)
	}
}
