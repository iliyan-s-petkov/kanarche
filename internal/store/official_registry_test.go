package store_test

import (
	"reflect"
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

// A station whose last reading is days old drops out of LatestSensors but is still a station inside the boundary.
func TestOfficialStationsListsActiveStationsRegardlessOfFreshness(t *testing.T) {
	ctx, pool, s := newStore(t)
	seedArea(t, ctx, pool, "sofia", "city", 23.32, 42.69)

	ids, err := s.UpsertStations(ctx, []store.StationUpsert{
		{SourceRef: "BG/SPO-BG0050A_00008_100", Code: "BG0050A", Name: "Дружба", Lon: 23.32, Lat: 42.69, LastSeen: time.Now().UTC()},
		{SourceRef: "BG/SPO-BG0052A_00008_100", Code: "BG0052A", Name: "Надежда", Lon: 23.321, Lat: 42.691, LastSeen: time.Now().UTC()},
	})
	if err != nil {
		t.Fatal(err)
	}
	stale := time.Now().UTC().Add(-72 * time.Hour)
	if _, err := s.WriteStationReadings(ctx, []store.StationReading{
		{SensorID: ids["BG/SPO-BG0050A_00008_100"], Metric: "P2", Value: 12, Timestamp: stale, Quality: "ok"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sensor SET active = false WHERE source_ref = $1`, "BG/SPO-BG0052A_00008_100"); err != nil {
		t.Fatal(err)
	}
	assignAreas(t, ctx, pool)

	live, err := s.LatestSensors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, sr := range live {
		if sr.StationCode == "BG0050A" {
			t.Fatalf("LatestSensors returned the stale station; the test premise is broken")
		}
	}

	got, err := s.OfficialStations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("OfficialStations = %+v, want only the active BG0050A", got)
	}
	g := got[0]
	if g.Source != "eea" || g.StationCode != "BG0050A" || g.SensorID != ids["BG/SPO-BG0050A_00008_100"] {
		t.Errorf("station = %+v, want eea BG0050A", g)
	}
	if !reflect.DeepEqual(g.AreaSlugs, []string{"sofia"}) {
		t.Errorf("AreaSlugs = %v, want [sofia]", g.AreaSlugs)
	}
	if g.Lon != 23.32 || g.Lat != 42.69 {
		t.Errorf("position = %v,%v, want 23.32,42.69", g.Lon, g.Lat)
	}
}
