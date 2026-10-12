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

// serve starts the collector only when sea.rzi.enabled is true, and the
// shipped config has it false.
func TestServeStartsRZICollectorOnlyWhenEnabled(t *testing.T) {
	cfg := shippedConfig(t)
	if cfg.Sea.RZI.Enabled {
		t.Fatal("shipped config has sea.rzi.enabled = true")
	}
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
