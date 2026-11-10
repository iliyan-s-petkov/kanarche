package snapshot

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"kanarche.eu/internal/store"
)

// WHODayPM25 is the WHO 2021 24-hour PM2.5 guideline in µg/m³.
const WHODayPM25 = 15

// whoDaysWindow is how far back the daily series reaches, the same window as the 1y chart.
const whoDaysWindow = 365 * 24 * time.Hour

// WHODays is a year of daily PM2.5 means for one area against the WHO guideline.
// Days are the UTC buckets of the 1y chart series.
type WHODays struct {
	DaysWithData int
	// DaysOver counts days whose mean is strictly above WHODayPM25.
	DaysOver  int
	WorstDay  time.Time
	WorstMean float64
}

// summariseWHODays reduces a daily series to its WHODays, or nil when no full day remains.
// The bucket of the current UTC day and any bucket starting before the window's first full UTC day are partial and skipped.
func summariseWHODays(points []store.Point, since, now time.Time) *WHODays {
	firstFull := since.UTC().Truncate(24 * time.Hour)
	if firstFull.Before(since) {
		firstFull = firstFull.Add(24 * time.Hour)
	}
	today := now.UTC().Truncate(24 * time.Hour)
	var out *WHODays
	for _, p := range points {
		if p.Time.Before(firstFull) || !p.Time.Before(today) {
			continue
		}
		if out == nil {
			out = &WHODays{WorstDay: p.Time, WorstMean: p.Value}
		}
		out.DaysWithData++
		if p.Value > WHODayPM25 {
			out.DaysOver++
		}
		// Strict, so the earliest of equal worst days is kept.
		if p.Value > out.WorstMean {
			out.WorstDay, out.WorstMean = p.Time, p.Value
		}
	}
	return out
}

// whoDaysRetry is how long a failed refresh blocks the next attempt.
const whoDaysRetry = 30 * time.Minute

// whoDaysFetchTimeout caps one refresh, above the statement's own 120 s timeout.
const whoDaysFetchTimeout = 3 * time.Minute

// whoDailyFetch reads the year of daily P2 means for every area.
type whoDailyFetch func(ctx context.Context, since time.Time) (map[string][]store.Point, error)

// whoDaysCache holds the per-slug WHO result across snapshot builds.
// It refreshes at most once per UTC day, in a goroutine, so a build never waits on it.
type whoDaysCache struct {
	mu       sync.Mutex
	day      time.Time
	bySlug   map[string]WHODays
	lastFail time.Time
	running  bool
	wg       sync.WaitGroup
}

// apply copies the cached result into snap and starts a refresh when the cache is from an earlier UTC day.
func (c *whoDaysCache) apply(ctx context.Context, snap *Snapshot, now time.Time, fetch whoDailyFetch) {
	today := now.UTC().Truncate(24 * time.Hour)
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running && c.day.Before(today) && (c.lastFail.IsZero() || now.Sub(c.lastFail) >= whoDaysRetry) {
		c.running = true
		c.wg.Add(1)
		go c.refresh(context.WithoutCancel(ctx), now, fetch)
	}
	for slug, meta := range snap.KnownSlugs {
		if w, ok := c.bySlug[slug]; ok {
			meta.WHODays = &w
			snap.KnownSlugs[slug] = meta
		}
	}
}

func (c *whoDaysCache) refresh(ctx context.Context, now time.Time, fetch whoDailyFetch) {
	defer c.wg.Done()
	ctx, cancel := context.WithTimeout(ctx, whoDaysFetchTimeout)
	defer cancel()
	start := time.Now()
	since := now.Add(-whoDaysWindow)
	series, err := fetch(ctx, since)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	if err != nil {
		c.lastFail = now
		slog.Warn("snapshot: days over WHO guideline unavailable", "error", err)
		return
	}
	by := make(map[string]WHODays, len(series))
	for slug, pts := range series {
		if sum := summariseWHODays(pts, since, now); sum != nil {
			by[slug] = *sum
		}
	}
	c.bySlug, c.day, c.lastFail = by, now.UTC().Truncate(24*time.Hour), time.Time{}
	slog.Info("snapshot: days over WHO guideline refreshed", "areas", len(by), "duration", time.Since(start))
}

// buildWHODays sets AreaMeta.WHODays from the daily cache; the query itself runs off the build path.
// The query has no per-day hour count, so a day counts however many hours it covers.
func buildWHODays(ctx context.Context, s *store.Store, h *Holder, snap *Snapshot, now time.Time) {
	h.who.apply(ctx, snap, now, func(ctx context.Context, since time.Time) (map[string][]store.Point, error) {
		return s.AllAreaDailySeries(ctx, "P2", since, 24*time.Hour)
	})
}
