package config

import (
	"log/slog"
	"os"
	"strings"
)

const (
	legacyEnvPrefix = "AIRBG_"
	envPrefix       = "KANARCHE_"
)

// LookupEnv reads an AIRBG_* key, preferring the KANARCHE_* spelling when set.
// A value found only under the AIRBG_ name logs a deprecation warning.
func LookupEnv(key string) (string, bool) {
	suffix := strings.TrimPrefix(key, legacyEnvPrefix)
	if v, ok := os.LookupEnv(envPrefix + suffix); ok {
		return v, true
	}
	v, ok := os.LookupEnv(legacyEnvPrefix + suffix)
	if ok {
		slog.Warn("deprecated env prefix", "key", legacyEnvPrefix+suffix, "use", envPrefix+suffix)
	}
	return v, ok
}

// Getenv is LookupEnv that returns "" for unset keys, like os.Getenv.
func Getenv(key string) string {
	v, _ := LookupEnv(key)
	return v
}
