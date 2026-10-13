package dobrich

import (
	"encoding/json"
	"os"
	"testing"

	"kanarche.eu/internal/store"
	"kanarche.eu/internal/upstream/rzi"
)

// TestRealFile runs the extractor over a downloaded RZI file. It is opt-in:
// the file is not committed (testdata/README.md, D3). RZI_DOBRICH_FILE is the
// workbook, RZI_DOBRICH_SITES an optional JSON list of EEA sites
// [{"ID":"BG33...","Lat":0,"Lon":0}]. Only counts are logged.
func TestRealFile(t *testing.T) {
	path := os.Getenv("RZI_DOBRICH_FILE")
	if path == "" {
		t.Skip("RZI_DOBRICH_FILE not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f := Extract(data)
	if f.Error != "" {
		t.Fatalf("extractor refused: %s", f.Error)
	}
	rows := 0
	for _, b := range f.Blocks {
		rows += len(b.Rows)
	}
	t.Logf("blocks %d, rows %d, warnings %d", len(f.Blocks), rows, len(f.Warnings))
	for _, w := range f.Warnings {
		t.Logf("warning: %s", w)
	}
	raw := os.Getenv("RZI_DOBRICH_SITES")
	if raw == "" {
		return
	}
	body, err := os.ReadFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	var sites []store.BathingSite
	if err := json.Unmarshal(body, &sites); err != nil {
		t.Fatal(err)
	}
	v := rzi.ValidateDetailed(f, sites, rzi.LastGood{}, now)
	t.Logf("status %s, accepted %d, rejects %d", v.Status, len(v.Accepted), len(v.Rejects))
	for _, r := range v.Rejects {
		t.Logf("reject: block %d row %d point %s date %s: %s", r.Block, r.Row, r.Point, r.Date, r.Reason)
	}
}
