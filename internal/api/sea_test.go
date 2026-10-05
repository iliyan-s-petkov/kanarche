package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"airbg.org/internal/store"
)

var seaImportedAt = time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC)

func seaData() store.BathingData {
	return store.BathingData{
		Sites: []store.BathingSite{
			{ID: "BG3242661710017001", NameBG: "ЯЗОВИР ПЧЕЛИНА 2", NameEN: "YAZOVIR PCHELINA 2", Zone: "lake", Lat: 43.4908, Lon: 26.4707},
			{ID: "BG3310610135003001", NameBG: "ЗЛАТНИ ПЯСЪЦИ-ПСОВ", NameEN: "ZLATNI PYASATSI-PSOV", Zone: "coastal", Lat: 43.3033, Lon: 28.0533, ProfileURL: "http://www.rzi-varna.com/prof/profil_1.pdf"},
		},
		Classes: []store.BathingClass{
			{SiteID: "BG3310610135003001", Season: 2023, Quality: "good"},
			{SiteID: "BG3310610135003001", Season: 2024, Quality: "excellent"},
		},
		Samples: []store.BathingSample{
			{SiteID: "BG3310610135003001", Date: time.Date(2023, 7, 3, 0, 0, 0, 0, time.UTC), Season: 2023, EC: 600, IE: 40},
			{SiteID: "BG3310610135003001", Date: time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC), Season: 2024, EC: 15, ECBelowDetection: true, IE: 40, PreSeason: true},
		},
	}
}

func seaSource() *stubSource {
	return &stubSource{bathing: seaData(), bathingAt: seaImportedAt, bathingHas: true}
}

type seaSitesBody struct {
	ImportedAt *time.Time `json:"imported_at"`
	Limits     map[string]struct {
		EColi       [2]int `json:"e_coli"`
		Enterococci [2]int `json:"enterococci"`
	} `json:"limits"`
	Sites []map[string]json.RawMessage `json:"sites"`
}

func TestSeaSitesShape(t *testing.T) {
	rec := serve(t, visitorsDeps(t, seaSource()), get("/api/v1/sea/sites", "203.0.113.7"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var b seaSitesBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if b.ImportedAt == nil || !b.ImportedAt.Equal(seaImportedAt) {
		t.Errorf("imported_at = %v, want %v", b.ImportedAt, seaImportedAt)
	}
	if b.Limits["coastal"].EColi != [2]int{250, 500} || b.Limits["lake"].Enterococci != [2]int{200, 400} {
		t.Errorf("limits = %+v", b.Limits)
	}
	if len(b.Sites) != 2 {
		t.Fatalf("%d sites, want 2", len(b.Sites))
	}
	coastal := b.Sites[1]
	for k, want := range map[string]string{
		"id": `"BG3310610135003001"`, "zone": `"coastal"`, "season": `2024`, "quality": `"excellent"`,
		"name_bg": `"ЗЛАТНИ ПЯСЪЦИ-ПСОВ"`, "name_en": `"ZLATNI PYASATSI-PSOV"`,
	} {
		if string(coastal[k]) != want {
			t.Errorf("site %s = %s, want %s", k, coastal[k], want)
		}
	}
	// A site with no class yet carries nulls, not a fake season.
	if string(b.Sites[0]["quality"]) != "null" || string(b.Sites[0]["season"]) != "null" {
		t.Errorf("unclassified site = %v, want null season and quality", b.Sites[0])
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q", cc)
	}
}

func TestSeaSiteDetail(t *testing.T) {
	rec := serve(t, visitorsDeps(t, seaSource()), get("/api/v1/sea/sites/BG3310610135003001", "203.0.113.7"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var b struct {
		Site struct {
			ID         string `json:"id"`
			ProfileURL string `json:"profile_url"`
		} `json:"site"`
		Limits struct {
			EColi [2]int `json:"e_coli"`
		} `json:"limits"`
		Classes []struct {
			Season  int    `json:"season"`
			Quality string `json:"quality"`
		} `json:"classes"`
		Samples []map[string]json.RawMessage `json:"samples"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if b.Site.ID != "BG3310610135003001" || b.Site.ProfileURL == "" || b.Limits.EColi != [2]int{250, 500} {
		t.Errorf("site = %+v, limits = %+v", b.Site, b.Limits)
	}
	// Newest first, so the panel leads with the current class and latest sample.
	if len(b.Classes) != 2 || b.Classes[0].Season != 2024 {
		t.Errorf("classes = %+v, want newest first", b.Classes)
	}
	if len(b.Samples) != 2 {
		t.Fatalf("%d samples, want 2", len(b.Samples))
	}
	s := b.Samples[0]
	for k, want := range map[string]string{
		"date": `"2024-07-02"`, "season": "2024", "e_coli": "15", "e_coli_below_detection": "true",
		"enterococci": "40", "enterococci_below_detection": "false", "pre_season": "true",
	} {
		if string(s[k]) != want {
			t.Errorf("sample %s = %s, want %s", k, s[k], want)
		}
	}
}

func TestSeaSiteUnknownAndMalformedIDs(t *testing.T) {
	d := visitorsDeps(t, seaSource())
	for path, want := range map[string]int{
		"/api/v1/sea/sites/BG0000000000000000":         http.StatusNotFound,
		"/api/v1/sea/sites/bg'--":                      http.StatusBadRequest,
		"/api/v1/sea/sites/" + strings.Repeat("A", 80): http.StatusBadRequest,
	} {
		if rec := serve(t, d, get(path, "203.0.113.7")); rec.Code != want {
			t.Errorf("%s: status = %d, want %d", path, rec.Code, want)
		}
	}
}

// One store load per TTL however many requests arrive.
func TestSeaIsCached(t *testing.T) {
	src := seaSource()
	h := router(t, visitorsDeps(t, src))
	for _, p := range []string{"/api/v1/sea/sites", "/api/v1/sea/sites/BG3310610135003001", "/api/v1/sea/sites"} {
		if rec := serveOn(t, h, p); rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", p, rec.Code)
		}
	}
	if src.bathingCalls != 1 {
		t.Errorf("%d store loads, want 1", src.bathingCalls)
	}
}

func TestSeaStoreErrorIsA500AndNotCached(t *testing.T) {
	src := seaSource()
	src.bathingErr = errors.New("db down")
	h := router(t, visitorsDeps(t, src))
	if rec := serveOn(t, h, "/api/v1/sea/sites"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	src.bathingErr = nil
	if rec := serveOn(t, h, "/api/v1/sea/sites"); rec.Code != http.StatusOK {
		t.Errorf("after recovery: status = %d, want 200", rec.Code)
	}
}

// Before the first import the layer is empty, not an error.
func TestSeaBeforeAnyImport(t *testing.T) {
	rec := serve(t, visitorsDeps(t, &stubSource{}), get("/api/v1/sea/sites", "203.0.113.7"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var b seaSitesBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if b.ImportedAt != nil || b.Sites == nil || len(b.Sites) != 0 {
		t.Errorf("body = %s, want null imported_at and an empty sites array", rec.Body.String())
	}
}
