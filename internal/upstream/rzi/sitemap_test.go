package rzi

import (
	"errors"
	"math"
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

func testNow() time.Time { return time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC) }

// testSites is a synthetic EEA site list. Coordinates are made up.
func testSites() []store.BathingSite {
	return []store.BathingSite{
		{ID: "BG3310610135003001", Zone: "coastal", Lat: 43.3033, Lon: 28.0533},
		{ID: "BG3310610135003002", Zone: "coastal", Lat: 43.2900, Lon: 28.0450},
		{ID: "BG3310610135003003", Zone: "coastal", Lat: 43.2700, Lon: 28.0300},
		{ID: "BG3310610135003004", Zone: "coastal", Lat: 43.2500, Lon: 28.0200},
		{ID: "BG3310610135003005", Zone: "coastal", Lat: 43.2400, Lon: 28.0100},
		{ID: "BG3410210135002001", Zone: "coastal", Lat: 42.6500, Lon: 27.7200},
		{ID: "BG3310610135008001", Zone: "coastal", Lat: 43.5500, Lon: 28.6000},
		// 02009 under the Varna prefix: a Burgas point must not map to it.
		{ID: "BG3310610135002009", Zone: "coastal", Lat: 42.7000, Lon: 27.7000},
		// 03006 has no usable EEA coordinate.
		{ID: "BG3310610135003006", Zone: "coastal", Lat: math.NaN(), Lon: 28.0},
		// 03007 exists twice, so it is ambiguous.
		{ID: "BG3310610135003007", Zone: "coastal", Lat: 43.2000, Lon: 28.0000},
		{ID: "BG3399910135003007", Zone: "coastal", Lat: 43.2000, Lon: 28.0000},
	}
}

// offsetNorth returns lat moved m metres north.
func offsetNorth(lat, m float64) float64 { return lat + m/111195.0 }

func TestDistanceMetres(t *testing.T) {
	// One degree of latitude on a 6371 km sphere.
	got := distanceMetres(43, 28, 44, 28)
	if math.Abs(got-111195) > 1 {
		t.Fatalf("1 degree = %.1f m", got)
	}
	if d := distanceMetres(43.3, 28.05, offsetNorth(43.3, 300), 28.05); math.Abs(d-300) > 1 {
		t.Fatalf("300 m offset = %.1f m", d)
	}
}

func TestSiteIndexMap(t *testing.T) {
	x := NewSiteIndex(testSites())
	cases := []struct {
		name   string
		source string
		point  string
		lat    float64
		lon    float64
		wantID string
		want   error
	}{
		{"exact", SourceVarna, "03001", 43.3033, 28.0533, "BG3310610135003001", nil},
		{"190 m off", SourceVarna, "03001", offsetNorth(43.3033, 190), 28.0533, "BG3310610135003001", nil},
		{"burgas", SourceBurgas, "02001", 42.65, 27.72, "BG3410210135002001", nil},
		{"dobrich", SourceDobrich, "08001", 43.55, 28.6, "BG3310610135008001", nil},
		{"300 m off", SourceVarna, "03001", offsetNorth(43.3033, 300), 28.0533, "", ErrSiteTooFar},
		{"210 m off", SourceVarna, "03001", offsetNorth(43.3033, 210), 28.0533, "", ErrSiteTooFar},
		{"four digits", SourceVarna, "3001", 43.3033, 28.0533, "", ErrPoint},
		{"six digits", SourceVarna, "030011", 43.3033, 28.0533, "", ErrPoint},
		{"letters", SourceVarna, "03a01", 43.3033, 28.0533, "", ErrPoint},
		{"arabic-indic digits", SourceVarna, "٠٣٠٠١", 43.3033, 28.0533, "", ErrPoint},
		{"padded", SourceVarna, " 03001", 43.3033, 28.0533, "", ErrPoint},
		{"unknown point", SourceVarna, "03099", 43.3033, 28.0533, "", ErrUnknownSite},
		{"EEA site without coordinate", SourceVarna, "03006", 43.2, 28.0, "", ErrSiteTooFar},
		{"two ids", SourceVarna, "03007", 43.2, 28.0, "", ErrAmbiguousSite},
		{"point of another source", SourceVarna, "02001", 42.65, 27.72, "", ErrPoint},
		{"wrong region prefix", SourceBurgas, "02009", 42.7, 27.7, "", ErrRegion},
		{"unknown source", "rzi_sofia", "03001", 43.3033, 28.0533, "", ErrSource},
		{"NaN coordinate", SourceVarna, "03001", math.NaN(), 28.0533, "", ErrCoord},
		{"infinite coordinate", SourceVarna, "03001", 43.3033, math.Inf(1), "", ErrCoord},
		{"latitude out of range", SourceVarna, "03001", 91, 28.0533, "", ErrCoord},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := x.Map(tc.source, tc.point, "", tc.lat, tc.lon)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if id != tc.wantID {
				t.Fatalf("id = %q, want %q", id, tc.wantID)
			}
		})
	}
}

func TestSiteIndexMapNameTolerance(t *testing.T) {
	x := NewSiteIndex([]store.BathingSite{
		{ID: "BG3412181178002024", NameBG: "КЪМПИНГ ГРАДИНА", Zone: "coastal", Lat: 42.40, Lon: 27.70},
		{ID: "BG3411348619002040", NameBG: "ПОПСКИ ПЛАЖ", Zone: "coastal", Lat: 42.30, Lon: 27.60},
		{ID: "BG3411000000002049", NameBG: "ПЛАЖ РОСЕНЕЦ-СЕВЕР", Zone: "coastal", Lat: 42.20, Lon: 27.50},
	})
	lons := map[string]float64{"02024": 27.70, "02040": 27.60, "02049": 27.50}
	lats := map[string]float64{"02024": 42.40, "02040": 42.30, "02049": 42.20}
	cases := []struct {
		name  string
		point string
		doc   string
		north float64 // metres north of the EEA coordinate
		want  error
	}{
		{"718 m with matching name", "02024", "Градина", 718, nil},
		{"718 m without name", "02024", "", 718, ErrSiteTooFar},
		{"718 m with other name", "02024", "Росенец", 718, ErrSiteTooFar},
		{"424 m with matching name", "02040", "Попски плаж", 424, nil},
		{"424 m without name", "02040", "", 424, ErrSiteTooFar},
		{"43 km with matching name", "02049", "Росенец-север", 43000, ErrSiteTooFar},
		{"1001 m with matching name", "02024", "Градина", 1001, ErrSiteTooFar},
		{"999 m with matching name", "02024", "Градина", 999, nil},
		{"150 m with empty name", "02024", "", 150, nil},
		{"250 m with empty name", "02024", "", 250, ErrSiteTooFar},
		{"mixed case", "02040", "попски ПЛАЖ", 424, nil},
		{"punctuation and hyphen", "02049", " Росенец—север. ", 500, nil},
		{"format characters are dropped", "02024", "Гра\u200bдина\x00", 718, nil},
		{"extra document token", "02024", "Градина 2", 718, ErrSiteTooFar},
		{"name of only punctuation", "02024", "- .", 718, ErrSiteTooFar},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := x.Map(SourceBurgas, tc.point, tc.doc, offsetNorth(lats[tc.point], tc.north), lons[tc.point])
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
