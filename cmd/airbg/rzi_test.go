package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/store"
	"kanarche.eu/internal/upstream/rzi"
)

func shippedConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.LoadFileOffline(filepath.Join("..", "..", "airbg.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// The shipped config turns the RZI feature on.
func TestShippedConfigEnablesRZI(t *testing.T) {
	if !shippedConfig(t).Sea.RZI.Enabled {
		t.Fatal("shipped config has sea.rzi.enabled = false")
	}
}

// serve starts the collector only when sea.rzi.enabled is true. The flag is set
// both ways here so the test does not depend on the shipped value.
func TestServeStartsRZICollectorOnlyWhenEnabled(t *testing.T) {
	cfg := shippedConfig(t)
	cfg.Sea.RZI.Enabled = false
	c, err := rziCollectorFor(cfg, nil)
	if err != nil || c != nil {
		t.Fatalf("disabled: collector = %v, err = %v, want none", c, err)
	}
	cfg.Sea.RZI.Enabled = true
	c, err = rziCollectorFor(cfg, nil)
	if err != nil || c == nil {
		t.Fatalf("enabled: collector = %v, err = %v, want one", c, err)
	}
}

// import-rzi runs one pass whatever the flag says.
func TestImportRZIBuildsACollectorWhenDisabled(t *testing.T) {
	cfg := shippedConfig(t)
	c, err := newRZICollector(cfg, nil)
	if err != nil || c == nil {
		t.Fatalf("collector = %v, err = %v", c, err)
	}
}

type fakeApprover struct {
	res  rzi.Approval
	err  error
	args []string
}

// fakeReviewer also has an Approve that fails the test if it is called.
type fakeReviewer struct {
	t   *testing.T
	rec rzi.File
	err error
}

func (f fakeReviewer) Review(string) (rzi.File, error) { return f.rec, f.err }

func (f fakeReviewer) Approve(context.Context, string) (rzi.Approval, error) {
	f.t.Error("rzi-review called Approve")
	return rzi.Approval{}, nil
}

func TestRZIReviewNeverApproves(t *testing.T) {
	var out, errb bytes.Buffer
	f := fakeReviewer{t: t, rec: rzi.File{Source: "s", SHA256: "x"}}
	runRZIReview([]string{strings.Repeat("ab", 32)}, f, &out, &errb)
}

func TestWriteRZIRemarksListsEveryRemark(t *testing.T) {
	rec := rzi.File{Blocks: []rzi.Block{
		{Point: "P1", Name: "Sunny Beach", Rows: []rzi.Row{
			{Date: "2026-07-01", Remarks: "rain"},
			{Date: "2026-07-02"},
		}},
		{Point: "P2", Rows: []rzi.Row{{Date: "2026-07-03", Remarks: "bad\x1b[31m\ncolour"}}},
	}}
	var out bytes.Buffer
	writeRZIRemarks(&out, rec)
	want := "remark 2026-07-01 Sunny Beach: rain\n" +
		"remark 2026-07-03 P2: bad [31m colour\n"
	if out.String() != want {
		t.Errorf("got %q want %q", out.String(), want)
	}
}

func TestWriteRZIRemarksSaysNone(t *testing.T) {
	rec := rzi.File{Blocks: []rzi.Block{{Name: "A", Rows: []rzi.Row{{Date: "2026-07-01"}}}}}
	var out bytes.Buffer
	writeRZIRemarks(&out, rec)
	if out.String() != "remarks: none\n" {
		t.Errorf("got %q", out.String())
	}
}

func TestRZIReviewPrintsRemarks(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	f := fakeReviewer{t: t, rec: rzi.File{Source: "rzi_varna", SHA256: sha,
		Blocks: []rzi.Block{{Name: "A", Rows: []rzi.Row{{Date: "2026-07-01", Remarks: "rain"}, {Date: "2026-07-02"}}}}}}
	var out, errb bytes.Buffer
	if code := runRZIReview([]string{sha}, f, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	want := fmt.Sprintf("review source=rzi_varna sha256=%s rows=2\nremark 2026-07-01 A: rain\n", sha)
	if out.String() != want || errb.Len() != 0 {
		t.Errorf("stdout %q stderr %q", out.String(), errb.String())
	}
}

func TestRZIReviewSaysNone(t *testing.T) {
	var out, errb bytes.Buffer
	f := fakeReviewer{t: t, rec: rzi.File{Source: "s", SHA256: "x", Blocks: []rzi.Block{{Rows: []rzi.Row{{Date: "2026-07-01"}}}}}}
	if code := runRZIReview([]string{strings.Repeat("ab", 32)}, f, &out, &errb); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.HasSuffix(out.String(), "\nremarks: none\n") {
		t.Errorf("stdout %q", out.String())
	}
}

func TestRZIReviewExitCodesAndFixedMessages(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	cases := []struct {
		name string
		args []string
		err  error
		code int
		msg  string
	}{
		{"no args", nil, nil, 2, "usage"},
		{"bad sha", []string{"zz"}, rzi.ErrBadSHA, 2, rzi.ErrBadSHA.Error()},
		{"no record", []string{sha}, rzi.ErrNoReviewRecord, 1, rzi.ErrNoReviewRecord.Error()},
		{"unknown", []string{sha}, errors.New("secret /path"), 1, "internal error"},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		code := runRZIReview(c.args, fakeReviewer{t: t, err: c.err}, &out, &errb)
		if code != c.code || !strings.Contains(errb.String(), c.msg) || strings.Contains(errb.String(), "secret") || out.Len() != 0 {
			t.Errorf("%s: code %d stderr %q stdout %q", c.name, code, errb.String(), out.String())
		}
	}
}

func (f *fakeApprover) Approve(_ context.Context, sha string) (rzi.Approval, error) {
	f.args = append(f.args, sha)
	return f.res, f.err
}

func TestRZIApprovePrintsCountsOnly(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	f := &fakeApprover{res: rzi.Approval{Source: "varna", SHA256: sha, Published: 12, Excluded: 3}}
	var out, errb bytes.Buffer
	if code := runRZIApprove(context.Background(), []string{sha}, f, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	want := fmt.Sprintf("approved source=varna sha256=%s published=12 excluded=3\n", sha)
	if out.String() != want || errb.Len() != 0 {
		t.Errorf("stdout %q stderr %q", out.String(), errb.String())
	}
}

func TestRZIApproveSaysWhenAlreadyApproved(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	f := &fakeApprover{res: rzi.Approval{Source: "varna", SHA256: sha, Published: 12, Excluded: 3, AlreadyApproved: true}}
	var out, errb bytes.Buffer
	if code := runRZIApprove(context.Background(), []string{sha}, f, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	want := fmt.Sprintf("already approved source=varna sha256=%s published=12 excluded=3\n", sha)
	if out.String() != want || errb.Len() != 0 {
		t.Errorf("stdout %q stderr %q", out.String(), errb.String())
	}
}

func TestRZIApproveNamesTheNewRefusals(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	for _, e := range []error{rzi.ErrReviewTampered, store.ErrRZIFileIsOK} {
		var out, errb bytes.Buffer
		code := runRZIApprove(context.Background(), []string{sha}, &fakeApprover{err: e}, &out, &errb)
		if code != 1 || !strings.Contains(errb.String(), e.Error()) {
			t.Errorf("%v: exit %d, stderr %q", e, code, errb.String())
		}
	}
}

func TestRZIApproveUsageAndExitCodes(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	cases := []struct {
		name string
		args []string
		err  error
		code int
	}{
		{"no argument", nil, nil, 2},
		{"two arguments", []string{sha, sha}, nil, 2},
		{"bad sha", []string{"xyz"}, rzi.ErrBadSHA, 2},
		{"not approvable", []string{sha}, fmt.Errorf("%w: status rejected", rzi.ErrNotApprovable), 1},
		{"unknown file", []string{sha}, store.ErrRZIUnknownFile, 1},
		{"nothing to publish", []string{sha}, store.ErrRZINothingToPublish, 1},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		code := runRZIApprove(context.Background(), c.args, &fakeApprover{err: c.err}, &out, &errb)
		if code != c.code {
			t.Errorf("%s: exit %d, want %d", c.name, code, c.code)
		}
		if out.Len() != 0 {
			t.Errorf("%s: wrote to stdout: %q", c.name, out.String())
		}
	}
}

// fakeSink answers only the lookup the fetcher hook makes.
type fakeSink struct {
	rzi.Sink
	row store.RZIFile
}

func (f fakeSink) RZIFileBySHA(context.Context, string) (store.RZIFile, bool, error) {
	return f.row, true, nil
}

// The collector's fetcher must skip settled hashes, or a pruned inbox is
// refilled on every pass.
func TestRZIFetchOptionsWireSettled(t *testing.T) {
	sha := strings.Repeat("a", 64)
	opts := rziFetchOptions(fakeSink{row: store.RZIFile{Status: store.RZIStatusOK}})
	if opts.Settled == nil {
		t.Fatal("Settled is not wired")
	}
	if !opts.Settled(context.Background(), sha) {
		t.Error("ok row not settled through the wired hook")
	}
	review := rziFetchOptions(fakeSink{row: store.RZIFile{Status: store.RZIStatusNeedsReview}})
	if review.Settled(context.Background(), sha) {
		t.Error("needs_review row settled through the wired hook")
	}
}

// An error nobody anticipated may wrap text from the database or the disk.
func TestRZIApproveDoesNotEchoUnknownErrors(t *testing.T) {
	var out, errb bytes.Buffer
	boom := errors.New("pq: secret document text 0xDEADBEEF")
	code := runRZIApprove(context.Background(), []string{strings.Repeat("ab", 32)}, &fakeApprover{err: boom}, &out, &errb)
	if code != 1 || strings.Contains(errb.String(), "DEADBEEF") || strings.Contains(errb.String(), "secret") {
		t.Errorf("exit %d, stderr %q", code, errb.String())
	}
}
