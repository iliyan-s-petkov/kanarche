package snapshot

import (
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

// ypNow is mid November 2026, so the 12 months run December 2025 to November 2026.
var ypNow = time.Date(2026, 11, 15, 2, 0, 0, 0, time.UTC)

// monthPoints is n daily points of one value in a month, starting on the first.
func monthPoints(year int, month time.Month, n int, v float64) []store.Point {
	out := make([]store.Point, 0, n)
	for d := 1; d <= n; d++ {
		out = append(out, store.Point{Time: time.Date(year, month, d, 0, 0, 0, 0, time.UTC), Value: v})
	}
	return out
}

func joinPoints(parts ...[]store.Point) []store.Point {
	var out []store.Point
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func ypSince() time.Time { return ypNow.Add(-whoDaysWindow) }

func TestYearProfileEmptySeries(t *testing.T) {
	if got := summariseYearProfile(nil, ypSince(), ypNow); got != nil {
		t.Errorf("got %+v, want nil", *got)
	}
}

// Months group across the year boundary, oldest first, with the year kept.
func TestYearProfileGroupsMonthsAcrossYearBoundary(t *testing.T) {
	pts := joinPoints(
		monthPoints(2025, time.December, 20, 30),
		monthPoints(2026, time.January, 10, 50),
		monthPoints(2026, time.February, 10, 10),
	)
	got := summariseYearProfile(pts, ypSince(), ypNow)
	if got == nil || len(got.Months) != 3 {
		t.Fatalf("got %+v", got)
	}
	want := []MonthMean{
		{2025, time.December, 30, 20},
		{2026, time.January, 50, 10},
		{2026, time.February, 10, 10},
	}
	for i, m := range want {
		if got.Months[i] != m {
			t.Errorf("month %d = %+v, want %+v", i, got.Months[i], m)
		}
	}
	if got.DaysWithData != 40 {
		t.Errorf("days = %d, want 40", got.DaysWithData)
	}
	// 20*30 + 10*50 + 10*10 over 40 days.
	if got.Mean != 30 {
		t.Errorf("mean = %v, want 30", got.Mean)
	}
}

// The month the window starts in (November 2025) is outside the 12 months and is dropped.
func TestYearProfileDropsThirteenthMonth(t *testing.T) {
	pts := joinPoints(
		monthPoints(2025, time.November, 28, 99),
		monthPoints(2025, time.December, 12, 20),
	)
	got := summariseYearProfile(pts, time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC), ypNow)
	if got == nil || got.DaysWithData != 12 || len(got.Months) != 1 {
		t.Fatalf("got %+v", got)
	}
}

// Months under 10 days never become best or worst, but still appear in Months.
func TestYearProfileBestWorstSkipShortMonths(t *testing.T) {
	pts := joinPoints(
		monthPoints(2026, time.March, 9, 1),   // too short, would be best
		monthPoints(2026, time.April, 10, 12), // exactly 10 days counts
		monthPoints(2026, time.May, 20, 25),
		monthPoints(2026, time.June, 9, 500), // too short, would be worst
	)
	got := summariseYearProfile(pts, ypSince(), ypNow)
	if got.Best == nil || got.Worst == nil {
		t.Fatalf("best/worst missing: %+v", got)
	}
	if got.Best.Month != time.April || got.Worst.Month != time.May {
		t.Errorf("best %v worst %v", got.Best.Month, got.Worst.Month)
	}
	if len(got.Months) != 4 {
		t.Errorf("months = %d, want 4", len(got.Months))
	}
}

// One qualifying month cannot be both the cleanest and the most polluted.
func TestYearProfileBestWorstNeedTwoMonths(t *testing.T) {
	got := summariseYearProfile(monthPoints(2026, time.May, 20, 25), ypSince(), ypNow)
	if got.Best != nil || got.Worst != nil {
		t.Errorf("best %+v worst %+v, want nil", got.Best, got.Worst)
	}
}

func TestYearProfileSeasonMeans(t *testing.T) {
	pts := joinPoints(
		monthPoints(2025, time.December, 10, 40),
		monthPoints(2026, time.January, 10, 20),
		monthPoints(2026, time.June, 10, 6),
		monthPoints(2026, time.August, 30, 4),
		monthPoints(2026, time.September, 5, 100),
	)
	got := summariseYearProfile(pts, ypSince(), ypNow)
	if !got.HasWinter || got.WinterMean != 30 {
		t.Errorf("winter = %v %v, want 30", got.HasWinter, got.WinterMean)
	}
	// (10*6 + 30*4) / 40
	if !got.HasSummer || got.SummerMean != 4.5 {
		t.Errorf("summer = %v %v, want 4.5", got.HasSummer, got.SummerMean)
	}
}

func TestYearProfileSeasonWithoutDataIsAbsent(t *testing.T) {
	got := summariseYearProfile(monthPoints(2026, time.April, 20, 10), ypSince(), ypNow)
	if got.HasWinter || got.HasSummer {
		t.Errorf("winter %v summer %v, want both absent", got.HasWinter, got.HasSummer)
	}
}

// Strictly above the guideline counts, equal does not, and the earliest equal worst day wins.
func TestYearProfileWHOCountIsStrict(t *testing.T) {
	d := func(m time.Month, day int, v float64) store.Point {
		return store.Point{Time: time.Date(2026, m, day, 0, 0, 0, 0, time.UTC), Value: v}
	}
	pts := []store.Point{d(3, 1, WHODayPM25), d(3, 2, 15.01), d(3, 3, 40), d(3, 4, 40), d(3, 5, 2)}
	got := summariseYearProfile(pts, ypSince(), ypNow)
	if got.DaysOver != 3 || got.WorstMean != 40 || !got.WorstDay.Equal(pts[2].Time) {
		t.Errorf("got %+v", *got)
	}
}

// The partial current day is not a day with data.
func TestYearProfileSkipsPartialToday(t *testing.T) {
	pts := []store.Point{
		{Time: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), Value: 10},
		{Time: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), Value: 900},
	}
	got := summariseYearProfile(pts, ypSince(), ypNow)
	if got.DaysWithData != 1 || got.WorstMean != 10 {
		t.Errorf("got %+v", *got)
	}
}
