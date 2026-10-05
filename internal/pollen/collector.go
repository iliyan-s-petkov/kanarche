package pollen

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"time"

	"airbg.org/internal/config"
	"airbg.org/internal/store"
)

// Store is the part of *store.Store the collector uses.
type Store interface {
	PollenLattice(ctx context.Context, country string, stepC int, marginM float64) ([]store.PollenCell, error)
	WritePollen(ctx context.Context, rows []store.PollenForecast, fetchedAt time.Time) (int64, error)
	LatestPollenFetch(ctx context.Context) (time.Time, bool, error)
}

// Collector fetches the forecast for the lattice at fixed UTC times.
type Collector struct {
	cfg    config.Pollen
	client *Client
	store  Store
	clock  func() time.Time
}

func NewCollector(cfg config.Pollen, s Store) *Collector {
	return &Collector{cfg: cfg, client: New(cfg), store: s, clock: time.Now}
}

func (c *Collector) SetClockForTesting(clock func() time.Time) { c.clock = clock }

// RunOnce fetches and stores one model run. Returns the number of rows written.
func (c *Collector) RunOnce(ctx context.Context) (int64, error) {
	stepC := int(math.Round(c.cfg.LatticeDeg * 100))
	cells, err := c.store.PollenLattice(ctx, c.cfg.Country, stepC, c.cfg.LatticeMarginKm*1000)
	if err != nil {
		return 0, err
	}
	if len(cells) == 0 {
		return 0, errors.New("pollen: empty lattice; is the country boundary for pollen.country imported?")
	}
	rows, err := c.client.Fetch(ctx, cells)
	if err != nil {
		return 0, err
	}
	return c.store.WritePollen(ctx, rows, c.clock().UTC())
}

// NextRun is the first configured UTC time of day strictly after now.
func NextRun(now time.Time, times []time.Duration) time.Time {
	now = now.UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	for day := 0; day < 2; day++ {
		base := midnight.AddDate(0, 0, day)
		for _, d := range times {
			if t := base.Add(d); t.After(now) {
				return t
			}
		}
	}
	return midnight.AddDate(0, 0, 1).Add(times[0])
}

// DueOnStart says whether a restart should fetch now rather than wait for the
// next scheduled time; a restart loop must not hammer the free tier.
func DueOnStart(latest time.Time, ok bool, now time.Time, staleAfter time.Duration) bool {
	return !ok || now.Sub(latest) > staleAfter
}

// Loop runs at each configured time until ctx is done. A failed run is logged;
// the table keeps serving the last stored run until it ages out.
func (c *Collector) Loop(ctx context.Context) {
	run := func() {
		n, err := c.RunOnce(ctx)
		if err != nil {
			slog.Error("pollen cycle failed", "error", err)
			return
		}
		slog.Info("pollen cycle complete", "rows", n, "domain", c.cfg.Domain)
	}

	latest, ok, err := c.store.LatestPollenFetch(ctx)
	if err != nil {
		slog.Error("pollen latest fetch", "error", err)
	}
	if err != nil || DueOnStart(latest, ok, c.clock(), c.cfg.StaleAfter) {
		run()
	}
	times := c.cfg.RunTimes()
	for {
		wait := NextRun(c.clock(), times).Sub(c.clock())
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			run()
		}
	}
}
