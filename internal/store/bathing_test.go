package store_test

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"airbg.org/internal/store"
)

func bathingFixture() store.BathingData {
	return store.BathingData{
		Sites: []store.BathingSite{
			{ID: "BG3310610135003001", NameBG: "ЗЛАТНИ ПЯСЪЦИ-ПСОВ", NameEN: "ZLATNI PYASATSI-PSOV", Zone: "coastal", Lat: 43.3033, Lon: 28.0533, ProfileURL: "http://www.rzi-varna.com/prof/profil_1.pdf"},
			{ID: "BG3242661710017001", NameBG: "ЯЗОВИР ПЧЕЛИНА 2", NameEN: "YAZOVIR PCHELINA 2", Zone: "lake", Lat: 43.4908, Lon: 26.4707},
		},
		Classes: []store.BathingClass{
			{SiteID: "BG3310610135003001", Season: 2023, Quality: "good"},
			{SiteID: "BG3310610135003001", Season: 2024, Quality: "excellent"},
			{SiteID: "BG3242661710017001", Season: 2024, Quality: "sufficient"},
		},
		Samples: []store.BathingSample{
			{SiteID: "BG3310610135003001", Date: time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC), Season: 2024, EC: 15, ECBelowDetection: true, IE: 40, PreSeason: false},
			{SiteID: "BG3310610135003001", Date: time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC), Season: 2024, EC: 30, IE: 15, IEBelowDetection: true, PreSeason: true},
		},
	}
}

func TestBathingLastImportOnAFreshTable(t *testing.T) {
	ctx, _, s := newStore(t)
	_, ok, err := s.BathingLastImport(ctx)
	if err != nil {
		t.Fatalf("BathingLastImport: %v", err)
	}
	if ok {
		t.Error("ok = true on a fresh table, want false")
	}
}

func TestReplaceBathingRoundTrips(t *testing.T) {
	ctx, _, s := newStore(t)
	at := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	if err := s.ReplaceBathing(ctx, bathingFixture(), at); err != nil {
		t.Fatalf("ReplaceBathing: %v", err)
	}

	got, err := s.LoadBathing(ctx)
	if err != nil {
		t.Fatalf("LoadBathing: %v", err)
	}
	if len(got.Sites) != 2 || len(got.Classes) != 3 || len(got.Samples) != 2 {
		t.Fatalf("loaded %d sites, %d classes, %d samples; want 2, 3, 2", len(got.Sites), len(got.Classes), len(got.Samples))
	}
	// Sites by id; classes by site then season; samples by site then date.
	if got.Sites[0].ID != "BG3242661710017001" || got.Sites[0].Zone != "lake" {
		t.Errorf("first site = %+v, want the lake, ordered by id", got.Sites[0])
	}
	if got.Classes[1].Season != 2023 || got.Classes[2].Quality != "excellent" {
		t.Errorf("classes = %+v, want ordered by site then season", got.Classes)
	}
	first := got.Samples[0]
	if !first.Date.Equal(time.Date(2024, 5, 20, 0, 0, 0, 0, time.UTC)) || !first.PreSeason || !first.IEBelowDetection || first.ECBelowDetection {
		t.Errorf("first sample = %+v, want the pre-season one with flags kept", first)
	}

	last, ok, err := s.BathingLastImport(ctx)
	if err != nil || !ok || !last.Equal(at) {
		t.Errorf("BathingLastImport = %v, %v, %v; want %v, true, nil", last, ok, err, at)
	}
}

// A second import replaces, so a site the EEA drops disappears from the map.
func TestReplaceBathingDropsRowsTheNewImportLacks(t *testing.T) {
	ctx, _, s := newStore(t)
	at := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	if err := s.ReplaceBathing(ctx, bathingFixture(), at); err != nil {
		t.Fatalf("ReplaceBathing: %v", err)
	}
	next := bathingFixture()
	next.Sites = next.Sites[:1]
	next.Classes = next.Classes[:2]
	if err := s.ReplaceBathing(ctx, next, at.Add(time.Hour)); err != nil {
		t.Fatalf("second ReplaceBathing: %v", err)
	}
	got, err := s.LoadBathing(ctx)
	if err != nil {
		t.Fatalf("LoadBathing: %v", err)
	}
	if len(got.Sites) != 1 || len(got.Classes) != 2 {
		t.Errorf("after replace: %d sites, %d classes; want 1, 2", len(got.Sites), len(got.Classes))
	}
	last, _, _ := s.BathingLastImport(ctx)
	if !last.Equal(at.Add(time.Hour)) {
		t.Errorf("BathingLastImport = %v, want the newer import", last)
	}
}

