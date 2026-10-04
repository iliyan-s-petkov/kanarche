package web_test

import (
	"strings"
	"testing"
	"time"

	"airbg.org/internal/snapshot"
	"airbg.org/internal/store"
)

// pollenSnap installs a sofia table built from rows, as Build would.
func pollenSnap(t *testing.T, rows []store.AreaPollenDay) *snapshot.Snapshot {
	t.Helper()
	snap := fixture(t)
	cfg := testConfig(t).Pollen
	days := []string{"2026-08-09", "2026-08-10", "2026-08-11"}
	v := snapshot.PollenViewForTesting(rows, cfg, days)["sofia"]
	if v == nil {
		t.Fatal("no view for sofia")
	}
	if err := snap.SetPollenForTesting("sofia", v, cfg, time.Date(2026, 8, 9, 9, 20, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestAreaPageRendersThePollenTableAndChip(t *testing.T) {
	rr := renderer(t, pollenSnap(t, []store.AreaPollenDay{
		{Slug: "sofia", Species: "ragweed", Day: "2026-08-09", Mean: 12, Max: 30},
		{Slug: "sofia", Species: "grass", Day: "2026-08-10", Mean: 3, Max: 5},
	}))
	cases := map[string][]string{
		"/en/area/sofia": {"Ragweed", "Moderate", "Pollen today: moderate · Ragweed", "Tomorrow", "12.0"},
		"/area/sofia":    {"Амброзия", "Умерен", "Прашец днес: умерен · Амброзия", "Утре", "12,0"},
	}
	for path, wants := range cases {
		body := fetch(t, rr, path).Body.String()
		for _, w := range []string{`<section id="pollen"`, `<table class="pollen-table"`, `href="#pollen"`, `href="https://open-meteo.com/"`, "Copernicus"} {
			if !strings.Contains(body, w) {
				t.Errorf("%s: missing %q", path, w)
			}
		}
		for _, w := range wants {
			if !strings.Contains(body, w) {
				t.Errorf("%s: missing %q", path, w)
			}
		}
	}
}

// Today's forecast is below every bound: the chip says so instead of naming a species.
func TestPollenChipIsQuietOffSeason(t *testing.T) {
	rr := renderer(t, pollenSnap(t, []store.AreaPollenDay{
		{Slug: "sofia", Species: "birch", Day: "2026-08-09", Mean: 0.01, Max: 0.1},
	}))
	body := fetch(t, rr, "/en/area/sofia").Body.String()
	if !strings.Contains(body, "Pollen today: very little") {
		t.Error("off-season chip missing")
	}
	if strings.Contains(body, "· Birch") {
		t.Error("off-season chip names a species")
	}
}

func TestAreaPageWithoutPollenHasNoTableOrChip(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/en/area/sofia").Body.String()
	if strings.Contains(body, `id="pollen"`) || strings.Contains(body, `href="#pollen"`) {
		t.Error("pollen section or chip rendered without a forecast")
	}
}

// Tomorrow only: the table shows, the chip does not.
func TestPollenChipNeedsToday(t *testing.T) {
	rr := renderer(t, pollenSnap(t, []store.AreaPollenDay{
		{Slug: "sofia", Species: "grass", Day: "2026-08-10", Mean: 40, Max: 50},
	}))
	body := fetch(t, rr, "/en/area/sofia").Body.String()
	if !strings.Contains(body, `id="pollen"`) {
		t.Error("table missing")
	}
	if strings.Contains(body, `href="#pollen"`) {
		t.Error("chip rendered without today's value")
	}
}
