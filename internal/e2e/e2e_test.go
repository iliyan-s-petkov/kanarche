//go:build e2e

// Package e2e boots the real stack and drives it with Playwright.
//
// A Go TEST rather than a Go command, and therefore Playwright is launched by
// Go rather than the other way round: testsupport.NewPostgres takes a *testing.T
// (container lifetime is tied to t.Cleanup), which a plain main() cannot supply.
// Inverting it would mean a second, divergent copy of the container setup.
package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/db"
	"kanarche.eu/internal/server"
	"kanarche.eu/internal/store"
	"kanarche.eu/internal/testsupport"
)

func TestBrowser(t *testing.T) {
	ctx := context.Background()
	pool := testsupport.NewPostgres(t)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// The same airbg.yaml the binary ships with — an E2E against a bespoke
	// config proves the config nobody runs. KANARCHE_DATABASE_URL is only
	// present to satisfy config.Validate; the pool this test actually talks
	// to is the container pool above, wired in directly via store.New.
	t.Setenv(config.PathEnv, filepath.Join("..", "..", "airbg.yaml"))
	t.Setenv(config.DatabaseURLEnv, "postgres://user:pass@localhost:5432/airbg")
	widenRateLimits(t)
	t.Setenv("KANARCHE_GEOCODER_URL", startGeocoderStub(t).URL)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	// design_kit.dir is the path the kit lands on INSIDE the image; on a
	// developer's machine it is the repo's own copy, or the route 404s.
	if kit, err := filepath.Abs(filepath.Join("..", "..", "design-kit")); err == nil {
		cfg.DesignKit.Dir = kit
	}

	st := store.New(pool, cfg.Store, cfg.Database.StatementTimeouts.Series)
	seedFixtures(t, st) // the fixtures every spec relies on; see seedFixtures

	public, _ := testsupport.StartServer(t, st, cfg, func(o *server.Options) {
		// The page treats a snapshot older than 30 minutes as stale and drops
		// the area chips, which shifts the map layout. Production republishes
		// every cycle; a run that outlasts 30 minutes needs the same.
		pub := o.Publisher
		stop := make(chan struct{})
		t.Cleanup(func() { close(stop) })
		go func() {
			tick := time.NewTicker(10 * time.Minute)
			defer tick.Stop()
			for {
				select {
				case <-stop:
					return
				case now := <-tick.C:
					_ = pub.Publish(context.Background(), now.UTC())
				}
			}
		}()
	})
	baseURL := "http://" + public

	args := []string{"playwright", "test"}
	// E2E_SHARD=i/N runs one slice of the suite; unset runs all of it.
	if shard := os.Getenv("E2E_SHARD"); shard != "" {
		args = append(args, "--shard="+shard)
	}
	// E2E_SPEC=e2e/sea.spec.js runs one spec file; unset runs all of them.
	if spec := os.Getenv("E2E_SPEC"); spec != "" {
		args = append(args, spec)
	}
	// E2E_ARGS adds Playwright flags, space separated: --repeat-each=5 or -g skip (no quoting).
	args = append(args, strings.Fields(os.Getenv("E2E_ARGS"))...)
	cmd := startGroup(e2eContext(t), groupGrace, "npx", args...)
	cmd.Dir = filepath.Join("..", "..", "web")
	// Playwright's transform cache defaults to os.tmpdir(); a repo-scoped TMPDIR
	// leaves it in the tree and trips the deploy dirty-tree guard.
	cmd.Env = append(os.Environ(),
		"KANARCHE_E2E_BASE_URL="+baseURL,
		"PWTEST_CACHE_DIR="+filepath.Join(t.TempDir(), "pw-cache"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("playwright: %v", err)
	}
	// Reaps Chrome workers left behind if the runner exits early.
	t.Cleanup(func() { killGroup(cmd) })
	if err := cmd.Wait(); err != nil {
		t.Fatalf("playwright: %v", err)
	}
}

// seedFixtures inserts the data every Playwright spec in this phase asserts
// against. New specs add data here rather than seeding their own, so the
// fixture set stays a single source of truth.
//
// All SQL below is parameterised — the query text is a fixed literal with
// placeholders, values travel as bound parameters, and the polygon WKT built
// with fmt.Sprintf is itself bound as a single parameter — copied from
// internal/server/e2e_test.go's seedArea/seedReading, which explain why that
// is not the string-concatenated-SQL pattern the project forbids.
func seedFixtures(t *testing.T, st *store.Store) {
	t.Helper()
	ctx := context.Background()

	const (
		lon = 23.32
		lat = 42.69
		// Comfortably outside the ~0.02° spread the sensors below sit in, and
		// area.AssignSensors (run by testsupport.StartServer) assigns by real
		// ST_Covers containment, so the polygon must actually cover them.
		delta = 0.5
	)
	wkt := fmt.Sprintf(
		"MULTIPOLYGON(((%f %f, %f %f, %f %f, %f %f, %f %f)))",
		lon-delta, lat-delta,
		lon+delta, lat-delta,
		lon+delta, lat+delta,
		lon-delta, lat+delta,
		lon-delta, lat-delta,
	)

	// name_bg contains "Sofia" in Latin script deliberately: the JS-disabled
	// smoke spec hits the unprefixed /area/sofia route, which i18n.DefaultLang
	// ("bg") renders with name_bg. name_en is distinct so a later spec can
	// assert the /en/area/sofia route renders the English name specifically.
	//
	// kind "city" (not "oblast"): area_derive_presentation (migration 00007)
	// sets default_zoom from kind, and default_zoom becomes the area page's
	// initial map zoom (data-zoom="{{.Area.Zoom}}" in area.gohtml). Task 13's
	// panel spec deep-links straight to #sensor=101 with no click to zoom in
	// first, so the area must open AT OR ABOVE zoom_sensor (11, airbg.yaml)
	// for the sensor tier's columnar data to load on that very first paint —
	// "oblast" (9) stays on the city tier and the deep-linked sensor can
	// never resolve. "city" (11) is exactly the zoom_sensor threshold.
	_, err := st.Pool().Exec(ctx,
		`INSERT INTO area (slug, kind, name_bg, name_en, geom)
		 VALUES ($1, $2, $3, $4, ST_SetSRID(ST_GeomFromText($5), 4326)::geography)`,
		"sofia", "city", "София (Sofia)", "Sofia", wkt)
	if err != nil {
		t.Fatalf("seed area: %v", err)
	}

	// A second, oblast-kind area over the same footprint. snapshot.Build
	// (internal/snapshot/build.go) draws the country tier from kind "oblast"
	// only — "sofia" above is kind "city" and never appears in it — and
	// locate.spec.js's find-me flow reads its candidate list entirely from
	// that country-tier response (islands/map.js's state.areas, populated on
	// the index page at zoom 7). Without an oblast entry near the geolocated
	// point, locateMe finds an empty list and reports "could not determine
	// your location" instead of navigating anywhere. Real data has exactly
	// this nesting — a city sits inside its oblast — so a second area row is
	// the accurate fixture, not a workaround. The slug embeds "sofia" so
	// locate.spec.js's `/\/area\/sofia/` assertion, which is not anchored,
	// still matches the URL this area's page lands on.
	_, err = st.Pool().Exec(ctx,
		`INSERT INTO area (slug, kind, name_bg, name_en, geom)
		 VALUES ($1, $2, $3, $4, ST_SetSRID(ST_GeomFromText($5), 4326)::geography)`,
		"sofia-oblast", "oblast", "Софийска област", "Sofia Oblast", wkt)
	if err != nil {
		t.Fatalf("seed oblast area: %v", err)
	}

	// 30 days of visitor totals for the /about chart (harness only).
	_, err = st.Pool().Exec(ctx,
		`INSERT INTO visitor_daily (day, uniques, requests, page_views, fetched_at)
		 SELECT d::date, 40 + (extract(day FROM d)::int * 7) % 25, 900, 300, now()
		 FROM generate_series(current_date - 29, current_date, interval '1 day') AS d`)
	if err != nil {
		t.Fatalf("seed visitor_daily: %v", err)
	}

	// 27 more oblasts, spaced 2° apart so their boxes never touch "sofia"'s
	// sensors or each other: /areas' ranked table (areaRows, kind "oblast")
	// needs 28 rows — Bulgaria's real count — to reproduce the per-page
	// select's option set (14, 7) and the resulting control-row wrap that
	// production shows; 2 rows was not enough for table.js to even mount
	// (it needs 2+), and a handful understated the OpenProject #609 CLS
	// reservation the table/pager islands need at each breakpoint.
	for i := 1; i <= 27; i++ {
		lon := 23.32 + float64(i)*2
		lat := 42.69
		boxWKT := fmt.Sprintf(
			"MULTIPOLYGON(((%f %f, %f %f, %f %f, %f %f, %f %f)))",
			lon-delta, lat-delta,
			lon+delta, lat-delta,
			lon+delta, lat+delta,
			lon-delta, lat+delta,
			lon-delta, lat-delta,
		)
		slug := fmt.Sprintf("oblast-%d", i)
		_, err = st.Pool().Exec(ctx,
			`INSERT INTO area (slug, kind, name_bg, name_en, geom)
			 VALUES ($1, $2, $3, $4, ST_SetSRID(ST_GeomFromText($5), 4326)::geography)`,
			slug, "oblast", fmt.Sprintf("Област %d", i), fmt.Sprintf("Oblast %d", i), boxWKT)
		if err != nil {
			t.Fatalf("seed oblast area %s: %v", slug, err)
		}
	}

	// A third area, kind "neighbourhood", wholly inside the "sofia" city
	// square above — store.AreaParents assigns it "sofia" as ParentSlug by
	// largest polygon overlap, giving the SEO6 breadcrumb/link-block specs a
	// real district to deep-link and the /areas directory a Sofia district to
	// list under the "sofia" group (§3 of the SEO6 plan).
	districtDelta := delta / 10
	districtWKT := fmt.Sprintf(
		"MULTIPOLYGON(((%f %f, %f %f, %f %f, %f %f, %f %f)))",
		lon-districtDelta, lat-districtDelta,
		lon+districtDelta, lat-districtDelta,
		lon+districtDelta, lat+districtDelta,
		lon-districtDelta, lat+districtDelta,
		lon-districtDelta, lat-districtDelta,
	)
	_, err = st.Pool().Exec(ctx,
		`INSERT INTO area (slug, kind, name_bg, name_en, geom)
		 VALUES ($1, $2, $3, $4, ST_SetSRID(ST_GeomFromText($5), 4326)::geography)`,
		"mladost", "neighbourhood", "Младост", "Mladost", districtWKT)
	if err != nil {
		t.Fatalf("seed district area: %v", err)
	}

	now := time.Now().UTC()

	// Two ordinary sensors: both metrics present, both readings ok.
	seedSensor(t, st, 101, 23.30, 42.68)
	seedReading(t, st, 101, "P1", 15.5, "ok", now)
	seedReading(t, st, 101, "P2", 10, "ok", now)

	seedSensor(t, st, 102, 23.34, 42.70)
	seedReading(t, st, 102, "P1", 20, "ok", now)
	seedReading(t, st, 102, "P2", 18, "ok", now)

	// P1 absent, P2 present: the reading table has no NOT NULL escape hatch
	// for "no value" (value is NOT NULL), so "null P1" here means no P1 row
	// exists for this sensor at all — the absent case the sensor panel must
	// render differently from a present-but-flagged value.
	seedSensor(t, st, 103, 23.31, 42.71)
	seedReading(t, st, 103, "P2", 22, "ok", now)

	// A non-ok quality flag on an otherwise ordinary sensor.
	seedSensor(t, st, 104, 23.33, 42.67)
	seedReading(t, st, 104, "P1", 12, "ok", now)
	seedReading(t, st, 104, "P2", 300, "stuck", now)

	// Wide weather figures on sensor 101: readout-overflow.spec.js needs a
	// "1013,0 hPa" card to prove no readout pushes the phone page sideways.
	// Three stations carry it: a citizen metric needs the coverage threshold
	// of stations to be the area headline.
	for _, id := range []int64{101, 102, 103} {
		seedReading(t, st, id, "pressure", 1013.04, "ok", now)
	}

	// Enough history on sensor 101's P2 series that a 24h chart is non-empty:
	// one point every two hours across the window.
	for i := 1; i <= 12; i++ {
		seedReading(t, st, 101, "P2", 10+float64(i%5), "ok", now.Add(-time.Duration(i)*2*time.Hour))
	}

	// One EEA reference station, inside the same fixture area. O3 is seeded only
	// here so the specs can exercise a metric one network alone reports.
	station := seedStation(t, st, lon, lat, "BG0050A", "София Дружба")
	seedReading(t, st, station, "P1", 18, "ok", now)
	seedReading(t, st, station, "O3", 40, "ok", now)
	// A 6-glyph ring figure ("2190,0"), for readout-overflow.spec.js.
	seedReading(t, st, station, "CO", 2190.04, "ok", now)

	// One bathing site east of the sensors, for sea.spec.js.
	sea := store.BathingData{
		Sites: []store.BathingSite{
			{ID: "BG0000000000000001", NameBG: "ПЛАЖ ТЕСТ", NameEN: "PLAZH TEST", Zone: "coastal", Lat: 42.75, Lon: 23.45, ProfileURL: "https://example.org/profile.pdf"},
			// A long-history site with a stale RZI sample: its card is the tallest one the phone sheet must hold.
			{ID: "BG0000000000000002", NameBG: "ДЪЛГА ИСТОРИЯ", NameEN: "LONG HISTORY", Zone: "coastal", Lat: 42.75, Lon: 23.53, ProfileURL: "https://example.org/profile2.pdf"},
			// A site whose in-season RZI sample carries an inspectorate remark.
			{ID: "BG0000000000000003", NameBG: "БЕЛЕЖКА", NameEN: "REMARK SITE", Zone: "coastal", Lat: 42.75, Lon: 23.37, ProfileURL: "https://example.org/profile3.pdf"},
		},
		Classes: func() []store.BathingClass {
			cs := []store.BathingClass{}
			for y := 2014; y <= 2022; y++ {
				cs = append(cs, store.BathingClass{SiteID: "BG0000000000000001", Season: y, Quality: "good"})
			}
			// Nineteen classified seasons, 2007 to 2025.
			for y := 2007; y <= 2025; y++ {
				cs = append(cs, store.BathingClass{SiteID: "BG0000000000000002", Season: y, Quality: "good"})
			}
			// Ten classified seasons for the remark site, so its card carries the season strip too.
			// The newest class comes from the EEA data hub, which adds a second line to the badge, as on the real Kavarna card.
			for y := 2016; y <= 2025; y++ {
				c := store.BathingClass{SiteID: "BG0000000000000003", Season: y, Quality: "good"}
				if y == 2025 {
					c.Source = "datahub"
				}
				cs = append(cs, c)
			}
			return append(cs,
				store.BathingClass{SiteID: "BG0000000000000001", Season: 2023, Quality: "good"},
				store.BathingClass{SiteID: "BG0000000000000001", Season: 2024, Quality: "excellent"},
			)
		}(),
		Samples: []store.BathingSample{
			{SiteID: "BG0000000000000001", Date: time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC), Season: 2024, EC: 15, ECBelowDetection: true, IE: 150},
			{SiteID: "BG0000000000000001", Date: time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC), Season: 2024, EC: 600, IE: 20, PreSeason: true},
			{SiteID: "BG0000000000000001", Date: time.Date(2023, 7, 3, 0, 0, 0, 0, time.UTC), Season: 2023, EC: 30, IE: 10},
		},
	}
	if err := st.ReplaceBathing(ctx, sea, now); err != nil {
		t.Fatalf("ReplaceBathing: %v", err)
	}
	// An RZI sample thirty days old is past the 21-day freshness window, so the card shows the stale note.
	// In season only: outside it the card shows the closed-season note instead, also a line of text.
	staleDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -30)
	rziFile := store.RZIFile{Source: "rzi_varna", URL: "https://example.org/rzi.pdf", SHA256: strings.Repeat("ab", 32), Bytes: 1, FetchedAt: now, ExtractorVersion: "e2e", Status: store.RZIStatusOK, RowsOK: 4}
	rziSample := store.RZISample{SiteID: "BG0000000000000002", Date: staleDay, Season: staleDay.Year(), EC: 20, IE: 10, Unit: "cfu/100ml"}
	remarkDay := remarkDates(now)[0]
	// Over 200 characters, so the one-line clamp has something to cut.
	const longRemark = "Пробата е взета след пороен дъжд. Водата в района на плажа е мътна, на брега има нанесени водорасли и дребни отпадъци след бурното вълнение от предишната нощ, затова се препоръчва повторно вземане на проба."
	remarkSample := store.RZISample{SiteID: "BG0000000000000003", Date: remarkDay, Season: remarkDay.Year(), EC: 20, IE: 10, Unit: "cfu/100ml", Remarks: longRemark}
	// Two older samples with their own remarks: the card shows the newest remark only, Details lists all three.
	olderRemarks := []store.RZISample{
		{SiteID: "BG0000000000000003", Date: remarkDates(now)[1], Season: remarkDay.Year(), EC: 30, IE: 15, Unit: "cfu/100ml", Remarks: "Втора бележка: водата е мътна след вълнение."},
		{SiteID: "BG0000000000000003", Date: remarkDates(now)[2], Season: remarkDay.Year(), EC: 40, IE: 20, Unit: "cfu/100ml", Remarks: "Трета бележка: пробата е взета при силен вятър."},
	}
	rziSamples := append([]store.RZISample{rziSample, remarkSample}, olderRemarks...)
	if err := st.ReplaceRZISource(ctx, "rzi_varna", rziFile, rziSamples); err != nil {
		t.Fatalf("ReplaceRZISource: %v", err)
	}

	// The 30d and 1y periods read reading_hourly, which only the rollup fills.
	if _, err := st.RollupAll(ctx); err != nil {
		t.Fatalf("RollupAll: %v", err)
	}

	// Pollen at one cell inside the fixture area, a day back and three ahead:
	// ragweed high, grass low.
	var pollen []store.PollenForecast
	start := now.Truncate(time.Hour).Add(-24 * time.Hour)
	for i := 0; i < 96; i++ {
		at := start.Add(time.Duration(i) * time.Hour)
		pollen = append(pollen,
			store.PollenForecast{LonC: 2330, LatC: 4270, Species: "ragweed", ValidAt: at, Grains: 60},
			store.PollenForecast{LonC: 2330, LatC: 4270, Species: "grass", ValidAt: at, Grains: 1})
	}
	if _, err := st.WritePollen(ctx, pollen, now.Add(-time.Hour)); err != nil {
		t.Fatalf("WritePollen: %v", err)
	}
}

