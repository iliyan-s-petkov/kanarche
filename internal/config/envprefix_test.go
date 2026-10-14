package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Load and LoadOffline both resolve the config path through the prefix lookup,
// so an image that sets only KANARCHE_CONFIG works and AIRBG_CONFIG still does.
func TestConfigPathPrefixes(t *testing.T) {
	file := filepath.Join("..", "..", "airbg.yaml")
	for _, tt := range []struct{ name, set, other string }{
		{"kanarche only", "KANARCHE_CONFIG", "AIRBG_CONFIG"},
		{"legacy only", "AIRBG_CONFIG", "KANARCHE_CONFIG"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			captureWarn(t)
			t.Setenv(tt.other, "")
			os.Unsetenv(tt.other)
			t.Setenv(tt.set, file)
			if _, err := LoadOffline(); err != nil {
				t.Fatalf("LoadOffline error = %v, want nil", err)
			}
			t.Setenv(DatabaseURLEnv, "postgres://u:p@h/d")
			if _, err := Load(); err != nil {
				t.Fatalf("Load error = %v, want nil", err)
			}
		})
	}
}

// captureWarn routes slog output to a buffer for the duration of the test.
func captureWarn(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestLookupEnvKanarcheOnly(t *testing.T) {
	buf := captureWarn(t)
	t.Setenv("KANARCHE_X_KEY", "new")
	v, ok := LookupEnv("AIRBG_X_KEY")
	if !ok || v != "new" {
		t.Fatalf("got %q,%v", v, ok)
	}
	if buf.Len() != 0 {
		t.Fatalf("unexpected log: %s", buf)
	}
}

func TestLookupEnvLegacyOnlyWarns(t *testing.T) {
	buf := captureWarn(t)
	t.Setenv("AIRBG_X_KEY", "old")
	v, ok := LookupEnv("AIRBG_X_KEY")
	if !ok || v != "old" {
		t.Fatalf("got %q,%v", v, ok)
	}
	out := buf.String()
	if strings.Count(out, "deprecated env prefix") != 1 || !strings.Contains(out, "AIRBG_X_KEY") || !strings.Contains(out, "level=WARN") {
		t.Fatalf("log: %s", out)
	}
}

func TestLookupEnvKanarcheWins(t *testing.T) {
	buf := captureWarn(t)
	t.Setenv("AIRBG_X_KEY", "old")
	t.Setenv("KANARCHE_X_KEY", "new")
	v, ok := LookupEnv("AIRBG_X_KEY")
	if !ok || v != "new" {
		t.Fatalf("got %q,%v", v, ok)
	}
	if buf.Len() != 0 {
		t.Fatalf("unexpected log: %s", buf)
	}
}

func TestLookupEnvNeither(t *testing.T) {
	buf := captureWarn(t)
	if v, ok := LookupEnv("AIRBG_X_UNSET_KEY"); ok || v != "" {
		t.Fatalf("got %q,%v", v, ok)
	}
	if buf.Len() != 0 {
		t.Fatalf("unexpected log: %s", buf)
	}
}
