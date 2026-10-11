package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"kanarche.eu/internal/store"
)

func key(site string, day int) store.RZIKey {
	return store.RZIKey{SiteID: site, Date: rziSample(site, day).Date}
}

// A flagged row whose key is already stored leaves the stored sample alone.
func TestApproveRZIFileKeepsStoredSampleForHeldKey(t *testing.T) {
	ctx, _, s := newStore(t)
	old := rziSample(siteA, 2)
	old.EC = 99
	mustReplace(t, ctx, s, rziFile("rzi_varna", hash("a")), old, rziSample(siteA, 9), rziSample(siteB, 2))
	if _, err := s.RecordRZIFile(ctx, reviewFile("rzi_varna", hash("c"))); err != nil {
		t.Fatal(err)
	}
	// siteA day 2 is held. siteB day 2 is held but also in the approved set,
	// where the approved row wins.
	newB := rziSample(siteB, 2)
	newB.EC = 5
	xs := []store.RZISample{rziSample(siteA, 16), newB}
	keep := []store.RZIKey{key(siteA, 2), key(siteB, 2)}

	if err := s.ApproveRZIFile(ctx, hash("c"), "rzi_varna", xs, keep, time.Now(), 1); err != nil {
		t.Fatal(err)
	}

	rows, _ := s.LoadRZI(ctx)
	got := map[store.RZIKey]store.RZIRow{}
	for _, r := range rows {
		got[store.RZIKey{SiteID: r.SiteID, Date: r.Date}] = r
	}
	if len(got) != 3 {
		t.Fatalf("rows = %d, want held A2, new A16, new B2", len(got))
	}
	if r := got[key(siteA, 2)]; r.EC != 99 || r.File.SHA256 != hash("a") {
		t.Errorf("held stored sample changed: %+v", r)
	}
	if _, ok := got[key(siteA, 9)]; ok {
		t.Error("a stored sample that is neither held nor in the new file survived")
	}
	if r := got[key(siteB, 2)]; r.EC != 5 || r.File.SHA256 != hash("c") {
		t.Errorf("approved row did not win over the held key: %+v", r)
	}
}

func TestApproveRZIFileRefusesWhenNewerFileIsOK(t *testing.T) {
	ctx, _, s := newStore(t)
	if _, err := s.RecordRZIFile(ctx, reviewFile("rzi_varna", hash("c"))); err != nil {
		t.Fatal(err)
	}
	later := rziFile("rzi_varna", hash("d"))
	later.FetchedAt = later.FetchedAt.Add(time.Hour)
	mustReplace(t, ctx, s, later, rziSample(siteB, 20))

	err := s.ApproveRZIFile(ctx, hash("c"), "rzi_varna", []store.RZISample{rziSample(siteA, 16)}, nil, time.Now(), 0)
	if !errors.Is(err, store.ErrRZISuperseded) {
		t.Fatalf("err = %v, want ErrRZISuperseded", err)
	}
	rows, _ := s.LoadRZI(ctx)
	if len(rows) != 1 || rows[0].SiteID != siteB {
		t.Errorf("a refused approval changed the samples: %+v", rows)
	}
}

