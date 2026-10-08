package cloudflare

import (
	"context"
	"log/slog"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/store"
)

// minLookbackDays always re-checks a few recent days for late corrections.
// maxLookbackDays bounds a gap-fill to the retention Cloudflare actually has.
const minLookbackDays = 3
const maxLookbackDays = 30

// Stats is one RunOnce cycle's outcome.
type Stats struct {
	Points   int
	Written  int
	Backfill bool
}

type Collector struct {
	cfg    config.Cloudflare
	token  string
	client *Client
	store  *store.Store
	clock  func() time.Time
}

// NewCollector takes the token directly so the empty-token "do nothing"
// branch lives in Loop, not behind an env lookup here.
func NewCollector(cfg config.Cloudflare, token string, s *store.Store) *Collector {
	return &Collector{cfg: cfg, token: token, client: New(cfg, token), store: s, clock: time.Now}
}

// SetClockForTesting overrides the clock RunOnce uses for "today" and fetched_at.
func (c *Collector) SetClockForTesting(clock func() time.Time) { c.clock = clock }

// window sizes the pull to the gap since the last stored day, clamped to
// [minLookbackDays, maxLookbackDays]. No prior day means the full window.
func window(now, maxDay time.Time, hasMaxDay bool) (since, until time.Time) {
	until = truncateDay(now)
	days := maxLookbackDays
	if hasMaxDay {
		gap := int(until.Sub(truncateDay(maxDay)).Hours()/24) + minLookbackDays
		days = clampDays(gap, minLookbackDays, maxLookbackDays)
	}
	since = until.AddDate(0, 0, -(days - 1))
	return since, until
}

func clampDays(d, lo, hi int) int {
	if d < lo {
		return lo
	}
	if d > hi {
		return hi
	}
	return d
}

// truncateDay normalizes to the UTC calendar date, regardless of the clock's zone.
func truncateDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// RunOnce fetches the gap-sized window and upserts it. Called with an empty
// token this sends an unauthenticated request; callers must route through
// Loop, which never calls RunOnce with an empty token.
func (c *Collector) RunOnce(ctx context.Context) (Stats, error) {
	var st Stats

	maxDay, hasMaxDay, err := c.store.VisitorDailyMaxDay(ctx)
	if err != nil {
		return st, err
	}
	st.Backfill = !hasMaxDay

	since, until := window(c.clock(), maxDay, hasMaxDay)
	points, err := c.client.FetchDaily(ctx, since, until)
	if err != nil {
		return st, err
	}
	st.Points = len(points)

	fetchedAt := c.clock().UTC()
	rows := make([]store.VisitorDaily, len(points))
	for i, p := range points {
		rows[i] = store.VisitorDaily{
			Day:       p.Date,
			Uniques:   p.Uniques,
			Requests:  p.Requests,
			PageViews: p.PageViews,
			FetchedAt: fetchedAt,
		}
	}
	n, err := c.store.UpsertVisitorDaily(ctx, rows)
	st.Written = int(n)
	return st, err
}

// Loop runs RunOnce immediately, then on cfg.PollInterval, until ctx is done.
// With no token it logs once and returns: the server must start fine without
// the secret, which prod will not have until it is added to Infisical.
func (c *Collector) Loop(ctx context.Context) {
	if c.token == "" {
		slog.Info("cloudflare analytics token not set; visitor_daily job disabled", "env", TokenEnv)
		return
	}

	run := func() {
		s, err := c.RunOnce(ctx)
		if err != nil {
			slog.Error("cloudflare visitor_daily cycle failed", "error", err)
			return
		}
		slog.Info("cloudflare visitor_daily cycle complete",
			"points", s.Points, "written", s.Written, "backfill", s.Backfill)
	}

	run()
	t := time.NewTicker(c.cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}
