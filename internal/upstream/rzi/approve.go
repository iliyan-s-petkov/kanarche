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
	ErrSuperseded     = store.ErrRZISuperseded
	ErrReviewTampered = errors.New("rzi: review record does not match the record that was reviewed")
)

// Approval is the outcome of an approval. Counts only, no document text.
type Approval struct {
	Source    string
	SHA256    string
	Published int
	Excluded  int
	// AlreadyApproved is set when the file was approved before and only the
	// move of its review copy was finished.
	AlreadyApproved bool
}

// Review returns the pending review record for sha, so the operator can read
// it before approving. It reads the same file Approve binds to.
func (g *Ingester) Review(sha string) (File, error) {
	if !validSHA256(sha) {
		return File{}, ErrBadSHA
	}
	rec, _, err := g.readRecord(filepath.Join(g.cfg.OutboxDir, dirReview, sha+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return File{}, ErrNoReviewRecord
	}
	if err != nil {
		return File{}, fmt.Errorf("rzi review record: %s", refusal(err))
	}
	if rec.SHA256 != sha {
		return File{}, ErrSHAMismatch
	}
	return rec, nil
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
	reviewPath := filepath.Join(g.cfg.OutboxDir, dirReview, name)
	rec, recSum, err := g.readRecord(reviewPath)
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
	if row.Source != rec.Source {
		return Approval{}, ErrSHAMismatch
	}
	if row.Status == store.RZIStatusOK && row.ApprovedAt != nil {
		// Approved before. Only the move of the review copy can be left.
		if row.RecordSHA256 != recSum {
			return Approval{}, ErrReviewTampered
		}
		g.finishMove(sha)
		slog.Info("rzi file already approved", "source", rec.Source, "sha256", sha)
		return Approval{Source: rec.Source, SHA256: sha, Published: row.RowsOK, Excluded: row.ExcludedRows, AlreadyApproved: true}, nil
	}
	if row.Status != store.RZIStatusNeedsReview {
		return Approval{}, fmt.Errorf("%w: status %s", ErrNotApprovable, row.Status)
	}
	// The verdict was made on the record hashed at ingest. A review copy
	// that changed since then is not what was reviewed.
	if row.RecordSHA256 != recSum {
		return Approval{}, ErrReviewTampered
	}

	sites, err := g.sink.LoadBathingSites(ctx)
	if err != nil {
		return Approval{}, fmt.Errorf("rzi load sites: %w", err)
	}
	last, err := g.lastGood(ctx, rec.Source)
	if err != nil {
		return Approval{}, err
	}
	v := ValidateDetailed(rec, sites, last, g.clock())
	accepted, status := v.Accepted, v.Status
	if status == StatusRejected {
		return Approval{}, fmt.Errorf("%w: structurally rejected", ErrNotApprovable)
	}
	if len(accepted) == 0 {
		return Approval{}, store.ErrRZINothingToPublish
	}
	excluded := rows(rec) - len(accepted)
	if err := g.sink.ApproveRZIFile(ctx, sha, rec.Source, toStore(accepted), toKeys(v.Held), g.clock(), excluded); err != nil {
		return Approval{}, err
	}
	slog.Info("rzi file approved", "source", rec.Source, "sha256", sha, "published", len(accepted), "excluded", excluded)
	g.finishMove(sha)
	return Approval{Source: rec.Source, SHA256: sha, Published: len(accepted), Excluded: excluded}, nil
}

// finishMove puts the review copy into done/. The approval is already stored,
// so a failed move only leaves a stale copy, which approving again clears.
func (g *Ingester) finishMove(sha string) {
	name := sha + ".json"
	if err := os.MkdirAll(filepath.Join(g.cfg.OutboxDir, dirDone), 0o750); err != nil {
		slog.Warn("rzi review record not moved", "sha256", sha)
		return
	}
	err := os.Rename(filepath.Join(g.cfg.OutboxDir, dirReview, name), filepath.Join(g.cfg.OutboxDir, dirDone, name))
	if err != nil {
		slog.Warn("rzi review record not moved", "sha256", sha)
	}
}

func toKeys(ks []Key) []store.RZIKey {
	out := make([]store.RZIKey, len(ks))
	for i, k := range ks {
		out[i] = store.RZIKey{SiteID: k.SiteID, Date: k.Date}
	}
	return out
}
