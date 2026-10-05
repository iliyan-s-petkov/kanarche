package store

import (
	"context"
	"fmt"
	"time"
)

// faultyPairsSQL selects the (sensor_id, metric) pairs faulty over the hourly
// buckets in [end - window, end). Arguments are placeholder names such as "$3".
// share is compared as numeric so 0.5 of 6 is exactly 3.
func faultyPairsSQL(end, windowSecs, share, minReadings string) string {
	return `SELECT q.sensor_id, q.metric
      FROM reading_quality_hourly q
     WHERE q.bucket >= ` + end + ` - make_interval(secs => ` + windowSecs + `::double precision)
       AND q.bucket < ` + end + `
     GROUP BY q.sensor_id, q.metric
    HAVING sum(q.total) >= ` + minReadings + `::integer
       AND sum(q.flagged) >= ` + share + `::numeric * sum(q.total)`
}

// notFaulty excludes a reading row aliased r whose pair is in the current set.
const notFaulty = `NOT EXISTS (SELECT 1 FROM sensor_faulty f
                    WHERE f.sensor_id = r.sensor_id AND f.metric = r.metric)`

// qualityCountsSQL records one bucket's total and unusable reading counts.
const qualityCountsSQL = `INSERT INTO reading_quality_hourly
	     (bucket, sensor_id, metric, total, flagged)
	 SELECT $1, sensor_id, metric, count(*),
	        count(*) FILTER (WHERE quality <> ALL($2::quality_flag[]))
	 FROM reading
	 WHERE time >= $1 AND time < $1 + interval '1 hour'
	 GROUP BY sensor_id, metric
	 ON CONFLICT (sensor_id, metric, bucket) DO UPDATE
	   SET total = EXCLUDED.total, flagged = EXCLUDED.flagged`

// refreshFaultySQL's window ends after the current hour, so the current
// partial hour counts as far as the last rollup of it reached.
var refreshFaultySQL = `INSERT INTO sensor_faulty (sensor_id, metric) ` +
	faultyPairsSQL("$1::timestamptz", "$2", "$3", "$4")

// faultyArgs are the rule's parameters in faultyPairsSQL's order after end.
func (s *Store) faultyArgs() (windowSecs, share float64, minReadings int) {
	f := s.cfg.Faulty
	return f.Window.Seconds(), f.Share, f.MinReadings
}

// RefreshFaulty replaces the faulty set with the one ending at now's hour. Run
// once per ingest cycle, after the rollup has counted the current hour. A zero
// window (an unconfigured store) yields an empty set.
func (s *Store) RefreshFaulty(ctx context.Context, now time.Time) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: begin refresh faulty: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful Commit

	if _, err := tx.Exec(ctx, `DELETE FROM sensor_faulty`); err != nil {
		return 0, fmt.Errorf("store: clear faulty: %w", err)
	}
	w, share, minReadings := s.faultyArgs()
	tag, err := tx.Exec(ctx, refreshFaultySQL, TruncateHour(now).Add(time.Hour), w, share, minReadings)
	if err != nil {
		return 0, fmt.Errorf("store: refresh faulty: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("store: commit faulty: %w", err)
	}
	return tag.RowsAffected(), nil
}
