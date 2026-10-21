package rzi

import (
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"kanarche.eu/internal/store"
)

// Status is the verdict for one outbox file, as stored in
// bathing_rzi_file.status.
type Status string

const (
	StatusOK          Status = "ok"
	StatusNeedsReview Status = "needs_review"
	StatusRejected    Status = "rejected"
)

// Caps and limits for one outbox file.
const (
	MaxBlocks        = 100
	MaxRowsPerBlock  = 60
	MaxRejects       = 100
	MaxBlockDrift    = 2
	MaxCount         = 1_000_000
	MaxRemarksRunes  = 500
	MaxNameRunes     = 200
	maxVersionBytes  = 64
	seasonStartMonth = time.May
	seasonEndMonth   = time.October
)

// Units accepted on a row. Extractors normalise "MPN/100 cm3" to MPN/100ml.
var units = map[string]bool{"MPN/100ml": true, "cfu/100ml": true}

// Sample is one accepted RZI row mapped to an EEA site. Remarks is stored and
// served by the API from the store row, not by marshalling a Sample (D5 reversed 2026-10-21).
type Sample struct {
	SiteID  string
	Date    time.Time
	Season  int
	EC      int
	ECBelow bool
	IE      int
	IEBelow bool
	Unit    string
	Remarks string `json:"-"`
}

// LastGood is what the last ok file of the same source left in the store.
// Blocks is 0 when there is none.
type LastGood struct {
	Blocks  int
	Samples []Sample
}

// Reject is one reason a file was not accepted. Block and Row are 0-based,
// -1 when not applicable. Point and Date are set only when well-formed, so
// no document text reaches logs.
type Reject struct {
	Block  int
	Row    int
	Point  string
	Date   string
	Reason string
}

type siteDate struct {
	site string
	date time.Time
}

// Validate checks a decoded outbox file against the EEA sites and the last
// good file. It is all or nothing: on any reject, accepted is nil and the
// status is not ok.
func Validate(f File, sites []store.BathingSite, last LastGood, now time.Time) ([]Sample, []Reject, Status) {
	accepted, rejects, status := ValidateRows(f, sites, last, now)
	if status != StatusOK {
		return nil, rejects, status
	}
	return accepted, nil, status
}

// ValidateRows is Validate without the all-or-nothing cut: it also returns
// the rows that passed when others did not. Only an operator approval may
// publish that partial set, never the ingest loop. A rejected file has none.
func ValidateRows(f File, sites []store.BathingSite, last LastGood, now time.Time) ([]Sample, []Reject, Status) {
	r := ValidateDetailed(f, sites, last, now)
	return r.Accepted, r.Rejects, r.Status
}

// Verdict is the full outcome of ValidateDetailed.
type Verdict struct {
	Accepted []Sample
	Rejects  []Reject
	Status   Status
	// Held lists the stored samples whose row in this file was flagged. An
	// approval keeps them as they are instead of dropping them.
	Held []Key
}

// Key is the identity of a sample: one site on one day.
type Key struct {
	SiteID string
	Date   time.Time
}

// ValidateDetailed is ValidateRows plus the stored samples that flagged
// rows point at.
func ValidateDetailed(f File, sites []store.BathingSite, last LastGood, now time.Time) Verdict {
	v := &validator{today: sofiaDate(now)}
	if f.Error != "" {
		v.reject(-1, -1, "", "", "extractor rejected the document")
		return Verdict{Rejects: v.rejects, Status: StatusRejected}
	}
	v.checkFile(f)
	if len(f.Blocks) > 0 && len(f.Blocks) <= MaxBlocks {
		v.checkBlocks(f, NewSiteIndex(sites), last)
	}
	if last.Blocks > 0 && abs(len(f.Blocks)-last.Blocks) > MaxBlockDrift {
		v.reject(-1, -1, "", "", "block count drifted from last good file")
	}
	if len(v.rejects) > 0 {
		return Verdict{Accepted: v.accepted, Rejects: v.rejects, Status: StatusNeedsReview, Held: v.held}
	}
	return Verdict{Accepted: v.accepted, Status: StatusOK}
}

type validator struct {
	today    time.Time
	season   int
	accepted []Sample
	rejects  []Reject
	held     []Key
}

// hold records the stored sample a flagged row points at, if there is one.
func (v *validator) hold(stored map[siteDate]Sample, siteID string, date time.Time) {
	if _, ok := stored[siteDate{siteID, date}]; ok {
		v.held = append(v.held, Key{SiteID: siteID, Date: date})
	}
}

func (v *validator) reject(block, row int, point, date, reason string) {
	if len(v.rejects) < MaxRejects {
		v.rejects = append(v.rejects, Reject{Block: block, Row: row, Point: point, Date: date, Reason: reason})
	}
}

func (v *validator) checkFile(f File) {
	if f.Schema != SchemaVersion {
		v.reject(-1, -1, "", "", "unsupported schema")
	}
	if !validSHA256(f.SHA256) {
		v.reject(-1, -1, "", "", "bad sha256")
	}
	if !validVersion(f.ExtractorVersion) {
		v.reject(-1, -1, "", "", "bad extractor version")
	}
	if len(f.Warnings) > 0 {
		v.reject(-1, -1, "", "", "extractor reported warnings")
	}
	switch {
	case len(f.Blocks) == 0:
		v.reject(-1, -1, "", "", "no blocks")
	case len(f.Blocks) > MaxBlocks:
		v.reject(-1, -1, "", "", "too many blocks")
	}
}

