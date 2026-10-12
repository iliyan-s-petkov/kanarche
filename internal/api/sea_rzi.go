package api

import (
	"cmp"
	"context"
	"slices"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/store"
	"kanarche.eu/internal/upstream/rzi"
)

// seaSourceEEA marks an EEA sample on a site that also has RZI samples.
const seaSourceEEA = "eea"

// seaEnv is what the sea cache needs besides the store: the EEA credit data and
// the configured RZI page per source id.
type seaEnv struct {
	meta  SeaSupplementMeta
	pages map[string]string
}

// seaPages maps each RZI source id to its configured results page.
func seaPages(c config.SeaRZI) map[string]string {
	return map[string]string{
		rzi.SourceVarna:   c.Varna.PageURL,
		rzi.SourceBurgas:  c.Burgas.PageURL,
		rzi.SourceDobrich: c.Dobrich.PageURL,
	}
}

// seaRZIInput is the RZI side of one encode.
type seaRZIInput struct {
	rows     []store.RZIRow
	lastGood map[string]store.RZIFile
	pages    map[string]string
	now      time.Time
}

// visible keeps the RZI rows the API may show, grouped by site and ascending
// by date. A row is hidden when the EEA holds its season for the site, or has a
// sample on the same date. Hidden rows stay in the store.
func (in seaRZIInput) visible(d store.BathingData) map[string][]store.RZIRow {
	type siteSeason struct {
		site   string
		season int
	}
	type siteDate struct {
		site string
		date time.Time
	}
	seasons := map[siteSeason]bool{}
	dates := map[siteDate]bool{}
	for _, c := range d.Classes {
		seasons[siteSeason{c.SiteID, c.Season}] = true
	}
	for _, s := range d.Samples {
		seasons[siteSeason{s.SiteID, s.Season}] = true
		dates[siteDate{s.SiteID, s.Date.UTC()}] = true
	}
	out := map[string][]store.RZIRow{}
	for _, r := range in.rows {
		if seasons[siteSeason{r.SiteID, r.Season}] || dates[siteDate{r.SiteID, r.Date.UTC()}] {
			continue
		}
		out[r.SiteID] = append(out[r.SiteID], r)
	}
	for _, rs := range out {
		slices.SortStableFunc(rs, func(a, b store.RZIRow) int { return a.Date.Compare(b.Date) })
	}
	return out
}

// latest is the newest of rs (ascending), nil off-season.
func (in seaRZIInput) latest(rs []store.RZIRow) *seaLatest {
	if len(rs) == 0 || !rzi.SeasonOpen(in.now) {
		return nil
	}
	r := rs[len(rs)-1]
	age := int(rzi.SofiaDate(in.now).Sub(r.Date.UTC()) / (24 * time.Hour))
	return &seaLatest{
		Date: r.Date.UTC().Format(time.DateOnly), Source: r.File.Source,
		EColi: r.EC, EColiBelow: r.ECBelow,
		Enterococci: r.IE, EnterococciBelow: r.IEBelow,
		Unit: r.Unit, Stale: age > seaStaleDays,
	}
}

// info names the source and page of the newest row. It is nil when the page is
// not configured, so a half-empty object is never served.
func (in seaRZIInput) info(rs []store.RZIRow) *seaRZIInfo {
	if len(rs) == 0 {
		return nil
	}
	source := rs[len(rs)-1].File.Source
	page := in.pages[source]
	if page == "" {
		return nil
	}
	fetched := time.Time{}
	if f, ok := in.lastGood[source]; ok {
		fetched = f.FetchedAt
	} else {
		for _, r := range rs {
			if r.File.Source == source && r.File.FetchedAt.After(fetched) {
				fetched = r.File.FetchedAt
			}
		}
	}
	return &seaRZIInfo{Source: source, PageURL: page, FetchedAt: fetched.UTC()}
}

// merge adds the RZI samples to the EEA ones, both newest first, and labels
// every sample with its source. It leaves the lists alone when rs is empty.
func (in seaRZIInput) merge(eea []seaSample, rs []store.RZIRow) []seaSample {
	if len(rs) == 0 {
		return eea
	}
	all := make([]seaSample, 0, len(eea)+len(rs))
	for _, s := range eea {
		s.Source = seaSourceEEA
		all = append(all, s)
	}
	for i := len(rs) - 1; i >= 0; i-- {
		r := rs[i]
		all = append(all, seaSample{
			Date: r.Date.UTC().Format(time.DateOnly), Season: r.Season,
			EColi: r.EC, EColiBelow: r.ECBelow,
			Enterococci: r.IE, EnterococciBelow: r.IEBelow,
			Source: r.File.Source,
		})
	}
	// ISO dates sort as text. Stable, so EEA rows keep their order.
	slices.SortStableFunc(all, func(a, b seaSample) int { return cmp.Compare(b.Date, a.Date) })
	return all
}

// loadRZI reads the RZI rows and the last good file of each source in them.
func loadRZI(ctx context.Context, src DataSource) ([]store.RZIRow, map[string]store.RZIFile, error) {
	rows, err := src.LoadRZI(ctx)
	if err != nil {
		return nil, nil, err
	}
	good := map[string]store.RZIFile{}
	asked := map[string]bool{}
	for _, r := range rows {
		if asked[r.File.Source] {
			continue
		}
		asked[r.File.Source] = true
		f, ok, err := src.RZILastGood(ctx, r.File.Source)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			good[r.File.Source] = f
		}
	}
	return rows, good, nil
}
