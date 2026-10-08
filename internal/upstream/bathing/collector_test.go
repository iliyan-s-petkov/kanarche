package bathing_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"airbg.org/internal/config"
	"airbg.org/internal/store"
	"airbg.org/internal/upstream/bathing"
)

type fakeSink struct {
	data     store.BathingData
	at       time.Time
	writes   int
	last     time.Time
	hasLast  bool
	writeErr error
	edition  string
	onWrite  func()
}

func (f *fakeSink) ReplaceBathing(_ context.Context, d store.BathingData, at time.Time) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.data, f.at, f.writes = d, at, f.writes+1
	if f.onWrite != nil {
		f.onWrite()
	}
	return nil
}

func (f *fakeSink) BathingSupplementEdition(context.Context) (string, error) { return f.edition, nil }

func (f *fakeSink) BathingLastImport(context.Context) (time.Time, bool, error) {
	return f.last, f.hasLast, nil
}

func TestRunOnceWritesTheBuiltSet(t *testing.T) {
	srv, _ := discodata(t)
	sink := &fakeSink{}
	now := time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC)
	c := bathing.NewCollector(testConfig(srv.URL), sink)
	c.SetClockForTesting(func() time.Time { return now })

	st, err := c.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if sink.writes != 1 || len(sink.data.Sites) != 3 || !sink.at.Equal(now) {
		t.Errorf("sink = %d writes, %d sites at %v; want 1, 3 at %v", sink.writes, len(sink.data.Sites), sink.at, now)
	}
	if st.Sites != 3 || st.Classes != 12 || st.Samples != 25 || st.Skipped.Retired != 1 {
		t.Errorf("stats = %+v", st)
	}
}

// A failed fetch writes nothing, so the stored layer survives an EEA outage.
func TestRunOnceWritesNothingOnAFailedFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()
	sink := &fakeSink{}
	if _, err := bathing.NewCollector(testConfig(srv.URL), sink).RunOnce(context.Background()); err == nil {
		t.Error("RunOnce succeeded against a failing upstream")
	}
	if sink.writes != 0 {
		t.Errorf("%d writes, want 0", sink.writes)
	}
}

func TestRunOnceReturnsAStoreError(t *testing.T) {
	srv, _ := discodata(t)
	sink := &fakeSink{writeErr: errors.New("disk full")}
	if _, err := bathing.NewCollector(testConfig(srv.URL), sink).RunOnce(context.Background()); err == nil {
		t.Error("RunOnce swallowed the store error")
	}
}

