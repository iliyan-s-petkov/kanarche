package bathing_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
}

func (f *fakeSink) ReplaceBathing(_ context.Context, d store.BathingData, at time.Time) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.data, f.at, f.writes = d, at, f.writes+1
	return nil
}

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
