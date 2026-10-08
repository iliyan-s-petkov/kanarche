package area_test

import (
	"testing"

	"kanarche.eu/internal/area"
)

// TestCommittedBoundariesImport imports the actual committed files, not a
// fixture. Phase 1 shipped a bulgaria.geojson that Import silently accepted as
// zero features, because every boundary test used a hand-written fixture and
// nothing ever read the real file. These assertions exist so that cannot recur.
func TestCommittedBoundariesImport(t *testing.T) {
	ctx, pool := migrated(t)

	for _, tc := range []struct {
		path    string
		kind    string
		wantMin int
		wantMax int
	}{
		// Bulgaria has 28 oblasti. The range is exact because the count is a
		// fact about the country, not an implementation detail.
		{"../../data/boundaries/oblasti.geojson", "oblast", 28, 28},
		// One city per oblast capital, but Sofia is capital of both Sofia-grad
		// and Sofia Oblast (confirmed via OSM: Sofia Oblast's admin_centre node
		// sits at the same coordinates as central Sofia), so 28 oblasti share
		// only 27 distinct capital cities. 27 is exact for the same reason 28
		// is exact above — it is a fact about the country, not a fixture
		// artifact. A file with 28 would mean a duplicated Sofia polygon.
		{"../../data/boundaries/cities.geojson", "city", 27, 27},
		// Sofia has 24 raiони (districts).
		{"../../data/boundaries/sofia-districts.geojson", "neighbourhood", 24, 24},
	} {
		n, err := area.Import(ctx, pool, tc.path, tc.kind)
		if err != nil {
			t.Fatalf("Import(%s, %s): %v", tc.path, tc.kind, err)
		}
		if n < tc.wantMin || n > tc.wantMax {
			t.Errorf("Import(%s) = %d features, want %d..%d", tc.path, n, tc.wantMin, tc.wantMax)
		}
	}
}

// TestSofiaSensorResolvesThroughAllTiers asserts a single point lands in an
// oblast, a city AND a district. Importing three files that each parse is not
// the requirement; the requirement is that the three tiers nest, because
// /overview?tier=city and /area/{slug}/sensors both depend on a sensor being
// reachable at more than one zoom.
func TestSofiaSensorResolvesThroughAllTiers(t *testing.T) {
	ctx, pool := migrated(t)

	for _, f := range []struct{ path, kind string }{
		{"../../data/boundaries/oblasti.geojson", "oblast"},
		{"../../data/boundaries/cities.geojson", "city"},
		{"../../data/boundaries/sofia-districts.geojson", "neighbourhood"},
	} {
		if _, err := area.Import(ctx, pool, f.path, f.kind); err != nil {
			t.Fatalf("Import(%s): %v", f.path, err)
		}
	}

	// Sofia, Lozenets: lon 23.3219, lat 42.6977. Longitude first — PostGIS
	// geography is (lon, lat), the inverse of the legacy [lat, long] order.
	const lon, lat = 23.3219, 42.6977

	rows, err := pool.Query(ctx,
		`SELECT kind FROM area
		  WHERE ST_Covers(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)
		  ORDER BY kind`,
		lon, lat)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	kinds := map[string]bool{}
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			t.Fatalf("scan: %v", err)
		}
		kinds[kind] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	for _, want := range []string{"oblast", "city", "neighbourhood"} {
		if !kinds[want] {
			t.Errorf("Sofia point (%v, %v) is not covered by any %s area", lon, lat, want)
		}
	}
}

