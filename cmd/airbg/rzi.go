package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

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
	f, err := rzi.New(cfg.Sea, rziFetchOptions(sink))
	if err != nil {
		return nil, fmt.Errorf("rzi fetcher: %w", err)
	}
	return rzi.NewCollector(cfg.Sea.RZI, sink, f), nil
}

// rziFetchOptions is what the host gives the fetcher. Settled keeps it from
// saving a document the app already accepted and pruned.
func rziFetchOptions(sink rzi.Sink) rzi.Options {
	return rzi.Options{Settled: rzi.SettledIn(sink)}
}

// approver is the part of the ingester the approve command needs.
type approver interface {
	Approve(ctx context.Context, sha string) (rzi.Approval, error)
}

// writeRZIRemarks prints one line per sample remark: date, site, text.
// Remarks are untrusted, so control characters are replaced with spaces.
func writeRZIRemarks(w io.Writer, rec rzi.File) {
	n := 0
	for _, b := range rec.Blocks {
		site := b.Name
		if site == "" {
			site = b.Point
		}
		for _, r := range b.Rows {
			if r.Remarks == "" {
				continue
			}
			n++
			fmt.Fprintf(w, "remark %s %s: %s\n", clean(r.Date), clean(site), clean(r.Remarks))
		}
	}
	if n == 0 {
		fmt.Fprintln(w, "remarks: none")
	}
}

// reviewer is the read-only part of the ingester.
type reviewer interface {
	Review(sha string) (rzi.File, error)
}

// runRZIReview prints the pending review record for one hash: source, row
// count and every sample remark. It writes nothing.
func runRZIReview(args []string, r reviewer, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: airbg rzi-review <sha256>")
		return 2
	}
	rec, err := r.Review(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "rzi-review refused:", approveMessage(err))
		if errors.Is(err, rzi.ErrBadSHA) {
			return 2
		}
		return 1
	}
	n := 0
	for _, b := range rec.Blocks {
		n += len(b.Rows)
	}
	fmt.Fprintf(stdout, "review source=%s sha256=%s rows=%d\n", clean(rec.Source), clean(rec.SHA256), n)
	writeRZIRemarks(stdout, rec)
	return 0
}

// clean replaces control characters with spaces. Record text is untrusted.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

// runRZIApprove publishes the passed rows of one needs_review file. It prints
// counts, the hash and the source only, never document text, and reports
// only the fixed sentinel messages. Read the remarks first with rzi-review.
func runRZIApprove(ctx context.Context, args []string, a approver, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: airbg rzi-approve <sha256>  (read the remarks first: airbg rzi-review <sha256>)")
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
