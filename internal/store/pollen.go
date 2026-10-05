package store

import (
	"context"
	"time"
)

// PollenCell is one lattice point in centidegrees (2330 is 23.30 degrees).
type PollenCell struct {
	LonC, LatC int
}

// PollenForecast is one cell's modelled concentration of one species at one
// hour, in grains/m3. Declared here, not in internal/pollen, for the same
// reason as WindForecast.
type PollenForecast struct {
	LonC, LatC int
	Species    string
	ValidAt    time.Time
	Grains     float64
}

// pollenWriteChunk bounds one INSERT's arrays.
const pollenWriteChunk = 5000

// WritePollen upserts a model run; a later run of the same hour replaces it.
func (s *Store) WritePollen(ctx context.Context, fs []PollenForecast, fetchedAt time.Time) (int64, error) {
	for start := 0; start < len(fs); start += pollenWriteChunk {
		chunk := fs[start:min(start+pollenWriteChunk, len(fs))]
		validAt := make([]time.Time, len(chunk))
		lon := make([]int32, len(chunk))
		lat := make([]int32, len(chunk))
		species := make([]string, len(chunk))
		grains := make([]float64, len(chunk))
		for i, f := range chunk {
			validAt[i], lon[i], lat[i], species[i], grains[i] = f.ValidAt, int32(f.LonC), int32(f.LatC), f.Species, f.Grains
		}
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO pollen_forecast (valid_at, lon_c, lat_c, species, grains, fetched_at)
			 SELECT v, x, y, sp, g, $6
			   FROM unnest($1::timestamptz[], $2::int[], $3::int[], $4::text[], $5::float8[]) AS u(v, x, y, sp, g)
			 ON CONFLICT (lon_c, lat_c, species, valid_at) DO UPDATE
			   SET grains = EXCLUDED.grains, fetched_at = EXCLUDED.fetched_at`,
			validAt, lon, lat, species, grains, fetchedAt); err != nil {
			return 0, err
		}
	}
	return int64(len(fs)), nil
}

// LatestPollenFetch returns when the newest stored run was fetched; false when
// the table is empty.
func (s *Store) LatestPollenFetch(ctx context.Context) (time.Time, bool, error) {
	var t *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT max(fetched_at) FROM pollen_forecast`).Scan(&t); err != nil {
		return time.Time{}, false, err
	}
	if t == nil {
		return time.Time{}, false, nil
	}
	return t.UTC(), true, nil
}

// PollenLattice returns the grid points, stepC centidegrees apart and aligned
// to multiples of stepC, that lie within marginM metres of the country's
// boundary row. Empty when that row is not imported.
func (s *Store) PollenLattice(ctx context.Context, country string, stepC int, marginM float64) ([]PollenCell, error) {
	rows, err := s.pool.Query(ctx,
		`WITH b AS (
		   SELECT geom,
		          (floor(ST_XMin(geom::geometry) * 100 / $2::int) * $2::int)::int AS x0,
		          (ceil(ST_XMax(geom::geometry) * 100 / $2::int) * $2::int)::int AS x1,
		          (floor(ST_YMin(geom::geometry) * 100 / $2::int) * $2::int)::int AS y0,
		          (ceil(ST_YMax(geom::geometry) * 100 / $2::int) * $2::int)::int AS y1
		     FROM area
		    WHERE kind = 'country' AND country_code = $1
		 )
		 SELECT DISTINCT lon_c, lat_c
		   FROM b,
		        generate_series(b.x0, b.x1, $2::int) AS lon_c,
		        generate_series(b.y0, b.y1, $2::int) AS lat_c
		  WHERE ST_DWithin(b.geom, ST_SetSRID(ST_MakePoint(lon_c / 100.0, lat_c / 100.0), 4326)::geography, $3)
		  ORDER BY lon_c, lat_c`,
		country, stepC, marginM)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PollenCell
	for rows.Next() {
		var c PollenCell
		if err := rows.Scan(&c.LonC, &c.LatC); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PollenDailyQuery bounds AreaPollenDaily.
type PollenDailyQuery struct {
	From, To time.Time
	// Zone names the IANA zone days are cut in.
	Zone string
	// MinHours drops a cell-day with fewer hourly values.
	MinHours int
	// ReachM is how far an area with no cell inside may look for its nearest.
	ReachM float64
	// Country is the one kind='country' row included; other countries are not covered.
	Country string
}

// AreaPollenDay is one area's daily figure for one species.
type AreaPollenDay struct {
	Slug, Species string
	// Day is the local date, YYYY-MM-DD.
	Day string
	// Mean is the worst cell's daily mean; Max the worst hourly value.
	Mean, Max float64
	Hours     int
}

// AreaPollenDaily aggregates the stored forecast per area and local day: the
// cells inside an area, or its nearest cell within reach when none is inside.
func (s *Store) AreaPollenDaily(ctx context.Context, q PollenDailyQuery) ([]AreaPollenDay, error) {
	rows, err := s.pool.Query(ctx,
		`WITH cells AS (
		   SELECT DISTINCT lon_c, lat_c,
		          ST_SetSRID(ST_MakePoint(lon_c / 100.0, lat_c / 100.0), 4326)::geography AS g
		     FROM pollen_forecast
		    WHERE valid_at >= $1 AND valid_at < $2
		 ), areas AS (
		   SELECT slug, geom FROM area WHERE kind <> 'country' OR country_code = $6
		 ), inside AS (
		   SELECT a.slug, c.lon_c, c.lat_c FROM areas a JOIN cells c ON ST_Covers(a.geom, c.g)
		 ), nearest AS (
		   SELECT a.slug, n.lon_c, n.lat_c
		     FROM areas a
		     CROSS JOIN LATERAL (
		       SELECT c.lon_c, c.lat_c FROM cells c
		        WHERE ST_DWithin(a.geom, c.g, $5)
		        ORDER BY ST_Distance(a.geom, c.g)
		        LIMIT 1) n
		    WHERE NOT EXISTS (SELECT 1 FROM inside i WHERE i.slug = a.slug)
		 ), mapping AS (
		   SELECT slug, lon_c, lat_c FROM inside
		   UNION ALL
		   SELECT slug, lon_c, lat_c FROM nearest
		 ), daily AS (
		   SELECT lon_c, lat_c, species, (valid_at AT TIME ZONE $3)::date AS day,
		          avg(grains) AS mean, max(grains) AS peak, count(*) AS hours
		     FROM pollen_forecast
		    WHERE valid_at >= $1 AND valid_at < $2
		    GROUP BY 1, 2, 3, 4
		 )
		 SELECT m.slug, d.species, to_char(d.day, 'YYYY-MM-DD'), max(d.mean), max(d.peak), max(d.hours)::int
		   FROM mapping m
		   JOIN daily d ON d.lon_c = m.lon_c AND d.lat_c = m.lat_c
		  WHERE d.hours >= $4
		  GROUP BY m.slug, d.species, d.day
		  ORDER BY m.slug, d.species, d.day`,
		q.From, q.To, q.Zone, q.MinHours, q.ReachM, q.Country)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AreaPollenDay
	for rows.Next() {
		var d AreaPollenDay
		if err := rows.Scan(&d.Slug, &d.Species, &d.Day, &d.Mean, &d.Max, &d.Hours); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
