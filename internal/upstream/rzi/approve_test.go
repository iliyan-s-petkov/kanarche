package rzi_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"kanarche.eu/internal/store"
	"kanarche.eu/internal/upstream/rzi"
)

func (r *rig) review(sha string) string { return filepath.Join(r.outbox, "review", sha+".json") }

// reviewFile ingests a file with one bad row, so it waits for approval.
func (r *rig) reviewFile(sha string) {
	r.t.Helper()
	drop(r.t, r.outbox, withBadUnit(varna(r.t, sha)))
	if s := r.process(); s.NeedsReview != 1 {
		r.t.Fatalf("stats = %+v", s)
	}
}

func TestApprovePublishesPassedRowsOnly(t *testing.T) {
	r := newRig(t)
	r.seedOK(shaOf("1"), siteA, 1)
	r.reviewFile(shaOf("b"))

	a, err := r.ing.Approve(r.ctx, shaOf("b"))
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if a.Source != rzi.SourceVarna || a.SHA256 != shaOf("b") || a.Published != 3 || a.Excluded != 1 {
		t.Errorf("approval = %+v", a)
	}
	rows := r.rows()
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want the 3 that passed", len(rows))
	}
	for _, x := range rows {
		// The flagged row is siteA on 2026-06-02. The older seeded row is gone.
		if x.SiteID == siteA && x.Date.Day() == 2 && x.Date.Month() == time.June {
			t.Errorf("flagged row published: %+v", x)
		}
		if x.File.SHA256 != shaOf("b") {
			t.Errorf("row from file %s", x.File.SHA256[:4])
		}
	}
	f := r.file(shaOf("b"))
	if f.Status != store.RZIStatusOK || f.ApprovedAt == nil || f.ExcludedRows != 1 || f.RowsOK != 3 {
		t.Errorf("file row = %+v", f)
	}
	if !f.ApprovedAt.Equal(r.now) {
		t.Errorf("approved_at = %v, want %v", f.ApprovedAt, r.now)
	}
	if exists(r.review(shaOf("b"))) || !exists(filepath.Join(r.outbox, "done", shaOf("b")+".json")) {
		t.Error("review record not moved to done/")
	}
	last, ok, _ := r.st.RZILastGood(r.ctx, rzi.SourceVarna)
	if !ok || last.SHA256 != shaOf("b") {
		t.Errorf("last good = %+v", last)
	}
}

func TestApproveBindsToOneSHA(t *testing.T) {
	r := newRig(t)
	r.reviewFile(shaOf("a"))
	r.reviewFile(shaOf("b"))

	if _, err := r.ing.Approve(r.ctx, shaOf("a")); err != nil {
		t.Fatal(err)
	}
	// The newer file is untouched and still needs its own approval.
	if f := r.file(shaOf("b")); f.Status != store.RZIStatusNeedsReview || f.ApprovedAt != nil {
		t.Errorf("newer file = %+v", f)
	}
	if !exists(r.review(shaOf("b"))) {
		t.Error("newer review record moved by another file's approval")
	}
	for _, x := range r.rows() {
		if x.File.SHA256 != shaOf("a") {
			t.Errorf("row from file %s after approving a", x.File.SHA256[:4])
		}
	}
	if _, err := r.ing.Approve(r.ctx, shaOf("a")); !errors.Is(err, rzi.ErrNoReviewRecord) {
		t.Errorf("second approval of a: err = %v", err)
	}
}

func TestApproveRefusesRecordUnderAnotherSHA(t *testing.T) {
	r := newRig(t)
	r.reviewFile(shaOf("a"))
	// A record for file b planted under a's name: approving a must not
	// publish b's rows.
	other := withBadUnit(varna(t, shaOf("b")))
	drop(t, filepath.Dir(r.review(shaOf("a"))), other)
	if err := os.Rename(r.review(shaOf("b")), r.review(shaOf("a"))); err != nil {
		t.Fatal(err)
	}

	_, err := r.ing.Approve(r.ctx, shaOf("a"))

	if !errors.Is(err, rzi.ErrSHAMismatch) {
		t.Fatalf("err = %v, want ErrSHAMismatch", err)
	}
	if len(r.rows()) != 0 || r.file(shaOf("a")).Status != store.RZIStatusNeedsReview {
		t.Error("a mismatched record changed the store")
	}
}

func TestApproveOnlyNeedsReview(t *testing.T) {
	r := newRig(t)
	drop(t, r.outbox, varna(t, shaOf("a")))
	r.process()
	before := r.rows()

	// Plant the record where approve looks, as a copied-back file would be.
	if err := os.MkdirAll(filepath.Dir(r.review(shaOf("a"))), 0o750); err != nil {
		t.Fatal(err)
	}
	drop(t, filepath.Dir(r.review(shaOf("a"))), varna(t, shaOf("a")))

	if _, err := r.ing.Approve(r.ctx, shaOf("a")); !errors.Is(err, rzi.ErrNotApprovable) {
		t.Errorf("ok file: err = %v, want ErrNotApprovable", err)
	}
	if !reflect.DeepEqual(before, r.rows()) {
		t.Error("store changed")
	}
	if f := r.file(shaOf("a")); f.ApprovedAt != nil {
		t.Errorf("ok file carries an approval: %+v", f)
	}
}