// hold takes the per-source lock the way a writer would and returns the
// release function.
func hold(t *testing.T, ctx context.Context, p interface {
	Begin(context.Context) (pgx.Tx, error)
}, source string) func() {
	t.Helper()
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('rzi:' || $1::text))`, source); err != nil {
		t.Fatal(err)
	}
	return func() { _ = tx.Commit(ctx) }
}

// blocked reports whether done is still open after a short wait.
func blocked(done <-chan error) bool {
	select {
	case <-done:
		return false
	case <-time.After(400 * time.Millisecond):
		return true
	}
}

func TestApproveRZIFileWaitsForTheSourceLock(t *testing.T) {
	ctx, pool, s := newStore(t)
	if _, err := s.RecordRZIFile(ctx, reviewFile("rzi_varna", hash("c"))); err != nil {
		t.Fatal(err)
	}
	release := hold(t, ctx, pool, "rzi_varna")
	done := make(chan error, 1)
	go func() {
		done <- s.ApproveRZIFile(ctx, hash("c"), "rzi_varna", []store.RZISample{rziSample(siteA, 16)}, nil, time.Now(), 0)
	}()
	if !blocked(done) {
		release()
		t.Fatal("approve ran while another writer held the source lock")
	}
	release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestReplaceRZISourceWaitsForTheSourceLock(t *testing.T) {
	ctx, pool, s := newStore(t)
	release := hold(t, ctx, pool, "rzi_varna")
	done := make(chan error, 1)
	go func() {
		f := rziFile("rzi_varna", hash("a"))
		done <- s.ReplaceRZISource(ctx, "rzi_varna", f, []store.RZISample{rziSample(siteA, 2)})
	}()
	if !blocked(done) {
		release()
		t.Fatal("replace ran while another writer held the source lock")
	}
	release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// Another source is not held up by it.
	release = hold(t, ctx, pool, "rzi_varna")
	defer release()
	if err := s.ReplaceRZISource(ctx, "rzi_burgas", rziFile("rzi_burgas", hash("b")), []store.RZISample{rziSample(siteB, 2)}); err != nil {
		t.Fatal(err)
	}
}

// After any interleaving of an approval of an older file and a newer ok file,
// the source's samples belong to one file.
func TestApproveAndReplaceRaceLeavesOneFile(t *testing.T) {
	ctx, pool, s := newStore(t)
	base := time.Date(2026, 6, 3, 8, 0, 0, 0, time.UTC)
	for i := 0; i < 25; i++ {
		b, c := fmt.Sprintf("%064x", 2*i+1), fmt.Sprintf("%064x", 2*i+2)
		fb := reviewFile("rzi_varna", b)
		fb.FetchedAt = base.Add(time.Duration(2*i) * time.Hour)
		fc := rziFile("rzi_varna", c)
		fc.FetchedAt = base.Add(time.Duration(2*i+1) * time.Hour)
		if _, err := s.RecordRZIFile(ctx, fb); err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			errs[0] = s.ApproveRZIFile(ctx, b, "rzi_varna",
				[]store.RZISample{rziSample(siteA, 2), rziSample(siteA, 3)}, nil, time.Now(), 0)
		}()
		go func() {
			defer wg.Done()
			errs[1] = s.ReplaceRZISource(ctx, "rzi_varna", fc,
				[]store.RZISample{rziSample(siteB, 10), rziSample(siteB, 11)})
		}()
		wg.Wait()

		if errs[1] != nil {
			t.Fatalf("round %d: replace: %v", i, errs[1])
		}
		if errs[0] != nil && !errors.Is(errs[0], store.ErrRZISuperseded) {
			t.Fatalf("round %d: approve: %v", i, errs[0])
		}
		var files int
		if err := pool.QueryRow(ctx, `SELECT count(DISTINCT file_id) FROM bathing_rzi_sample`).Scan(&files); err != nil {
			t.Fatal(err)
		}
		rows, _ := s.LoadRZI(ctx)
		if files != 1 || len(rows) != 2 || rows[0].File.SHA256 != c {
			t.Fatalf("round %d: %d files, %d rows, first from %.4s: mixed samples", i, files, len(rows), rows[0].File.SHA256)
		}
	}
}

// An ok file is never rewritten by a later verdict on the same document.
func TestRecordRZIFileNeverDowngradesAnOKFile(t *testing.T) {
	ctx, _, s := newStore(t)
	mustReplace(t, ctx, s, rziFile("rzi_varna", hash("a")), rziSample(siteA, 2))

	down := rziFile("rzi_varna", hash("a"))
	down.Status, down.Reason = store.RZIStatusNeedsReview, "block count drifted from last good file"
	if _, err := s.RecordRZIFile(ctx, down); !errors.Is(err, store.ErrRZIFileIsOK) {
		t.Fatalf("err = %v, want ErrRZIFileIsOK", err)
	}
	f, _, _ := s.RZIFileBySHA(ctx, hash("a"))
	rows, _ := s.LoadRZI(ctx)
	if f.Status != store.RZIStatusOK || len(rows) != 1 {
		t.Errorf("ok file changed: %+v with %d samples", f, len(rows))
	}
	// The same document going ok again is still allowed.
	mustReplace(t, ctx, s, rziFile("rzi_varna", hash("a")), rziSample(siteA, 2), rziSample(siteA, 9))
}

func TestRecordRZIFileMayMoveARejectedFileToReview(t *testing.T) {
	ctx, _, s := newStore(t)
	rej := rziFile("rzi_varna", hash("a"))
	rej.Status = store.RZIStatusRejected
	rej.ExtractorVersion = "rzi-extract 1"
	if _, err := s.RecordRZIFile(ctx, rej); err != nil {
		t.Fatal(err)
	}
	rev := reviewFile("rzi_varna", hash("a"))
	rev.ExtractorVersion = "rzi-extract 2"
	rev.RecordSHA256 = hash("e")
	if _, err := s.RecordRZIFile(ctx, rev); err != nil {
		t.Fatalf("rejected -> needs_review: %v", err)
	}
	f, _, _ := s.RZIFileBySHA(ctx, hash("a"))
	if f.Status != store.RZIStatusNeedsReview || f.RecordSHA256 != hash("e") {
		t.Errorf("row = %+v", f)
	}
}
