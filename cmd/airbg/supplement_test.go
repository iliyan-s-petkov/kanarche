package main

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"kanarche.eu/internal/api"
	"kanarche.eu/internal/config"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func committedConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.LoadFileOffline(filepath.Join("..", "..", "airbg.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestLoadSupplementUsesTheEmbeddedSnapshot(t *testing.T) {
	log := captureLog(t)
	sup := loadSupplement(committedConfig(t))
	if sup == nil || len(sup.Classes) != 1795 || sup.Edition == "" {
		t.Fatalf("supplement = %+v", sup)
	}
	if log.Len() != 0 {
		t.Errorf("unexpected log: %s", log)
	}
}

// A bad embed turns the fill off and says so. Discodata is unaffected.
func TestLoadSupplementDisabledOnBadPin(t *testing.T) {
	log := captureLog(t)
	cfg := committedConfig(t)
	cfg.Sea.Datahub.SHA256 = strings.Repeat("0", 64)
	if sup := loadSupplement(cfg); sup != nil {
		t.Fatalf("supplement = %+v, want nil", sup)
	}
	for _, want := range []string{"level=ERROR", "sea supplement disabled", "invalid snapshot"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
}

func TestValidateConfigFailsOnBadSnapshotPin(t *testing.T) {
	cfgPath := writePinnedConfig(t, t.TempDir(), strings.Repeat("0", 64), 43096327)
	t.Setenv(config.PathEnv, cfgPath)
	t.Setenv(config.DatabaseURLEnv, "postgres://user:pass@localhost:5432/airbg")
	var out, errOut bytes.Buffer
	if code := runValidateConfig(&out, &errOut); code != 1 {
		t.Fatalf("exit %d, want 1; stdout:\n%s", code, out.String())
	}
	if !strings.Contains(errOut.String(), "invalid snapshot") {
		t.Errorf("stderr = %q, want the snapshot error", errOut.String())
	}
}

func TestSeaSupplementMetaComesFromTheSnapshotHeader(t *testing.T) {
	cfg := committedConfig(t)
	f, err := checkSnapshot(cfg)
	if err != nil {
		t.Fatal(err)
	}
	m := seaSupplementMeta(loadSupplement(cfg))
	if m.Published == "" || m.URL == "" || m.Published != f.Header.Published || m.URL != f.Header.SourceURL {
		t.Errorf("meta = %+v, header = %+v", m, f.Header)
	}
	if m := seaSupplementMeta(nil); m != (api.SeaSupplementMeta{}) {
		t.Errorf("nil supplement meta = %+v, want zero", m)
	}
}
