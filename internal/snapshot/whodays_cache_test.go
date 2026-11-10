package snapshot

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

// fakeWHO counts fetches; block, when set, holds each fetch until closed.
type fakeWHO struct {
	mu    sync.Mutex
	calls int
	err   error
	block chan struct{}
}

func (f *fakeWHO) fetch(ctx context.Context, since time.Time) (map[string][]store.Point, error) {
	f.mu.Lock()
	f.calls++
	err, block := f.err, f.block
	f.mu.Unlock()
	if block != nil {
		<-block
	}
	if err != nil {
		return nil, err
	}
	return map[string][]store.Point{"sofia": {dayPoint(1, 40), dayPoint(2, 3)}}, nil
}

func (f *fakeWHO) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func whoSnap() *Snapshot {
	return &Snapshot{KnownSlugs: map[string]AreaMeta{"sofia": {}}}
}

func TestWHOCacheOneQueryPerUTCDay(t *testing.T) {
	var c whoDaysCache
	f := &fakeWHO{}
	ctx := context.Background()
	c.apply(ctx, whoSnap(), whoNow, f.fetch)
	c.wg.Wait()
	for i := 1; i <= 20; i++ {
		c.apply(ctx, whoSnap(), whoNow.Add(time.Duration(i)*time.Minute), f.fetch)
		c.wg.Wait()
	}
	if f.count() != 1 {
		t.Fatalf("same-day builds ran %d queries, want 1", f.count())
	}
	c.apply(ctx, whoSnap(), whoNow.Add(24*time.Hour), f.fetch)
	c.wg.Wait()
	if f.count() != 2 {
		t.Fatalf("new UTC day ran %d queries total, want 2", f.count())
	}
}

func TestWHOCacheCarriesIntoLaterSnapshots(t *testing.T) {
	var c whoDaysCache
	f := &fakeWHO{}
	ctx := context.Background()
	first := whoSnap()
	c.apply(ctx, first, whoNow, f.fetch)
	c.wg.Wait()
	if first.KnownSlugs["sofia"].WHODays != nil {
		t.Fatal("first snapshot has WHODays before any refresh finished")
	}
	later := whoSnap()
	c.apply(ctx, later, whoNow.Add(time.Minute), f.fetch)
	got := later.KnownSlugs["sofia"].WHODays
	if got == nil || got.DaysWithData != 2 || got.DaysOver != 1 {
		t.Fatalf("later snapshot WHODays = %+v", got)
	}
}

func TestWHOCacheFailureBacksOffThirtyMinutes(t *testing.T) {
	var c whoDaysCache
	f := &fakeWHO{err: errors.New("statement timeout")}
	ctx := context.Background()
	c.apply(ctx, whoSnap(), whoNow, f.fetch)
	c.wg.Wait()
	c.apply(ctx, whoSnap(), whoNow.Add(29*time.Minute), f.fetch)
	c.wg.Wait()
	if f.count() != 1 {
		t.Fatalf("retried inside the backoff: %d queries", f.count())
	}
	c.apply(ctx, whoSnap(), whoNow.Add(30*time.Minute), f.fetch)
	c.wg.Wait()
	if f.count() != 2 {
		t.Fatalf("no retry after the backoff: %d queries", f.count())
	}
}

func TestWHOCacheBuildNeverWaitsOnRefresh(t *testing.T) {
	var c whoDaysCache
	f := &fakeWHO{block: make(chan struct{})}
	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		c.apply(ctx, whoSnap(), whoNow, f.fetch)
		c.apply(ctx, whoSnap(), whoNow.Add(time.Second), f.fetch)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("apply blocked on an in-flight refresh")
	}
	close(f.block)
	c.wg.Wait()
	if f.count() != 1 {
		t.Fatalf("in-flight refresh was not single-flight: %d queries", f.count())
	}
}
