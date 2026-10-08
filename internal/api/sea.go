package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"sync"
	"time"

	"airbg.org/internal/store"
	"airbg.org/internal/upstream/bathing"
)

// The data changes once a year and is imported weekly; an hour-old answer is never visibly stale.
const seaTTL = time.Hour

// Matches the bathing_site CHECK, so a bad id is a 400 without a lookup.
var seaSiteID = regexp.MustCompile(`^[A-Z]{2}[A-Za-z0-9]{1,40}$`)

type seaSite struct {
	ID      string  `json:"id"`
	NameBG  string  `json:"name_bg"`
	NameEN  string  `json:"name_en"`
	Zone    string  `json:"zone"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Season  *int    `json:"season"`
	Quality *string `json:"quality"`
	// Source is the dataset behind Season and Quality, null with no class.
	Source *string `json:"source"`
}

// SeaSupplementMeta is the snapshot header data the API echoes. The caller
// loads it once at startup, so no request touches the embedded file.
type SeaSupplementMeta struct {
	Published string
	URL       string
}

// seaSupplement names the Excel release behind the datahub classes. Edition
// comes from the store, the rest from the embedded snapshot header.
type seaSupplement struct {
	Edition   string `json:"edition"`
	Published string `json:"published"`
	URL       string `json:"url"`
}

type seaSitesBody struct {
	ImportedAt *time.Time                `json:"imported_at"`
	Limits     map[string]bathing.Limits `json:"limits"`
	Sites      []seaSite                 `json:"sites"`
	// Supplement is omitted unless a served class came from datahub.
	Supplement *seaSupplement `json:"supplement,omitempty"`
}

type seaSiteInfo struct {
	ID         string  `json:"id"`
	NameBG     string  `json:"name_bg"`
	NameEN     string  `json:"name_en"`
	Zone       string  `json:"zone"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	ProfileURL string  `json:"profile_url"`
}

type seaClass struct {
	Season  int    `json:"season"`
	Quality string `json:"quality"`
	Source  string `json:"source"`
}

type seaSample struct {
	Date             string `json:"date"`
	Season           int    `json:"season"`
	EColi            int    `json:"e_coli"`
	EColiBelow       bool   `json:"e_coli_below_detection"`
	Enterococci      int    `json:"enterococci"`
	EnterococciBelow bool   `json:"enterococci_below_detection"`
	PreSeason        bool   `json:"pre_season"`
}

type seaSiteBody struct {
	ImportedAt *time.Time     `json:"imported_at"`
	Site       seaSiteInfo    `json:"site"`
	Limits     bathing.Limits `json:"limits"`
	Classes    []seaClass     `json:"classes"`
	Samples    []seaSample    `json:"samples"`
	// Supplement is omitted unless a served class came from datahub.
	Supplement *seaSupplement `json:"supplement,omitempty"`
}

// seaBodies is one store load, encoded once: the list and every site's detail.
type seaBodies struct {
	list  []byte
	sites map[string][]byte
}

// seaCache holds encoded bodies for seaTTL. The lock spans the load so a burst makes one query.
type seaCache struct {
	mu      sync.Mutex
	bodies  seaBodies
	fetched time.Time
	ok      bool
}

func (c *seaCache) get(ctx context.Context, src DataSource, meta SeaSupplementMeta, now time.Time) (seaBodies, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ok && now.Sub(c.fetched) < seaTTL {
		return c.bodies, nil
	}
	d, err := src.LoadBathing(ctx)
	if err != nil {
		return seaBodies{}, err
	}
	at, has, err := src.BathingLastImport(ctx)
	if err != nil {
		return seaBodies{}, err
	}
	var importedAt *time.Time
	if has {
		importedAt = &at
	}
	var sup *seaSupplement
	if hasDatahub(d.Classes) {
		ed, err := src.BathingSupplementEdition(ctx)
		if err != nil {
			// The classes are still right; only the credit line is lost.
			slog.Error("sea supplement edition query failed", "error", err)
			ed = ""
		}
		sup = newSeaSupplement(ed, meta)
	}
	b, err := encodeSea(d, importedAt, sup)
	if err != nil {
		return seaBodies{}, err
	}
	c.bodies, c.fetched, c.ok = b, now, true
	return b, nil
}

