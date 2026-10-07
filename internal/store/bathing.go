package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// BathingSite is one active EEA bathing water. Zone is "coastal" or "lake".
type BathingSite struct {
	ID         string
	NameBG     string
	NameEN     string
	Zone       string
	Lat        float64
	Lon        float64
	ProfileURL string
}

// Where a class came from. An empty Source is stored as SourceDiscodata.
const (
	SourceDiscodata = "discodata"
	SourceDatahub   = "datahub"
)

// BathingClass is one season's annual EU classification; Quality is a key such
// as "excellent". Source is SourceDiscodata or SourceDatahub.
type BathingClass struct {
	SiteID  string
	Season  int
	Quality string
	Source  string
}

// BathingSample is one lab result in cfu/100 ml; *BelowDetection means "< value".
type BathingSample struct {
	SiteID           string
	Date             time.Time
	Season           int
	EC               int
	ECBelowDetection bool
	IE               int
	IEBelowDetection bool
	PreSeason        bool
}

// BathingData is a whole import, as written and as read back.
type BathingData struct {
	Sites   []BathingSite
	Classes []BathingClass
	Samples []BathingSample
	// SupplementEdition is the snapshot edition this import used, empty if none.
	SupplementEdition string
}

// ReplaceBathing swaps the stored set for d in one transaction and records the import at at.
func (s *Store) ReplaceBathing(ctx context.Context, d BathingData, at time.Time) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Children cascade from bathing_site.
		if _, err := tx.Exec(ctx, `DELETE FROM bathing_site`); err != nil {
			return err
		}
		b := &pgx.Batch{}
		for _, x := range d.Sites {
			b.Queue(`INSERT INTO bathing_site (site_id, name_bg, name_en, zone, lat, lon, profile_url)
			         VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				x.ID, x.NameBG, x.NameEN, x.Zone, x.Lat, x.Lon, x.ProfileURL)
		}
		for _, x := range d.Classes {
			src := x.Source
			if src == "" {
				src = SourceDiscodata
			}
			b.Queue(`INSERT INTO bathing_class (site_id, season, quality, source) VALUES ($1, $2, $3, $4)`,
				x.SiteID, x.Season, x.Quality, src)
		}
		for _, x := range d.Samples {
			b.Queue(`INSERT INTO bathing_sample (site_id, sample_date, season, ec_value, ec_below_detection,
			                                     ie_value, ie_below_detection, pre_season)
			         VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				x.SiteID, x.Date, x.Season, x.EC, x.ECBelowDetection, x.IE, x.IEBelowDetection, x.PreSeason)
		}
		b.Queue(`INSERT INTO bathing_import (imported_at, sites, classes, samples, supplement_edition)
		         VALUES ($1, $2, $3, $4, NULLIF($5, ''))`,
			at.UTC(), len(d.Sites), len(d.Classes), len(d.Samples), d.SupplementEdition)
		if err := tx.SendBatch(ctx, b).Close(); err != nil {
			return fmt.Errorf("replace bathing: %w", err)
		}
		return nil
	})
}

// BathingLastImport returns the newest successful import time, false if none.
func (s *Store) BathingLastImport(ctx context.Context) (time.Time, bool, error) {
	var at *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT max(imported_at) FROM bathing_import`).Scan(&at); err != nil {
		return time.Time{}, false, err
	}
	if at == nil {
		return time.Time{}, false, nil
	}
	return at.UTC(), true, nil
}

// BathingSupplementEdition returns the snapshot edition of the newest import,
// empty if there is no import or it used none.
func (s *Store) BathingSupplementEdition(ctx context.Context) (string, error) {
	var ed *string
	err := s.pool.QueryRow(ctx,
		`SELECT supplement_edition FROM bathing_import ORDER BY imported_at DESC LIMIT 1`).Scan(&ed)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil || ed == nil {
		return "", err
	}
	return *ed, nil
}

// LoadBathing reads the whole stored set: sites by id, classes by site and
// season, samples by site and date.
func (s *Store) LoadBathing(ctx context.Context) (BathingData, error) {
	var d BathingData
	rows, err := s.pool.Query(ctx,
		`SELECT site_id, name_bg, name_en, zone, lat, lon, profile_url FROM bathing_site ORDER BY site_id`)
	if err != nil {
		return d, err
	}
	d.Sites, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (BathingSite, error) {
		var x BathingSite
		err := r.Scan(&x.ID, &x.NameBG, &x.NameEN, &x.Zone, &x.Lat, &x.Lon, &x.ProfileURL)
		return x, err
	})
	if err != nil {
		return d, err
	}

	rows, err = s.pool.Query(ctx, `SELECT site_id, season, quality, source FROM bathing_class ORDER BY site_id, season`)
	if err != nil {
		return d, err
	}
	d.Classes, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (BathingClass, error) {
		var x BathingClass
		err := r.Scan(&x.SiteID, &x.Season, &x.Quality, &x.Source)
		return x, err
	})
	if err != nil {
		return d, err
	}

	rows, err = s.pool.Query(ctx,
		`SELECT site_id, sample_date, season, ec_value, ec_below_detection, ie_value, ie_below_detection, pre_season
		 FROM bathing_sample ORDER BY site_id, sample_date`)
	if err != nil {
		return d, err
	}
	d.Samples, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (BathingSample, error) {
		var x BathingSample
		err := r.Scan(&x.SiteID, &x.Date, &x.Season, &x.EC, &x.ECBelowDetection, &x.IE, &x.IEBelowDetection, &x.PreSeason)
		x.Date = x.Date.UTC()
		return x, err
	})
	return d, err
}
