package snapshot

import (
	"context"
	"log/slog"
	"time"

	"kanarche.eu/internal/store"
)

// minHourlyMeanPoints is how many of the last 24 hourly medians must exist before a 24h mean is published.
const minHourlyMeanPoints = 18

// dayMeanMetrics are the metrics whose 24h mean the city page compares with the limits.
var dayMeanMetrics = []string{"P2", "P1"}

// meanLast24Hours is the mean of the hourly medians whose hour starts inside (now-24h, now].
// It reports false when fewer than minHourlyMeanPoints of those hours have a value.
func meanLast24Hours(points []store.Point, now time.Time) (float64, bool) {
	from := now.Add(-24 * time.Hour)
	var sum float64
	var n int
	for _, p := range points {
		if p.Time.After(from) && !p.Time.After(now) {
			sum += p.Value
			n++
		}
	}
	if n < minHourlyMeanPoints {
		return 0, false
	}
	return sum / float64(n), true
}

// buildDayMeans sets AreaMeta.Day24Mean from one hourly series query per metric.
// A failed query is logged and leaves that metric unset, so the page drops the block instead of failing the build.
func buildDayMeans(ctx context.Context, s *store.Store, snap *Snapshot, now time.Time) {
	for _, metric := range dayMeanMetrics {
		series, err := s.AllAreaSeries(ctx, metric, now.Add(-24*time.Hour), false, time.Hour)
		if err != nil {
			slog.Warn("snapshot: 24h mean unavailable", "metric", metric, "error", err)
			continue
		}
		for slug, meta := range snap.KnownSlugs {
			mean, ok := meanLast24Hours(series[slug], now)
			if !ok {
				continue
			}
			if meta.Day24Mean == nil {
				meta.Day24Mean = map[string]float64{}
			}
			meta.Day24Mean[metric] = mean
			snap.KnownSlugs[slug] = meta
		}
	}
}
