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

var (
	ErrRZIUnknownFile      = errors.New("rzi file is unknown")
	ErrRZINotReviewable    = errors.New("rzi file is not awaiting review")
	ErrRZINothingToPublish = errors.New("rzi approval has no rows to publish")
	ErrRZISuperseded       = errors.New("rzi: a newer file is already published for this source")
	ErrRZIFileIsOK         = errors.New("rzi file is already ok and is not rewritten")
)

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
	// ApprovedAt is set when an operator approved a needs_review file.
	// ExcludedRows is how many rows that approval left out.
	ApprovedAt   *time.Time
	ExcludedRows int
	// RecordSHA256 is the hash of the outbox record the verdict came from.
	// Approval checks the review copy against it.
	RecordSHA256 string
}

// RZIKey names one stored sample.
type RZIKey struct {
	SiteID string
	Date   time.Time
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
                              extractor_version, status, rows_ok, rows_rejected, reason, record_sha256)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULLIF($13, ''))
ON CONFLICT (sha256) DO UPDATE SET
    url = EXCLUDED.url, etag = EXCLUDED.etag, last_modified = EXCLUDED.last_modified,
    fetched_at = EXCLUDED.fetched_at, extractor_version = EXCLUDED.extractor_version,
    status = EXCLUDED.status, rows_ok = EXCLUDED.rows_ok,
    rows_rejected = EXCLUDED.rows_rejected, reason = EXCLUDED.reason,
    record_sha256 = EXCLUDED.record_sha256
WHERE bathing_rzi_file.status <> 'ok' OR EXCLUDED.status = 'ok'
RETURNING id`

func upsertRZIFile(ctx context.Context, q pgx.Tx, f RZIFile) (int64, error) {
	var id int64
	err := q.QueryRow(ctx, rziFileUpsert, f.Source, f.URL, f.SHA256, f.Bytes, f.ETag, f.LastModified,
		f.FetchedAt.UTC(), f.ExtractorVersion, f.Status, f.RowsOK, f.RowsRejected, f.Reason, f.RecordSHA256).Scan(&id)
	// The guard in the upsert filters out a rewrite of an ok row.
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrRZIFileIsOK
	}
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
	if errors.Is(err, ErrRZIFileIsOK) {
		return 0, err
	}
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
		if err := lockRZISource(ctx, tx, source); err != nil {
			return err
		}
		// Delete before the file upsert so a same-hash file re-adopts its rows.
		if _, err := tx.Exec(ctx, `DELETE FROM bathing_rzi_sample
			WHERE file_id IN (SELECT id FROM bathing_rzi_file WHERE source = $1)`, source); err != nil {
			return fmt.Errorf("replace rzi: %w", err)
		}
		id, err := upsertRZIFile(ctx, tx, f)
		if errors.Is(err, ErrRZIFileIsOK) {
			return err
		}
		if err != nil {
			return fmt.Errorf("replace rzi: %w", err)
		}
		return insertRZISamples(ctx, tx, id, xs)
	})
}

// lockRZISource serializes the writers of one source until the transaction
// ends, so approve and replace cannot interleave their delete and insert.
func lockRZISource(ctx context.Context, tx pgx.Tx, source string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('rzi:' || $1::text))`, source); err != nil {
		return fmt.Errorf("lock rzi source: %w", err)
	}
	return nil
}

