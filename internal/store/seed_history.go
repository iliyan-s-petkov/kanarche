package store

import (
	"context"
	"fmt"
	"time"

	"kanarche.eu/internal/quality"
)

// SeedHistory replays the last `depth` stored readings of every (sensor, metric)
// seen within `window` into h, oldest first, and returns the rows replayed. It
// runs once at startup so a frozen sensor is not re-learned after each restart.
//
// The time bound prunes to the newest chunks and the (sensor_id, metric, time
// DESC) index serves the per-series ordering. Rows flagged clamped or
// out_of_range are skipped because Score never observes them either.
func (s *Store) SeedHistory(ctx context.Context, h *quality.History, window time.Duration, depth int) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT sensor_id, metric, value FROM (
		  SELECT sensor_id, metric, value, time,
		         row_number() OVER (PARTITION BY sensor_id, metric ORDER BY time DESC) AS rn
		  FROM reading
		  WHERE time > now() - $1::interval
		    AND quality NOT IN ('clamped', 'out_of_range')
		) recent
		WHERE rn <= $2
		ORDER BY sensor_id, metric, time ASC`,
		window, depth)
	if err != nil {
		return 0, fmt.Errorf("seed history: %w", err)
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var (
			id     int64
			metric string
			value  float64
		)
		if err := rows.Scan(&id, &metric, &value); err != nil {
			return n, fmt.Errorf("seed history: scan: %w", err)
		}
		h.Observe(id, metric, value)
		n++
	}
	if err := rows.Err(); err != nil {
		return n, fmt.Errorf("seed history: %w", err)
	}
	return n, nil
}
