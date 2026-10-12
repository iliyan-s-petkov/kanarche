package testsupport_test

import (
	"context"
	"testing"

	"kanarche.eu/internal/db"
	"kanarche.eu/internal/testsupport"
)

// The retention policies run on the real clock, so a job that fires during a
// test drops fixtures dated more than a few days before today. The test
// container must not start background jobs.
func TestPostgresRunsNoBackgroundJobs(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var workers string
	if err := pool.QueryRow(ctx, `SELECT current_setting('timescaledb.max_background_workers')`).Scan(&workers); err != nil {
		t.Fatal(err)
	}
	if workers != "0" {
		t.Errorf("timescaledb.max_background_workers = %s, want 0", workers)
	}
	var ran int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM timescaledb_information.job_stats WHERE last_run_started_at IS NOT NULL`).Scan(&ran); err != nil {
		t.Fatal(err)
	}
	if ran != 0 {
		t.Errorf("%d background jobs ran after migrate, want 0", ran)
	}
}
