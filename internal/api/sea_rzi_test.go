package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/api"
	"kanarche.eu/internal/store"
)

const (
	seaCoastal = "BG3310610135003001"
	seaLake    = "BG3242661710017001"
	// The remark is hostile on purpose. No API byte may carry it.
	seaRemark      = "SECRET-REMARK <img src=x onerror=alert(1)>"
	seaRZIFileURL  = "https://rzi-varna.com/wp-content/uploads/2026/rezultati.pdf"
	seaVarnaPage   = "https://rzi-varna.com/?page_id=2635"
	seaRZIFetched  = "2026-07-20T06:00:00Z"
	seaVarnaSource = "rzi_varna"
)

var sofia = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		panic(err)
	}
	return loc
}()

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func rziFile(id int64) store.RZIFile {
	fetched, _ := time.Parse(time.RFC3339, seaRZIFetched)
	return store.RZIFile{ID: id, Source: seaVarnaSource, URL: seaRZIFileURL, FetchedAt: fetched, Status: store.RZIStatusOK}
}

func rziRow(site string, date time.Time, season, ec, ie int) store.RZIRow {
	return store.RZIRow{
		RZISample: store.RZISample{SiteID: site, Date: date, Season: season, EC: ec, IE: ie, Unit: "MPN/100ml", Remarks: seaRemark},
		File:      rziFile(1),
	}
}

// rziSource is the EEA fixture plus the given RZI rows for the Varna source.
func rziSource(rows ...store.RZIRow) *stubSource {
	src := seaSource()
	src.rzi = rows
	src.rziLastGood = map[string]store.RZIFile{seaVarnaSource: rziFile(1)}
	return src
}

func rziDeps(t *testing.T, src *stubSource, now time.Time) api.Deps {
	d := visitorsDeps(t, src)
	d.Clock = func() time.Time { return now }
	return d
}

func sofiaAt(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, sofia)
}

type seaLatestBody struct {
	Date             string `json:"date"`
	Source           string `json:"source"`
	EColi            int    `json:"e_coli"`
	EColiBelow       bool   `json:"e_coli_below_detection"`
	Enterococci      int    `json:"enterococci"`
	EnterococciBelow bool   `json:"enterococci_below_detection"`
	Unit             string `json:"unit"`
	Stale            bool   `json:"stale"`
}