// TestAllFourFilesImportWithoutRowLoss imports bulgaria.geojson, oblasti.geojson,
// cities.geojson and sofia-districts.geojson together, in the order the README
// documents, and asserts the resulting per-kind row counts in the database —
// not just the parsed feature counts Import returns.
//
// This is the test that would have caught the slug collision between oblasti
// and cities: 26 of 27 city slugs used to be byte-identical to an oblast slug
// (same Bulgarian name, same transliteration — "Варна" oblast and "Варна"
// city both slugified to "varna"). area.slug is the table's global PRIMARY
// KEY across every kind, and Import's ON CONFLICT (slug) DO UPDATE SET kind =
// EXCLUDED.kind, ... rewrote 26 oblast rows into city rows on the second
// import, leaving only 2 oblasti in the table. Every existing test still
// passed: TestCommittedBoundariesImport only checks Import's returned feature
// count (a property of the parsed JSON, unconditionally correct regardless of
// what the database ends up holding), and
// TestSofiaSensorResolvesThroughAllTiers only probes the one point whose
// oblast slugs (sofia-grad/sofia oblast) happen not to collide with any city
// slug. A per-kind count after all imports is the one assertion that cannot
// be fooled by a row silently changing kind.
func TestAllFourFilesImportWithoutRowLoss(t *testing.T) {
	ctx, pool := migrated(t)

	for _, f := range []struct{ path, kind string }{
		{"../../data/boundaries/bulgaria.geojson", "country"},
		{"../../data/boundaries/oblasti.geojson", "oblast"},
		{"../../data/boundaries/cities.geojson", "city"},
		{"../../data/boundaries/sofia-districts.geojson", "neighbourhood"},
	} {
		if _, err := area.Import(ctx, pool, f.path, f.kind); err != nil {
			t.Fatalf("Import(%s, %s): %v", f.path, f.kind, err)
		}
	}

	wantByKind := map[string]int{
		"country":       1,
		"oblast":        28,
		"city":          27,
		"neighbourhood": 24,
	}

	rows, err := pool.Query(ctx, `SELECT kind, count(*) FROM area GROUP BY kind`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	gotByKind := map[string]int{}
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		gotByKind[kind] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	for kind, want := range wantByKind {
		if got := gotByKind[kind]; got != want {
			t.Errorf("area rows with kind = %q: got %d, want %d (got map: %v)", kind, got, want, gotByKind)
		}
	}
	for kind := range gotByKind {
		if _, known := wantByKind[kind]; !known {
			t.Errorf("unexpected kind %q in area table with %d rows", kind, gotByKind[kind])
		}
	}

	// A row silently changing kind (Finding 1's failure mode) never changes
	// the total row count or the distinct-slug count on its own, but a slug
	// collision that caused one row to overwrite another WOULD show up here:
	// distinct slugs would be lower than the row count only if two different
	// logical rows had been coalesced into one by a primary-key clash.
	var total, distinctSlugs int
	err = pool.QueryRow(ctx, `SELECT count(*), count(DISTINCT slug) FROM area`).Scan(&total, &distinctSlugs)
	if err != nil {
		t.Fatalf("distinct slug query: %v", err)
	}
	if distinctSlugs != total {
		t.Errorf("count(DISTINCT slug) = %d, count(*) = %d — slugs are not unique across kinds", distinctSlugs, total)
	}
}

// TestBoundaryPartsAssembleIntoAPlausibleArea catches a regeneration that
// keeps every OSM relation member way as its own ring instead of assembling
// them into the relation's actual outer/inner rings (ST_LineMerge +
// ST_BuildArea, or ST_Polygonize, over the unioned member ways). That defect
// still parses, still imports, and still produces a MultiPolygon with a
// non-zero area — it just traces only the perimeter, so the result is a
// sliver instead of a country, oblast or city.
//
// Two independent signals catch it because either one alone can be fooled:
//
//   - Total area. A perimeter sliver is orders of magnitude smaller than the
//     real place, but not necessarily zero, so a raw "is it > 0" check would
//     pass on a broken file.
//   - Containment of a known interior point. A ring-per-way MultiPolygon can
//     still integrate to a plausible-looking total area (many thin slivers
//     add up) while not covering the point a reader would expect the area to
//     contain, e.g. the city centre.
//
// Plovdiv is the motivating case: 19 fragments tracing only the perimeter,
// ~24 km^2 total against a real ~102 km^2 municipality, and its own city
// centre falls outside every one of the 19 parts.
func TestBoundaryPartsAssembleIntoAPlausibleArea(t *testing.T) {
	ctx, pool := migrated(t)

	for _, f := range []struct{ path, kind string }{
		{"../../data/boundaries/oblasti.geojson", "oblast"},
		{"../../data/boundaries/cities.geojson", "city"},
		{"../../data/boundaries/sofia-districts.geojson", "neighbourhood"},
	} {
		if _, err := area.Import(ctx, pool, f.path, f.kind); err != nil {
			t.Fatalf("Import(%s, %s): %v", f.path, f.kind, err)
		}
	}

	// Minimum plausible area in km^2, by kind. An oblast is a chunk of a
	// country; even the smallest Bulgarian oblast is well over 1000 km^2. A
	// city — city-proper or whole municipality, see docs/known-limitations.md
	// — is at least a few km^2; Sofia's districts are smaller still but each
	// is a real neighbourhood, not a sliver.
	minAreaKM2 := map[string]float64{
		"oblast":        1000,
		"city":          5,
		"neighbourhood": 0.2,
	}

	rows, err := pool.Query(ctx,
		`SELECT slug, kind, ST_Area(geom) / 1e6 FROM area WHERE kind IN ('oblast', 'city', 'neighbourhood') ORDER BY slug`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var slug, kind string
		var areaKM2 float64
		if err := rows.Scan(&slug, &kind, &areaKM2); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if want := minAreaKM2[kind]; areaKM2 < want {
			t.Errorf("area %q (kind %s): %.2f km^2, want >= %v km^2 — looks like a perimeter sliver, not an assembled polygon", slug, kind, areaKM2, want)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	// Known interior points, lon/lat (PostGIS geography order), for cities
	// whose union of parts must contain them.
	knownPoints := []struct {
		slug     string
		lon, lat float64
	}{
		{"plovdiv", 24.7490, 42.1420},
		{"sofiya", 23.3219, 42.6977},
		{"varna", 27.9147, 43.2141},
		{"burgas", 27.4626, 42.5048},
		{"ruse", 25.9657, 43.8356},
	}

	for _, p := range knownPoints {
		var covers bool
		err := pool.QueryRow(ctx,
			`SELECT ST_Covers(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)
			   FROM area WHERE slug = $3 AND kind = 'city'`,
			p.lon, p.lat, p.slug,
		).Scan(&covers)
		if err != nil {
			t.Fatalf("point-in-polygon query for %q: %v", p.slug, err)
		}
		if !covers {
			t.Errorf("city %q does not cover its own known interior point (%v, %v) — assembled polygon is missing the centre", p.slug, p.lon, p.lat)
		}
	}
}

// TestBoundariesDoNotSwapCoordinates is the swap detector for this data. A
// GeoJSON file written with [lat, lon] instead of [lon, lat] still parses, still
// imports, and still produces valid polygons — they simply sit in the Indian
// Ocean. Asserting the bbox catches it; asserting "the import succeeded" does
// not.
func TestBoundariesDoNotSwapCoordinates(t *testing.T) {
	ctx, pool := migrated(t)

	if _, err := area.Import(ctx, pool, "../../data/boundaries/oblasti.geojson", "oblast"); err != nil {
		t.Fatalf("Import: %v", err)
	}

	var minLon, minLat, maxLon, maxLat float64
	err := pool.QueryRow(ctx,
		`SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e)
		   FROM (SELECT ST_Extent(geom::geometry) AS e FROM area WHERE kind = 'oblast') s`,
	).Scan(&minLon, &minLat, &maxLon, &maxLat)
	if err != nil {
		t.Fatalf("extent: %v", err)
	}

	// Bulgaria: lon 22.3..28.7, lat 41.2..44.3. The two ranges do not overlap,
	// which is exactly what makes a swap detectable.
	if minLon < 22.0 || maxLon > 29.0 {
		t.Errorf("oblast longitude extent %v..%v outside Bulgaria's 22..29 (values in 41..45 mean lat/lon are swapped)", minLon, maxLon)
	}
	if minLat < 41.0 || maxLat > 45.0 {
		t.Errorf("oblast latitude extent %v..%v outside Bulgaria's 41..45", minLat, maxLat)
	}
}

// A seam left by unioning the oblasti shows up as one oblast's point-on-surface falling outside.
func TestCountryBoundaryCoversEveryOblast(t *testing.T) {
	ctx, pool := migrated(t)

	if _, err := area.Import(ctx, pool, "../../data/boundaries/bulgaria.geojson", area.NationalBoundaryKind); err != nil {
		t.Fatalf("Import(bulgaria.geojson): %v", err)
	}
	if _, err := area.Import(ctx, pool, "../../data/boundaries/oblasti.geojson", "oblast"); err != nil {
		t.Fatalf("Import(oblasti.geojson): %v", err)
	}

	rows, err := pool.Query(ctx,
		`SELECT slug FROM area WHERE kind = 'oblast' ORDER BY slug`)
	if err != nil {
		t.Fatalf("query oblast slugs: %v", err)
	}
	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			t.Fatalf("scan: %v", err)
		}
		slugs = append(slugs, slug)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(slugs) != 28 {
		t.Fatalf("got %d oblasti, want 28", len(slugs))
	}

	for _, slug := range slugs {
		var covered bool
		err := pool.QueryRow(ctx, `
			SELECT ST_Covers(
				(SELECT geom FROM area WHERE slug = 'bulgaria'),
				ST_PointOnSurface((SELECT geom::geometry FROM area WHERE slug = $1))::geography
			)`, slug).Scan(&covered)
		if err != nil {
			t.Fatalf("ST_Covers for %s: %v", slug, err)
		}
		if !covered {
			t.Errorf("oblast %q has a point-on-surface not covered by the country boundary — seam or gap left by the union", slug)
		}
	}
}

// FilterByBoundary breaks overlaps by country_code, so any overlap hands a neighbour's sensors to BG.
func TestCountryBoundaryDoesNotOverlapNeighbours(t *testing.T) {
	ctx, pool := migrated(t)

	if _, err := area.Import(ctx, pool, "../../data/boundaries/bulgaria.geojson", area.NationalBoundaryKind); err != nil {
		t.Fatalf("Import(bulgaria.geojson): %v", err)
	}

	const maxOverlapKM2 = 0.01

	for _, neighbour := range []string{
		"greece", "north-macedonia", "romania", "serbia", "turkey",
	} {
		path := "../../data/boundaries/" + neighbour + ".geojson"
		if _, err := area.Import(ctx, pool, path, area.NationalBoundaryKind); err != nil {
			t.Fatalf("Import(%s): %v", path, err)
		}
	}

	for _, neighbour := range []string{
		"greece", "north-macedonia", "romania", "serbia", "turkey",
	} {
		var areaKM2 float64
		err := pool.QueryRow(ctx, `
			SELECT COALESCE(ST_Area(
				ST_Intersection(bg.geom::geometry, nb.geom::geometry)::geography
			), 0) / 1e6
			FROM area bg, area nb
			WHERE bg.slug = 'bulgaria' AND nb.slug = $1`, neighbour).Scan(&areaKM2)
		if err != nil {
			t.Fatalf("ST_Intersection area for %s: %v", neighbour, err)
		}
		if areaKM2 > maxOverlapKM2 {
			t.Errorf("bulgaria overlaps %s by %.5f km^2, want <= %.2f km^2", neighbour, areaKM2, maxOverlapKM2)
		}
	}
}
