package snapshot

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"time"

	"airbg.org/internal/config"
	"airbg.org/internal/store"
)

// PollenLevels names the bands, indexed by how many configured bounds a value reaches.
var PollenLevels = []string{"none", "low", "moderate", "high", "very_high"}

const (
	pollenUnit           = "grains/m³"
	pollenAttributionURL = "https://open-meteo.com/"
)

// PollenView is one area's table, for the area page template.
type PollenView struct {
	FetchedAt time.Time
	Days      []string
	Species   []PollenSpeciesView
	// Summary is today's worst level; nil when today has no data.
	Summary *PollenSummary
}

type PollenSpeciesView struct {
	Name       string
	Thresholds []float64
	Days       []PollenDayView
}

// PollenDayView is one table cell. Has is false where the forecast has no value.
type PollenDayView struct {
	Date      string
	Has       bool
	Level     string
	LevelRank int
	Mean, Max float64
}

// PollenSummary is the chip: Species is empty when Level is "none".
type PollenSummary struct {
	Date    string
	Level   string
	Species string
}

// pollenLevel counts the bounds v reaches.
func pollenLevel(v float64, bounds []float64) string {
	return PollenLevels[pollenRank(v, bounds)]
}

func pollenRank(v float64, bounds []float64) int {
	n := 0
	for _, b := range bounds {
		if v >= b {
			n++
		}
	}
	return n
}

// pollenDays returns the local dates shown and the UTC span covering them.
func pollenDays(now time.Time, loc *time.Location, n int) ([]string, time.Time, time.Time) {
	l := now.In(loc)
	midnight := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
	days := make([]string, n)
	for i := range days {
		days[i] = midnight.AddDate(0, 0, i).Format("2006-01-02")
	}
	return days, midnight.UTC(), midnight.AddDate(0, 0, n).UTC()
}

// pollenViews groups the daily rows by area into the configured species order.
func pollenViews(rows []store.AreaPollenDay, cfg config.Pollen, days []string) map[string]*PollenView {
	type key struct{ slug, species, day string }
	byKey := make(map[key]store.AreaPollenDay, len(rows))
	slugs := map[string]bool{}
	for _, r := range rows {
		byKey[key{r.Slug, r.Species, r.Day}] = r
		slugs[r.Slug] = true
	}
	out := make(map[string]*PollenView, len(slugs))
	for slug := range slugs {
		v := &PollenView{Days: days}
		hasData := false
		for _, sp := range cfg.Species {
			sv := PollenSpeciesView{Name: sp.Name, Thresholds: sp.Levels, Days: make([]PollenDayView, len(days))}
			for i, d := range days {
				sv.Days[i].Date = d
				r, ok := byKey[key{slug, sp.Name, d}]
				if !ok {
					continue
				}
				hasData = true
				rank := pollenRank(r.Mean, sp.Levels)
				sv.Days[i] = PollenDayView{
					Date: d, Has: true, Level: PollenLevels[rank], LevelRank: rank,
					Mean: round1(r.Mean), Max: round1(r.Max),
				}
			}
			v.Species = append(v.Species, sv)
		}
		if !hasData {
			continue
		}
		v.Summary = pollenSummary(v)
		out[slug] = v
	}
	return out
}

// pollenSummary takes today's highest rank; ties keep the earlier species.
func pollenSummary(v *PollenView) *PollenSummary {
	best, species, seen := 0, "", false
	for _, sv := range v.Species {
		d := sv.Days[0]
		if !d.Has {
			continue
		}
		seen = true
		if d.LevelRank > best {
			best, species = d.LevelRank, sv.Name
		}
	}
	if !seen {
		return nil
	}
	return &PollenSummary{Date: v.Days[0], Level: PollenLevels[best], Species: species}
}

// pollenPayload is GET /api/v1/area/{slug}/pollen.
type pollenPayload struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Area        string              `json:"area"`
	FetchedAt   time.Time           `json:"fetched_at"`
	Domain      string              `json:"domain"`
	Forecast    bool                `json:"forecast"`
	Unit        string              `json:"unit"`
	Statistic   string              `json:"statistic"`
	Days        []string            `json:"days"`
	Species     []pollenSpeciesJSON `json:"species"`
	Summary     *pollenSummaryJSON  `json:"summary"`
	Attribution pollenAttribution   `json:"attribution"`
}

