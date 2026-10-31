package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/snapshot"
)

// fakeClock advances only when the retry loop sleeps, so no test waits.
type fakeClock struct {
	mu     sync.Mutex
	t      time.Time
	sleeps []time.Duration
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Sleep(_ context.Context, d time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sleeps = append(c.sleeps, d)
	c.t = c.t.Add(d)
	return true
}

func retryCfg(clk *fakeClock, publish func(context.Context, time.Time) error, loaded func() bool) RetryConfig {
	return RetryConfig{
		Publish: publish,
		Loaded:  loaded,
		Now:     clk.Now,
		Sleep:   clk.Sleep,
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 10, 31, 14, 49, 0, 0, time.UTC)} }

func TestRetryPublishesAfterNFailures(t *testing.T) {
	clk := newClock()
	var calls int
	publish := func(context.Context, time.Time) error {
		calls++
		if calls <= 3 {
			return errors.New("the database system is starting up")
		}
		return nil
	}
	RetryInitialPublish(context.Background(), retryCfg(clk, publish, func() bool { return calls > 3 }))
	if calls != 4 {
		t.Fatalf("publish calls = %d, want 4 (3 failures then success)", calls)
	}
	// 2s doubling, then stops after the success.
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 15 * time.Second}
	if len(clk.sleeps) != len(want) {
		t.Fatalf("sleeps = %v, want %v", clk.sleeps, want[:len(clk.sleeps)])
	}
	for i := range want[:3] {
		if clk.sleeps[i] != want[i] {
			t.Errorf("sleep %d = %v, want %v", i, clk.sleeps[i], want[i])
		}
	}
}

func TestRetryBackoffIsCapped(t *testing.T) {
	clk := newClock()
	calls := 0
	publish := func(context.Context, time.Time) error {
		calls++
		if calls < 8 {
			return errors.New("down")
		}
		return nil
	}
	RetryInitialPublish(context.Background(), retryCfg(clk, publish, func() bool { return false }))
	for i, d := range clk.sleeps {
		if d > 15*time.Second {
			t.Errorf("sleep %d = %v exceeds the 15s cap", i, d)
		}
	}
	if last := clk.sleeps[len(clk.sleeps)-1]; last != 15*time.Second {
		t.Errorf("final sleep = %v, want the 15s cap", last)
	}
}

func TestRetryStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	clk := newClock()
	calls := 0
	cfg := retryCfg(clk, func(context.Context, time.Time) error {
		calls++
		cancel()
		return errors.New("down")
	}, func() bool { return false })
	RetryInitialPublish(ctx, cfg)
	if calls != 1 {
		t.Fatalf("publish calls = %d, want 1: no attempt after cancel", calls)
	}

	// Cancel during the sleep itself.
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	calls = 0
	cfg.Sleep = func(ctx context.Context, d time.Duration) bool { return sleepCtx(ctx, d) }
	RetryInitialPublish(ctx2, cfg)
	if calls != 0 {
		t.Fatalf("publish ran %d times on a cancelled context", calls)
	}
}

func TestRetryDoesNotPublishOnceSnapshotExists(t *testing.T) {
	clk := newClock()
	calls := 0
	RetryInitialPublish(context.Background(), retryCfg(clk,
		func(context.Context, time.Time) error { calls++; return nil },
		func() bool { return true }))
	if calls != 0 {
		t.Fatalf("publish calls = %d, want 0 when a snapshot is already served", calls)
	}
}

func TestRetryStopsWhenCycleSucceedsMidway(t *testing.T) {
	clk := newClock()
	var loaded atomic.Bool
	calls := 0
	publish := func(context.Context, time.Time) error {
		calls++
		if calls == 2 {
			loaded.Store(true) // the regular cycle publishes during the next wait
		}
		return errors.New("down")
	}
	RetryInitialPublish(context.Background(), retryCfg(clk, publish, loaded.Load))
	if calls != 2 {
		t.Fatalf("publish calls = %d, want 2: no attempt once a snapshot exists", calls)
	}
}

func TestRetryGivesUpAfterBudget(t *testing.T) {
	clk := newClock()
	calls := 0
	RetryInitialPublish(context.Background(), retryCfg(clk,
		func(context.Context, time.Time) error { calls++; return errors.New("down") },
		func() bool { return false }))
	if calls == 0 {
		t.Fatal("no attempts made")
	}
	if got := clk.Now().Sub(newClock().Now()); got < 5*time.Minute || got > 5*time.Minute+15*time.Second {
		t.Fatalf("gave up after %v, want about 5m", got)
	}
}

// A publish that overlaps another one would let an older build land last.
func TestPublisherSerialisesBuilds(t *testing.T) {
	pub := NewPublisher(nil, snapshot.NewHolder(testSeries, config.Wind{}), slog.New(slog.NewTextHandler(io.Discard, nil)))
	var inFlight, maxInFlight atomic.Int32
	pub.build = func(context.Context, time.Time) (*snapshot.Snapshot, error) {
		n := inFlight.Add(1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		inFlight.Add(-1)
		return nil, errors.New("down")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pub.Publish(context.Background(), time.Now())
		}()
	}
	wg.Wait()
	if maxInFlight.Load() != 1 {
		t.Fatalf("max concurrent builds = %d, want 1", maxInFlight.Load())
	}
}
