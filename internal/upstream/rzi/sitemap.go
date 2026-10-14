package rzi

import (
	"errors"
	"math"
	"strings"
	"unicode"

	"kanarche.eu/internal/store"
)

// MaxSiteDistanceMetres is how far a document coordinate may sit from the
// EEA coordinate of the mapped site.
const MaxSiteDistanceMetres = 200

// MaxNamedSiteDistanceMetres is the limit when the document point name
// matches the EEA site name.
const MaxNamedSiteDistanceMetres = 1000

var (
	ErrSource        = errors.New("rzi: unknown source")
	ErrPoint         = errors.New("rzi: bad point number")
	ErrCoord         = errors.New("rzi: bad coordinate")
	ErrUnknownSite   = errors.New("rzi: point maps to no EEA site")
	ErrAmbiguousSite = errors.New("rzi: point maps to more than one EEA site")
	ErrRegion        = errors.New("rzi: EEA site outside the source region")
	ErrSiteTooFar    = errors.New("rzi: point too far from EEA site")
)

// region is the EEA id prefix and point number prefix of one source.
type region struct {
	eeaPrefix   string
	pointPrefix string
}

var regions = map[string]region{
	SourceVarna:   {eeaPrefix: "BG33", pointPrefix: "03"},
	SourceDobrich: {eeaPrefix: "BG33", pointPrefix: "08"},
	SourceBurgas:  {eeaPrefix: "BG34", pointPrefix: "02"},
}

// SiteIndex finds EEA sites by the last five characters of their id.
type SiteIndex struct {
	bySuffix map[string][]store.BathingSite
}

// NewSiteIndex indexes sites by id suffix.
func NewSiteIndex(sites []store.BathingSite) SiteIndex {
	x := SiteIndex{bySuffix: map[string][]store.BathingSite{}}
	for _, s := range sites {
		if len(s.ID) <= 5 {
			continue
		}
		suf := s.ID[len(s.ID)-5:]
		x.bySuffix[suf] = append(x.bySuffix[suf], s)
	}
	return x
}

// Map returns the EEA id for point. The suffix must match exactly one site,
// that site must carry the source's region prefix, and the document
// coordinate must be within MaxSiteDistanceMetres of it, or within
// MaxNamedSiteDistanceMetres when name matches the site name.
func (x SiteIndex) Map(source, point, name string, lat, lon float64) (string, error) {
	reg, ok := regions[source]
	if !ok {
		return "", ErrSource
	}
	if !validPoint(point) || !strings.HasPrefix(point, reg.pointPrefix) {
		return "", ErrPoint
	}
	if !validCoord(lat, lon) {
		return "", ErrCoord
	}
	cands := x.bySuffix[point]
	switch {
	case len(cands) == 0:
		return "", ErrUnknownSite
	case len(cands) > 1:
		return "", ErrAmbiguousSite
	}
	s := cands[0]
	if !strings.HasPrefix(s.ID, reg.eeaPrefix) {
		return "", ErrRegion
	}
	limit := float64(MaxSiteDistanceMetres)
	if nameMatches(name, s.NameBG) {
		limit = MaxNamedSiteDistanceMetres
	}
	// Written as !(d <= max) so a NaN distance is refused too.
	if !(distanceMetres(lat, lon, s.Lat, s.Lon) <= limit) {
		return "", ErrSiteTooFar
	}
	return s.ID, nil
}

// nameMatches is true when doc has tokens and every one is a token of site.
func nameMatches(doc, site string) bool {
	d := nameTokens(doc)
	if len(d) == 0 {
		return false
	}
	have := map[string]bool{}
	for _, t := range nameTokens(site) {
		have[t] = true
	}
	for _, t := range d {
		if !have[t] {
			return false
		}
	}
	return true
}

// nameTokens drops control and format characters, case-folds, and splits on
// everything that is not a letter or digit.
func nameTokens(s string) []string {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return unicode.ToLower(unicode.ToUpper(r))
	}, s)
	return strings.FieldsFunc(clean, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// validPoint is ^[0-9]{5}$ over ASCII digits only.
func validPoint(p string) bool {
	if len(p) != 5 {
		return false
	}
	for i := 0; i < 5; i++ {
		if p[i] < '0' || p[i] > '9' {
			return false
		}
	}
	return true
}

// validCoord range-checks; NaN fails every comparison.
func validCoord(lat, lon float64) bool {
	return lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

// distanceMetres is the haversine distance on a 6371 km sphere.
func distanceMetres(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusM = 6371000.0
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLon := rad(lat2-lat1), rad(lon2-lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
