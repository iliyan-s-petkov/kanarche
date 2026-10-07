package main

import (
	"log/slog"
	"time"

	"airbg.org/internal/config"
	"airbg.org/internal/upstream/bathing"
	"airbg.org/internal/upstream/bathing/datahub"
)

func snapshotPins(cfg config.Config) datahub.Pins {
	return datahub.Pins{SHA256: cfg.Sea.Datahub.SHA256, Size: cfg.Sea.Datahub.Size, Country: cfg.Sea.Country}
}

// checkSnapshot validates the embedded snapshot against the config pins.
func checkSnapshot(cfg config.Config) (datahub.File, error) {
	return datahub.Load(snapshotPins(cfg), time.Now().UTC())
}

// loadSupplement returns the class fill for the sea import. A bad embedded
// snapshot disables the fill with an error log and returns nil: Discodata
// still imports and serve still starts.
func loadSupplement(cfg config.Config) *bathing.Supplement {
	f, err := checkSnapshot(cfg)
	if err != nil {
		slog.Error("sea supplement disabled, importing Discodata classes only", "error", err)
		return nil
	}
	return f.Supplement()
}
