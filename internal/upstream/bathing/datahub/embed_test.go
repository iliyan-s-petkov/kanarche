package datahub

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"airbg.org/internal/config"
)

var testNow = time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)

// committedPins reads the pins from the committed airbg.yaml.
func committedPins(t *testing.T) Pins {
	t.Helper()
	cfg, err := config.LoadFileOffline(filepath.Join("..", "..", "..", "..", "airbg.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return Pins{SHA256: cfg.Sea.Datahub.SHA256, Size: cfg.Sea.Datahub.Size, Country: cfg.Sea.Country}
}

func TestEmbeddedSnapshotValid(t *testing.T) {
	p := committedPins(t)
	f, err := Load(p, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if p.Country != "BG" || f.Header.Country != "BG" {
		t.Errorf("country = %q / %q, want BG", p.Country, f.Header.Country)
	}
	if len(f.Classes) != 1795 {
		t.Errorf("%d classes, want 1795", len(f.Classes))
	}
	h := f.Header
	if h.Edition == "" || h.Published == "" || h.Licence == "" || h.SourceURL == "" {
		t.Errorf("header incomplete: %+v", h)
	}
	sup := f.Supplement()
	if sup.Edition != h.Edition || len(sup.Classes) != len(f.Classes) {
		t.Errorf("supplement = %q with %d classes", sup.Edition, len(sup.Classes))
	}
}

// validDoc is a tiny snapshot that passes Decode with validPins.
func validDoc() File {
	return File{
		Header: Header{
			SourceURL: "https://sdi.eea.europa.eu/x.xlsx", SHA256: strings.Repeat("a", 64), Size: 100,
			Edition: "2025 v1.0", Published: "2026-06-02", Licence: "CC BY 4.0", Country: "BG",
		},
		Classes: []Class{
			{SiteID: "BG0000000000000001", Season: 2024, Quality: "excellent"},
			{SiteID: "BG0000000000000001", Season: 2025, Quality: "good"},
		},
	}
}

func validPins() Pins { return Pins{SHA256: strings.Repeat("a", 64), Size: 100, Country: "BG"} }

func encode(t *testing.T, f File) []byte {
	t.Helper()
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDecodeAcceptsValid(t *testing.T) {
	if _, err := Decode(encode(t, validDoc()), validPins(), testNow); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeRejects(t *testing.T) {
	cases := map[string]func(*File){
		"empty classes":     func(f *File) { f.Classes = nil },
		"bad site id":       func(f *File) { f.Classes[0].SiteID = "bg-1" },
		"other country id":  func(f *File) { f.Classes[0].SiteID = "DK0000000000000001" },
		"season too old":    func(f *File) { f.Classes[0].Season = 1999 },
		"season in future":  func(f *File) { f.Classes[0].Season = 2099 },
		"unknown quality":   func(f *File) { f.Classes[0].Quality = "5 - Superb" },
		"label not key":     func(f *File) { f.Classes[0].Quality = "1 - Excellent" },
		"duplicate key":     func(f *File) { f.Classes[1] = f.Classes[0] },
		"header sha":        func(f *File) { f.Header.SHA256 = strings.Repeat("b", 64) },
		"header size":       func(f *File) { f.Header.Size = 101 },
		"header country":    func(f *File) { f.Header.Country = "DK" },
		"missing edition":   func(f *File) { f.Header.Edition = "" },
		"missing published": func(f *File) { f.Header.Published = "" },
		"missing licence":   func(f *File) { f.Header.Licence = "" },
		"missing url":       func(f *File) { f.Header.SourceURL = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := validDoc()
			mutate(&f)
			if _, err := Decode(encode(t, f), validPins(), testNow); err == nil {
				t.Error("accepted")
			}
		})
	}
}

func TestDecodeRejectsWrongPins(t *testing.T) {
	raw := encode(t, validDoc())
	for name, p := range map[string]Pins{
		"sha":     {SHA256: strings.Repeat("c", 64), Size: 100, Country: "BG"},
		"size":    {SHA256: strings.Repeat("a", 64), Size: 7, Country: "BG"},
		"country": {SHA256: strings.Repeat("a", 64), Size: 100, Country: "DK"},
	} {
		if _, err := Decode(raw, p, testNow); err == nil {
			t.Errorf("%s pin mismatch accepted", name)
		}
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	good := string(encode(t, validDoc()))
	for name, raw := range map[string]string{
		"empty":          "",
		"not json":       "<html>",
		"truncated":      good[:len(good)/2],
		"trailing data":  good + `{"x":1}`,
		"unknown field":  strings.Replace(good, `"header"`, `"extra":1,"header"`, 1),
		"unknown nested": strings.Replace(good, `"site_id"`, `"x":1,"site_id"`, 1),
	} {
		if _, err := Decode([]byte(raw), validPins(), testNow); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