type pollenSpeciesJSON struct {
	Name       string          `json:"name"`
	Thresholds []float64       `json:"thresholds"`
	Days       []pollenDayJSON `json:"days"`
}

type pollenDayJSON struct {
	Date  string   `json:"date"`
	Level *string  `json:"level"`
	Mean  *float64 `json:"mean"`
	Max   *float64 `json:"max"`
}

type pollenSummaryJSON struct {
	Date    string  `json:"date"`
	Level   string  `json:"level"`
	Species *string `json:"species"`
}

type pollenAttribution struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

func (p pollenPayload) withoutGeneratedAt() any {
	p.GeneratedAt = time.Time{}
	return p
}

var _ canonicalisable = pollenPayload{}

// PollenAttributionText is the credit the API carries; the page has its own copy in i18n.
func PollenAttributionText(year int) string {
	return fmt.Sprintf("Contains modified Copernicus Atmosphere Monitoring Service information %d, via Open-Meteo.com (CC BY 4.0)", year)
}

func pollenPayloadFrom(now time.Time, slug string, fetchedAt time.Time, cfg config.Pollen, days []string, v *PollenView) pollenPayload {
	p := pollenPayload{
		GeneratedAt: now, Area: slug, FetchedAt: fetchedAt, Domain: cfg.Domain,
		Forecast: true, Unit: pollenUnit, Statistic: "daily_mean", Days: days,
		Attribution: pollenAttribution{Text: PollenAttributionText(fetchedAt.Year()), URL: pollenAttributionURL},
	}
	for _, sv := range v.Species {
		sj := pollenSpeciesJSON{Name: sv.Name, Thresholds: sv.Thresholds, Days: make([]pollenDayJSON, len(sv.Days))}
		for i, d := range sv.Days {
			sj.Days[i].Date = d.Date
			if d.Has {
				level, mean, peak := d.Level, d.Mean, d.Max
				sj.Days[i].Level, sj.Days[i].Mean, sj.Days[i].Max = &level, &mean, &peak
			}
		}
		p.Species = append(p.Species, sj)
	}
	if v.Summary != nil {
		s := &pollenSummaryJSON{Date: v.Summary.Date, Level: v.Summary.Level}
		if v.Summary.Species != "" {
			species := v.Summary.Species
			s.Species = &species
		}
		p.Summary = s
	}
	return p
}

// pollenState is what Build carries between cycles: the forecast changes twice
// a day, so it is re-read only when the run or the local date changes.
type pollenState struct {
	key    string
	bodies map[string]Body
	views  map[string]*PollenView
	// mapBody is GET /api/v1/pollen.
	mapBody Body
}

// buildPollen fills the pollen fields. A failure is logged and leaves them
// empty: an optional layer must not fail the build.
func buildPollen(ctx context.Context, s *store.Store, h *Holder, prev, snap *Snapshot, now time.Time) error {
	fetchedAt, ok, err := s.LatestPollenFetch(ctx)
	if err != nil {
		slog.Warn("snapshot: pollen unavailable", "error", err)
		return nil
	}
	if !ok {
		return nil
	}
	days, from, to := pollenDays(now, h.pollenZone, h.pollen.DaysShown)
	key := fetchedAt.Format(time.RFC3339Nano) + "/" + days[0]
	if prev != nil && prev.pollen.key == key {
		snap.pollen = prev.pollen
		return nil
	}
	rows, err := s.AreaPollenDaily(ctx, store.PollenDailyQuery{
		From: from, To: to, Zone: h.pollenZone.String(), MinHours: h.pollen.MinHours,
		ReachM: h.pollen.CellReachKm * 1000, Country: h.pollen.Country,
	})
	if err != nil {
		slog.Warn("snapshot: pollen aggregate failed", "error", err)
		return nil
	}
	views := pollenViews(rows, h.pollen, days)
	st := pollenState{key: key, bodies: make(map[string]Body, len(views)), views: make(map[string]*PollenView, len(views))}
	for slug, v := range views {
		if _, known := snap.KnownSlugs[slug]; !known {
			continue
		}
		v.FetchedAt = fetchedAt
		b, err := encode(pollenPayloadFrom(now, slug, fetchedAt, h.pollen, days, v))
		if err != nil {
			return fmt.Errorf("snapshot: encode pollen %s: %w", slug, err)
		}
		st.bodies[slug], st.views[slug] = b, v
	}
	if st.mapBody, err = encode(pollenMapPayloadFrom(now, fetchedAt, days[0], st.views, snap.KnownSlugs)); err != nil {
		return fmt.Errorf("snapshot: encode pollen map: %w", err)
	}
	snap.pollen = st
	return nil
}