// newSeaSupplement returns nil unless every field is set, so a bad embed or a
// missing edition never yields a half-empty object.
func newSeaSupplement(edition string, m SeaSupplementMeta) *seaSupplement {
	if edition == "" || m.Published == "" || m.URL == "" {
		return nil
	}
	return &seaSupplement{Edition: edition, Published: m.Published, URL: m.URL}
}

func hasDatahub(cs []store.BathingClass) bool {
	for _, c := range cs {
		if c.Source == store.SourceDatahub {
			return true
		}
	}
	return false
}

// encodeSea builds both shapes. Store order is site, then season or date
// ascending; the detail lists are reversed to newest first.
func encodeSea(d store.BathingData, importedAt *time.Time, sup *seaSupplement) (seaBodies, error) {
	classes := map[string][]seaClass{}
	for _, c := range d.Classes {
		classes[c.SiteID] = append([]seaClass{{Season: c.Season, Quality: c.Quality, Source: c.Source}}, classes[c.SiteID]...)
	}
	samples := map[string][]seaSample{}
	for _, s := range d.Samples {
		samples[s.SiteID] = append([]seaSample{{
			Date: s.Date.Format(time.DateOnly), Season: s.Season,
			EColi: s.EC, EColiBelow: s.ECBelowDetection,
			Enterococci: s.IE, EnterococciBelow: s.IEBelowDetection,
			PreSeason: s.PreSeason,
		}}, samples[s.SiteID]...)
	}

	list := seaSitesBody{ImportedAt: importedAt, Limits: bathing.LimitsByZone, Sites: make([]seaSite, 0, len(d.Sites))}
	out := seaBodies{sites: make(map[string][]byte, len(d.Sites))}
	for _, s := range d.Sites {
		row := seaSite{ID: s.ID, NameBG: s.NameBG, NameEN: s.NameEN, Zone: s.Zone, Lat: s.Lat, Lon: s.Lon}
		if cs := classes[s.ID]; len(cs) > 0 {
			row.Season, row.Quality, row.Source = &cs[0].Season, &cs[0].Quality, &cs[0].Source
		}
		list.Sites = append(list.Sites, row)
		if sup != nil && row.Source != nil && *row.Source == store.SourceDatahub {
			list.Supplement = sup
		}

		detail := seaSiteBody{
			ImportedAt: importedAt,
			Site: seaSiteInfo{ID: s.ID, NameBG: s.NameBG, NameEN: s.NameEN, Zone: s.Zone,
				Lat: s.Lat, Lon: s.Lon, ProfileURL: s.ProfileURL},
			Limits:  bathing.LimitsByZone[s.Zone],
			Classes: nonNil(classes[s.ID]),
			Samples: nonNil(samples[s.ID]),
		}
		// Only a site that shows a datahub class carries the credit.
		for _, c := range classes[s.ID] {
			if sup != nil && c.Source == store.SourceDatahub {
				detail.Supplement = sup
			}
		}
		enc, err := json.Marshal(detail)
		if err != nil {
			return seaBodies{}, err
		}
		out.sites[s.ID] = enc
	}
	enc, err := json.Marshal(list)
	if err != nil {
		return seaBodies{}, err
	}
	out.list = enc
	return out, nil
}

func nonNil[T seaClass | seaSample](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// handleSeaSites serves every bathing site with its latest annual class.
func (d Deps) handleSeaSites(w http.ResponseWriter, r *http.Request) {
	b, err := d.sea.get(r.Context(), d.Store, d.SeaSupplement, time.Now())
	if err != nil {
		slog.Error("sea sites query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "Internal server error.")
		return
	}
	writeSea(w, b.list)
}

// handleSeaSite serves one site's class history and samples, newest first.
func (d Deps) handleSeaSite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !seaSiteID.MatchString(id) {
		writeError(w, http.StatusBadRequest, "bad_request", "The site id is not valid.")
		return
	}
	b, err := d.sea.get(r.Context(), d.Store, d.SeaSupplement, time.Now())
	if err != nil {
		slog.Error("sea site query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "Internal server error.")
		return
	}
	body, ok := b.sites[id]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "No bathing site with that id.")
		return
	}
	writeSea(w, body)
}

// Public: open CC BY data, identical for every caller.
func writeSea(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	setCacheControl(w.Header(), cachePublic, int(seaTTL.Seconds()))
	_, _ = w.Write(body)
}
