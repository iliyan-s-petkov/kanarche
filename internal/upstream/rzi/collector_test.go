package rzi_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/upstream/rzi"
)

func TestIntervalSwitchesByDate(t *testing.T) {
	cfg := config.SeaRZI{InSeasonInterval: 6 * time.Hour, OffSeasonInterval: 24 * time.Hour}
	at := func(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, time.UTC) }
	cases := []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{"mid january", at(2027, time.January, 15, 12), 24 * time.Hour},
		{"14 may", at(2027, time.May, 14, 12), 24 * time.Hour},
		{"15 may", at(2027, time.May, 15, 12), 6 * time.Hour},
		{"july", at(2027, time.July, 1, 3), 6 * time.Hour},
		{"15 october", at(2026, time.October, 15, 12), 6 * time.Hour},
		{"16 october", at(2026, time.October, 16, 12), 24 * time.Hour},
		// 22:00 UTC on 15 October is already 16 October in Sofia.
		{"15 october late UTC", at(2026, time.October, 15, 22), 24 * time.Hour},
		// 21:30 UTC on 14 May is 00:30 on 15 May in Sofia.
		{"14 may late UTC", time.Date(2027, time.May, 14, 21, 30, 0, 0, time.UTC), 6 * time.Hour},
		{"december", at(2026, time.December, 31, 23), 24 * time.Hour},
	}
	for _, c := range cases {
		if got := rzi.Interval(cfg, c.now); got != c.want {
			t.Errorf("%s: Interval = %v, want %v", c.name, got, c.want)
		}
	}
}

// fakeFetcher records calls and answers from a script.
type fakeFetcher struct {
	mu     sync.Mutex
	calls  []string
	priors map[string][]rzi.Validators
	reply  map[string]rzi.Result
	fail   map[string]error
	// onFetch runs inside every Fetch, for a test that needs the world to
	// change in the middle of a pass.
	onFetch func()
}

func (f *fakeFetcher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeFetcher) Fetch(_ context.Context, id string, prior rzi.Validators) (rzi.Result, error) {
	if f.onFetch != nil {
		f.onFetch()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, id)
	if f.priors == nil {
		f.priors = map[string][]rzi.Validators{}
	}
	f.priors[id] = append(f.priors[id], prior)
	if err := f.fail[id]; err != nil {
		return rzi.Result{}, err
	}
	return f.reply[id], nil
}

func TestRunOnceFetchesEverySourceThenIngests(t *testing.T) {
	r := newRig(t)
	drop(t, r.outbox, varna(t, shaOf("a")))
	ff := &fakeFetcher{
		fail: map[string]error{rzi.SourceBurgas: errors.New("boom")},
		reply: map[string]rzi.Result{
			rzi.SourceVarna: {Source: rzi.SourceVarna, Status: rzi.StatusWritten, SHA256: shaOf("a"),
				URL: "https://rzi-varna.com/mvodi-2026.pdf", ETag: `"e1"`, LastModified: "Mon, 01 Jun 2026 08:00:00 GMT"},
			rzi.SourceDobrich: {Source: rzi.SourceDobrich, Status: rzi.StatusNotModified},
		},
	}
	c := rzi.NewCollector(r.cfg, r.st, ff)
	c.SetClockForTesting(func() time.Time { return r.now })

	s, err := c.RunOnce(r.ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(ff.calls) != 3 {
		t.Fatalf("fetch calls = %v, want one per source", ff.calls)
	}
	if s.OK != 1 {
		t.Errorf("a failed fetch of one source stopped the others or the ingest: %+v", s)
	}
	f := r.file(shaOf("a"))
	if f.URL != "https://rzi-varna.com/mvodi-2026.pdf" || f.ETag != `"e1"` || f.LastModified == nil {
		t.Errorf("file row lost the fetch metadata: %+v", f)
	}

	// The next pass sends back what the last one learned.
	if _, err := c.RunOnce(r.ctx); err != nil {
		t.Fatal(err)
	}
	second := ff.priors[rzi.SourceVarna][1]
	if second.ETag != `"e1"` || second.SHA256 != shaOf("a") || second.LastModified == "" {
		t.Errorf("second prior = %+v", second)
	}
	if p := ff.priors[rzi.SourceDobrich][1]; p != (rzi.Validators{}) {
		t.Errorf("not_modified invented validators: %+v", p)
	}
}

func TestLoopDoesNothingWhenDisabled(t *testing.T) {
	r := newRig(t)
	r.cfg.Enabled = false
	ff := &fakeFetcher{}
	c := rzi.NewCollector(r.cfg, r.st, ff)

	done := make(chan struct{})
	go func() { c.Loop(r.ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Loop did not return")
	}
	if len(ff.calls) != 0 {
		t.Errorf("disabled loop fetched: %v", ff.calls)
	}
}

func TestLoopRunsAtOnceAndStopsWithContext(t *testing.T) {
	r := newRig(t)
	ff := &fakeFetcher{}
	c := rzi.NewCollector(r.cfg, r.st, ff)
	c.SetClockForTesting(func() time.Time { return r.now })
	ctx, cancel := context.WithCancel(r.ctx)

	done := make(chan struct{})
	go func() { c.Loop(ctx); close(done) }()
	deadline := time.Now().Add(10 * time.Second)
	for ff.count() < 3 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Loop did not stop")
	}
	if ff.count() != 3 {
		t.Errorf("fetch calls = %d, want one pass and then the wait", ff.count())
	}
}
