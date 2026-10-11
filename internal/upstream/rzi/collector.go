package rzi

import (
	"context"
	"log/slog"
	"time"

	"kanarche.eu/internal/config"
)

// retryAfter is the wait after a pass that hit a store or disk error, capped
// by the interval.
const retryAfter = time.Hour

// Fetcher downloads one source into the inbox. *Fetcher satisfies it.
type Fetching interface {
	Fetch(ctx context.Context, id string, prior Validators) (Result, error)
}

// Collector fetches the RZI documents into the inbox and ingests whatever
// records the sandbox run has left in the outbox. It never runs the
// extractor: that is a host unit.
type Collector struct {
	cfg     config.SeaRZI
	fetcher Fetching
	ing     *Ingester
	clock   func() time.Time
}

func NewCollector(cfg config.SeaRZI, sink Sink, fetcher Fetching) *Collector {
	c := &Collector{cfg: cfg, fetcher: fetcher, clock: time.Now}
	c.ing = NewIngester(cfg, sink, func() time.Time { return c.clock() })
	return c
}

// SetClockForTesting overrides the clock for the schedule and file rows.
func (c *Collector) SetClockForTesting(clock func() time.Time) { c.clock = clock }

// Ingester is the ingest half, for the approve command.
func (c *Collector) Ingester() *Ingester { return c.ing }

// RunOnce fetches every source, then ingests the outbox and refreshes the
// metrics. A failed fetch is logged and does not stop the others.
func (c *Collector) RunOnce(ctx context.Context) (Stats, error) {
	for _, id := range Sources() {
		res, err := c.fetcher.Fetch(ctx, id, c.ing.Prior(ctx, id))
		if err != nil {
			slog.Warn("rzi fetch failed", "source", id, "error", err)
			continue
		}
		slog.Info("rzi fetch", "source", id, "status", res.Status)
		c.ing.NoteFetch(res)
	}
	st, err := c.ing.Process(ctx)
	if mErr := c.ing.PublishMetrics(ctx); mErr != nil {
		slog.Warn("rzi metrics", "error", mErr)
	}
	return st, err
}

// Interval is the wait between passes: in_season_interval from 15 May to
// 15 October inclusive by the Sofia date, off_season_interval otherwise.
func Interval(cfg config.SeaRZI, now time.Time) time.Duration {
	d := sofiaDate(now)
	start := time.Date(d.Year(), time.May, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(d.Year(), time.October, 15, 0, 0, 0, 0, time.UTC)
	if !d.Before(start) && !d.After(end) {
		return cfg.InSeasonInterval
	}
	return cfg.OffSeasonInterval
}

// Loop runs a pass at once, then waits Interval between passes, until ctx is
// done.
func (c *Collector) Loop(ctx context.Context) {
	if !c.cfg.Enabled {
		slog.Info("rzi ingest disabled")
		return
	}
	for {
		wait := Interval(c.cfg, c.clock())
		st, err := c.RunOnce(ctx)
		if err != nil {
			slog.Error("rzi pass failed", "error", err)
			wait = min(retryAfter, wait)
		} else {
			slog.Info("rzi pass complete", "ok", st.OK, "needs_review", st.NeedsReview,
				"rejected", st.Rejected, "duplicate", st.Duplicate, "refused", st.Refused)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}
