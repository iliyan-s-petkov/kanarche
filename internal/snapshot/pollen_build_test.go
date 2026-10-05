package snapshot_test

import (
	"testing"
	"time"

	"airbg.org/internal/config"
	"airbg.org/internal/snapshot"
	"airbg.org/internal/store"
)

func TestBuildServesPollenPerAreaAndCarriesItForward(t *testing.T) {
	ctx, pool := migrated(t)
	seed(t, ctx, pool)
	st := testStore(t, pool)
	cfg := testConfig(t)

	now := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	from := time.Date(2026, 10, 3, 21, 0, 0, 0, time.UTC)
	rows := make([]store.PollenForecast, 0, 72)
	for i := 0; i < 72; i++ {
		rows = append(rows, store.PollenForecast{LonC: 2340, LatC: 4270, Species: "ragweed", ValidAt: from.Add(time.Duration(i) * time.Hour), Grains: 40})
	}
	if _, err := st.WritePollen(ctx, rows, now.Add(-time.Hour)); err != nil {
		t.Fatalf("WritePollen: %v", err)
	}

	h := snapshot.NewHolder(cfg.Series, config.Wind{}, snapshot.WithPollen(cfg.Pollen))
	snap, err := snapshot.Build(ctx, st, h, now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	body, ok := snap.PollenBody("sofia")
	if !ok || len(body.Gzip) == 0 || body.ETag == "" {
		t.Fatalf("PollenBody(sofia) = %v, %v; want a prepared body", len(body.JSON), ok)
	}
	v := snap.Pollen("sofia")
	if v == nil || v.Summary == nil || v.Summary.Level != "high" || v.Summary.Species != "ragweed" {
		t.Fatalf("Pollen(sofia) summary = %+v, want high ragweed", v)
	}
	// plovdiv is far from the only cell.
	if _, ok := snap.PollenBody("plovdiv"); ok {
		t.Error("plovdiv has a pollen body with no cell within reach")
	}

	// Same run, same local day: reused, not re-read.
	h.Store(snap)
	if _, err := pool.Exec(ctx, `UPDATE pollen_forecast SET grains = 1`); err != nil {
		t.Fatal(err)
	}
	again, err := snapshot.Build(ctx, st, h, now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("Build again: %v", err)
	}
	if b, _ := again.PollenBody("sofia"); b.ETag != body.ETag {
		t.Error("pollen re-read although neither the run nor the local day changed")
	}
	// A new local day re-reads.
	h.Store(again)
	nextDay, err := snapshot.Build(ctx, st, h, time.Date(2026, 10, 4, 21, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Build next day: %v", err)
	}
	if v := nextDay.Pollen("sofia"); v == nil || v.Days[0] != "2026-10-05" || v.Summary.Level != "low" {
		t.Errorf("next-day view = %+v, want 2026-10-05 at low", v)
	}
}

func TestBuildWithoutPollenOptionServesNone(t *testing.T) {
	ctx, pool := migrated(t)
	seed(t, ctx, pool)
	snap, err := snapshot.Build(ctx, testStore(t, pool), testHolder(t), time.Now().UTC())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, ok := snap.PollenBody("sofia"); ok {
		t.Error("pollen body built without WithPollen")
	}
}