// latestOf returns the coastal site's latest from /sites, nil when the key is absent.
func latestOf(t *testing.T, d api.Deps) (*seaLatestBody, []byte) {
	t.Helper()
	rec := serve(t, d, get("/api/v1/sea/sites", "203.0.113.7"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var b struct {
		Sites []struct {
			ID     string         `json:"id"`
			Latest *seaLatestBody `json:"latest"`
		} `json:"sites"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	for _, s := range b.Sites {
		if s.ID == seaCoastal {
			return s.Latest, rec.Body.Bytes()
		}
	}
	t.Fatal("coastal site missing")
	return nil, nil
}

func TestSeaLatestShape(t *testing.T) {
	row := rziRow(seaCoastal, day(2026, 7, 14), 2026, 15, 40)
	row.ECBelow = true
	older := rziRow(seaCoastal, day(2026, 7, 1), 2026, 900, 900)
	d := rziDeps(t, rziSource(older, row), sofiaAt(2026, 7, 20, 12, 0))
	got, raw := latestOf(t, d)
	want := seaLatestBody{Date: "2026-07-14", Source: seaVarnaSource, EColi: 15, EColiBelow: true, Enterococci: 40, Unit: "MPN/100ml"}
	if got == nil || *got != want {
		t.Fatalf("latest = %+v, want %+v", got, want)
	}
	// The lake site has no RZI rows: the key is absent, not null.
	var b struct {
		Sites []map[string]json.RawMessage `json:"sites"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	if _, has := b.Sites[0]["latest"]; has {
		t.Errorf("site without RZI data carries latest: %s", b.Sites[0]["latest"])
	}
}

// Stale is more than 21 calendar days by the Sofia date: day 21 is fresh, day 22 is stale.
func TestSeaStaleThreshold(t *testing.T) {
	sample := rziRow(seaCoastal, day(2026, 7, 1), 2026, 100, 50)
	for _, c := range []struct {
		name  string
		now   time.Time
		stale bool
	}{
		{"day 0", sofiaAt(2026, 7, 1, 12, 0), false},
		{"day 21 morning", sofiaAt(2026, 7, 22, 0, 0), false},
		{"day 21 last minute", sofiaAt(2026, 7, 22, 23, 59), false},
		{"day 22 first minute", sofiaAt(2026, 7, 23, 0, 0), true},
		{"day 40", sofiaAt(2026, 8, 10, 12, 0), true},
	} {
		got, _ := latestOf(t, rziDeps(t, rziSource(sample), c.now))
		if got == nil {
			t.Fatalf("%s: no latest", c.name)
		}
		if got.Stale != c.stale {
			t.Errorf("%s: stale = %v, want %v", c.name, got.Stale, c.stale)
		}
		if got.Date != "2026-07-01" {
			t.Errorf("%s: a stale sample keeps its date, got %q", c.name, got.Date)
		}
	}
}

// The day is cut in Sofia, not UTC: 21:00Z on 22 July is already 23 July in Sofia.
func TestSeaStaleUsesSofiaDay(t *testing.T) {
	sample := rziRow(seaCoastal, day(2026, 7, 1), 2026, 100, 50)
	for _, c := range []struct {
		now   time.Time
		stale bool
	}{
		{time.Date(2026, 7, 22, 20, 59, 0, 0, time.UTC), false},
		{time.Date(2026, 7, 22, 21, 0, 0, 0, time.UTC), true},
	} {
		got, _ := latestOf(t, rziDeps(t, rziSource(sample), c.now))
		if got == nil || got.Stale != c.stale {
			t.Errorf("now %s: latest = %+v, want stale %v", c.now, got, c.stale)
		}
	}
}

// The season is 15 May to 15 October inclusive by the Sofia date, as the collector has it.
func TestSeaLatestSeasonBoundaries(t *testing.T) {
	spring := rziRow(seaCoastal, day(2026, 5, 2), 2026, 10, 10)
	autumn := rziRow(seaCoastal, day(2026, 10, 5), 2026, 10, 10)
	for _, c := range []struct {
		name string
		row  store.RZIRow
		now  time.Time
		want bool
	}{
		{"14 May last minute", spring, sofiaAt(2026, 5, 14, 23, 59), false},
		{"15 May first minute", spring, sofiaAt(2026, 5, 15, 0, 0), true},
		{"15 October last minute", autumn, sofiaAt(2026, 10, 15, 23, 59), true},
		{"16 October first minute", autumn, sofiaAt(2026, 10, 16, 0, 0), false},
		{"January", autumn, sofiaAt(2027, 1, 10, 12, 0), false},
	} {
		got, _ := latestOf(t, rziDeps(t, rziSource(c.row), c.now))
		if (got != nil) != c.want {
			t.Errorf("%s: latest = %+v, want present %v", c.name, got, c.want)
		}
	}
}

func TestSeaLatestIsNewestVisibleSample(t *testing.T) {
	// 2024 is covered by the EEA for this site, so its RZI row is hidden and cannot lead.
	hidden := rziRow(seaCoastal, day(2024, 8, 20), 2024, 1, 1)
	d := rziDeps(t, rziSource(hidden), sofiaAt(2026, 7, 20, 12, 0))
	if got, _ := latestOf(t, d); got != nil {
		t.Errorf("latest from an EEA-covered season: %+v", got)
	}
}

type seaDetailRZI struct {
	Site struct {
		RZI *struct {
			Source    string `json:"source"`
			PageURL   string `json:"page_url"`
			FetchedAt string `json:"fetched_at"`
		} `json:"rzi"`
	} `json:"site"`
	Samples []struct {
		Date    string `json:"date"`
		Season  int    `json:"season"`
		Source  string `json:"source"`
		EColi   int    `json:"e_coli"`
		PreSeas bool   `json:"pre_season"`
	} `json:"samples"`
}

func detailOf(t *testing.T, d api.Deps, id string) (seaDetailRZI, []byte) {
	t.Helper()
	rec := serve(t, d, get("/api/v1/sea/sites/"+id, "203.0.113.7"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var b seaDetailRZI
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	return b, rec.Body.Bytes()
}

func TestSeaDetailCarriesSourceAndRZI(t *testing.T) {
	d := rziDeps(t, rziSource(
		rziRow(seaCoastal, day(2026, 7, 14), 2026, 120, 30),
		rziRow(seaCoastal, day(2026, 6, 30), 2026, 80, 20),
	), sofiaAt(2026, 7, 20, 12, 0))
	b, raw := detailOf(t, d, seaCoastal)
	if b.Site.RZI == nil {
		t.Fatalf("site.rzi missing: %s", raw)
	}
	if b.Site.RZI.Source != seaVarnaSource || b.Site.RZI.PageURL != seaVarnaPage || b.Site.RZI.FetchedAt != seaRZIFetched {
		t.Errorf("rzi = %+v", *b.Site.RZI)
	}
	if bytes.Contains(raw, []byte(seaRZIFileURL)) || bytes.Contains(raw, []byte("wp-content")) {
		t.Errorf("the file URL leaked into the body: %s", raw)
	}
	// Newest first across both sources.
	var order []string
	for _, s := range b.Samples {
		order = append(order, s.Date+" "+s.Source)
	}
	want := []string{"2026-07-14 rzi_varna", "2026-06-30 rzi_varna", "2024-07-02 eea", "2023-07-03 eea"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("samples = %v, want %v", order, want)
	}
}

func TestSeaDetailOfSiteWithoutRZIHasNoRZI(t *testing.T) {
	d := rziDeps(t, rziSource(rziRow(seaCoastal, day(2026, 7, 14), 2026, 120, 30)), sofiaAt(2026, 7, 20, 12, 0))
	_, raw := detailOf(t, d, seaLake)
	if bytes.Contains(raw, []byte(`"rzi"`)) || bytes.Contains(raw, []byte(`"source":"eea"`)) {
		t.Errorf("site without RZI rows carries RZI fields: %s", raw)
	}
}

// The same site and date from both sources shows the EEA row only.
func TestSeaEEAWinsOverRZIOnTheSameDate(t *testing.T) {
	// 2025 has no EEA data, so the RZI row is visible, except that the EEA also
	// holds a sample on that very date.
	src := rziSource(
		rziRow(seaCoastal, day(2025, 7, 2), 2025, 777, 777),
		rziRow(seaCoastal, day(2025, 7, 16), 2025, 111, 11),
	)
	src.bathing.Samples = append(src.bathing.Samples,
		store.BathingSample{SiteID: seaCoastal, Date: day(2025, 7, 2), Season: 2024, EC: 5, IE: 5})
	b, raw := detailOf(t, rziDeps(t, src, sofiaAt(2026, 3, 1, 12, 0)), seaCoastal)
	var onDate []string
	for _, s := range b.Samples {
		if s.Date == "2025-07-02" {
			onDate = append(onDate, s.Source)
			if s.EColi != 5 {
				t.Errorf("e_coli = %d, want the EEA value 5", s.EColi)
			}
		}
	}
	if len(onDate) != 1 || onDate[0] != "eea" {
		t.Errorf("2025-07-02 rows = %v, want one eea row: %s", onDate, raw)
	}
	if !bytes.Contains(raw, []byte(`"2025-07-16"`)) {
		t.Errorf("the other RZI row was dropped: %s", raw)
	}
}

// Once the EEA holds a season for the site, that season's RZI rows are hidden, not deleted.
func TestSeaEEACoveredSeasonsAreHidden(t *testing.T) {
	src := rziSource(
		rziRow(seaCoastal, day(2024, 8, 20), 2024, 1, 1), // EEA has samples for 2024
		rziRow(seaCoastal, day(2023, 9, 5), 2023, 2, 2),  // EEA has a 2023 class and sample
		rziRow(seaCoastal, day(2026, 7, 14), 2026, 3, 3), // not covered
		rziRow(seaLake, day(2024, 8, 21), 2024, 4, 4),    // lake has no EEA season, so its RZI row stays
	)
	b, raw := detailOf(t, rziDeps(t, src, sofiaAt(2026, 7, 20, 12, 0)), seaCoastal)
	for _, s := range b.Samples {
		if s.Source != "eea" && s.Season != 2026 {
			t.Errorf("RZI row of an EEA-covered season shown: %+v", s)
		}
	}
	if bytes.Contains(raw, []byte(`"2024-08-20"`)) || bytes.Contains(raw, []byte(`"2023-09-05"`)) || !bytes.Contains(raw, []byte(`"2026-07-14"`)) {
		t.Errorf("body = %s", raw)
	}
	lake, _ := detailOf(t, rziDeps(t, src, sofiaAt(2026, 7, 20, 12, 0)), seaLake)
	if len(lake.Samples) != 1 || lake.Samples[0].Date != "2024-08-21" {
		t.Errorf("lake samples = %+v, want its own RZI row", lake.Samples)
	}
}

// An EEA sample alone, with no class, also covers the season.
func TestSeaEEASampleCoversTheSeason(t *testing.T) {
	src := rziSource(rziRow(seaLake, day(2025, 8, 10), 2025, 1, 1))
	src.bathing.Samples = append(src.bathing.Samples,
		store.BathingSample{SiteID: seaLake, Date: day(2025, 7, 1), Season: 2025, EC: 5, IE: 5})
	b, raw := detailOf(t, rziDeps(t, src, sofiaAt(2026, 3, 1, 12, 0)), seaLake)
	if len(b.Samples) != 1 || b.Samples[0].Date != "2025-07-01" || b.Site.RZI != nil {
		t.Errorf("sample-covered season still shows RZI: %s", raw)
	}
}

// A class alone, with no EEA samples, also covers the season.
func TestSeaEEAClassCoversTheSeason(t *testing.T) {
	src := rziSource(rziRow(seaLake, day(2025, 8, 1), 2025, 1, 1))
	src.bathing.Classes = append(src.bathing.Classes, store.BathingClass{SiteID: seaLake, Season: 2025, Quality: "good"})
	b, raw := detailOf(t, rziDeps(t, src, sofiaAt(2026, 3, 1, 12, 0)), seaLake)
	if len(b.Samples) != 0 || b.Site.RZI != nil {
		t.Errorf("class-covered season still shows RZI: %s", raw)
	}
}

// After a manual approve the samples of one source sit under two files.
func TestSeaSamplesAcrossTwoFiles(t *testing.T) {
	oldRow := rziRow(seaCoastal, day(2026, 6, 10), 2026, 1, 1)
	oldRow.File = rziFile(1)
	oldRow.File.FetchedAt = oldRow.File.FetchedAt.Add(-48 * time.Hour)
	newRow := rziRow(seaCoastal, day(2026, 7, 14), 2026, 2, 2)
	newRow.File = rziFile(2)
	src := rziSource(oldRow, newRow)
	src.rziLastGood = map[string]store.RZIFile{seaVarnaSource: rziFile(2)}
	b, _ := detailOf(t, rziDeps(t, src, sofiaAt(2026, 7, 20, 12, 0)), seaCoastal)
	if b.Site.RZI == nil || b.Site.RZI.FetchedAt != seaRZIFetched {
		t.Errorf("rzi = %+v, want fetched_at of the last good file %s", b.Site.RZI, seaRZIFetched)
	}
	if len(b.Samples) != 4 {
		t.Errorf("%d samples, want 4", len(b.Samples))
	}
}

// Off-season the samples table still lists the RZI rows.
func TestSeaOffSeasonDetailKeepsRZISamples(t *testing.T) {
	d := rziDeps(t, rziSource(rziRow(seaCoastal, day(2026, 7, 14), 2026, 120, 30)), sofiaAt(2026, 12, 1, 12, 0))
	b, _ := detailOf(t, d, seaCoastal)
	if len(b.Samples) != 3 || b.Samples[0].Source != seaVarnaSource {
		t.Errorf("samples = %+v", b.Samples)
	}
	if got, _ := latestOf(t, d); got != nil {
		t.Errorf("off-season latest = %+v, want none", got)
	}
}

// remarks is stored for review only. Neither endpoint may carry it, in any form.
func TestSeaRemarksNeverInTheAPI(t *testing.T) {
	d := rziDeps(t, rziSource(
		rziRow(seaCoastal, day(2026, 7, 14), 2026, 120, 30),
		rziRow(seaCoastal, day(2026, 6, 14), 2026, 120, 30),
	), sofiaAt(2026, 7, 20, 12, 0))
	for _, p := range []string{"/api/v1/sea/sites", "/api/v1/sea/sites/" + seaCoastal} {
		rec := serve(t, d, get(p, "203.0.113.7"))
		raw := rec.Body.Bytes()
		if !bytes.Contains(raw, []byte("rzi_varna")) {
			t.Fatalf("%s: the RZI data is absent, the check proves nothing: %s", p, raw)
		}
		for _, bad := range []string{"remarks", "Remarks", "SECRET-REMARK", "onerror", "<img"} {
			if bytes.Contains(raw, []byte(bad)) {
				t.Errorf("%s: body contains %q: %s", p, bad, raw)
			}
		}
	}
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// With no RZI data, which is production today, the bytes are the ones served before the RZI work.
func TestSeaNoRZIOutputIsUnchanged(t *testing.T) {
	now := sofiaAt(2026, 7, 20, 12, 0)
	for _, c := range []struct {
		name, path, golden string
		src                *stubSource
		supplement         bool
	}{
		{"list", "/api/v1/sea/sites", "sea_sites_no_rzi.golden.json", seaSource(), false},
		{"detail", "/api/v1/sea/sites/" + seaCoastal, "sea_site_no_rzi.golden.json", seaSource(), false},
		{"list with supplement", "/api/v1/sea/sites", "sea_sites_supplement_no_rzi.golden.json", mixedSeaSource(), true},
		{"detail with supplement", "/api/v1/sea/sites/" + seaCoastal, "sea_site_supplement_no_rzi.golden.json", mixedSeaSource(), true},
	} {
		d := rziDeps(t, c.src, now)
		d.SeaSupplement = seaSupplementMeta
		rec := serve(t, d, get(c.path, "203.0.113.7"))
		if got, want := rec.Body.Bytes(), readGolden(t, c.golden); !bytes.Equal(got, want) {
			t.Errorf("%s changed:\n got %s\nwant %s", c.name, got, want)
		}
	}
}

// RZI rows that are all hidden or off-season leave the bytes as they were.
func TestSeaHiddenOrOffSeasonRZIOutputIsUnchanged(t *testing.T) {
	hidden := rziSource(rziRow(seaCoastal, day(2024, 8, 20), 2024, 1, 1))
	d := rziDeps(t, hidden, sofiaAt(2026, 7, 20, 12, 0))
	rec := serve(t, d, get("/api/v1/sea/sites/"+seaCoastal, "203.0.113.7"))
	if !bytes.Equal(rec.Body.Bytes(), readGolden(t, "sea_site_no_rzi.golden.json")) {
		t.Errorf("detail with only hidden RZI rows changed: %s", rec.Body.String())
	}
	rec = serve(t, d, get("/api/v1/sea/sites", "203.0.113.7"))
	if !bytes.Equal(rec.Body.Bytes(), readGolden(t, "sea_sites_no_rzi.golden.json")) {
		t.Errorf("list with only hidden RZI rows changed: %s", rec.Body.String())
	}
	off := rziDeps(t, rziSource(rziRow(seaCoastal, day(2026, 7, 14), 2026, 1, 1)), sofiaAt(2026, 12, 1, 12, 0))
	rec = serve(t, off, get("/api/v1/sea/sites", "203.0.113.7"))
	if !bytes.Equal(rec.Body.Bytes(), readGolden(t, "sea_sites_no_rzi.golden.json")) {
		t.Errorf("off-season list changed: %s", rec.Body.String())
	}
}

// The injected clock drives the cache: crossing a Sofia midnight inside the
// TTL still flips stale, and the store is read again.
func TestSeaClockDrivesStaleThroughTheCache(t *testing.T) {
	src := rziSource(rziRow(seaCoastal, day(2026, 7, 1), 2026, 1, 1))
	now := sofiaAt(2026, 7, 22, 23, 30)
	d := visitorsDeps(t, src)
	d.Clock = func() time.Time { return now }
	h := router(t, d)
	check := func(wantStale bool) {
		t.Helper()
		rec := serveOn(t, h, "/api/v1/sea/sites")
		var b struct {
			Sites []struct {
				ID     string         `json:"id"`
				Latest *seaLatestBody `json:"latest"`
			} `json:"sites"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
			t.Fatal(err)
		}
		if l := b.Sites[1].Latest; l == nil || l.Stale != wantStale {
			t.Errorf("latest = %+v, want stale %v", l, wantStale)
		}
	}
	check(false)
	now = sofiaAt(2026, 7, 23, 0, 10)
	check(true)
	if src.rziCalls != 2 {
		t.Errorf("%d RZI loads, want 2 (one per Sofia day)", src.rziCalls)
	}
	now = sofiaAt(2026, 7, 23, 0, 20)
	check(true)
	if src.rziCalls != 2 {
		t.Errorf("%d RZI loads after a same-day request, want 2", src.rziCalls)
	}
}

func TestSeaRZIStoreErrorIsA500(t *testing.T) {
	src := rziSource()
	src.rziErr = errors.New("db down")
	rec := serve(t, rziDeps(t, src, sofiaAt(2026, 7, 20, 12, 0)), get("/api/v1/sea/sites", "203.0.113.7"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
