package rzi_test

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kanarche.eu/internal/store"
	"kanarche.eu/internal/upstream/rzi"
)

// overlapSink fails the test when two ingest passes use the sink at once.
// It yields inside the calls that every pass makes, so an unlocked second
// pass has time to arrive.
type overlapSink struct {
	rzi.Sink
	active  atomic.Int32
	overlap atomic.Bool
}

func (o *overlapSink) enter() func() {
	if o.active.Add(1) > 1 {
		o.overlap.Store(true)
	}
	for range 200 {
		runtime.Gosched()
	}
	return func() { o.active.Add(-1) }
}

func (o *overlapSink) LoadBathingSites(ctx context.Context) ([]store.BathingSite, error) {
	defer o.enter()()
	return o.Sink.LoadBathingSites(ctx)
}

func (o *overlapSink) RZILatest(ctx context.Context, source string) (store.RZIFile, bool, error) {
	defer o.enter()()
	return o.Sink.RZILatest(ctx, source)
}

// A record in the outbox is ingested by the watcher alone: no fetch happens,
// and the watcher stops with its context.
func TestWatchOutboxIngestsWithoutFetching(t *testing.T) {
	r := newRig(t)
	logs := captureSyncLog(t)
	ff := &fakeFetcher{}
	c := rzi.NewCollector(r.cfg, r.st, ff)
	c.SetClockForTesting(func() time.Time { return r.now })
	drop(t, r.outbox, varna(t, shaOf("a")))

	ctx, cancel := context.WithCancel(r.ctx)
	done := make(chan struct{})
	go func() { c.WatchOutbox(ctx, 5*time.Millisecond); close(done) }()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(logs.String(), "rzi outbox pass") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WatchOutbox did not stop")
	}

	if !strings.Contains(logs.String(), "rzi outbox pass") {
		t.Fatalf("the record was never ingested:\n%s", logs)
	}
	if got := r.file(shaOf("a")); got.Status != store.RZIStatusOK {
		t.Errorf("file status = %q", got.Status)
	}
	if n := ff.count(); n != 0 {
		t.Errorf("WatchOutbox fetched %d times", n)
	}
	if strings.Contains(logs.String(), "rzi fetch") {
		t.Errorf("WatchOutbox logged a fetch:\n%s", logs)
	}
}

// The fetch loop and the outbox watcher never run an ingest pass together.
func TestOutboxWatcherAndRunOnceDoNotOverlap(t *testing.T) {
	r := newRig(t)
	sink := &overlapSink{Sink: r.st}
	c := rzi.NewCollector(r.cfg, sink, &fakeFetcher{})
	c.SetClockForTesting(func() time.Time { return r.now })

	ctx, cancel := context.WithCancel(r.ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); c.WatchOutbox(ctx, time.Millisecond) }()

	for _, ch := range "0123456789abcdef" {
		drop(t, r.outbox, varna(t, shaOf(string(ch))))
		if _, err := c.RunOnce(r.ctx); err != nil {
			t.Fatal(err)
		}
	}
	cancel()
	wg.Wait()

	if sink.overlap.Load() {
		t.Error("two ingest passes ran at the same time")
	}
}
