package snapshot

import (
	"context"
	"log/slog"
	"time"

	"kanarche.eu/internal/store"
)

// WHODayPM25 is the WHO 2021 24-hour PM2.5 guideline in µg/m³.
const WHODayPM25 = 15

// whoDaysWindow is how far back the daily series reaches, the same window as the 1y chart.
const whoDaysWindow = 365 * 24 * time.Hour

// WHODays is a year of daily PM2.5 means for one area against the WHO guideline.
// Days are the UTC buckets of the 1y chart series.
type WHODays struct {
	DaysWithData int
	// DaysOver counts days whose mean is strictly above WHODayPM25.
	DaysOver  int
	WorstDay  time.Time
	WorstMean float64
}

// summariseWHODays reduces a daily series to its WHODays, or nil for an empty series.
func summariseWHODays(points []store.Point) *WHODays {
	if len(points) == 0 {
		return nil
	}
	out := &WHODays{DaysWithData: len(points), WorstDay: points[0].Time, WorstMean: points[0].Value}
	for _, p := range points {
		if p.Value > WHODayPM25 {
			out.DaysOver++
		}
		// Strict, so the earliest of equal worst days is kept.
		if p.Value > out.WorstMean {
			out.WorstDay, out.WorstMean = p.Time, p.Value
		}
	}
	return out
}

// buildWHODays sets AreaMeta.WHODays from one daily P2 query over every area.
// The query has no per-day hour count, so a day counts however many hours it covers.
// A failed query is logged and leaves the field unset.
func buildWHODays(ctx context.Context, s *store.Store, snap *Snapshot, now time.Time) {
	series, err := s.AllAreaSeries(ctx, "P2", now.Add(-whoDaysWindow), true, 24*time.Hour)
	if err != nil {
		slog.Warn("snapshot: days over WHO guideline unavailable", "error", err)
		return
	}
	for slug, meta := range snap.KnownSlugs {
		if sum := summariseWHODays(series[slug]); sum != nil {
			meta.WHODays = sum
			snap.KnownSlugs[slug] = meta
		}
	}
}
