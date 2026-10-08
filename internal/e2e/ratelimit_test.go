//go:build e2e

package e2e

import (
	"path/filepath"
	"testing"

	"kanarche.eu/internal/config"
)

// The suite cold-loads every spec from one IP, and a cold load is dozens of
// /static requests. The pages bucket (airbg.yaml: 20/s, burst 300) drains
// faster than it refills on a fast runner, the 429s land on the map chunk, and
// mapSettled times out. Pin that the harness widens it like the API buckets.
func TestHarnessWidensPagesLimiter(t *testing.T) {
	t.Setenv(config.PathEnv, filepath.Join("..", "..", "airbg.yaml"))
	t.Setenv(config.DatabaseURLEnv, "postgres://user:pass@localhost:5432/airbg")
	widenRateLimits(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if got := cfg.RateLimit.Pages.PerSecond; got < 500 {
		t.Errorf("pages per_second = %v, want >= 500", got)
	}
	if got := cfg.RateLimit.Pages.Burst; got < 5000 {
		t.Errorf("pages burst = %v, want >= 5000", got)
	}
}
