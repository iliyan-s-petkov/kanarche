package rzi_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/store"
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

// failSink fails the calls a test switches on and passes the rest through.
type failSink struct {
	rzi.Sink
	replace, lookup bool
}

var errSink = errors.New("sink down")

func (f failSink) ReplaceRZISource(ctx context.Context, source string, file store.RZIFile, xs []store.RZISample) error {
	if f.replace {
		return errSink
	}
	return f.Sink.ReplaceRZISource(ctx, source, file, xs)
}

func (f failSink) RZIFileBySHA(ctx context.Context, sha string) (store.RZIFile, bool, error) {
	if f.lookup {
		return store.RZIFile{}, false, errSink
	}
	return f.Sink.RZIFileBySHA(ctx, sha)
}

func TestIngestKeepsInboxOnNeedsReview(t *testing.T) {
	r := newRig(t)
	doc := r.inboxDoc(shaOf("b"))
	drop(t, r.outbox, withBadUnit(varna(t, shaOf("b"))))

	r.process()

	if !exists(doc) {
		t.Error("needs_review document was pruned")
	}
	if !exists(filepath.Join(r.outbox, "review", shaOf("b")+".json")) {
		t.Error("review record missing")
	}
}

func TestIngestKeepsInboxOnRejected(t *testing.T) {
	r := newRig(t)
	f := fixture(t, "rejected.json")
	doc := r.inboxDoc(f.SHA256)
	drop(t, r.outbox, f)

	r.process()

	if !exists(doc) {
		t.Error("rejected document was pruned")
	}
}

func TestIngestPrunesInboxOnDuplicateOfOK(t *testing.T) {
	r := newRig(t)
	drop(t, r.outbox, varna(t, shaOf("a")))
	r.process()
	doc := r.inboxDoc(shaOf("a"))
	drop(t, r.outbox, varna(t, shaOf("a")))

	if s := r.process(); s.Duplicate != 1 {
		t.Fatalf("stats = %+v", s)
	}
	if exists(doc) {
		t.Error("document of a stored ok file left in the inbox")
	}
}

func TestIngestKeepsInboxOnDuplicateOfNonOK(t *testing.T) {
	r := newRig(t)
	drop(t, r.outbox, withBadUnit(varna(t, shaOf("b"))))
	r.process()
	doc := r.inboxDoc(shaOf("b"))
	drop(t, r.outbox, withBadUnit(varna(t, shaOf("b"))))

	if s := r.process(); s.Duplicate != 1 {
		t.Fatalf("stats = %+v", s)
	}
	if !exists(doc) {
		t.Error("document of a needs_review file was pruned by a duplicate")
	}
}

func TestIngestKeepsInboxOnRefusedRecord(t *testing.T) {
	r := newRig(t)
	doc := r.inboxDoc(shaOf("c"))
	if err := os.WriteFile(filepath.Join(r.outbox, shaOf("c")+".json"), []byte("{not json"), 0o640); err != nil {
		t.Fatal(err)
	}

	if s := r.process(); s.Refused != 1 {
		t.Fatalf("stats = %+v", s)
	}
	if !exists(doc) {
		t.Error("a refused record's name deleted an inbox document")
	}
}

func TestIngestKeepsInboxOnStoreError(t *testing.T) {
	r := newRig(t)
	doc := r.inboxDoc(shaOf("a"))
	drop(t, r.outbox, varna(t, shaOf("a")))
	ing := rzi.NewIngester(r.cfg, failSink{Sink: r.st, replace: true}, func() time.Time { return r.now })

	if _, err := ing.Process(r.ctx); err == nil {
		t.Fatal("Process: want the store error")
	}
	if !exists(doc) {
		t.Error("document pruned though the store write failed")
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

func TestIngestPruneSkipsSymlinkAndDirectory(t *testing.T) {
	r := newRig(t)
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(r.cfg.InboxDir, shaOf("a")+".pdf")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(r.cfg.InboxDir, shaOf("a")+".xlsx")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	drop(t, r.outbox, varna(t, shaOf("a")))

	r.process()

	if !exists(link) || !exists(dir) || !exists(target) {
		t.Errorf("symlink %v, directory %v or target %v was removed", exists(link), exists(dir), exists(target))
	}
}

func TestSettledInIsTrueOnlyForOKRows(t *testing.T) {
	r := newRig(t)
	rej := fixture(t, "rejected.json")
	drop(t, r.outbox, varna(t, shaOf("a")))
	drop(t, r.outbox, withBadUnit(varna(t, shaOf("b"))))
	drop(t, r.outbox, rej)
	r.process()
	settled := rzi.SettledIn(r.st)

	if !settled(r.ctx, shaOf("a")) {
		t.Error("ok hash not settled")
	}
	if settled(r.ctx, shaOf("b")) {
		t.Error("needs_review hash reported settled")
	}
	if settled(r.ctx, rej.SHA256) {
		t.Error("rejected hash reported settled")
	}
	if settled(r.ctx, shaOf("9")) {
		t.Error("unknown hash reported settled")
	}
}

func TestSettledInStoreErrorIsNotSettledAndLogs(t *testing.T) {
	r := newRig(t)
	r.seedOK(shaOf("1"), siteA, 1)
	buf := captureLog(t)

	if rzi.SettledIn(failSink{Sink: r.st, lookup: true})(r.ctx, shaOf("1")) {
		t.Error("store error reported settled")
	}
	if !strings.Contains(buf.String(), "level=WARN") {
		t.Errorf("no warning logged: %q", buf.String())
	}
}
