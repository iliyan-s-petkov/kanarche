package rzi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"kanarche.eu/internal/store"
)

var (
	ErrBadSHA         = errors.New("rzi: sha256 must be 64 lowercase hex characters")
	ErrNoReviewRecord = errors.New("rzi: no record awaiting review for this sha256")
	ErrSHAMismatch    = errors.New("rzi: review record does not carry this sha256")
	ErrNotApprovable  = errors.New("rzi: file is not awaiting review")
	ErrSuperseded     = errors.New("rzi: a newer file is already published for this source")
)

// Approval is the outcome of an approval. Counts only, no document text.
type Approval struct {
	Source    string
	SHA256    string
	Published int
	Excluded  int
}

// Approve publishes the rows of a needs_review file that passed validation.
// It binds to this one hash: the record is read from review/<sha>.json and
// must carry that hash, and the stored row for that hash must be needs_review.
// Flagged rows stay out. A rejected file, an extractor error record, an ok
// file or a source with a newer ok file is never approved.
func (g *Ingester) Approve(ctx context.Context, sha string) (Approval, error) {
	if !validSHA256(sha) {
		return Approval{}, ErrBadSHA
	}
	name := sha + ".json"
	rec, err := g.readRecord(filepath.Join(g.cfg.OutboxDir, dirReview, name))
	if errors.Is(err, os.ErrNotExist) {
		return Approval{}, ErrNoReviewRecord
	}
	if err != nil {
		return Approval{}, fmt.Errorf("rzi review record: %s", refusal(err))
	}
	if rec.SHA256 != sha {
		return Approval{}, ErrSHAMismatch
	}
	if rec.Error != "" {
		return Approval{}, fmt.Errorf("%w: extractor rejected the document", ErrNotApprovable)
	}

	row, found, err := g.sink.RZIFileBySHA(ctx, sha)
	if err != nil {
		return Approval{}, fmt.Errorf("rzi file lookup: %w", err)
	}
	if !found {
		return Approval{}, store.ErrRZIUnknownFile
	}
	if row.Status != store.RZIStatusNeedsReview {
		return Approval{}, fmt.Errorf("%w: status %s", ErrNotApprovable, row.Status)
	}
	if row.Source != rec.Source {
		return Approval{}, ErrSHAMismatch
	}
	if good, ok, err := g.sink.RZILastGood(ctx, rec.Source); err != nil {
		return Approval{}, fmt.Errorf("rzi last good: %w", err)
	} else if ok && good.FetchedAt.After(row.FetchedAt) {
		return Approval{}, ErrSuperseded
	}

	sites, err := g.sink.LoadBathingSites(ctx)
	if err != nil {
		return Approval{}, fmt.Errorf("rzi load sites: %w", err)
	}
	last, err := g.lastGood(ctx, rec.Source)
	if err != nil {
		return Approval{}, err
	}
	accepted, _, status := ValidateRows(rec, sites, last, g.clock())
	if status == StatusRejected {
		return Approval{}, fmt.Errorf("%w: structurally rejected", ErrNotApprovable)
	}
	if len(accepted) == 0 {
		return Approval{}, store.ErrRZINothingToPublish
	}
	excluded := rows(rec) - len(accepted)
	if err := g.sink.ApproveRZIFile(ctx, sha, rec.Source, toStore(accepted), g.clock(), excluded); err != nil {
		return Approval{}, err
	}
	slog.Info("rzi file approved", "source", rec.Source, "sha256", sha, "published", len(accepted), "excluded", excluded)
	// The approval is stored. A failed move only leaves a stale review copy,
	// which the next pass or an operator can clear.
	if err := os.MkdirAll(filepath.Join(g.cfg.OutboxDir, dirDone), 0o750); err == nil {
		err = os.Rename(filepath.Join(g.cfg.OutboxDir, dirReview, name), filepath.Join(g.cfg.OutboxDir, dirDone, name))
		if err != nil {
			slog.Warn("rzi review record not moved", "sha256", sha)
		}
	}
	return Approval{Source: rec.Source, SHA256: sha, Published: len(accepted), Excluded: excluded}, nil
}
