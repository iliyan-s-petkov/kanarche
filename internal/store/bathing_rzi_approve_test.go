package store_test

import (
	"errors"
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

func reviewFile(source, sha string) store.RZIFile {
	f := rziFile(source, sha)
	f.Status, f.RowsOK, f.RowsRejected, f.Reason = store.RZIStatusNeedsReview, 1, 1, "block count drifted from last good file"
	return f
}

func TestApproveRZIFilePublishesAndRecords(t *testing.T) {
	ctx, _, s := newStore(t)
	mustReplace(t, ctx, s, rziFile("rzi_varna", hash("a")), rziSample(siteA, 2), rziSample(siteA, 9))
	mustReplace(t, ctx, s, rziFile("rzi_burgas", hash("b")), rziSample(siteB, 2))
	if _, err := s.RecordRZIFile(ctx, reviewFile("rzi_varna", hash("c"))); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	if err := s.ApproveRZIFile(ctx, hash("c"), "rzi_varna", []store.RZISample{rziSample(siteA, 16)}, nil, at, 3); err != nil {
		t.Fatalf("ApproveRZIFile: %v", err)
	}

	f, ok, err := s.RZIFileBySHA(ctx, hash("c"))
	if err != nil || !ok {
		t.Fatalf("RZIFileBySHA = %v, %v", ok, err)
	}
	if f.Status != store.RZIStatusOK || f.ApprovedAt == nil || !f.ApprovedAt.Equal(at) ||
		f.ExcludedRows != 3 || f.RowsOK != 1 || f.RowsRejected != 3 {
		t.Errorf("file row = %+v", f)
	}
	rows, _ := s.LoadRZI(ctx)
	var varna, burgas int
	for _, r := range rows {
		switch r.File.Source {
		case "rzi_varna":
			varna++
			if r.Date.Day() != 16 {
				t.Errorf("varna row %v, want only the approved set", r.Date)
			}
		case "rzi_burgas":
			burgas++
		}
	}
	if varna != 1 || burgas != 1 {
		t.Errorf("varna %d burgas %d, want 1 and 1", varna, burgas)
	}
}

func TestApproveRZIFileOnlyFromNeedsReview(t *testing.T) {
	for _, status := range []string{store.RZIStatusOK, store.RZIStatusRejected} {
		t.Run(status, func(t *testing.T) {
			ctx, _, s := newStore(t)
			mustReplace(t, ctx, s, rziFile("rzi_varna", hash("a")), rziSample(siteA, 2))
			f := rziFile("rzi_varna", hash("c"))
			f.Status = status
			if _, err := s.RecordRZIFile(ctx, f); err != nil {
				t.Fatal(err)
			}
			err := s.ApproveRZIFile(ctx, hash("c"), "rzi_varna", []store.RZISample{rziSample(siteA, 16)}, nil, time.Now(), 0)
			if !errors.Is(err, store.ErrRZINotReviewable) {
				t.Fatalf("err = %v, want ErrRZINotReviewable", err)
			}
			rows, _ := s.LoadRZI(ctx)
			if len(rows) != 1 || rows[0].Date.Day() != 2 {
				t.Errorf("rows = %+v, want the old set kept", rows)
			}
		})
	}
}

func TestApproveRZIFileGuards(t *testing.T) {
	ctx, _, s := newStore(t)
	if _, err := s.RecordRZIFile(ctx, reviewFile("rzi_varna", hash("c"))); err != nil {
		t.Fatal(err)
	}
	one := []store.RZISample{rziSample(siteA, 16)}
	if err := s.ApproveRZIFile(ctx, hash("d"), "rzi_varna", one, nil, time.Now(), 0); !errors.Is(err, store.ErrRZIUnknownFile) {
		t.Errorf("unknown sha: err = %v", err)
	}
	if err := s.ApproveRZIFile(ctx, hash("c"), "rzi_burgas", one, nil, time.Now(), 0); err == nil {
		t.Error("source mismatch accepted")
	}
	if err := s.ApproveRZIFile(ctx, hash("c"), "rzi_varna", nil, nil, time.Now(), 0); !errors.Is(err, store.ErrRZINothingToPublish) {
		t.Errorf("empty set: err = %v", err)
	}
	f, _, _ := s.RZIFileBySHA(ctx, hash("c"))
	if f.Status != store.RZIStatusNeedsReview || f.ApprovedAt != nil {
		t.Errorf("file row changed by refused approvals: %+v", f)
	}
}

func TestRZILatestIsNewestOfAnyStatus(t *testing.T) {
	ctx, _, s := newStore(t)
	if _, ok, err := s.RZILatest(ctx, "rzi_varna"); ok || err != nil {
		t.Fatalf("empty: %v, %v", ok, err)
	}
	mustReplace(t, ctx, s, rziFile("rzi_varna", hash("a")), rziSample(siteA, 2))
	r := reviewFile("rzi_varna", hash("c"))
	r.FetchedAt = r.FetchedAt.Add(time.Hour)
	if _, err := s.RecordRZIFile(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.RZILatest(ctx, "rzi_varna")
	if err != nil || !ok || got.SHA256 != hash("c") {
		t.Errorf("latest = %+v, %v, %v", got, ok, err)
	}
}
