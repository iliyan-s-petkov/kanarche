package rzi_test

import (
	"os"
	"path/filepath"
	"testing"

	"kanarche.eu/internal/upstream/rzi"
)

// inboxDoc puts a fake fetched document in the inbox as <sha>.pdf.
func (r *rig) inboxDoc(sha string) string {
	r.t.Helper()
	p := filepath.Join(r.cfg.InboxDir, sha+".pdf")
	if err := os.WriteFile(p, []byte("%PDF-1.5 test"), 0o640); err != nil {
		r.t.Fatal(err)
	}
	return p
}

func TestIngestPrunesInboxOnOK(t *testing.T) {
	r := newRig(t)
	doc := r.inboxDoc(shaOf("a"))
	other := r.inboxDoc(shaOf("e"))
	drop(t, r.outbox, varna(t, shaOf("a")))

	r.process()

	if exists(doc) {
		t.Error("settled ok document left in the inbox")
	}
	if !exists(other) {
		t.Error("a document with no record was pruned")
	}
	if got := r.file(shaOf("a")).Bytes; got != len("%PDF-1.5 test") {
		t.Errorf("file row bytes = %d, want the size read before the prune", got)
	}
}

func TestIngestPrunesInboxOnNeedsReview(t *testing.T) {
	r := newRig(t)
	doc := r.inboxDoc(shaOf("b"))
	drop(t, r.outbox, withBadUnit(varna(t, shaOf("b"))))

	r.process()

	if exists(doc) {
		t.Error("needs_review document left in the inbox")
	}
	if !exists(filepath.Join(r.outbox, "review", shaOf("b")+".json")) {
		t.Error("review record missing")
	}
	// Approval reads the review record only, never the inbox.
	if _, err := r.ing.Approve(r.ctx, shaOf("b")); err != nil {
		t.Fatalf("Approve after prune: %v", err)
	}
}

func TestIngestPrunesInboxOnRejected(t *testing.T) {
	r := newRig(t)
	f := fixture(t, "rejected.json")
	doc := r.inboxDoc(f.SHA256)
	drop(t, r.outbox, f)

	r.process()

	if exists(doc) {
		t.Error("rejected document left in the inbox")
	}
}

func TestIngestPrunesInboxOnDuplicate(t *testing.T) {
	r := newRig(t)
	drop(t, r.outbox, varna(t, shaOf("a")))
	r.process()
	doc := r.inboxDoc(shaOf("a"))
	drop(t, r.outbox, varna(t, shaOf("a")))

	if s := r.process(); s.Duplicate != 1 {
		t.Fatalf("stats = %+v", s)
	}
	if exists(doc) {
		t.Error("duplicate's document left in the inbox")
	}
}

func TestIngestPrunesInboxOnRefusedRecord(t *testing.T) {
	r := newRig(t)
	doc := r.inboxDoc(shaOf("c"))
	if err := os.WriteFile(filepath.Join(r.outbox, shaOf("c")+".json"), []byte("{not json"), 0o640); err != nil {
		t.Fatal(err)
	}

	if s := r.process(); s.Refused != 1 {
		t.Fatalf("stats = %+v", s)
	}
	if exists(doc) {
		t.Error("document of a refused record left in the inbox")
	}
}

func TestIngestKeepsInboxWhenRecordIsNotSettled(t *testing.T) {
	r := newRig(t)
	doc := r.inboxDoc(shaOf("a"))
	drop(t, r.outbox, varna(t, shaOf("a")))
	// A file named done blocks the move of the record.
	if err := os.WriteFile(filepath.Join(r.outbox, "done"), nil, 0o640); err != nil {
		t.Fatal(err)
	}

	if _, err := r.ing.Process(r.ctx); err == nil {
		t.Fatal("Process: want the move error")
	}
	if !exists(doc) {
		t.Error("document pruned before its record was settled")
	}
}

func TestSettledInReportsStoredHashes(t *testing.T) {
	r := newRig(t)
	r.seedOK(shaOf("1"), siteA, 1)
	settled := rzi.SettledIn(r.st)

	if !settled(r.ctx, shaOf("1")) {
		t.Error("stored hash not reported settled")
	}
	if settled(r.ctx, shaOf("2")) {
		t.Error("unknown hash reported settled")
	}
}
