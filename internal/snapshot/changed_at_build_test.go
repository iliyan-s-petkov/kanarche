package snapshot_test

import (
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

// A Build with a fresh holder is a restart: only the database carries the
// change time across.
func TestBuildChangedAtSurvivesRestart(t *testing.T) {
	ctx, pool := migrated(t)
	seed(t, ctx, pool)
	st := testStore(t, pool)

	t0 := time.Now().UTC().Truncate(time.Second)
	first, err := snapshot.Build(ctx, st, testHolder(t), t0)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := first.KnownSlugs["sofia"].ChangedAt; !got.Equal(t0) {
		t.Fatalf("first build: got %v, want %v", got, t0)
	}

	t1 := t0.Add(48 * time.Hour)
	second, err := snapshot.Build(ctx, st, testHolder(t), t1)
	if err != nil {
		t.Fatalf("Build after restart: %v", err)
	}
	if got := second.KnownSlugs["sofia"].ChangedAt; !got.Equal(t0) {
		t.Errorf("unchanged data after restart: got %v, want kept %v", got, t0)
	}

	if _, err := pool.Exec(ctx, `UPDATE reading SET value = value + 40`); err != nil {
		t.Fatalf("change readings: %v", err)
	}
	t2 := t1.Add(time.Hour)
	third, err := snapshot.Build(ctx, st, testHolder(t), t2)
	if err != nil {
		t.Fatalf("Build after change: %v", err)
	}
	if got := third.KnownSlugs["sofia"].ChangedAt; !got.Equal(t2) {
		t.Errorf("changed data: got %v, want %v", got, t2)
	}
}
