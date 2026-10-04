package bathing_test

import (
	"context"
	"testing"
	"time"

	"airbg.org/internal/upstream/bathing"
)

func fixtureRaw(t *testing.T) bathing.Raw {
	t.Helper()
	srv, _ := discodata(t)
	raw, err := bathing.New(testConfig(srv.URL)).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	return raw
}

func TestBuildFromTheFixture(t *testing.T) {
	d, sk, err := bathing.Build(fixtureRaw(t), "BG")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// The retired Ovcharovski site is dropped.
	if len(d.Sites) != 3 {
		t.Fatalf("%d sites, want 3 active", len(d.Sites))
	}
	if sk.Retired != 1 {
		t.Errorf("Retired = %d, want 1", sk.Retired)
	}
	if len(d.Classes) != 12 || len(d.Samples) != 25 {
		t.Errorf("%d classes, %d samples; want 12, 25", len(d.Classes), len(d.Samples))
	}
	zones := map[string]string{}
	for _, s := range d.Sites {
		zones[s.ID] = s.Zone
	}
	if zones["BG3242661710017001"] != "lake" || zones["BG3310610135003001"] != "coastal" {
		t.Errorf("zones = %v", zones)
	}
	var pre, below int
	for _, s := range d.Samples {
		if s.PreSeason {
			pre++
		}
		if s.ECBelowDetection {
			below++
		}
	}
	if pre != 3 || below != 12 {
		t.Errorf("pre-season = %d, E. coli below detection = %d; want 3, 12", pre, below)
	}
}

func site(id, status, zone string) bathing.SiteRow {
	return bathing.SiteRow{
		ID: id, NameBG: "ПЛАЖ  ", NameEN: "PLAZH ", Zone: zone, Lat: 42.5, Lon: 27.6, Status: status,
		Link: "http://rzi.invalid/profile.pdf",
	}
}

func ptr[T int | string](v T) *T { return &v }

func TestBuildNormalisesASite(t *testing.T) {
	raw := bathing.Raw{Sites: []bathing.SiteRow{site("BG0001", "stable", "coastalBathingWater")}}
	d, _, err := bathing.Build(raw, "BG")
	if err != nil {
		t.Fatal(err)
	}
	s := d.Sites[0]
	if s.NameBG != "ПЛАЖ" || s.NameEN != "PLAZH" {
		t.Errorf("names = %q, %q; want trimmed", s.NameBG, s.NameEN)
	}
	if s.ProfileURL != "http://rzi.invalid/profile.pdf" {
		t.Errorf("ProfileURL = %q", s.ProfileURL)
	}
}

func TestBuildRejectsBadSites(t *testing.T) {
	bad := map[string]bathing.SiteRow{
		"foreign country": site("RO0001", "stable", "coastalBathingWater"),
		"id with a quote": site("BG00'1", "stable", "coastalBathingWater"),
		"river":           site("BG0001", "stable", "riverBathingWater"),
		"null island": func() bathing.SiteRow {
			s := site("BG0001", "stable", "coastalBathingWater")
			s.Lat, s.Lon = 0, 0
			return s
		}(),
		"latitude too big": func() bathing.SiteRow { s := site("BG0001", "stable", "coastalBathingWater"); s.Lat = 142; return s }(),
		"no name": func() bathing.SiteRow {
			s := site("BG0001", "stable", "coastalBathingWater")
			s.NameBG, s.NameEN = " ", ""
			return s
		}(),
	}
	for name, row := range bad {
		t.Run(name, func(t *testing.T) {
			raw := bathing.Raw{Sites: []bathing.SiteRow{site("BG0009", "stable", "lakeBathingWater"), row}}
			d, sk, err := bathing.Build(raw, "BG")
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Sites) != 1 || sk.Invalid != 1 {
				t.Errorf("kept %d sites, Invalid = %d; want 1, 1", len(d.Sites), sk.Invalid)
			}
		})
	}
}

// Only http(s) links become outbound hrefs; anything else is dropped, not the site.
func TestBuildDropsANonHTTPProfileLink(t *testing.T) {
	s := site("BG0001", "stable", "coastalBathingWater")
	s.Link = "javascript:alert(1)"
	d, _, err := bathing.Build(bathing.Raw{Sites: []bathing.SiteRow{s}}, "BG")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Sites) != 1 || d.Sites[0].ProfileURL != "" {
		t.Errorf("sites = %+v, want the site kept with no profile link", d.Sites)
	}
}

