package main

import (
	"context"
	"log/slog"
	"time"

	"airbg.org/internal/api"
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

// seaSupplementMeta takes the header fields from the loaded supplement, so the
// snapshot is parsed once. A nil supplement yields empty fields.
func seaSupplementMeta(sup *bathing.Supplement) api.SeaSupplementMeta {
	if sup == nil {
		return api.SeaSupplementMeta{}
	}
	return api.SeaSupplementMeta{Published: sup.Published, URL: sup.URL}
}

// editionWatch probes the Datahub for a newer edition than the pinned one.
func editionWatch(cfg config.Config) func(context.Context) error {
	d := cfg.Sea.Datahub
	fc := datahub.FetchConfig{URL: d.URL, AllowedHosts: d.AllowedHosts, Timeout: d.RequestTimeout}
	return func(ctx context.Context) error {
		_, err := datahub.Watch(ctx, fc)
		return err
	}
}
