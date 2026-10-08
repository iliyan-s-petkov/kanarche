package eea

import (
	"path/filepath"
	"testing"

	"kanarche.eu/internal/config"
)

// The WARN threshold must track the window that drops EEA rows from the snapshot.
func TestStalenessWarnThresholdMatchesOfficialFreshnessWindow(t *testing.T) {
	t.Setenv(config.DatabaseURLEnv, "postgres://user:pass@localhost:5432/airbg")
	cfg, err := config.LoadFile(filepath.Join("..", "..", "..", "airbg.yaml"))
	if err != nil {
		t.Fatalf("LoadFile(airbg.yaml) error = %v", err)
	}
	if got, want := StalenessWarnThreshold, cfg.Store.OfficialFreshnessWindow; got != want {
		t.Errorf("StalenessWarnThreshold = %v, want store.official_freshness_window %v", got, want)
	}
}
