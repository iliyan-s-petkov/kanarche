package bathing

import (
	"context"
	"log/slog"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/store"
)

// retryAfter is the wait after a failed import, capped by refresh_interval.
const retryAfter = time.Hour

// watchEvery is the gap between new-edition probes. Imports run more often.
const watchEvery = 7 * 24 * time.Hour

// Sink is the store side of an import.
type Sink interface {
	ReplaceBathing(ctx context.Context, d store.BathingData, at time.Time) error
	BathingLastImport(ctx context.Context) (time.Time, bool, error)
	// BathingSupplementEdition is the snapshot edition of the newest import, empty if none.
	BathingSupplementEdition(ctx context.Context) (string, error)
}

// Stats is one RunOnce outcome.
type Stats struct {
	Sites, Classes, Samples int
	Skipped                 Skipped
	Supplement              SupplementStats
}

type Collector struct {
	cfg    config.Sea
	client *Client
	sink   Sink
	clock  func() time.Time
	sup    *Supplement
	// watch probes for a newer Datahub edition. Nil disables it.
	watch     func(context.Context) error
	lastWatch time.Time
}

func NewCollector(cfg config.Sea, sink Sink) *Collector {
	return &Collector{cfg: cfg, client: New(cfg), sink: sink, clock: time.Now}
}

// SetClockForTesting overrides the import timestamp clock.
func (c *Collector) SetClockForTesting(clock func() time.Time) { c.clock = clock }

// SetSupplement sets the class fill. Nil, the default, imports Discodata only.
func (c *Collector) SetSupplement(sup *Supplement) { c.sup = sup }

// SetEditionWatch sets the new-edition probe. Nil, the default, disables it.
func (c *Collector) SetEditionWatch(fn func(context.Context) error) { c.watch = fn }

// WatchEdition runs the probe if the last one is a week old or more. A failed
// probe counts, so an outage is retried next week and is only logged. It never
// blocks the import.
func (c *Collector) WatchEdition(ctx context.Context) {
	if c.watch == nil {
		return
	}
	now := c.clock()
	if !c.lastWatch.IsZero() && now.Sub(c.lastWatch) < watchEvery {
		return
	}
	c.lastWatch = now
	if err := c.watch(ctx); err != nil {
		slog.Warn("sea datahub watch failed", "error", err)
	}
}

// RunOnce fetches, validates and replaces the stored set. Nothing is written on failure.
func (c *Collector) RunOnce(ctx context.Context) (Stats, error) {
	raw, err := c.client.Fetch(ctx)
	if err != nil {
		return Stats{}, err
	}
	d, sk, err := Build(raw, c.cfg.Country)
	if err != nil {
		return Stats{}, err
	}
	d, ss := Merge(d, c.sup, Guards{MaxDisagree: c.cfg.Datahub.MaxDisagree, MinSiteCoverage: c.cfg.Datahub.MinSiteCoverage})
	st := Stats{Sites: len(d.Sites), Classes: len(d.Classes), Samples: len(d.Samples), Skipped: sk, Supplement: ss}
	return st, c.sink.ReplaceBathing(ctx, d, c.clock().UTC())
}

// NextDelay is how long to wait before the next import: none if never imported
// or overdue, the rest of interval otherwise, never more than interval.
func NextDelay(now, last time.Time, hasLast bool, interval time.Duration) time.Duration {
	if !hasLast {
		return 0
	}
	d := last.Add(interval).Sub(now)
	if d < 0 {
		return 0
	}
	return min(d, interval)
}

// Loop imports when due, then every refresh_interval, until ctx is done.
// The first wait counts from the last stored import, so a redeploy does not re-fetch.
func (c *Collector) Loop(ctx context.Context) {
	if !c.cfg.Enabled {
		slog.Info("sea import disabled")
		return
	}
	last, ok, err := c.sink.BathingLastImport(ctx)
	if err != nil {
		slog.Error("sea import: read last import", "error", err)
	}
	wait := NextDelay(c.clock(), last, ok, c.cfg.RefreshInterval)
	// A new snapshot edition is imported at once instead of waiting out the interval.
	if c.sup != nil && wait > 0 {
		ed, err := c.sink.BathingSupplementEdition(ctx)
		if err != nil {
			slog.Error("sea import: read supplement edition", "error", err)
		} else if ed != c.sup.Edition {
			slog.Info("sea import: supplement edition changed", "stored", ed, "embedded", c.sup.Edition)
			wait = 0
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		st, err := c.RunOnce(ctx)
		if err != nil {
			slog.Error("sea import failed", "error", err)
			wait = min(retryAfter, c.cfg.RefreshInterval)
			continue
		}
		slog.Info("sea import complete", "sites", st.Sites, "classes", st.Classes, "samples", st.Samples,
			"retired", st.Skipped.Retired, "invalid", st.Skipped.Invalid, "orphan", st.Skipped.Orphan,
			"supplement_applied", st.Supplement.Applied, "supplement_shadowed", st.Supplement.Shadowed,
			"supplement_inactive", st.Supplement.Inactive)

		c.WatchEdition(ctx)
		wait = c.cfg.RefreshInterval
	}
}
