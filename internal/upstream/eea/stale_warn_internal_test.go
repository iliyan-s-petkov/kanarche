package eea

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type staleStep struct {
	at     time.Time
	newest time.Time
}

// countStaleWarnings runs the staleness check once per step and returns the cumulative number of WARN lines after each step.
func countStaleWarnings(t *testing.T, steps []staleStep) []int {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	c := &Collector{}
	var counts []int
	for _, s := range steps {
		c.warnIfStale(s.newest, s.at)
		counts = append(counts, strings.Count(buf.String(), "eea data stale"))
	}
	return counts
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestStaleWarningLogsAtMostOncePerHour(t *testing.T) {
	newest := time.Date(2026, 9, 9, 9, 0, 0, 0, time.UTC)
	t0 := newest.Add(20 * time.Hour)
	got := countStaleWarnings(t, []staleStep{
		{at: t0, newest: newest},
		{at: t0.Add(10 * time.Minute), newest: newest},
		{at: t0.Add(61 * time.Minute), newest: newest},
	})
	if want := []int{1, 1, 2}; !equalInts(got, want) {
		t.Fatalf("cumulative warnings = %v, want %v", got, want)
	}
}

// An all-unmodified cycle decodes no rows, so newest is zero. The remembered time still drives the warning.
func TestStaleWarningRepeatsWhenEveryFileIsUnmodified(t *testing.T) {
	newest := time.Date(2026, 9, 9, 9, 0, 0, 0, time.UTC)
	t0 := newest.Add(20 * time.Hour)
	got := countStaleWarnings(t, []staleStep{
		{at: t0, newest: newest},
		{at: t0.Add(30 * time.Minute)},
		{at: t0.Add(65 * time.Minute)},
	})
	if want := []int{1, 1, 2}; !equalInts(got, want) {
		t.Fatalf("cumulative warnings = %v, want %v", got, want)
	}
}

func TestStaleWarningSilentOnFreshData(t *testing.T) {
	newest := time.Date(2026, 9, 9, 9, 0, 0, 0, time.UTC)
	got := countStaleWarnings(t, []staleStep{
		{at: newest.Add(time.Hour), newest: newest},
		{at: newest.Add(2 * time.Hour)},
	})
	if want := []int{0, 0}; !equalInts(got, want) {
		t.Fatalf("cumulative warnings = %v, want %v", got, want)
	}
}

func TestStaleWarningSilentBeforeAnyRowWasSeen(t *testing.T) {
	got := countStaleWarnings(t, []staleStep{{at: time.Now()}})
	if want := []int{0}; !equalInts(got, want) {
		t.Fatalf("cumulative warnings = %v, want %v", got, want)
	}
}

func TestStaleWarningKeepsMessageAndFields(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	newest := time.Date(2026, 9, 9, 9, 0, 0, 0, time.UTC)
	(&Collector{}).warnIfStale(newest, newest.Add(20*time.Hour))
	for _, want := range []string{"level=WARN", `msg="eea data stale"`, "age=20h0m0s", "newest_row=", "threshold=12h0m0s"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log missing %q:\n%s", want, buf.String())
		}
	}
}
