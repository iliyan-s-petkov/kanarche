package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/store"
	"kanarche.eu/internal/upstream/rzi"
)

// rziCollectorFor returns the RZI collector, or nil when sea.rzi.enabled is
// false so serve starts nothing.
func rziCollectorFor(cfg config.Config, sink rzi.Sink) (*rzi.Collector, error) {
	if !cfg.Sea.RZI.Enabled {
		return nil, nil
	}
	return newRZICollector(cfg, sink)
}

// newRZICollector builds the collector regardless of the enabled flag, for
// the one-shot import-rzi.
func newRZICollector(cfg config.Config, sink rzi.Sink) (*rzi.Collector, error) {
	f, err := rzi.New(cfg.Sea, rzi.Options{Settled: rzi.SettledIn(sink)})
	if err != nil {
		return nil, fmt.Errorf("rzi fetcher: %w", err)
	}
	return rzi.NewCollector(cfg.Sea.RZI, sink, f), nil
}

// approver is the part of the ingester the approve command needs.
type approver interface {
	Approve(ctx context.Context, sha string) (rzi.Approval, error)
}

// runRZIApprove publishes the passed rows of one needs_review file. It prints
// counts, the hash and the source only, never document text, and reports
// only the fixed sentinel messages.
func runRZIApprove(ctx context.Context, args []string, a approver, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: airbg rzi-approve <sha256>")
		return 2
	}
	res, err := a.Approve(ctx, args[0])
	if err != nil {
		fmt.Fprintln(stderr, "rzi-approve refused:", approveMessage(err))
		if errors.Is(err, rzi.ErrBadSHA) {
			return 2
		}
		return 1
	}
	if res.AlreadyApproved {
		fmt.Fprintf(stdout, "already approved source=%s sha256=%s published=%d excluded=%d\n",
			res.Source, res.SHA256, res.Published, res.Excluded)
		return 0
	}
	fmt.Fprintf(stdout, "approved source=%s sha256=%s published=%d excluded=%d\n",
		res.Source, res.SHA256, res.Published, res.Excluded)
	return 0
}

// approveMessage maps an error to a fixed string. An unknown error is not
// echoed: it may wrap text from the database or the disk.
func approveMessage(err error) string {
	for _, known := range []error{
		rzi.ErrBadSHA, rzi.ErrNoReviewRecord, rzi.ErrSHAMismatch, rzi.ErrNotApprovable, rzi.ErrSuperseded, rzi.ErrReviewTampered, store.ErrRZIFileIsOK,
		store.ErrRZIUnknownFile, store.ErrRZINotReviewable, store.ErrRZINothingToPublish,
	} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return "internal error, see the log"
}