// seedSensor upserts one sensor at (lon, lat). Every value travels as a bound
// parameter.
func seedSensor(t *testing.T, st *store.Store, sensorID int64, lon, lat float64) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	_, err := st.Pool().Exec(ctx,
		`INSERT INTO sensor (sensor_id, sensor_type, location, last_seen)
		 VALUES ($1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography, $5)
		 ON CONFLICT (sensor_id) DO UPDATE
		   SET location = EXCLUDED.location, last_seen = EXCLUDED.last_seen, active = true`,
		sensorID, "test-sensor", lon, lat, now)
	if err != nil {
		t.Fatalf("seedSensor(%d): %v", sensorID, err)
	}
}

// seedStation inserts one EEA reference station and returns its sensor_id.
// sensor_id comes from official_sensor_id_seq (migration 00011), the same
// sequence store.UpsertStations draws from, so the id lands in the reserved
// >= 9_000_000_000 range real official sensors use. Every value travels as a
// bound parameter.
func seedStation(t *testing.T, st *store.Store, lon, lat float64, code, name string) int64 {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	var id int64
	err := st.Pool().QueryRow(ctx,
		`INSERT INTO sensor (sensor_id, sensor_type, location, last_seen,
		                     source, station_code, station_name)
		 VALUES (nextval('official_sensor_id_seq'), 'eea_reference',
		         ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3,
		         'eea', $4, $5)
		 RETURNING sensor_id`,
		lon, lat, now, code, name).Scan(&id)
	if err != nil {
		t.Fatalf("seedStation(%s): %v", code, err)
	}
	return id
}

