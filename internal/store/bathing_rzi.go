package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Sources and statuses of RZI files, as stored.
const (
	RZIStatusOK          = "ok"
	RZIStatusNeedsReview = "needs_review"
	RZIStatusRejected    = "rejected"
)

// ErrRZIFileNotOK is returned when samples are offered with a file that is not status ok.
var ErrRZIFileNotOK = errors.New("rzi file is not status ok")

// RZIFile is one fetched source document. SHA256 is its identity.
type RZIFile struct {
	ID               int64
	Source           string // rzi_varna, rzi_burgas or rzi_dobrich
	URL              string
	SHA256           string
	Bytes            int
	ETag             string
	LastModified     *time.Time
	FetchedAt        time.Time
	ExtractorVersion string
	Status           string
	RowsOK           int
	RowsRejected     int
	Reason           string
}

// RZISample is one validated lab result. Unit is "MPN/100ml" or "cfu/100ml".
type RZISample struct {
	SiteID  string
	Date    time.Time
	Season  int
	EC      int
	ECBelow bool
	IE      int
	IEBelow bool
	Unit    string
	Remarks string
}

// RZIRow is a stored sample with the file it came from.
type RZIRow struct {
	RZISample
	File RZIFile
}

const rziFileUpsert = `
INSERT INTO bathing_rzi_file (source, url, sha256, bytes, etag, last_modified, fetched_at,
                              extractor_version, status, rows_ok, rows_rejected, reason)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (sha256) DO UPDATE SET
    url = EXCLUDED.url, etag = EXCLUDED.etag, last_modified = EXCLUDED.last_modified,
    fetched_at = EXCLUDED.fetched_at, extractor_version = EXCLUDED.extractor_version,
    status = EXCLUDED.status, rows_ok = EXCLUDED.rows_ok,
    rows_rejected = EXCLUDED.rows_rejected, reason = EXCLUDED.reason
RETURNING id`

func upsertRZIFile(ctx context.Context, q pgx.Tx, f RZIFile) (int64, error) {
	var id int64
	err := q.QueryRow(ctx, rziFileUpsert, f.Source, f.URL, f.SHA256, f.Bytes, f.ETag, f.LastModified,
		f.FetchedAt.UTC(), f.ExtractorVersion, f.Status, f.RowsOK, f.RowsRejected, f.Reason).Scan(&id)
	return id, err
}

// RecordRZIFile stores a file's outcome without touching any samples. Use it
// for needs_review and rejected files. A file with the same hash is updated.
func (s *Store) RecordRZIFile(ctx context.Context, f RZIFile) (int64, error) {
	var id int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		id, err = upsertRZIFile(ctx, tx, f)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("record rzi file: %w", err)
	}
	return id, nil
}

// ReplaceRZISource swaps the samples of one source for xs in one transaction.
// Other sources and all EEA tables are untouched. f must be status ok and
// f.Source must equal source.
func (s *Store) ReplaceRZISource(ctx context.Context, source string, f RZIFile, xs []RZISample) error {
	if f.Source != source {
		return fmt.Errorf("replace rzi: file source %q, want %q", f.Source, source)
	}
	if f.Status != RZIStatusOK {
		return ErrRZIFileNotOK
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Delete before the file upsert so a same-hash file re-adopts its rows.
		if _, err := tx.Exec(ctx, `DELETE FROM bathing_rzi_sample
			WHERE file_id IN (SELECT id FROM bathing_rzi_file WHERE source = $1)`, source); err != nil {
			return fmt.Errorf("replace rzi: %w", err)
		}
		id, err := upsertRZIFile(ctx, tx, f)
		if err != nil {
			return fmt.Errorf("replace rzi: %w", err)
		}
		b := &pgx.Batch{}
		for _, x := range xs {
			b.Queue(`INSERT INTO bathing_rzi_sample (site_id, sample_date, season, ec_value, ec_below,
			                                         ie_value, ie_below, unit, remarks, file_id)
			         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				x.SiteID, x.Date, x.Season, x.EC, x.ECBelow, x.IE, x.IEBelow, x.Unit, x.Remarks, id)
		}
		if err := tx.SendBatch(ctx, b).Close(); err != nil {
			return fmt.Errorf("replace rzi: %w", err)
		}
		return nil
	})
}

// LoadRZI reads every stored sample with its file, ordered by site then date.
func (s *Store) LoadRZI(ctx context.Context) ([]RZIRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT x.site_id, x.sample_date, x.season, x.ec_value, x.ec_below, x.ie_value, x.ie_below, x.unit, x.remarks,
		       f.id, f.source, f.url, f.sha256, f.bytes, f.etag, f.last_modified, f.fetched_at,
		       f.extractor_version, f.status, f.rows_ok, f.rows_rejected, f.reason
		FROM bathing_rzi_sample x JOIN bathing_rzi_file f ON f.id = x.file_id
		ORDER BY x.site_id, x.sample_date`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (RZIRow, error) {
		var x RZIRow
		err := r.Scan(&x.SiteID, &x.Date, &x.Season, &x.EC, &x.ECBelow, &x.IE, &x.IEBelow, &x.Unit, &x.Remarks,
			&x.File.ID, &x.File.Source, &x.File.URL, &x.File.SHA256, &x.File.Bytes, &x.File.ETag,
			&x.File.LastModified, &x.File.FetchedAt, &x.File.ExtractorVersion, &x.File.Status,
			&x.File.RowsOK, &x.File.RowsRejected, &x.File.Reason)
		x.Date = x.Date.UTC()
		x.File.FetchedAt = x.File.FetchedAt.UTC()
		return x, err
	})
}

// RZILastGood returns the newest status ok file of a source, false if none.
func (s *Store) RZILastGood(ctx context.Context, source string) (RZIFile, bool, error) {
	var f RZIFile
	err := s.pool.QueryRow(ctx, `
		SELECT id, source, url, sha256, bytes, etag, last_modified, fetched_at,
		       extractor_version, status, rows_ok, rows_rejected, reason
		FROM bathing_rzi_file WHERE source = $1 AND status = 'ok'
		ORDER BY fetched_at DESC, id DESC LIMIT 1`, source).Scan(
		&f.ID, &f.Source, &f.URL, &f.SHA256, &f.Bytes, &f.ETag, &f.LastModified, &f.FetchedAt,
		&f.ExtractorVersion, &f.Status, &f.RowsOK, &f.RowsRejected, &f.Reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return RZIFile{}, false, nil
	}
	if err != nil {
		return RZIFile{}, false, err
	}
	f.FetchedAt = f.FetchedAt.UTC()
	return f, true, nil
}
