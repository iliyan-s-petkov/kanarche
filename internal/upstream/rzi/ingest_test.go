package rzi_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kanarche.eu/internal/metrics"
	"kanarche.eu/internal/store"
	"kanarche.eu/internal/upstream/rzi"
)

func TestIngestOKReplacesOnlyThatSource(t *testing.T) {
	r := newRig(t)
	r.seedOK(shaOf("1"), siteA, 1)
	burgas := store.RZIFile{Source: rzi.SourceBurgas, URL: "u", SHA256: shaOf("2"), FetchedAt: r.now, ExtractorVersion: "v", Status: store.RZIStatusOK, RowsOK: 1}
	bx := store.RZISample{SiteID: "BG3410210135002001", Date: r.now.AddDate(0, 0, -30), Season: 2026, EC: 5, IE: 5, Unit: "MPN/100ml"}
	if err := r.st.ReplaceRZISource(r.ctx, rzi.SourceBurgas, burgas, []store.RZISample{bx}); err != nil {
		t.Fatal(err)
	}
	drop(t, r.outbox, varna(t, shaOf("a")))

	s := r.process()

	if s.OK != 1 || s.NeedsReview != 0 {
		t.Fatalf("stats = %+v", s)
	}
	var varnaRows, burgasRows int
	for _, x := range r.rows() {
		switch x.File.Source {
		case rzi.SourceVarna:
			varnaRows++
			if x.File.SHA256 != shaOf("a") {
				t.Errorf("varna row from file %s, want the new one", x.File.SHA256[:4])
			}
		case rzi.SourceBurgas:
			burgasRows++
		}
	}
	if varnaRows != 4 || burgasRows != 1 {
		t.Errorf("varna %d, burgas %d, want 4 and 1", varnaRows, burgasRows)
	}
	f := r.file(shaOf("a"))
	if f.Status != store.RZIStatusOK || f.RowsOK != 4 || f.RowsRejected != 0 || f.ApprovedAt != nil {
		t.Errorf("file row = %+v", f)
	}
	if exists(filepath.Join(r.outbox, shaOf("a")+".json")) || !exists(filepath.Join(r.outbox, "done", shaOf("a")+".json")) {
		t.Error("record not moved to done/")
	}
}

// I13: one bad row means the store is not written and the old set stays.
func TestIngestOneBadRowWritesNothing(t *testing.T) {
	r := newRig(t)
	r.seedOK(shaOf("1"), siteA, 1)
	before := r.rows()
	log := captureLog(t)
	drop(t, r.outbox, withBadUnit(varna(t, shaOf("b"))))

	s := r.process()

	if s.NeedsReview != 1 || s.OK != 0 {
		t.Fatalf("stats = %+v", s)
	}
	if after := r.rows(); !reflect.DeepEqual(before, after) {
		t.Errorf("store changed by a needs_review file:\nbefore %+v\nafter  %+v", before, after)
	}
	f := r.file(shaOf("b"))
	if f.Status != store.RZIStatusNeedsReview || f.RowsOK != 3 || f.RowsRejected != 1 || f.ApprovedAt != nil {
		t.Errorf("file row = %+v", f)
	}
	if !strings.Contains(f.Reason, "unit not allowed") {
		t.Errorf("reason = %q", f.Reason)
	}
	if !exists(filepath.Join(r.outbox, "review", shaOf("b")+".json")) {
		t.Error("record not kept in review/")
	}
	line := log.String()
	for _, want := range []string{"rzi_varna", shaOf("b"), "unit not allowed"} {
		if !strings.Contains(line, want) {
			t.Errorf("log lacks %q:\n%s", want, line)
		}
	}
	for _, leak := range []string{"синтетичен", "synthetic point", "след пороен дъжд", "пробата е изместена"} {
		if strings.Contains(line, leak) {
			t.Errorf("log carries document text %q:\n%s", leak, line)
		}
	}
}

func TestIngestChangedValueNeedsReview(t *testing.T) {
	r := newRig(t)
	r.process()
	drop(t, r.outbox, varna(t, shaOf("a")))
	r.process()
	before := r.rows()

	g := varna(t, shaOf("c"))
	g.Blocks[1].Rows = []rzi.Row{g.Blocks[1].Rows[0]}
	g.Blocks[1].Rows[0].EC.Value++
	drop(t, r.outbox, g)

	if s := r.process(); s.NeedsReview != 1 {
		t.Fatalf("stats = %+v", s)
	}
	if after := r.rows(); !reflect.DeepEqual(before, after) {
		t.Error("rows changed by a file that rewrites a stored sample")
	}
}

func TestIngestRejectedRecord(t *testing.T) {
	r := newRig(t)
	r.seedOK(shaOf("1"), siteA, 1)
	before := r.rows()
	f := fixture(t, "rejected.json")
	drop(t, r.outbox, f)

	s := r.process()

	if s.Rejected != 1 {
		t.Fatalf("stats = %+v", s)
	}
	row := r.file(f.SHA256)
	if row.Status != store.RZIStatusRejected || row.Source != rzi.SourceBurgas {
		t.Errorf("file row = %+v", row)
	}
	if strings.Contains(row.Reason, f.Error) {
		t.Errorf("reason carries the extractor's text: %q", row.Reason)
	}
	if !reflect.DeepEqual(before, r.rows()) {
		t.Error("store changed by a rejected file")
	}
	if !exists(filepath.Join(r.outbox, "done", f.SHA256+".json")) || exists(filepath.Join(r.outbox, "review", f.SHA256+".json")) {
		t.Error("rejected record belongs in done/ only")
	}
}

