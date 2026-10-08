package datahub

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"kanarche.eu/internal/upstream/bathing"
)

//go:embed bg.json
var embedded []byte

var ErrSnapshot = errors.New("datahub: invalid snapshot")

// Pins are the values the snapshot header must match, taken from config.
type Pins struct {
	SHA256  string
	Size    int64
	Country string
}

// Load decodes and validates the embedded snapshot.
func Load(p Pins, now time.Time) (File, error) { return Decode(embedded, p, now) }

// Decode parses raw strictly and checks it against the pins. Any problem is an
// error and the caller must not use the file.
func Decode(raw []byte, p Pins, now time.Time) (File, error) {
	var f File
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return File{}, fmt.Errorf("%w: %v", ErrSnapshot, err)
	}
	// Nothing may follow the document.
	if _, err := dec.Token(); err != io.EOF {
		return File{}, fmt.Errorf("%w: trailing data", ErrSnapshot)
	}
	if err := f.validate(p, now); err != nil {
		return File{}, fmt.Errorf("%w: %v", ErrSnapshot, err)
	}
	return f, nil
}

func (f File) validate(p Pins, now time.Time) error {
	h := f.Header
	if h.SHA256 != p.SHA256 || h.Size != p.Size {
		return fmt.Errorf("header pins sha256 %s size %d, config pins %s size %d", h.SHA256, h.Size, p.SHA256, p.Size)
	}
	if h.Country != p.Country {
		return fmt.Errorf("header country %q, want %q", h.Country, p.Country)
	}
	if h.Edition == "" || h.Published == "" || h.Licence == "" || h.SourceURL == "" {
		return errors.New("header has an empty field")
	}
	if len(f.Classes) == 0 {
		return errors.New("no classes")
	}
	type key struct {
		id     string
		season int
	}
	seen := make(map[key]bool, len(f.Classes))
	for _, c := range f.Classes {
		if !bathing.ValidSiteID(c.SiteID, p.Country) {
			return fmt.Errorf("bad site id %q", c.SiteID)
		}
		if c.Season < MinSeason || c.Season > now.Year() {
			return fmt.Errorf("%s: season %d out of range", c.SiteID, c.Season)
		}
		if !bathing.ValidQualityKey(c.Quality) {
			return fmt.Errorf("%s %d: bad quality %q", c.SiteID, c.Season, c.Quality)
		}
		k := key{c.SiteID, c.Season}
		if seen[k] {
			return fmt.Errorf("duplicate %s %d", c.SiteID, c.Season)
		}
		seen[k] = true
	}
	return nil
}

// Supplement converts the file for the merge.
func (f File) Supplement() *bathing.Supplement {
	cs := make([]bathing.SupplementClass, len(f.Classes))
	for i, c := range f.Classes {
		cs[i] = bathing.SupplementClass{SiteID: c.SiteID, Season: c.Season, Quality: c.Quality}
	}
	return &bathing.Supplement{Edition: f.Header.Edition, Published: f.Header.Published, URL: f.Header.SourceURL, Classes: cs}
}