func TestApproveNeverApprovesRejected(t *testing.T) {
	r := newRig(t)
	rej := fixture(t, "rejected.json")
	drop(t, r.outbox, rej)
	r.process()
	if r.file(rej.SHA256).Status != store.RZIStatusRejected {
		t.Fatal("setup: file not rejected")
	}
	if err := os.MkdirAll(filepath.Dir(r.review(rej.SHA256)), 0o750); err != nil {
		t.Fatal(err)
	}
	// Planted record that decodes cleanly, so only the stored status stops it.
	clean := varna(t, rej.SHA256)
	clean.Source = rzi.SourceBurgas
	clean.Blocks = nil
	drop(t, filepath.Dir(r.review(rej.SHA256)), clean)
	if _, err := r.ing.Approve(r.ctx, rej.SHA256); !errors.Is(err, rzi.ErrNotApprovable) {
		t.Errorf("rejected row: err = %v, want ErrNotApprovable", err)
	}

	// A needs_review row whose record carries an extractor error: the record
	// is what makes it structurally rejected.
	r.reviewFile(shaOf("b"))
	bad := withBadUnit(varna(t, shaOf("b")))
	bad.Error = "magic mismatch"
	drop(t, filepath.Dir(r.review(shaOf("b"))), bad)
	if _, err := r.ing.Approve(r.ctx, shaOf("b")); !errors.Is(err, rzi.ErrNotApprovable) {
		t.Errorf("record with extractor error: err = %v, want ErrNotApprovable", err)
	}
	if f := r.file(shaOf("b")); f.Status != store.RZIStatusNeedsReview || len(r.rows()) != 0 {
		t.Errorf("refused approval changed the store: %+v", f)
	}
}

func TestApproveRefusesBadInput(t *testing.T) {
	r := newRig(t)
	for _, sha := range []string{"", "abc", "../../etc/passwd", shaOf("A"), shaOf("a") + "x"} {
		if _, err := r.ing.Approve(r.ctx, sha); !errors.Is(err, rzi.ErrBadSHA) {
			t.Errorf("Approve(%q) = %v, want ErrBadSHA", sha, err)
		}
	}
	if _, err := r.ing.Approve(r.ctx, shaOf("9")); !errors.Is(err, rzi.ErrNoReviewRecord) {
		t.Errorf("no record: err = %v", err)
	}
}

func TestApproveWithNothingToPublish(t *testing.T) {
	r := newRig(t)
	r.seedOK(shaOf("1"), siteA, 1)
	before := r.rows()
	f := varna(t, shaOf("b"))
	for bi := range f.Blocks {
		for ri := range f.Blocks[bi].Rows {
			f.Blocks[bi].Rows[ri].Unit = "mg/l"
		}
	}
	drop(t, r.outbox, f)
	r.process()

	if _, err := r.ing.Approve(r.ctx, shaOf("b")); !errors.Is(err, store.ErrRZINothingToPublish) {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(before, r.rows()) {
		t.Error("empty approval wiped the source")
	}
}

func TestApproveRefusesAfterNewerGoodFile(t *testing.T) {
	r := newRig(t)
	r.reviewFile(shaOf("b"))
	// A later file went ok. Approving the older one would roll data back.
	later := varna(t, shaOf("c"))
	f := store.RZIFile{
		Source: rzi.SourceVarna, URL: "u", SHA256: later.SHA256, FetchedAt: r.now.Add(time.Hour),
		ExtractorVersion: later.ExtractorVersion, Status: store.RZIStatusOK, RowsOK: 1,
	}
	x := store.RZISample{SiteID: siteC, Date: time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), Season: 2026, EC: 15, ECBelow: true, IE: 15, IEBelow: true, Unit: "MPN/100ml"}
	if err := r.st.ReplaceRZISource(r.ctx, rzi.SourceVarna, f, []store.RZISample{x}); err != nil {
		t.Fatal(err)
	}

	if _, err := r.ing.Approve(r.ctx, shaOf("b")); !errors.Is(err, rzi.ErrSuperseded) {
		t.Fatalf("err = %v, want ErrSuperseded", err)
	}
	if r.file(shaOf("b")).Status != store.RZIStatusNeedsReview {
		t.Error("superseded file changed")
	}
}

// A row that rewrites a stored sample is flagged too, and approval leaves it out.
func TestApproveLeavesOutRowThatRewritesAStoredSample(t *testing.T) {
	r := newRig(t)
	r.process()
	drop(t, r.outbox, varna(t, shaOf("a")))
	r.process()

	g := varna(t, shaOf("c"))
	g.Blocks[1].Rows = []rzi.Row{g.Blocks[1].Rows[0]}
	g.Blocks[1].Rows[0].EC.Value++
	changed := g.Blocks[1].Rows[0]
	total := 0
	for _, b := range g.Blocks {
		total += len(b.Rows)
	}
	drop(t, r.outbox, g)
	if s := r.process(); s.NeedsReview != 1 {
		t.Fatalf("stats = %+v", s)
	}

	a, err := r.ing.Approve(r.ctx, shaOf("c"))
	if err != nil {
		t.Fatal(err)
	}
	if a.Excluded != 1 || a.Published != total-1 {
		t.Errorf("approval = %+v, want 1 excluded of %d", a, total)
	}
	for _, x := range r.rows() {
		if x.Date.Format("2006-01-02") == changed.Date && int64(x.EC) == changed.EC.Value {
			t.Errorf("the rewriting row was published: %+v", x)
		}
	}
}