func TestNextDelay(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	week := 168 * time.Hour
	cases := []struct {
		name    string
		last    time.Time
		hasLast bool
		want    time.Duration
	}{
		{"never imported", time.Time{}, false, 0},
		{"imported a day ago", now.Add(-24 * time.Hour), true, week - 24*time.Hour},
		{"overdue", now.Add(-2 * week), true, 0},
		{"clock went backwards", now.Add(time.Hour), true, week},
	}
	for _, tc := range cases {
		if got := bathing.NextDelay(now, tc.last, tc.hasLast, week); got != tc.want {
			t.Errorf("%s: NextDelay = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func fillSupplement() *bathing.Supplement {
	return &bathing.Supplement{Edition: "2025 v1.0", Classes: []bathing.SupplementClass{
		{SiteID: "BG3242661710017001", Season: 2025, Quality: "good"},
		{SiteID: "BG3310610135003001", Season: 2025, Quality: "good"},
		{SiteID: "BG3412737023002036", Season: 2025, Quality: "good"},
	}}
}

func TestRunOnceMergesTheSupplement(t *testing.T) {
	srv, _ := discodata(t)
	sink := &fakeSink{}
	c := bathing.NewCollector(testConfig(srv.URL), sink)
	c.SetSupplement(fillSupplement())
	st, err := c.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Classes != 15 || st.Supplement.Applied != 3 || sink.data.SupplementEdition != "2025 v1.0" {
		t.Errorf("stats = %+v, edition %q", st, sink.data.SupplementEdition)
	}
	var fromDatahub int
	for _, c := range sink.data.Classes {
		if c.Source == store.SourceDatahub {
			fromDatahub++
		}
	}
	if fromDatahub != 3 {
		t.Errorf("%d datahub classes written, want 3", fromDatahub)
	}
}

// With no snapshot, Discodata is imported exactly as before.
func TestRunOnceWithoutSupplementKeepsDiscodata(t *testing.T) {
	srv, _ := discodata(t)
	sink := &fakeSink{}
	st, err := bathing.NewCollector(testConfig(srv.URL), sink).RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Classes != 12 || sink.data.SupplementEdition != "" {
		t.Errorf("stats = %+v, edition %q", st, sink.data.SupplementEdition)
	}
	for _, c := range sink.data.Classes {
		if c.Source != store.SourceDiscodata {
			t.Errorf("class %+v is not from Discodata", c)
		}
	}
}

// runLoop runs Loop until the first write or until wait elapses; it reports the writes.
func runLoop(t *testing.T, c *bathing.Collector, sink *fakeSink, wait time.Duration) int {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink.onWrite = cancel
	done := make(chan struct{})
	go func() { c.Loop(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(wait):
		cancel()
		<-done
	}
	return sink.writes
}

func TestLoopImportsNowWhenEditionChanged(t *testing.T) {
	srv, _ := discodata(t)
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	sink := &fakeSink{last: now.Add(-time.Hour), hasLast: true, edition: "2024 v1.0"}
	c := bathing.NewCollector(testConfig(srv.URL), sink)
	c.SetClockForTesting(func() time.Time { return now })
	c.SetSupplement(fillSupplement())
	if n := runLoop(t, c, sink, 5*time.Second); n != 1 {
		t.Errorf("%d writes, want an immediate import", n)
	}
}

func TestLoopWaitsWhenEditionUnchanged(t *testing.T) {
	srv, _ := discodata(t)
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	sink := &fakeSink{last: now.Add(-time.Hour), hasLast: true, edition: "2025 v1.0"}
	c := bathing.NewCollector(testConfig(srv.URL), sink)
	c.SetClockForTesting(func() time.Time { return now })
	c.SetSupplement(fillSupplement())
	if n := runLoop(t, c, sink, 300*time.Millisecond); n != 0 {
		t.Errorf("%d writes, want none until refresh_interval", n)
	}
}

// No snapshot means no edition to compare, so the schedule is unchanged.
func TestLoopWaitsWithoutSupplement(t *testing.T) {
	srv, _ := discodata(t)
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	sink := &fakeSink{last: now.Add(-time.Hour), hasLast: true, edition: "2024 v1.0"}
	c := bathing.NewCollector(testConfig(srv.URL), sink)
	c.SetClockForTesting(func() time.Time { return now })
	if n := runLoop(t, c, sink, 300*time.Millisecond); n != 0 {
		t.Errorf("%d writes, want none", n)
	}
}

func watchCollector(t *testing.T, now *time.Time, calls *int, err error) *bathing.Collector {
	t.Helper()
	c := bathing.NewCollector(config.Sea{}, &fakeSink{})
	c.SetClockForTesting(func() time.Time { return *now })
	c.SetEditionWatch(func(context.Context) error { *calls++; return err })
	return c
}

// The probe is weekly, though imports run more often.
func TestEditionWatchRunsWeekly(t *testing.T) {
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	calls := 0
	c := watchCollector(t, &now, &calls, nil)
	c.WatchEdition(context.Background())
	now = now.Add(24 * time.Hour)
	c.WatchEdition(context.Background())
	now = now.Add(5 * 24 * time.Hour)
	c.WatchEdition(context.Background())
	if calls != 1 {
		t.Fatalf("calls inside 7 days = %d, want 1", calls)
	}
	now = now.Add(25 * time.Hour)
	c.WatchEdition(context.Background())
	if calls != 2 {
		t.Errorf("calls after 7 days = %d, want 2", calls)
	}
}

// A failed probe still counts, so an outage is retried next week, not every import.
func TestEditionWatchFailureStillThrottled(t *testing.T) {
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	calls := 0
	c := watchCollector(t, &now, &calls, errors.New("boom"))
	c.WatchEdition(context.Background())
	c.WatchEdition(context.Background())
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestEditionWatchUnsetIsNoop(t *testing.T) {
	bathing.NewCollector(config.Sea{}, &fakeSink{}).WatchEdition(context.Background())
}
