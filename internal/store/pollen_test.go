package store_test

import (
	"testing"
	"time"

	"airbg.org/internal/store"
)

// seedCountry inserts a country row as a lon/lat box.
func seedCountry(t *testing.T, ctx contextT, pool poolT, slug, code string, minLon, minLat, maxLon, maxLat float64) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO area (slug, kind, name_bg, name_en, country_code, geom)
		 VALUES ($1, 'country', $1, $1, $2,
		         ST_Multi(ST_MakeEnvelope($3, $4, $5, $6, 4326))::geography)`,
		slug, code, minLon, minLat, maxLon, maxLat)
	if err != nil {
		t.Fatalf("seed country %s: %v", slug, err)
	}
}

func TestWritePollenReplacesAnEarlierRunOfTheSameHour(t *testing.T) {
	ctx, pool, s := newStore(t)
	hour := time.Date(2026, 9, 5, 14, 0, 0, 0, time.UTC)

	if _, err := s.WritePollen(ctx, []store.PollenForecast{{LonC: 2330, LatC: 4270, Species: "ragweed", ValidAt: hour, Grains: 3}}, hour); err != nil {
		t.Fatalf("WritePollen: %v", err)
	}
	if _, err := s.WritePollen(ctx, []store.PollenForecast{{LonC: 2330, LatC: 4270, Species: "ragweed", ValidAt: hour, Grains: 9}}, hour.Add(time.Hour)); err != nil {
		t.Fatalf("WritePollen (second run): %v", err)
	}

	var n int
	var grains float64
	if err := pool.QueryRow(ctx, `SELECT count(*), max(grains) FROM pollen_forecast`).Scan(&n, &grains); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 || grains != 9 {
		t.Errorf("rows = %d, grains = %v; want 1 row holding the second run's 9", n, grains)
	}

	latest, ok, err := s.LatestPollenFetch(ctx)
	if err != nil || !ok {
		t.Fatalf("LatestPollenFetch = %v, %v, %v", latest, ok, err)
	}
	if !latest.Equal(hour.Add(time.Hour)) {
		t.Errorf("LatestPollenFetch = %v, want %v", latest, hour.Add(time.Hour))
	}
}

func TestLatestPollenFetchEmpty(t *testing.T) {
	ctx, _, s := newStore(t)
	if _, ok, err := s.LatestPollenFetch(ctx); err != nil || ok {
		t.Errorf("LatestPollenFetch on an empty table = ok %v, err %v; want false, nil", ok, err)
	}
}

// The lattice is the country's bounding box at the step, aligned to multiples
// of the step, and clipped to cells within the margin of the country itself.
func TestPollenLatticeClipsToTheCountry(t *testing.T) {
	ctx, pool, s := newStore(t)
	seedCountry(t, ctx, pool, "bulgaria", "BG", 23.05, 42.05, 23.55, 42.35)
	seedCountry(t, ctx, pool, "greece", "GR", 21.0, 39.0, 22.0, 40.0)

	cells, err := s.PollenLattice(ctx, "BG", 20, 0)
	if err != nil {
		t.Fatalf("PollenLattice: %v", err)
	}
	// Inside the box: lon 23.20, 23.40; lat 42.20. Nothing from the GR row.
	want := []store.PollenCell{{LonC: 2320, LatC: 4220}, {LonC: 2340, LatC: 4220}}
	if len(cells) != len(want) {
		t.Fatalf("cells = %+v, want %+v", cells, want)
	}
	for i := range want {
		if cells[i] != want[i] {
			t.Errorf("cells[%d] = %+v, want %+v", i, cells[i], want[i])
		}
	}

	// A margin admits the ring of cells just outside the border.
	wide, err := s.PollenLattice(ctx, "BG", 20, 10000)
	if err != nil {
		t.Fatalf("PollenLattice with margin: %v", err)
	}
	if len(wide) <= len(cells) {
		t.Errorf("margin 10 km gave %d cells, want more than %d", len(wide), len(cells))
	}
}

func TestPollenLatticeWithoutTheCountryIsEmpty(t *testing.T) {
	ctx, _, s := newStore(t)
	cells, err := s.PollenLattice(ctx, "BG", 20, 0)
	if err != nil {
		t.Fatalf("PollenLattice: %v", err)
	}
	if len(cells) != 0 {
		t.Errorf("cells = %+v, want none", cells)
	}
}

// writeHours writes one value per hour for [from, from+n hours).
func writeHours(t *testing.T, s *store.Store, ctxv contextT, lonC, latC int, species string, from time.Time, n int, grains func(i int) float64) {
	t.Helper()
	rows := make([]store.PollenForecast, n)
	for i := range rows {
		rows[i] = store.PollenForecast{LonC: lonC, LatC: latC, Species: species, ValidAt: from.Add(time.Duration(i) * time.Hour), Grains: grains(i)}
	}
	if _, err := s.WritePollen(ctxv, rows, from); err != nil {
		t.Fatalf("WritePollen: %v", err)
	}
}

func dailyQuery(from time.Time) store.PollenDailyQuery {
	return store.PollenDailyQuery{
		From: from, To: from.Add(48 * time.Hour),
		Zone: "Europe/Sofia", MinHours: 12, ReachM: 25000, Country: "BG",
	}
}

func findDay(rows []store.AreaPollenDay, slug, species, day string) (store.AreaPollenDay, bool) {
	for _, r := range rows {
		if r.Slug == slug && r.Species == species && r.Day == day {
			return r, true
		}
	}
	return store.AreaPollenDay{}, false
}

// An area's daily value is the worst cell inside it: the max of the cells'
// daily means, and the max of their hourly values.
func TestAreaPollenDailyTakesTheWorstCellInside(t *testing.T) {
	ctx, pool, s := newStore(t)
	_, err := pool.Exec(ctx,
		`INSERT INTO area (slug, kind, name_bg, name_en, geom)
		 VALUES ('big', 'oblast', 'big', 'big', ST_Multi(ST_MakeEnvelope(23.1, 42.5, 23.5, 42.9, 4326))::geography)`)
	if err != nil {
		t.Fatalf("seed area: %v", err)
	}
	// Local midnight, Europe/Sofia in October is UTC+3.
	day0 := time.Date(2026, 10, 3, 21, 0, 0, 0, time.UTC)
	writeHours(t, s, ctx, 2320, 4260, "ragweed", day0, 24, func(int) float64 { return 2 })
	writeHours(t, s, ctx, 2340, 4280, "ragweed", day0, 24, func(i int) float64 {
		if i == 5 {
			return 40
		}
		return 4
	})
	// Outside the area: must not count.
	writeHours(t, s, ctx, 2400, 4300, "ragweed", day0, 24, func(int) float64 { return 500 })

	rows, err := s.AreaPollenDaily(ctx, dailyQuery(day0))
	if err != nil {
		t.Fatalf("AreaPollenDaily: %v", err)
	}
	got, ok := findDay(rows, "big", "ragweed", "2026-10-04")
	if !ok {
		t.Fatalf("no row for big/ragweed/2026-10-04 in %+v", rows)
	}
	wantMean := (4.0*23 + 40) / 24
	if got.Mean != wantMean || got.Max != 40 {
		t.Errorf("mean, max = %v, %v; want %v, 40", got.Mean, got.Max, wantMean)
	}
}

// A district smaller than a cell takes the nearest cell within reach; an area
// beyond reach of every cell gets nothing rather than a far-away value.
func TestAreaPollenDailyFallsBackToTheNearestCell(t *testing.T) {
	ctx, pool, s := newStore(t)
	seedAreaBuffer(t, ctx, pool, "small", "neighbourhood", 23.33, 42.69, 1000)
	seedAreaBuffer(t, ctx, pool, "far", "neighbourhood", 27.9, 43.2, 1000)
	day0 := time.Date(2026, 10, 3, 21, 0, 0, 0, time.UTC)
	writeHours(t, s, ctx, 2340, 4260, "grass", day0, 24, func(int) float64 { return 6 })
	writeHours(t, s, ctx, 2380, 4260, "grass", day0, 24, func(int) float64 { return 60 })

	rows, err := s.AreaPollenDaily(ctx, dailyQuery(day0))
	if err != nil {
		t.Fatalf("AreaPollenDaily: %v", err)
	}
	got, ok := findDay(rows, "small", "grass", "2026-10-04")
	if !ok || got.Mean != 6 {
		t.Errorf("small = %+v (found %v), want the nearest cell's mean 6", got, ok)
	}
	if r, ok := findDay(rows, "far", "grass", "2026-10-04"); ok {
		t.Errorf("far = %+v, want no row: no cell within reach", r)
	}
}

// Days are local (Europe/Sofia), and a day with too few hours is dropped
// rather than averaged over the hours it has.
func TestAreaPollenDailyUsesLocalDaysAndDropsShortDays(t *testing.T) {
	ctx, pool, s := newStore(t)
	seedAreaBuffer(t, ctx, pool, "town", "city", 23.4, 42.6, 3000)
	day0 := time.Date(2026, 10, 3, 21, 0, 0, 0, time.UTC) // 00:00 on the 4th, local
	// 24 hours of the 4th, then 6 hours of the 5th.
	writeHours(t, s, ctx, 2340, 4260, "mugwort", day0, 30, func(i int) float64 {
		if i >= 24 {
			return 100
		}
		return 1
	})

	rows, err := s.AreaPollenDaily(ctx, dailyQuery(day0))
	if err != nil {
		t.Fatalf("AreaPollenDaily: %v", err)
	}
	got, ok := findDay(rows, "town", "mugwort", "2026-10-04")
	if !ok || got.Mean != 1 || got.Max != 1 || got.Hours != 24 {
		t.Errorf("2026-10-04 = %+v (found %v), want mean 1, max 1, 24 hours", got, ok)
	}
	if r, ok := findDay(rows, "town", "mugwort", "2026-10-05"); ok {
		t.Errorf("2026-10-05 = %+v, want dropped: 6 hours is below MinHours", r)
	}
}

// Other countries' rows are boundary filters, not places this lattice covers.
func TestAreaPollenDailySkipsOtherCountries(t *testing.T) {
	ctx, pool, s := newStore(t)
	seedCountry(t, ctx, pool, "bulgaria", "BG", 23.1, 42.5, 23.5, 42.9)
	seedCountry(t, ctx, pool, "serbia", "RS", 23.45, 42.5, 23.9, 42.9)
	day0 := time.Date(2026, 10, 3, 21, 0, 0, 0, time.UTC)
	writeHours(t, s, ctx, 2340, 4260, "birch", day0, 24, func(int) float64 { return 3 })

	rows, err := s.AreaPollenDaily(ctx, dailyQuery(day0))
	if err != nil {
		t.Fatalf("AreaPollenDaily: %v", err)
	}
	if _, ok := findDay(rows, "bulgaria", "birch", "2026-10-04"); !ok {
		t.Errorf("no row for the configured country in %+v", rows)
	}
	if r, ok := findDay(rows, "serbia", "birch", "2026-10-04"); ok {
		t.Errorf("serbia = %+v, want no row", r)
	}
}
