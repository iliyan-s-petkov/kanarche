package rzi_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kanarche.eu/internal/store"
	"kanarche.eu/internal/upstream/rzi"
)

// The review copy is what approval publishes from, so it must be the bytes
// the verdict was made on.
func TestApproveRefusesTamperedReviewRecord(t *testing.T) {
	cases := map[string]func([]byte) []byte{
		// The flagged row is made valid, so approval would publish one more.
		"content edited": func(b []byte) []byte {
			return []byte(strings.Replace(string(b), `"mg/l"`, `"MPN/100ml"`, 1))
		},
		"whitespace appended": func(b []byte) []byte { return append(b, '\n') },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRig(t)
			r.reviewFile(shaOf("b"))
			b, err := os.ReadFile(r.review(shaOf("b")))
			if err != nil {
				t.Fatal(err)
			}
			edited := edit(b)
			if string(edited) == string(b) {
				t.Fatal("setup: the edit changed nothing")
			}
			if err := os.WriteFile(r.review(shaOf("b")), edited, 0o640); err != nil {
				t.Fatal(err)
			}

			_, err = r.ing.Approve(r.ctx, shaOf("b"))

			if !errors.Is(err, rzi.ErrReviewTampered) {
				t.Fatalf("err = %v, want ErrReviewTampered", err)
			}
			if len(r.rows()) != 0 || r.file(shaOf("b")).Status != store.RZIStatusNeedsReview {
				t.Error("a tampered record changed the store")
			}
			if !exists(r.review(shaOf("b"))) {
				t.Error("the tampered record was moved away")
			}
		})
	}
}

func TestIngestStoresTheRecordHash(t *testing.T) {
	r := newRig(t)
	r.reviewFile(shaOf("b"))
	b, err := os.ReadFile(r.review(shaOf("b")))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := r.file(shaOf("b")).RecordSHA256, shaOfBytes(b); got != want {
		t.Errorf("record_sha256 = %q, want %q", got, want)
	}
}

func TestApproveRefusesAReviewRecordThatIsASymlink(t *testing.T) {
	r := newRig(t)
	r.reviewFile(shaOf("b"))
	real := filepath.Join(r.outbox, "elsewhere.json")
	if err := os.Rename(r.review(shaOf("b")), real); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, r.review(shaOf("b"))); err != nil {
		t.Fatal(err)
	}

	_, err := r.ing.Approve(r.ctx, shaOf("b"))

	if err == nil || errors.Is(err, rzi.ErrNoReviewRecord) {
		t.Fatalf("err = %v, want a refusal of the symlink", err)
	}
	if strings.Contains(err.Error(), real) {
		t.Errorf("error names the target: %v", err)
	}
	if len(r.rows()) != 0 || r.file(shaOf("b")).Status != store.RZIStatusNeedsReview {
		t.Error("a symlinked record changed the store")
	}
}

// The store write and the move are two steps. If the move was lost, running
// approve again finishes it instead of failing.
func TestApproveFinishesTheMoveOfAnAlreadyApprovedFile(t *testing.T) {
	r := newRig(t)
	r.reviewFile(shaOf("b"))
	first, err := r.ing.Approve(r.ctx, shaOf("b"))
	if err != nil {
		t.Fatal(err)
	}
	done := filepath.Join(r.outbox, "done", shaOf("b")+".json")
	if err := os.Rename(done, r.review(shaOf("b"))); err != nil {
		t.Fatal(err)
	}
	before := r.rows()

	again, err := r.ing.Approve(r.ctx, shaOf("b"))

	if err != nil {
		t.Fatalf("second approve: %v", err)
	}
	if !again.AlreadyApproved || again.Published != first.Published || again.Excluded != first.Excluded {
		t.Errorf("second = %+v, first = %+v", again, first)
	}
	if first.AlreadyApproved {
		t.Error("first approval reported as already approved")
	}
	if exists(r.review(shaOf("b"))) || !exists(done) {
		t.Error("the stale review copy was not moved to done/")
	}
	if !reflect.DeepEqual(before, r.rows()) {
		t.Error("the repeat approval changed the samples")
	}
}

// An ok file is not re-validated, whatever the extractor version says.
func TestIngestNeverRevalidatesAnOKFile(t *testing.T) {
	r := newRig(t)
	drop(t, r.outbox, varna(t, shaOf("a")))
	r.process()
	before := r.rows()

	again := withBadUnit(varna(t, shaOf("a")))
	again.ExtractorVersion = "rzi-extract/9"
	drop(t, r.outbox, again)
	s := r.process()

	if s.Duplicate != 1 || s.NeedsReview != 0 {
		t.Fatalf("stats = %+v", s)
	}
	if f := r.file(shaOf("a")); f.Status != store.RZIStatusOK {
		t.Errorf("ok file downgraded: %+v", f)
	}
	if !reflect.DeepEqual(before, r.rows()) {
		t.Error("samples changed")
	}
	if !exists(filepath.Join(r.outbox, "done", shaOf("a")+".json")) {
		t.Error("record not moved to done/")
	}
}

// A rejected document read again by a newer extractor may now be reviewable.
func TestIngestMovesARejectedFileToReviewUnderANewExtractor(t *testing.T) {
	r := newRig(t)
	rej := fixture(t, "rejected.json")
	drop(t, r.outbox, rej)
	r.process()
	if r.file(rej.SHA256).Status != store.RZIStatusRejected {
		t.Fatal("setup: not rejected")
	}

	next := withBadUnit(varna(t, rej.SHA256))
	next.ExtractorVersion = "rzi-extract/9"
	drop(t, r.outbox, next)
	s := r.process()

	if s.NeedsReview != 1 {
		t.Fatalf("stats = %+v", s)
	}
	if f := r.file(rej.SHA256); f.Status != store.RZIStatusNeedsReview || f.ExtractorVersion != "rzi-extract/9" {
		t.Errorf("file = %+v", f)
	}
}

func shaOfBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
