package server

import (
	"context"
	"log/slog"
	"time"
)

// RetryConfig tunes RetryInitialPublish. Zero durations take the defaults.
type RetryConfig struct {
	Initial time.Duration // first wait, doubled after each failure
	Max     time.Duration // cap on the wait
	Budget  time.Duration // give up after this much time since the call
	Publish func(ctx context.Context, now time.Time) error
	Loaded  func() bool // true once any snapshot is being served
	Now     func() time.Time
	// Sleep waits d and reports false if ctx ended first.
	Sleep func(ctx context.Context, d time.Duration) bool
	Log   *slog.Logger
}

// RetryInitialPublish re-runs the startup publish until one succeeds, a
// snapshot appears from elsewhere (the regular cycle), ctx ends, or the budget
// runs out. The startup attempt in main counts as attempt 1.
func RetryInitialPublish(ctx context.Context, c RetryConfig) {
	if c.Initial <= 0 {
		c.Initial = 2 * time.Second
	}
	if c.Max <= 0 {
		c.Max = 15 * time.Second
	}
	if c.Budget <= 0 {
		c.Budget = 5 * time.Minute
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Sleep == nil {
		c.Sleep = sleepCtx
	}
	start := c.Now()
	wait := c.Initial
	for attempt := 2; ; attempt++ {
		if !c.Sleep(ctx, wait) {
			return
		}
		// Checked after the sleep: the cycle may have published meanwhile.
		if c.Loaded() {
			return
		}
		if c.Now().Sub(start) >= c.Budget {
			c.Log.Warn("startup publish retry gave up; the regular cycle takes over",
				"attempts", attempt-1)
			return
		}
		if err := c.Publish(ctx, c.Now().UTC()); err != nil {
			if ctx.Err() != nil {
				return
			}
			c.Log.Warn("startup publish retry failed", "attempt", attempt, "error", err)
			wait = min(wait*2, c.Max)
			continue
		}
		c.Log.Info("startup publish recovered",
			"attempt", attempt, "after", c.Now().Sub(start).String())
		return
	}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
