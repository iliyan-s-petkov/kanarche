package web

import (
	"html/template"
	"sort"
	"strings"

	"kanarche.eu/internal/snapshot"
)

// homeRankPerList is how many provinces each ranked list names.
const homeRankPerList = 3

// homeCityCount is how many city links the home block carries.
const homeCityCount = 10

// sofiaHubSlug is the Sofia city page, the hub its districts hang off.
const sofiaHubSlug = "sofiya"

// citiesBySensors returns up to limit cities, busiest first, ties by slug.
func (rr *Renderer) citiesBySensors(lang string, limit int) []FooterArea {
	snap := rr.holder.Load()
	if snap == nil {
		return nil
	}
	cities := make([]snapshot.AreaMeta, 0, len(snap.KnownSlugs))
	for _, meta := range snap.KnownSlugs {
		if meta.Kind == "city" {
			cities = append(cities, meta)
		}
	}
	sort.Slice(cities, func(i, j int) bool {
		if cities[i].SensorCount != cities[j].SensorCount {
			return cities[i].SensorCount > cities[j].SensorCount
		}
		return cities[i].Slug < cities[j].Slug
	})
	if len(cities) > limit {
		cities = cities[:limit]
	}
	out := make([]FooterArea, len(cities))
	for i, meta := range cities {
		out[i] = FooterArea{Slug: meta.Slug, Name: rr.rowFrom(meta, lang).Name}
	}
	return out
}

// hasSofiaHub reports whether the snapshot knows the Sofia city page.
func (rr *Renderer) hasSofiaHub() bool {
	snap := rr.holder.Load()
	if snap == nil {
		return false
	}
	meta, ok := snap.KnownSlugs[sofiaHubSlug]
	return ok && meta.Kind == "city"
}

// rankedLists splits the valued provinces (sorted highest first) into the most
// polluted and the cleanest, never naming one province in both.
func rankedLists(valued []AreaRow) (worst, best []AreaRow) {
	w := homeRankPerList
	if len(valued) < w {
		w = len(valued)
	}
	worst = valued[:w]
	rest := valued[w:]
	if len(rest) > homeRankPerList {
		rest = rest[len(rest)-homeRankPerList:]
	}
	// Cleanest first.
	for i := len(rest) - 1; i >= 0; i-- {
		best = append(best, rest[i])
	}
	return worst, best
}

// rankItem is one list entry: the province link, then its figure and band.
func (p PageData) rankItem(a AreaRow) string {
	link := `<a class="link" href="` + htmlEscape(p.Path("/area/"+a.Slug)) + `">` + htmlEscape(p.oblastFormRow(a)) + `</a>`
	return `<li>` + strings.NewReplacer(
		"{area}", link,
		"{metric}", htmlEscape(p.T("metric."+p.DefaultMetric)),
		"{value}", htmlEscape(formatValue(a.Value, p.Lang)),
		"{unit}", p.unitMarkup(p.DefaultMetric),
		"{band}", htmlEscape(p.T("band."+bandLabel(p.DefaultMetric, a.Value))),
	).Replace(p.T("home.rank.item")) + `</li>`
}

func (p PageData) rankList(heading string, rows []AreaRow) string {
	var b strings.Builder
	b.WriteString(`<div><h3>` + htmlEscape(heading) + `</h3><ol class="home-rank__list">`)
	for _, a := range rows {
		b.WriteString(p.rankItem(a))
	}
	b.WriteString(`</ol></div>`)
	return b.String()
}

// staleSentence is home.rank.stale with the snapshot time filled in.
func (p PageData) staleSentence() string {
	text := p.T("home.rank.stale")
	// bg's time layout ends in "ч."; don't close the sentence with a second period.
	if strings.HasSuffix(p.sofiaLayout(p.GeneratedAt), ".") {
		text = strings.ReplaceAll(text, "{time}.", "{time}")
	}
	return `<p class="t-body">` + strings.ReplaceAll(text, "{time}", timeTag(p.GeneratedAt, p.sofiaLayout(p.GeneratedAt))) + `</p>`
}

// HomeRankHTML is the crawlable ranking under the map: the three most polluted
// and three cleanest provinces by the default metric, linked to their pages.
func (p PageData) HomeRankHTML() template.HTML {
	if len(p.Areas) == 0 {
		return ""
	}
	// p.Areas arrives sorted highest first with the silent provinces last.
	valued := make([]AreaRow, 0, len(p.Areas))
	for _, a := range p.Areas {
		if a.HasValue {
			valued = append(valued, a)
		}
	}

	var b strings.Builder
	b.WriteString(`<section class="home-rank" aria-labelledby="home-rank-h"><h2 id="home-rank-h">` + htmlEscape(p.T("home.rank.heading")) + `</h2>`)
	if len(valued) == 0 {
		b.WriteString(`<p class="t-body">` + htmlEscape(p.T("home.rank.none")) + `</p>`)
		if p.stale() {
			b.WriteString(p.staleSentence())
		}
	} else {
		b.WriteString(`<p class="t-body">` + htmlEscape(strings.ReplaceAll(p.T("home.rank.intro"), "{metric}", p.T("metric."+p.DefaultMetric))) + `</p>`)
		if p.stale() {
			b.WriteString(p.staleSentence())
		}
		worst, best := rankedLists(valued)
		b.WriteString(`<div class="home-rank__cols">` + p.rankList(p.T("home.rank.worst"), worst))
		if len(best) > 0 {
			b.WriteString(p.rankList(p.T("home.rank.best"), best))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</section>`)
	return template.HTML(b.String())
}

// CityLinkText is the anchor text for a city link on the home page.
func (p PageData) CityLinkText(name string) string {
	return strings.ReplaceAll(p.T("home.cities.link"), "{name}", name)
}
