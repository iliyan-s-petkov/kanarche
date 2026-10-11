package rzi_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"kanarche.eu/internal/upstream/rzi"
)

// Stopping the process is not a failed pass.
func TestLoopLogsNoErrorWhenStoppedMidPass(t *testing.T) {
	r := newRig(t)
	drop(t, r.outbox, varna(t, shaOf("a")))
	ctx, cancel := context.WithCancel(r.ctx)
	ff := &fakeFetcher{onFetch: cancel}
	c := rzi.NewCollector(r.cfg, r.st, ff)
	c.SetClockForTesting(func() time.Time { return r.now })
	logs := captureLog(t)

	done := make(chan struct{})
	go func() { c.Loop(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Loop did not stop")
	}
	if strings.Contains(logs.String(), "rzi pass failed") {
		t.Errorf("shutdown logged as a failure:\n%s", logs)
	}
}

// syncBuf is a log sink that a test may read while the loop still writes.
type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureSyncLog(t *testing.T) *syncBuf {
	t.Helper()
	b := &syncBuf{}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(b, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })
	return b
}

// A failed pass is still an error.
func TestLoopLogsAFailedPass(t *testing.T) {
	r := newRig(t)
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	// A file where the outbox directory should be makes Process fail.
	r.cfg.OutboxDir = r.cfg.InboxDir + "/not-a-dir"
	if err := os.WriteFile(r.cfg.OutboxDir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := rzi.NewCollector(r.cfg, r.st, &fakeFetcher{})
	c.SetClockForTesting(func() time.Time { return r.now })
	logs := captureSyncLog(t)

	done := make(chan struct{})
	go func() { c.Loop(ctx); close(done) }()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(logs.String(), "rzi pass failed") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	if !strings.Contains(logs.String(), "rzi pass failed") {
		t.Errorf("a failing pass was not logged:\n%s", logs)
	}
}

// An approval shows on the gauges without waiting for the next fetch pass.
func TestWatchMetricsFollowsTheStoreBetweenPasses(t *testing.T) {
	r := newRig(t)
	r.reviewFile(shaOf("b"))
	if err := r.ing.PublishMetrics(r.ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(scrape(t), `airbg_rzi_file_status{source="rzi_varna",status="needs_review"} 1`) {
		t.Fatal("setup: gauge not at needs_review")
	}
	if _, err := r.ing.Approve(r.ctx, shaOf("b")); err != nil {
		t.Fatal(err)
	}

	c := rzi.NewCollector(r.cfg, r.st, &fakeFetcher{})
	ctx, cancel := context.WithCancel(r.ctx)
	done := make(chan struct{})
	go func() { c.WatchMetrics(ctx, 20*time.Millisecond); close(done) }()
	deadline := time.Now().Add(10 * time.Second)
	want := `airbg_rzi_file_status{source="rzi_varna",status="ok"} 1`
	for !strings.Contains(scrape(t), want) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	ok := strings.Contains(scrape(t), want)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WatchMetrics did not stop")
	}
	if !ok {
		t.Error("the gauge never followed the approval")
	}
}
