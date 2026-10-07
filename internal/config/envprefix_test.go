package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

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