// pollenMapPayload is GET /api/v1/pollen: today's worst level per province, for the map layer.
type pollenMapPayload struct {
	GeneratedAt time.Time         `json:"generated_at"`
	FetchedAt   time.Time         `json:"fetched_at"`
	Date        string            `json:"date"`
	Forecast    bool              `json:"forecast"`
	Levels      []string          `json:"levels"`
	Areas       []pollenMapArea   `json:"areas"`
	Attribution pollenAttribution `json:"attribution"`
}

type pollenMapArea struct {
	Slug    string  `json:"slug"`
	Level   string  `json:"level"`
	Species *string `json:"species"`
}

func (p pollenMapPayload) withoutGeneratedAt() any {
	p.GeneratedAt = time.Time{}
	return p
}

var _ canonicalisable = pollenMapPayload{}

// pollenMapPayloadFrom keeps the country-tier areas (the outlines /api/v1/boundaries
// draws) that have a summary for today, sorted by slug.
func pollenMapPayloadFrom(now, fetchedAt time.Time, today string, views map[string]*PollenView, known map[string]AreaMeta) pollenMapPayload {
	p := pollenMapPayload{
		GeneratedAt: now, FetchedAt: fetchedAt, Date: today, Forecast: true,
		Levels: PollenLevels, Areas: []pollenMapArea{},
		Attribution: pollenAttribution{Text: PollenAttributionText(fetchedAt.Year()), URL: pollenAttributionURL},
	}
	for slug, v := range views {
		if v.Summary == nil || !slices.Contains(countryKinds, known[slug].Kind) {
			continue
		}
		a := pollenMapArea{Slug: slug, Level: v.Summary.Level}
		if v.Summary.Species != "" {
			species := v.Summary.Species
			a.Species = &species
		}
		p.Areas = append(p.Areas, a)
	}
	sort.Slice(p.Areas, func(i, j int) bool { return p.Areas[i].Slug < p.Areas[j].Slug })
	return p
}

// PollenMapBody is the encoded per-province summary; false when there is no forecast.
func (s *Snapshot) PollenMapBody() (Body, bool) {
	return s.pollen.mapBody, s.pollen.mapBody.JSON != nil
}

// PollenBody is the encoded table for slug; false when there is none.
func (s *Snapshot) PollenBody(slug string) (Body, bool) {
	b, ok := s.pollen.bodies[slug]
	return b, ok
}

// Pollen is the table for slug, or nil.
func (s *Snapshot) Pollen(slug string) *PollenView {
	return s.pollen.views[slug]
}

// SetPollenForTesting installs one area's table and its encoded body, for
// handler and page tests outside this package.
func (s *Snapshot) SetPollenForTesting(slug string, v *PollenView, cfg config.Pollen, fetchedAt time.Time) error {
	b, err := encode(pollenPayloadFrom(fetchedAt, slug, fetchedAt, cfg, v.Days, v))
	if err != nil {
		return err
	}
	if s.pollen.bodies == nil {
		s.pollen = pollenState{bodies: map[string]Body{}, views: map[string]*PollenView{}}
	}
	s.pollen.bodies[slug], s.pollen.views[slug] = b, v
	s.pollen.mapBody, err = encode(pollenMapPayloadFrom(fetchedAt, fetchedAt, v.Days[0], s.pollen.views, s.KnownSlugs))
	return err
}

// PollenViewForTesting builds a view from daily rows, as Build does.
func PollenViewForTesting(rows []store.AreaPollenDay, cfg config.Pollen, days []string) map[string]*PollenView {
	return pollenViews(rows, cfg, days)
}

// HolderOption configures an optional layer at construction.
type HolderOption func(*Holder)

// WithPollen enables the pollen table. Days are cut in Europe/Sofia, like the pages.
func WithPollen(cfg config.Pollen) HolderOption {
	return func(h *Holder) {
		h.pollen = cfg
		loc, err := time.LoadLocation("Europe/Sofia")
		if err != nil {
			loc = time.UTC
		}
		h.pollenZone = loc
	}
}