func (v *validator) checkBlocks(f File, idx SiteIndex, last LastGood) {
	stored := make(map[siteDate]Sample, len(last.Samples))
	for _, s := range last.Samples {
		stored[siteDate{s.SiteID, s.Date}] = s
	}
	points := map[string]bool{}
	seen := map[siteDate]bool{}
	for bi, b := range f.Blocks {
		point := ""
		if validPoint(b.Point) {
			point = b.Point
		}
		if !utf8.ValidString(b.Name) || utf8.RuneCountInString(b.Name) > MaxNameRunes {
			v.reject(bi, -1, point, "", "bad point name")
		}
		if point != "" && points[point] {
			v.reject(bi, -1, point, "", "duplicate point")
		}
		points[point] = true
		siteID, err := idx.Map(f.Source, b.Point, b.Name, b.Lat, b.Lon)
		if err != nil {
			v.reject(bi, -1, point, "", err.Error())
		}
		switch {
		case len(b.Rows) == 0:
			v.reject(bi, -1, point, "", "block without rows")
			continue
		case len(b.Rows) > MaxRowsPerBlock:
			v.reject(bi, -1, point, "", "too many rows in block")
			continue
		}
		for ri, r := range b.Rows {
			s, ok := v.checkRow(bi, ri, point, r)
			if !ok {
				if d, parsed := parseDate(r.Date); parsed && siteID != "" {
					v.hold(stored, siteID, d)
				}
				continue
			}
			if siteID == "" {
				continue
			}
			s.SiteID = siteID
			key := siteDate{siteID, s.Date}
			if seen[key] {
				v.reject(bi, ri, point, r.Date, "duplicate site and date")
				continue
			}
			seen[key] = true
			if old, ok := stored[key]; ok && !sameValues(old, s) {
				v.reject(bi, ri, point, r.Date, "value changed for a stored sample")
				v.hold(stored, siteID, s.Date)
				continue
			}
			v.accepted = append(v.accepted, s)
		}
	}
}

// checkRow validates one row. SiteID is filled in by the caller.
func (v *validator) checkRow(bi, ri int, point string, r Row) (Sample, bool) {
	bad := func(date, reason string) (Sample, bool) {
		v.reject(bi, ri, point, date, reason)
		return Sample{}, false
	}
	date, ok := parseDate(r.Date)
	if !ok {
		return bad("", "bad sample date")
	}
	if v.season == 0 {
		// The first good date fixes the file's season: this year or last.
		y := v.today.Year()
		if date.Year() != y && date.Year() != y-1 {
			return bad(r.Date, "season out of range")
		}
		v.season = date.Year()
	}
	if !inSeason(date, v.season) {
		return bad(r.Date, "sample date outside the bathing season")
	}
	if date.After(v.today) {
		return bad(r.Date, "sample date in the future")
	}
	if r.Planned != "" {
		p, ok := parseDate(r.Planned)
		if !ok || !inSeason(p, v.season) {
			return bad(r.Date, "bad planned date")
		}
	}
	ec, ecBelow, ok := checkCount(r.EC)
	if !ok {
		return bad(r.Date, "bad E. coli value")
	}
	ie, ieBelow, ok := checkCount(r.IE)
	if !ok {
		return bad(r.Date, "bad enterococci value")
	}
	if !units[r.Unit] {
		return bad(r.Date, "unit not allowed")
	}
	if !utf8.ValidString(r.Remarks) || utf8.RuneCountInString(r.Remarks) > MaxRemarksRunes {
		return bad(r.Date, "bad remarks")
	}
	return Sample{
		Date: date, Season: v.season,
		EC: ec, ECBelow: ecBelow,
		IE: ie, IEBelow: ieBelow,
		Unit: r.Unit, Remarks: stripControl(r.Remarks),
	}, true
}

// checkCount allows op "=" with 0..MaxCount and op "<" with 1..MaxCount.
func checkCount(c Count) (int, bool, bool) {
	if c.Value < 0 || c.Value > MaxCount {
		return 0, false, false
	}
	switch c.Op {
	case "=":
		return int(c.Value), false, true
	case "<":
		if c.Value < 1 {
			return 0, false, false
		}
		return int(c.Value), true, true
	}
	return 0, false, false
}

func sameValues(a, b Sample) bool {
	return a.EC == b.EC && a.ECBelow == b.ECBelow &&
		a.IE == b.IE && a.IEBelow == b.IEBelow && a.Unit == b.Unit
}

// parseDate accepts exactly YYYY-MM-DD and returns UTC midnight.
func parseDate(s string) (time.Time, bool) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// inSeason is 1 May..31 October of year.
func inSeason(d time.Time, year int) bool {
	first := time.Date(year, seasonStartMonth, 1, 0, 0, 0, 0, time.UTC)
	lastDay := time.Date(year, seasonEndMonth+1, 0, 0, 0, 0, 0, time.UTC)
	return !d.Before(first) && !d.After(lastDay)
}

var sofia = sync.OnceValue(func() *time.Location {
	if loc, err := time.LoadLocation("Europe/Sofia"); err == nil {
		return loc
	}
	// Summer offset; the season is in summer.
	return time.FixedZone("EEST", 3*3600)
})

// sofiaDate is now's calendar date in Sofia, as UTC midnight.
func sofiaDate(now time.Time) time.Time {
	y, m, d := now.In(sofia()).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// stripControl drops control and format characters (including bidi
// overrides and zero-width spaces).
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}

func validSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// validVersion is 1..64 bytes of printable ASCII without spaces.
func validVersion(s string) bool {
	if s == "" || len(s) > maxVersionBytes {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