func TestIngestProcessesEachFileOnce(t *testing.T) {
	r := newRig(t)
	drop(t, r.outbox, varna(t, shaOf("a")))
	drop(t, r.outbox, withBadUnit(varna(t, shaOf("b"))))

	if s := r.process(); s.OK != 1 || s.NeedsReview != 1 {
		t.Fatalf("first pass = %+v", s)
	}
	if s := r.process(); s != (rzi.Stats{}) {
		t.Fatalf("second pass saw files again: %+v", s)
	}

	// The sandbox may emit the same record again. Same hash and extractor
	// version is a duplicate: no second row, no status change.
	drop(t, r.outbox, varna(t, shaOf("a")))
	drop(t, r.outbox, withBadUnit(varna(t, shaOf("b"))))
	s := r.process()
	if s.Duplicate != 2 || s.OK != 0 || s.NeedsReview != 0 {
		t.Fatalf("re-emitted pass = %+v", s)
	}
	if r.file(shaOf("a")).Status != store.RZIStatusOK || r.file(shaOf("b")).Status != store.RZIStatusNeedsReview {
		t.Error("a duplicate changed a file status")
	}
	if !exists(filepath.Join(r.outbox, "review", shaOf("b")+".json")) {
		t.Error("review record lost on duplicate")
	}
}

func TestIngestRefusesUnusableRecords(t *testing.T) {
	r := newRig(t)
	log := captureLog(t)
	// Not JSON.
	bad1 := filepath.Join(r.outbox, shaOf("1")+".json")
	if err := os.WriteFile(bad1, []byte("not json, secret document text"), 0o640); err != nil {
		t.Fatal(err)
	}
	// Name and sha256 field disagree.
	other := varna(t, shaOf("3"))
	p := drop(t, r.outbox, other)
	renamed := filepath.Join(r.outbox, shaOf("2")+".json")
	if err := os.Rename(p, renamed); err != nil {
		t.Fatal(err)
	}
	// Unknown source.
	unk := varna(t, shaOf("4"))
	unk.Source = "rzi_sofia"
	drop(t, r.outbox, unk)

	s := r.process()

	if s.Refused != 3 {
		t.Fatalf("stats = %+v", s)
	}
	for _, c := range []string{"1", "2", "4"} {
		if !exists(filepath.Join(r.outbox, "bad", shaOf(c)+".json")) {
			t.Errorf("record %s not moved to bad/", c)
		}
	}
	if n := len(r.rows()); n != 0 {
		t.Errorf("rows = %d", n)
	}
	if _, ok, _ := r.st.RZIFileBySHA(r.ctx, shaOf("3")); ok {
		t.Error("a mismatched record reached the store")
	}
	if strings.Contains(log.String(), "secret document text") {
		t.Error("log carries record content")
	}
}

func TestIngestIgnoresEntriesItDoesNotOwn(t *testing.T) {
	r := newRig(t)
	if err := os.Mkdir(filepath.Join(r.outbox, shaOf("a")+".json"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.outbox, shaOf("b")+".json.tmp"), []byte("{}"), 0o640); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "x.json")
	if err := os.WriteFile(target, []byte("{}"), 0o640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(r.outbox, shaOf("c")+".json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if s := r.process(); s != (rzi.Stats{}) {
		t.Fatalf("stats = %+v", s)
	}
	if !exists(link) {
		t.Error("symlink was touched")
	}
}

func TestIngestMissingOutboxIsEmpty(t *testing.T) {
	r := newRig(t)
	if err := os.Remove(r.outbox); err != nil {
		t.Fatal(err)
	}
	if s := r.process(); s != (rzi.Stats{}) {
		t.Fatalf("stats = %+v", s)
	}
}

func scrape(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}

func TestMetricsFollowLatestFile(t *testing.T) {
	r := newRig(t)
	drop(t, r.outbox, varna(t, shaOf("a")))
	r.process()
	if err := r.ing.PublishMetrics(r.ctx); err != nil {
		t.Fatal(err)
	}
	body := scrape(t)
	for _, want := range []string{
		`airbg_rzi_file_status{source="rzi_varna",status="ok"} 1`,
		`airbg_rzi_file_status{source="rzi_varna",status="needs_review"} 0`,
		`airbg_rzi_file_status{source="rzi_varna",status="rejected"} 0`,
		`airbg_rzi_last_ok_timestamp{source="rzi_varna"} ` + "1.7",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape lacks %q", want)
		}
	}

	g := withBadUnit(varna(t, shaOf("b")))
	drop(t, r.outbox, g)
	r.process()
	if err := r.ing.PublishMetrics(r.ctx); err != nil {
		t.Fatal(err)
	}
	body = scrape(t)
	for _, want := range []string{
		`airbg_rzi_file_status{source="rzi_varna",status="ok"} 0`,
		`airbg_rzi_file_status{source="rzi_varna",status="needs_review"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("after a bad file, scrape lacks %q", want)
		}
	}
	if !strings.Contains(body, `airbg_rzi_last_ok_timestamp{source="rzi_varna"} 1.7`) {
		t.Error("last ok timestamp lost after a needs_review file")
	}
}
