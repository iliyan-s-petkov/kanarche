// Package datahub turns the EEA Datahub bathing-water workbook into a
// country-only snapshot of annual classes. The workbook is untrusted input:
// every row is validated, counted by reject reason, and never coerced.
package datahub

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"kanarche.eu/internal/upstream/bathing"
	"kanarche.eu/internal/xlsx"
)

// First season with a BWD class in our data.
const MinSeason = 2007

var (
	ErrNoRows         = errors.New("datahub: no accepted rows for the country")
	ErrTooManyRejects = errors.New("datahub: too many rejected rows")
	ErrThinNewest     = errors.New("datahub: newest season has too few sites")
	ErrDuplicate      = errors.New("datahub: duplicate (site, season)")
	ErrNoSheet        = errors.New("datahub: workbook has no worksheet")
)

// Reason says why a row of the requested country was dropped.
type Reason string

const (
	ReasonID      Reason = "bad_id"
	ReasonSeason  Reason = "bad_season"
	ReasonQuality Reason = "bad_quality"
)

// Class is one annual class. Quality is the stored key, not the EEA label.
type Class struct {
	SiteID  string `json:"site_id"`
	Season  int    `json:"season"`
	Quality string `json:"quality"`
}

// Snapshot holds the accepted classes, sorted by site then season.
type Snapshot struct {
	Classes []Class
}

// Rejects counts what Parse dropped.
type Rejects struct {
	Country      int // rows of the requested country
	OtherCountry int // rows of any other country (expected, not an error)
	ByReason     map[Reason]int
}

// Rejected is the number of country rows dropped.
func (r Rejects) Rejected() int {
	n := 0
	for _, c := range r.ByReason {
		n += c
	}
	return n
}

// Options are the fail-closed thresholds.
type Options struct {
	MaxRejectRatio float64 // of the country's rows
	MinNewestSites int     // distinct sites in the newest season
}

var DefaultOptions = Options{MaxRejectRatio: 0.01, MinNewestSites: 50}

// required are the columns Parse reads. Lookup is by name.
var required = []string{"countryCode", "bathingWaterIdentifier", "season", "quality"}

// Parse validates the first worksheet with the default thresholds.
func Parse(book *xlsx.Book, country string, now time.Time) (Snapshot, Rejects, error) {
	return ParseWith(book, country, now, DefaultOptions)
}

// ParseWith is Parse with explicit thresholds. On any error the snapshot is empty.
func ParseWith(book *xlsx.Book, country string, now time.Time, opt Options) (Snapshot, Rejects, error) {
	rej := Rejects{ByReason: map[Reason]int{}}
	sheets := book.Sheets()
	if len(sheets) == 0 {
		return Snapshot{}, rej, ErrNoSheet
	}
	type key struct {
		id     string
		season int
	}
	seen := map[key]bool{}
	var classes []Class
	err := book.Rows(sheets[0], required, func(r xlsx.Row) error {
		if r["countryCode"] != country {
			rej.OtherCountry++
			return nil
		}
		rej.Country++
		id := r["bathingWaterIdentifier"]
		if !bathing.ValidSiteID(id, country) {
			rej.ByReason[ReasonID]++
			return nil
		}
		season, ok := parseSeason(r["season"], now)
		if !ok {
			rej.ByReason[ReasonSeason]++
			return nil
		}
		q, ok := bathing.QualityKey(r["quality"])
		if !ok {
			rej.ByReason[ReasonQuality]++
			return nil
		}
		k := key{id, season}
		if seen[k] {
			return fmt.Errorf("%w: %s %d", ErrDuplicate, id, season)
		}
		seen[k] = true
		classes = append(classes, Class{SiteID: id, Season: season, Quality: q})
		return nil
	})
	if err != nil {
		return Snapshot{}, rej, err
	}
	if len(classes) == 0 {
		return Snapshot{}, rej, ErrNoRows
	}
	if float64(rej.Rejected()) > opt.MaxRejectRatio*float64(rej.Country) {
		return Snapshot{}, rej, fmt.Errorf("%w: %d of %d", ErrTooManyRejects, rej.Rejected(), rej.Country)
	}
	newest := 0
	for _, c := range classes {
		newest = max(newest, c.Season)
	}
	sites := 0
	for _, c := range classes {
		if c.Season == newest {
			sites++
		}
	}
	if sites < opt.MinNewestSites {
		return Snapshot{}, rej, fmt.Errorf("%w: %d in %d, need %d", ErrThinNewest, sites, newest, opt.MinNewestSites)
	}
	sort.Slice(classes, func(i, j int) bool {
		if classes[i].SiteID != classes[j].SiteID {
			return classes[i].SiteID < classes[j].SiteID
		}
		return classes[i].Season < classes[j].Season
	})
	return Snapshot{Classes: classes}, rej, nil
}

// parseSeason accepts only a canonical integer (no sign, padding or spaces) in [MinSeason, now's year].
func parseSeason(s string, now time.Time) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || strconv.Itoa(n) != s || n < MinSeason || n > now.Year() {
		return 0, false
	}
	return n, true
}
