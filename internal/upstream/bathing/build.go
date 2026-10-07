package bathing

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"airbg.org/internal/store"
)

// Skipped counts rows Build dropped, for the import log line.
type Skipped struct {
	Retired int // sites with statusCode "retired"
	Invalid int // rows that failed validation or repeated a key
	Orphan  int // classes and samples for a site not kept
}

// Matches the bathing_site CHECK.
var siteIDPattern = regexp.MustCompile(`^[A-Z]{2}[A-Za-z0-9]{1,40}$`)

var zones = map[string]string{
	"coastalBathingWater": "coastal",
	"lakeBathingWater":    "lake",
}

// Keys by the label after "N - "; "Good or Sufficient" is the pre-2015 transitional class.
var qualities = map[string]string{
	"0 - Not classified":     "not_classified",
	"1 - Excellent":          "excellent",
	"2 - Good":               "good",
	"3 - Sufficient":         "sufficient",
	"3 - Good or Sufficient": "good_or_sufficient",
	"4 - Poor":               "poor",
}

const (
	maxNameRunes = 200
	// Well above any real count; a larger number is a unit or entry error.
	maxCFU    = 10_000_000
	minSeason = 1990
	maxSeason = 2100
)

// Build validates raw rows into a storable set. It errors only when no active
// site survives, so an empty answer cannot wipe the stored layer.
func Build(raw Raw, country string) (store.BathingData, Skipped, error) {
	var d store.BathingData
	var sk Skipped

	kept := map[string]bool{}
	for _, r := range raw.Sites {
		if r.Status == "retired" {
			sk.Retired++
			continue
		}
		s, ok := buildSite(r, country)
		if !ok || kept[s.ID] {
			sk.Invalid++
			continue
		}
		kept[s.ID] = true
		d.Sites = append(d.Sites, s)
	}
	if len(d.Sites) == 0 {
		return d, sk, errors.New("bathing: no active sites in the answer")
	}

	seasons := map[string]bool{}
	for _, r := range raw.Status {
		if !kept[r.SiteID] {
			sk.Orphan++
			continue
		}
		key := r.SiteID + "/" + strconv.Itoa(r.Season)
		q, ok := "", r.Quality != nil
		if ok {
			q, ok = qualities[strings.TrimSpace(*r.Quality)]
		}
		if !ok || !validSeason(r.Season) || seasons[key] {
			sk.Invalid++
			continue
		}
		seasons[key] = true
		d.Classes = append(d.Classes, store.BathingClass{SiteID: r.SiteID, Season: r.Season, Quality: q})
	}

	dates := map[string]bool{}
	for _, r := range raw.Samples {
		if !kept[r.SiteID] {
			sk.Orphan++
			continue
		}
		s, ok := buildSample(r)
		key := r.SiteID + "/" + r.Date
		if !ok || dates[key] {
			sk.Invalid++
			continue
		}
		dates[key] = true
		d.Samples = append(d.Samples, s)
	}
	return d, sk, nil
}

// QualityKey maps an EEA class label to the stored key. The label is trimmed first.
func QualityKey(label string) (string, bool) {
	q, ok := qualities[strings.TrimSpace(label)]
	return q, ok
}

// ValidSiteID reports whether id fits the bathing_site CHECK and carries the country prefix.
func ValidSiteID(id, country string) bool {
	return siteIDPattern.MatchString(id) && strings.HasPrefix(id, country)
}

func buildSite(r SiteRow, country string) (store.BathingSite, bool) {
	zone, ok := zones[r.Zone]
	if !ok || !siteIDPattern.MatchString(r.ID) || !strings.HasPrefix(r.ID, country) {
		return store.BathingSite{}, false
	}
	if r.Lat < -90 || r.Lat > 90 || r.Lon < -180 || r.Lon > 180 || (r.Lat == 0 && r.Lon == 0) {
		return store.BathingSite{}, false
	}
	bg, en := cleanName(r.NameBG), cleanName(r.NameEN)
	if bg == "" {
		bg = en
	}
	if en == "" {
		en = bg
	}
	if bg == "" {
		return store.BathingSite{}, false
	}
	return store.BathingSite{
		ID: r.ID, NameBG: bg, NameEN: en, Zone: zone, Lat: r.Lat, Lon: r.Lon,
		ProfileURL: cleanLink(r.Link),
	}, true
}

func cleanName(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxNameRunes {
		return ""
	}
	return s
}

// cleanLink keeps an absolute http(s) URL and drops anything else.
func cleanLink(s string) string {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

func buildSample(r SampleRow) (store.BathingSample, bool) {
	date, err := time.Parse(time.DateOnly, r.Date)
	if err != nil || !validSeason(r.Season) || r.EC == nil || r.IE == nil {
		return store.BathingSample{}, false
	}
	ec, ecOK := censored(r.ECStatus)
	ie, ieOK := censored(r.IEStatus)
	if !ecOK || !ieOK || !validCFU(*r.EC) || !validCFU(*r.IE) {
		return store.BathingSample{}, false
	}
	return store.BathingSample{
		SiteID: r.SiteID, Date: date, Season: r.Season,
		EC: *r.EC, ECBelowDetection: ec, IE: *r.IE, IEBelowDetection: ie,
		PreSeason: r.SampleStatus != nil && *r.SampleStatus == "preSeasonSample",
	}, true
}

// censored maps a value status to "below detection"; an unknown status is not guessed at.
func censored(status *string) (below, ok bool) {
	if status == nil {
		return false, true
	}
	switch *status {
	case "limitOfDetectionValue":
		return true, true
	case "confirmedValue":
		return false, true
	}
	return false, false
}

func validCFU(v int) bool    { return v >= 0 && v <= maxCFU }
func validSeason(s int) bool { return s >= minSeason && s <= maxSeason }
