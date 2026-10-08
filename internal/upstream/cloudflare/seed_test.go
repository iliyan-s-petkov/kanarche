package cloudflare_test

import (
	"strings"
	"testing"

	"kanarche.eu/internal/upstream/cloudflare"
)

func TestParseSeedFileParsesEveryRow(t *testing.T) {
	r := strings.NewReader(`[
		{"date": "2026-09-26", "uniques": 104, "requests": 500, "pageViews": 109},
		{"date": "2026-09-27", "uniques": 181, "requests": 900, "pageViews": 878}
	]`)
	points, err := cloudflare.ParseSeedFile(r)
	if err != nil {
		t.Fatalf("ParseSeedFile: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("got %d points, want 2", len(points))
	}
	if points[0].Uniques != 104 || points[0].Requests != 500 || points[0].PageViews != 109 {
		t.Errorf("points[0] = %+v", points[0])
	}
	if points[0].Date.Format("2006-01-02") != "2026-09-26" {
		t.Errorf("points[0].Date = %v, want 2026-09-26", points[0].Date)
	}
}

func TestParseSeedFileRejectsMalformedJSON(t *testing.T) {
	_, err := cloudflare.ParseSeedFile(strings.NewReader(`not json`))
	if err == nil {
		t.Fatal("err = nil, want an error for malformed JSON")
	}
}

func TestParseSeedFileRejectsBadDate(t *testing.T) {
	_, err := cloudflare.ParseSeedFile(strings.NewReader(`[{"date": "not-a-date", "uniques": 1, "requests": 1, "pageViews": 1}]`))
	if err == nil {
		t.Fatal("err = nil, want an error for a bad date")
	}
}

func TestParseSeedFileEmptyArrayIsFine(t *testing.T) {
	points, err := cloudflare.ParseSeedFile(strings.NewReader(`[]`))
	if err != nil {
		t.Fatalf("ParseSeedFile: %v", err)
	}
	if len(points) != 0 {
		t.Errorf("got %d points, want 0", len(points))
	}
}
