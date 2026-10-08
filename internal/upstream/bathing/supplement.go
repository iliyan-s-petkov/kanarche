package bathing

import (
	"log/slog"

	"kanarche.eu/internal/store"
)

// SupplementClass is one annual class from a second source.
type SupplementClass struct {
	SiteID  string
	Season  int
	Quality string // stored key, as in qualities
}

// Supplement is a validated snapshot that fills class gaps. A nil Supplement
// means no fill. The datahub package builds it; this package cannot import
// datahub because datahub imports this one.
type Supplement struct {
	Edition   string
	Published string
	URL       string
	Classes   []SupplementClass
}

// Guards are the log-only sanity thresholds, ratios in (0,1].
type Guards struct {
	MaxDisagree     float64 // of the keys both sources have
	MinSiteCoverage float64 // of the kept sites, in the snapshot's newest season
}

// SupplementStats counts what Merge did with the snapshot's classes.
type SupplementStats struct {
	Applied  int // added: Discodata had no class for the site and season
	Shadowed int // Discodata had the class, so Discodata's stays
	Disagree int // shadowed classes whose quality differs
	Inactive int // site not kept by Build (retired, invalid or unknown)
}

type seasonKey struct {
	site   string
	season int
}

// Merge adds the snapshot's classes to d where Discodata has none for that
// site and season. Discodata always wins, and only sites kept by Build are
// filled. Added classes carry store.SourceDatahub. d is not modified.
//
// The guards only log: a trip never stops the fill.
func Merge(d store.BathingData, sup *Supplement, g Guards) (store.BathingData, SupplementStats) {
	var st SupplementStats
	if sup == nil {
		return d, st
	}
	active := make(map[string]bool, len(d.Sites))
	for _, s := range d.Sites {
		active[s.ID] = true
	}
	have := make(map[seasonKey]string, len(d.Classes))
	for _, c := range d.Classes {
		have[seasonKey{c.SiteID, c.Season}] = c.Quality
	}

	out := d
	out.Classes = append([]store.BathingClass(nil), d.Classes...)
	out.SupplementEdition = sup.Edition
	newest := 0
	for _, c := range sup.Classes {
		newest = max(newest, c.Season)
	}
	covered := map[string]bool{}
	for _, c := range sup.Classes {
		if !active[c.SiteID] {
			st.Inactive++
			continue
		}
		if c.Season == newest {
			covered[c.SiteID] = true
		}
		if q, ok := have[seasonKey{c.SiteID, c.Season}]; ok {
			st.Shadowed++
			if q != c.Quality {
				st.Disagree++
			}
			continue
		}
		out.Classes = append(out.Classes, store.BathingClass{
			SiteID: c.SiteID, Season: c.Season, Quality: c.Quality, Source: store.SourceDatahub,
		})
		st.Applied++
	}
	checkGuards(st, len(d.Sites), len(covered), newest, g)
	return out, st
}

// checkGuards logs when the snapshot looks wrong. A column shift or a wrong
// file shows first as disagreement on seasons both sources have.
func checkGuards(st SupplementStats, sites, covered, newest int, g Guards) {
	if st.Shadowed > 0 {
		if r := float64(st.Disagree) / float64(st.Shadowed); r > g.MaxDisagree {
			slog.Error("sea supplement guard: disagreement above limit",
				"shared", st.Shadowed, "disagree", st.Disagree, "ratio", r, "max", g.MaxDisagree)
		}
	}
	if sites > 0 {
		if r := float64(covered) / float64(sites); r < g.MinSiteCoverage {
			slog.Error("sea supplement guard: coverage below limit",
				"sites", sites, "covered", covered, "newest_season", newest, "ratio", r, "min", g.MinSiteCoverage)
		}
	}
}
