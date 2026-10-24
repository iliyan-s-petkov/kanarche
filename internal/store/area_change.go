package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// AreaChange is when an area's published figures last changed, keyed by a
// fingerprint of what the page shows. It lives in the database so the time
// survives a restart or deploy.
type AreaChange struct {
	Slug        string
	Fingerprint string
	ChangedAt   time.Time
}

// AreaChanges returns every stored stamp by slug.
func (s *Store) AreaChanges(ctx context.Context) (map[string]AreaChange, error) {
	rows, err := s.pool.Query(ctx, `SELECT slug, fingerprint, changed_at FROM area_change`)
	if err != nil {
		return nil, fmt.Errorf("store: area changes: %w", err)
	}
	defer rows.Close()
	out := map[string]AreaChange{}
	for rows.Next() {
		var c AreaChange
		if err := rows.Scan(&c.Slug, &c.Fingerprint, &c.ChangedAt); err != nil {
			return nil, fmt.Errorf("store: scan area change: %w", err)
		}
		out[c.Slug] = c
	}
	return out, rows.Err()
}

// SaveAreaChanges upserts the given stamps in one batch.
func (s *Store) SaveAreaChanges(ctx context.Context, changes []AreaChange) error {
	if len(changes) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, c := range changes {
		batch.Queue(`INSERT INTO area_change (slug, fingerprint, changed_at) VALUES ($1, $2, $3)
			ON CONFLICT (slug) DO UPDATE SET fingerprint = EXCLUDED.fingerprint, changed_at = EXCLUDED.changed_at`,
			c.Slug, c.Fingerprint, c.ChangedAt)
	}
	if err := s.pool.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("store: save area changes: %w", err)
	}
	return nil
}
