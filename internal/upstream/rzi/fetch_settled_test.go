package rzi

import (
	"context"
	"testing"
	"time"
)

func settledFetcher(t *testing.T, r *rig, settled func(context.Context, string) bool) *Fetcher {
	t.Helper()
	f, err := New(r.cfg, Options{
		Transport: r.tr,
		Now:       func() time.Time { return time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC) },
		Settled:   settled,
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFetchSettledHashIsNotSavedAgain(t *testing.T) {
	body := []byte(pdfBody)
	r := varnaRig(t, serveBytes("application/pdf", body))
	var asked string
	f := settledFetcher(t, r, func(_ context.Context, sha string) bool { asked = sha; return true })
	res, err := f.Fetch(context.Background(), SourceVarna, Validators{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusUnchanged || asked != sum(body) {
		t.Errorf("Status = %v, asked = %q", res.Status, asked)
	}
	if names := inboxNames(t, r.inbox); len(names) != 0 {
		t.Errorf("inbox = %v, want empty", names)
	}
}

func TestFetchUnsettledHashIsSaved(t *testing.T) {
	body := []byte(pdfBody)
	r := varnaRig(t, serveBytes("application/pdf", body))
	f := settledFetcher(t, r, func(context.Context, string) bool { return false })
	res, err := f.Fetch(context.Background(), SourceVarna, Validators{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusWritten {
		t.Errorf("Status = %v, want written", res.Status)
	}
}
