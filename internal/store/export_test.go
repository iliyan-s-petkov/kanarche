package store

import "context"

// RefreshFaultySelectSQL is the faulty-set query, exported for the plan test.
var RefreshFaultySelectSQL = faultyPairsSQL("$1::timestamptz", "$2", "$3", "$4")

// HeadlineRowsPerMetric counts the per_area rows of each "slug/metric": the
// rule must leave exactly one, which the jsonb output alone cannot show
// because a duplicate key silently keeps one of the two.
func (s *Store) HeadlineRowsPerMetric(ctx context.Context, kinds []string) (map[string]int, error) {
	official, community := s.cutoffs()
	q := "WITH" + latestCTE(4) + "," + perAreaSourceCTE("l.value", "") + "," + perAreaCTE(5) +
		" SELECT slug || '/' || metric, count(*) FROM per_area GROUP BY 1"
	rows, err := s.pool.Query(ctx, q, official, usableQuality, kinds, community, s.cfg.CoverageThreshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}