// A row the database rejects leaves the previous import in place.
func TestReplaceBathingIsAllOrNothing(t *testing.T) {
	ctx, _, s := newStore(t)
	at := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	if err := s.ReplaceBathing(ctx, bathingFixture(), at); err != nil {
		t.Fatalf("ReplaceBathing: %v", err)
	}
	bad := bathingFixture()
	bad.Sites = bad.Sites[:1]
	bad.Classes = append(bad.Classes, store.BathingClass{SiteID: "BG0000000000000000", Season: 2024, Quality: "excellent"})
	if err := s.ReplaceBathing(ctx, bad, at.Add(time.Hour)); err == nil {
		t.Fatal("ReplaceBathing accepted a class for an unknown site")
	}
	got, err := s.LoadBathing(ctx)
	if err != nil {
		t.Fatalf("LoadBathing: %v", err)
	}
	if len(got.Sites) != 2 {
		t.Errorf("after a failed replace: %d sites, want the previous 2", len(got.Sites))
	}
	last, _, _ := s.BathingLastImport(ctx)
	if !last.Equal(at) {
		t.Errorf("BathingLastImport = %v, want the earlier successful import", last)
	}
}

// Source is stored per class; an unset source is Discodata.
func TestBathingClassSourceRoundTrips(t *testing.T) {
	ctx, _, s := newStore(t)
	d := bathingFixture()
	d.Classes = append(d.Classes, store.BathingClass{SiteID: "BG3310610135003001", Season: 2025, Quality: "good", Source: store.SourceDatahub})
	d.SupplementEdition = "2025 v1.0"
	if err := s.ReplaceBathing(ctx, d, time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("ReplaceBathing: %v", err)
	}
	got, err := s.LoadBathing(ctx)
	if err != nil {
		t.Fatalf("LoadBathing: %v", err)
	}
	var sources []string
	for _, c := range got.Classes {
		sources = append(sources, c.SiteID[len(c.SiteID)-4:]+"/"+strconv.Itoa(c.Season)+"="+c.Source)
	}
	want := []string{"7001/2024=discodata", "3001/2023=discodata", "3001/2024=discodata", "3001/2025=datahub"}
	if !slices.Equal(sources, want) {
		t.Errorf("sources = %v, want %v", sources, want)
	}
}

func TestBathingRejectsUnknownSource(t *testing.T) {
	ctx, _, s := newStore(t)
	d := bathingFixture()
	d.Classes[0].Source = "wikipedia"
	if err := s.ReplaceBathing(ctx, d, time.Now()); err == nil {
		t.Fatal("unknown source accepted")
	}
}

func TestBathingSupplementEdition(t *testing.T) {
	ctx, _, s := newStore(t)
	if ed, err := s.BathingSupplementEdition(ctx); err != nil || ed != "" {
		t.Fatalf("fresh table: %q, %v; want empty", ed, err)
	}
	at := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	d := bathingFixture()
	d.SupplementEdition = "2025 v1.0"
	if err := s.ReplaceBathing(ctx, d, at); err != nil {
		t.Fatal(err)
	}
	if ed, err := s.BathingSupplementEdition(ctx); err != nil || ed != "2025 v1.0" {
		t.Fatalf("after import: %q, %v", ed, err)
	}
	// The newest import decides, and an import without a snapshot clears it.
	if err := s.ReplaceBathing(ctx, bathingFixture(), at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if ed, err := s.BathingSupplementEdition(ctx); err != nil || ed != "" {
		t.Fatalf("after import without snapshot: %q, %v; want empty", ed, err)
	}
}