func insertRZISamples(ctx context.Context, tx pgx.Tx, fileID int64, xs []RZISample) error {
	b := &pgx.Batch{}
	for _, x := range xs {
		b.Queue(`INSERT INTO bathing_rzi_sample (site_id, sample_date, season, ec_value, ec_below,
		                                         ie_value, ie_below, unit, remarks, file_id)
		         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			x.SiteID, x.Date, x.Season, x.EC, x.ECBelow, x.IE, x.IEBelow, x.Unit, x.Remarks, fileID)
	}
	if err := tx.SendBatch(ctx, b).Close(); err != nil {
		return fmt.Errorf("write rzi samples: %w", err)
	}
	return nil
}

// LoadRZI reads every stored sample with its file, ordered by site then date.
func (s *Store) LoadRZI(ctx context.Context) ([]RZIRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT x.site_id, x.sample_date, x.season, x.ec_value, x.ec_below, x.ie_value, x.ie_below, x.unit, x.remarks,
		       f.id, f.source, f.url, f.sha256, f.bytes, f.etag, f.last_modified, f.fetched_at,
		       f.extractor_version, f.status, f.rows_ok, f.rows_rejected, f.reason, f.approved_at, f.excluded_rows,
		       COALESCE(f.record_sha256, '')
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
			&x.File.RowsOK, &x.File.RowsRejected, &x.File.Reason, &x.File.ApprovedAt, &x.File.ExcludedRows,
			&x.File.RecordSHA256)
		x.Date = x.Date.UTC()
		x.File.FetchedAt = x.File.FetchedAt.UTC()
		return x, err
	})
}

// LoadBathingSites reads the EEA sites only, ordered by id.
func (s *Store) LoadBathingSites(ctx context.Context) ([]BathingSite, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT site_id, name_bg, name_en, zone, lat, lon, profile_url FROM bathing_site ORDER BY site_id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (BathingSite, error) {
		var x BathingSite
		err := r.Scan(&x.ID, &x.NameBG, &x.NameEN, &x.Zone, &x.Lat, &x.Lon, &x.ProfileURL)
		return x, err
	})
}

// RZILastGood returns the newest status ok file of a source, false if none.
func (s *Store) RZILastGood(ctx context.Context, source string) (RZIFile, bool, error) {
	return s.queryRZIFile(ctx, `source = $1 AND status = 'ok' ORDER BY fetched_at DESC, id DESC LIMIT 1`, source)
}

// RZILatest returns the newest file of a source in any status, false if none.
func (s *Store) RZILatest(ctx context.Context, source string) (RZIFile, bool, error) {
	return s.queryRZIFile(ctx, `source = $1 ORDER BY fetched_at DESC, id DESC LIMIT 1`, source)
}

// RZIFileBySHA returns the file row with this document hash, false if none.
func (s *Store) RZIFileBySHA(ctx context.Context, sha string) (RZIFile, bool, error) {
	return s.queryRZIFile(ctx, `sha256 = $1`, sha)
}

// queryRZIFile runs a one-row query. where is a fixed clause, never input.
func (s *Store) queryRZIFile(ctx context.Context, where, arg string) (RZIFile, bool, error) {
	var f RZIFile
	err := s.pool.QueryRow(ctx, `
		SELECT id, source, url, sha256, bytes, etag, last_modified, fetched_at,
		       extractor_version, status, rows_ok, rows_rejected, reason, approved_at, excluded_rows,
		       COALESCE(record_sha256, '')
		FROM bathing_rzi_file WHERE `+where, arg).Scan(
		&f.ID, &f.Source, &f.URL, &f.SHA256, &f.Bytes, &f.ETag, &f.LastModified, &f.FetchedAt,
		&f.ExtractorVersion, &f.Status, &f.RowsOK, &f.RowsRejected, &f.Reason, &f.ApprovedAt, &f.ExcludedRows,
		&f.RecordSHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return RZIFile{}, false, nil
	}
	if err != nil {
		return RZIFile{}, false, err
	}
	f.FetchedAt = f.FetchedAt.UTC()
	return f, true, nil
}

// ApproveRZIFile publishes xs for a file that is awaiting review. The row
// must exist, belong to source and be needs_review, and no newer ok file may
// exist for the source, or nothing is written. It replaces the source's
// samples with xs, except the stored samples named in keep, which stay as
// they are unless xs has the same key. It sets the row ok and records the
// approval time and the number of rows left out. xs must not be empty.
func (s *Store) ApproveRZIFile(ctx context.Context, sha, source string, xs []RZISample, keep []RZIKey, at time.Time, excluded int) error {
	if len(xs) == 0 {
		return ErrRZINothingToPublish
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// The lock comes first: the checks below are only true while no other
		// writer of this source can commit.
		if err := lockRZISource(ctx, tx, source); err != nil {
			return err
		}
		var id int64
		var gotSource, status string
		var fetched time.Time
		err := tx.QueryRow(ctx, `SELECT id, source, status, fetched_at FROM bathing_rzi_file WHERE sha256 = $1 FOR UPDATE`, sha).
			Scan(&id, &gotSource, &status, &fetched)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRZIUnknownFile
		}
		if err != nil {
			return fmt.Errorf("approve rzi: %w", err)
		}
		if gotSource != source {
			return fmt.Errorf("approve rzi: file source %q, want %q", gotSource, source)
		}
		if status != RZIStatusNeedsReview {
			return ErrRZINotReviewable
		}
		var newer bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM bathing_rzi_file
			WHERE source = $1 AND status = 'ok' AND (fetched_at > $2 OR (fetched_at = $2 AND id > $3)))`,
			source, fetched, id).Scan(&newer); err != nil {
			return fmt.Errorf("approve rzi: %w", err)
		}
		if newer {
			return ErrRZISuperseded
		}

		inNew := make(map[RZIKey]bool, len(xs))
		for _, x := range xs {
			inNew[RZIKey{x.SiteID, x.Date.UTC()}] = true
		}
		var sites []string
		var dates []time.Time
		for _, k := range keep {
			if !inNew[RZIKey{k.SiteID, k.Date.UTC()}] {
				sites, dates = append(sites, k.SiteID), append(dates, k.Date.UTC())
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM bathing_rzi_sample
			WHERE file_id IN (SELECT id FROM bathing_rzi_file WHERE source = $1)
			  AND (site_id, sample_date) NOT IN (SELECT * FROM unnest($2::text[], $3::date[]))`,
			source, sites, dates); err != nil {
			return fmt.Errorf("approve rzi: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE bathing_rzi_file
			SET status = 'ok', rows_ok = $2, rows_rejected = $3, excluded_rows = $3,
			    approved_at = $4, reason = 'approved'
			WHERE id = $1`, id, len(xs), excluded, at.UTC()); err != nil {
			return fmt.Errorf("approve rzi: %w", err)
		}
		return insertRZISamples(ctx, tx, id, xs)
	})
}