func TestBuildParsesEveryQualityLabel(t *testing.T) {
	want := map[string]string{
		"0 - Not classified":     "not_classified",
		"1 - Excellent":          "excellent",
		"2 - Good":               "good",
		"3 - Sufficient":         "sufficient",
		"3 - Good or Sufficient": "good_or_sufficient",
		"4 - Poor":               "poor",
	}
	for label, key := range want {
		raw := bathing.Raw{
			Sites:  []bathing.SiteRow{site("BG0001", "stable", "coastalBathingWater")},
			Status: []bathing.StatusRow{{SiteID: "BG0001", Season: 2024, Quality: ptr(label)}},
		}
		d, _, err := bathing.Build(raw, "BG")
		if err != nil {
			t.Fatal(err)
		}
		if len(d.Classes) != 1 || d.Classes[0].Quality != key {
			t.Errorf("%q -> %+v, want %q", label, d.Classes, key)
		}
	}
}

func TestBuildSkipsBadClassesAndSamples(t *testing.T) {
	raw := bathing.Raw{
		Sites: []bathing.SiteRow{site("BG0001", "stable", "coastalBathingWater"), site("BG0002", "retired", "coastalBathingWater")},
		Status: []bathing.StatusRow{
			{SiteID: "BG0001", Season: 2024, Quality: ptr("1 - Excellent")},
			{SiteID: "BG0001", Season: 2024, Quality: ptr("2 - Good")},    // duplicate season
			{SiteID: "BG0001", Season: 2023, Quality: ptr("5 - Unknown")}, // unknown label
			{SiteID: "BG0001", Season: 2022, Quality: nil},
			{SiteID: "BG0002", Season: 2024, Quality: ptr("1 - Excellent")}, // retired site
		},
		Samples: []bathing.SampleRow{
			{SiteID: "BG0001", Season: 2024, Date: "2024-07-01", EC: ptr(15), IE: ptr(15)},
			{SiteID: "BG0001", Season: 2024, Date: "2024-07-01", EC: ptr(30), IE: ptr(30)}, // duplicate date
			{SiteID: "BG0001", Season: 2024, Date: "01.07.2024", EC: ptr(15), IE: ptr(15)}, // bad date
			{SiteID: "BG0001", Season: 2024, Date: "2024-07-15", EC: nil, IE: ptr(15)},     // missing value
			{SiteID: "BG0001", Season: 2024, Date: "2024-07-29", EC: ptr(-1), IE: ptr(15)}, // negative
			{SiteID: "BG0001", Season: 2024, Date: "2024-08-12", EC: ptr(15), IE: ptr(15), ECStatus: ptr("somethingElse")},
			{SiteID: "BG0002", Season: 2024, Date: "2024-07-01", EC: ptr(15), IE: ptr(15)}, // retired site
		},
	}
	d, sk, err := bathing.Build(raw, "BG")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Classes) != 1 || d.Classes[0].Quality != "excellent" {
		t.Errorf("classes = %+v, want only the first 2024 row", d.Classes)
	}
	if len(d.Samples) != 1 || !d.Samples[0].Date.Equal(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)) || d.Samples[0].EC != 15 {
		t.Errorf("samples = %+v, want only the first 2024-07-01 row", d.Samples)
	}
	if sk.Invalid != 8 || sk.Orphan != 2 {
		t.Errorf("skipped = %+v, want Invalid 8, Orphan 2", sk)
	}
}

func TestBuildMarksCensoredValues(t *testing.T) {
	raw := bathing.Raw{
		Sites: []bathing.SiteRow{site("BG0001", "stable", "coastalBathingWater")},
		Samples: []bathing.SampleRow{{
			SiteID: "BG0001", Season: 2024, Date: "2024-06-10", EC: ptr(15), IE: ptr(40),
			ECStatus: ptr("limitOfDetectionValue"), IEStatus: ptr("confirmedValue"), SampleStatus: ptr("preSeasonSample"),
		}},
	}
	d, _, err := bathing.Build(raw, "BG")
	if err != nil {
		t.Fatal(err)
	}
	s := d.Samples[0]
	if !s.ECBelowDetection || s.IEBelowDetection || !s.PreSeason {
		t.Errorf("sample = %+v, want E. coli censored, enterococci confirmed, pre-season", s)
	}
}

// An empty answer must not wipe the stored layer.
func TestBuildRefusesAnImportWithNoSites(t *testing.T) {
	if _, _, err := bathing.Build(bathing.Raw{}, "BG"); err == nil {
		t.Error("Build accepted an import with no active sites")
	}
}