// seedReading inserts one reading for an already-seeded sensor. Every value
// travels as a bound parameter.
func seedReading(t *testing.T, st *store.Store, sensorID int64, metric string, value float64, quality string, at time.Time) {
	t.Helper()
	ctx := context.Background()

	_, err := st.Pool().Exec(ctx,
		`INSERT INTO reading (time, sensor_id, metric, value, quality)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (sensor_id, metric, time) DO UPDATE
		   SET value = EXCLUDED.value, quality = EXCLUDED.quality`,
		at, sensorID, metric, value, quality)
	if err != nil {
		t.Fatalf("seedReading(%d, %s): %v", sensorID, metric, err)
	}
}

// widenRateLimits widens the API, series and pages limits for this suite only
// (cold-load-per-test churns far more requests than a real visitor, and every
// browser shares one client IP); airbg.yaml itself is untouched. Pages covers
// /static, so a drained bucket 429s the map chunk and the spec times out.
func widenRateLimits(t *testing.T) {
	t.Helper()
	t.Setenv("KANARCHE_RATELIMIT_API_PER_SECOND", "500")
	t.Setenv("KANARCHE_RATELIMIT_API_BURST", "2000")
	t.Setenv("KANARCHE_RATELIMIT_SERIES_PER_SECOND", "50")
	t.Setenv("KANARCHE_RATELIMIT_SERIES_BURST", "200")
	t.Setenv("KANARCHE_RATELIMIT_PAGES_PER_SECOND", "1000")
	t.Setenv("KANARCHE_RATELIMIT_PAGES_BURST", "10000")
	t.Setenv("KANARCHE_RATELIMIT_GEOCODE_PER_SECOND", "50")
	t.Setenv("KANARCHE_RATELIMIT_GEOCODE_BURST", "200")
	t.Setenv("KANARCHE_GEOCODER_UPSTREAM_PER_SECOND", "50")
}

// startGeocoderStub stands in for Nominatim; the real one is never called. A
// query containing "nowhere" gets no match, anything else one match in Sofia.
func startGeocoderStub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(strings.ToLower(r.URL.Query().Get("q")), "nowhere") {
			_, _ = w.Write([]byte("[]"))
			return
		}
		_, _ = w.Write([]byte(`[{"display_name":"Stub street 1, Sofia, Bulgaria","lat":"42.6977","lon":"23.3219","boundingbox":["42.6970","42.6984","23.3210","23.3228"]}]`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// remarkDates returns the three remarked samples' days, newest first. They are 3, 10 and 17 days
// before now, and all in one calendar year so they share a season. Early in January the newest
// would sit within 14 days of New Year, so the trio moves to the end of the previous year instead.
func remarkDates(now time.Time) [3]time.Time {
	newest := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -3)
	if newest.YearDay() <= 14 {
		newest = time.Date(newest.Year()-1, time.December, 31, 0, 0, 0, 0, time.UTC)
	}
	return [3]time.Time{newest, newest.AddDate(0, 0, -7), newest.AddDate(0, 0, -14)}
}
