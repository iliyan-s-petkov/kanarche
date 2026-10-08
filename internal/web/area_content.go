package web

import (
	"html/template"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"kanarche.eu/internal/snapshot"
)

// staleAfter is how long past a snapshot's GeneratedAt a reader is told "this
// is the latest we have, nothing newer has arrived" instead of a plain
// present-tense reading — six poll cycles at the collector's five-minute
// cadence (§1 of the SEO6 plan). The boundary is exclusive: exactly 30
// minutes is not yet stale.
const staleAfter = 30 * time.Minute

// Crumb is one link in an area page's breadcrumb chain. URL is empty for the
// last crumb — the page's own subject, printed as text rather than a link to
// itself.
type Crumb struct {
	Name string
	URL  string
}

// AreaLinkBlock is one nav.area-links section: a heading and the areas it
// links to.
type AreaLinkBlock struct {
	Label string
	Items []AreaRow
}

// ParentChain returns meta's ancestors, immediate parent first and the root
// oblast last — "" ParentSlug (an oblast itself) yields an empty chain. Owned
// here, by name, so the SEO5 JSON-LD BreadcrumbList can call it once both
// branches have merged (see snapshot.AreaMeta.ParentSlug's doc comment) rather
// than recomputing the walk a second way.
func (rr *Renderer) ParentChain(meta snapshot.AreaMeta, snap *snapshot.Snapshot, lang string) []AreaRow {
	var chain []AreaRow
	cur := meta
	for cur.ParentSlug != "" {
		parent, ok := snap.KnownSlugs[cur.ParentSlug]
		if !ok {
			break
		}
		chain = append(chain, rr.rowFrom(parent, lang))
		cur = parent
	}
	return chain
}

// crumbLabel is an area's breadcrumb/link text: the oblast label form
// ("Област Пловдив") for an oblast, the plain name otherwise — a district
// crumb reads "Младост", not "Район Младост"; the subject-form prefix belongs
// to the sentence, not the link trail.
func (p PageData) crumbLabel(row AreaRow) string {
	if row.Kind == "oblast" {
		return p.oblastFormRow(row)
	}
	return row.Name
}

// AreaHeading is the H1 of an area page: the oblast label form for an oblast,
// the plain name otherwise.
func (p PageData) AreaHeading() string { return p.crumbLabel(*p.Area) }

// oblastFormRow resolves row's oblast label form the same way areaSEO's
// oblastForm does, without needing the Renderer: PageData already carries the
// catalogue.
func (p PageData) oblastFormRow(row AreaRow) string {
	if key := "seo.oblast.label." + row.Slug; p.cat.Has(p.Lang, key) {
		return p.cat.T(p.Lang, key)
	}
	return strings.ReplaceAll(p.T("seo.oblast.label"), "{name}", row.Name)
}

// Breadcrumbs assembles the full chain for one area page: the home crumb,
// then chain (as ParentChain returned it) reversed to root-first order, then
// the current area last with no link.
func (p PageData) Breadcrumbs(row AreaRow, chain []AreaRow) []Crumb {
	out := make([]Crumb, 0, len(chain)+2)
	out = append(out, Crumb{Name: p.T("area.breadcrumb.home"), URL: p.Path("/")})
	for i := len(chain) - 1; i >= 0; i-- {
		a := chain[i]
		out = append(out, Crumb{Name: p.crumbLabel(a), URL: p.Path("/area/" + a.Slug)})
	}
	out = append(out, Crumb{Name: p.crumbLabel(row)})
	return out
}

// areaSubject is the sentence-start form of an area's name: the oblast label
// for an oblast, the plain name for a city, "Район {name}"/"{name} district"
// for a Sofia district.
func (p PageData) areaSubject(row AreaRow) string {
	switch row.Kind {
	case "oblast":
		return p.oblastFormRow(row)
	case "city":
		return row.Name
	default:
		return strings.ReplaceAll(p.T("area.subject.district"), "{name}", row.Name)
	}
}

// sensorsText is the key/key_one plural pattern (§4): "1 сензор" versus
// "N сензора" — no plural engine, just two catalogue keys picked by count.
func (p PageData) sensorsText(n int) string {
	if n == 1 {
		return p.T("area.sensors_one")
	}
	return strings.ReplaceAll(p.T("area.sensors"), "{n}", strconv.Itoa(n))
}

// htmlEscape is template.HTMLEscapeString wrapped to return a string, for the
// manual {placeholder} substitution below — the sentence is built as text and
// wrapped in template.HTML only once, complete, so every piece dropped into
// it must be escaped by hand first.
func htmlEscape(s string) string {
	var b strings.Builder
	template.HTMLEscape(&b, []byte(s))
	return b.String()
}

// timeTag wraps display in a machine-readable <time>, its datetime attribute
// in UTC RFC 3339 — so a page left open after the snapshot refreshes still
// carries the moment its own sentence describes, rather than only the words.
func timeTag(t time.Time, display string) string {
	return `<time datetime="` + t.UTC().Format(time.RFC3339) + `">` + htmlEscape(display) + `</time>`
}

// sofiaLayout formats t in Europe/Sofia using the catalogue's time.layout — a
// Go layout string, so a third language brings its own date order without a
// code change.
func (p PageData) sofiaLayout(t time.Time) string {
	loc := p.sofiaLoc
	if loc == nil {
		loc = time.UTC
	}
	return t.In(loc).Format(p.T("time.layout"))
}

// clockOrYesterday is {min_at}/{max_at}: "at 13:20" when t falls on the same
// Sofia calendar date as ref (GeneratedAt), "yesterday at 21:40" otherwise.
func (p PageData) clockOrYesterday(t, ref time.Time) string {
	loc := p.sofiaLoc
	if loc == nil {
		loc = time.UTC
	}
	ts, rs := t.In(loc), ref.In(loc)
	clock := ts.Format("15:04")
	if ts.Year() == rs.Year() && ts.YearDay() == rs.YearDay() {
		return strings.ReplaceAll(p.T("time.clock"), "{clock}", clock)
	}
	return strings.ReplaceAll(p.T("time.yesterday"), "{clock}", clock)
}

// stale reports whether Now has drifted more than staleAfter past the
// snapshot's GeneratedAt — see staleAfter.
func (p PageData) stale() bool { return p.Now.Sub(p.GeneratedAt) > staleAfter }

// areaNowHTML renders the air-now sentence — the four-state table in §1 of
// the SEO6 plan, evaluated in this order: stale first (it overrides
// covered/few/none because a stale reading needs different wording however
// many sensors last reported), then covered-with-a-value, then no sensors at
// all, then everything else as "few". A covered area with no value for the
// page's default metric gets area.now_no_metric.
func (p PageData) areaNowHTML(row AreaRow) template.HTML {
	subject := htmlEscape(p.areaSubject(row))
	timeHTML := timeTag(p.GeneratedAt, p.sofiaLayout(p.GeneratedAt))

	var key string
	switch {
	case p.stale():
		if row.Covered && row.HasValue {
			key = "area.now_stale"
		} else {
			key = "area.now_none_stale"
		}
	case row.Covered && row.HasValue:
		key = "area.now"
	case row.Covered:
		key = "area.now_no_metric"
	case row.SensorCount == 0:
		key = "area.now_none"
	default:
		key = "area.now_few"
	}

	text := p.T(key)
	text = strings.ReplaceAll(text, "{subject}", subject)
	// bg's time.layout ends in "ч."; don't close the sentence with a second period.
	if strings.HasSuffix(p.sofiaLayout(p.GeneratedAt), ".") {
		text = strings.ReplaceAll(text, "{time}.", "{time}")
	}
	text = strings.ReplaceAll(text, "{time}", timeHTML)
	text = strings.ReplaceAll(text, "{sensors}", htmlEscape(p.sensorsText(row.SensorCount)))
	text = strings.ReplaceAll(text, "{metric}", htmlEscape(p.T("metric."+p.DefaultMetric)))
	if row.HasValue {
		text = strings.ReplaceAll(text, "{unit}", htmlEscape(p.T("unit."+p.DefaultMetric)))
		text = strings.ReplaceAll(text, "{value}", htmlEscape(formatValue(row.Value, p.Lang)))
		text = strings.ReplaceAll(text, "{band}", htmlEscape(p.T("band."+bandLabel(p.DefaultMetric, row.Value))))
	}
	return template.HTML(`<p class="area-now">` + text + `</p>`)
}

// areaDayHTML renders the 24h min/max sentence, or "" when the state does not
// call for one: stale, uncovered, no value, or no DayRange (either the area
// has under six hours of coverage-gated buckets, or buildDayRange never ran
// for it).
func (p PageData) areaDayHTML(row AreaRow) template.HTML {
	if p.stale() || !row.Covered || !row.HasValue || row.Day == nil {
		return ""
	}
	unit := htmlEscape(p.T("unit." + p.DefaultMetric))
	text := p.T("area.day")
	text = strings.NewReplacer(
		"{min}", htmlEscape(formatValue(row.Day.Min, p.Lang)),
		"{max}", htmlEscape(formatValue(row.Day.Max, p.Lang)),
		"{unit}", unit,
		"{min_at}", htmlEscape(p.clockOrYesterday(row.Day.MinAt, p.GeneratedAt)),
		"{max_at}", htmlEscape(p.clockOrYesterday(row.Day.MaxAt, p.GeneratedAt)),
	).Replace(text)
	return template.HTML(`<p class="area-day">` + text + `</p>`)
}

// haversineKM is the great-circle distance between two centroids, in
// kilometres — enough for ranking "nearest", not for anything a boundary
// query should answer instead.
func haversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKM = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLon := toRad(lat2-lat1), toRad(lon2-lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusKM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// nearestSameKind ranks every OTHER area of meta's own kind by centroid
// distance and returns the closest n. Ties broken by slug, so the order is
// deterministic across renders — a "4 nearest" list that reshuffled on every
// page load would be a list that isn't answering a question.
func (rr *Renderer) nearestSameKind(meta snapshot.AreaMeta, snap *snapshot.Snapshot, lang string, n int) []AreaRow {
	type candidate struct {
		meta snapshot.AreaMeta
		dist float64
	}
	candidates := make([]candidate, 0, len(snap.KnownSlugs))
	for slug, m := range snap.KnownSlugs {
		if slug == meta.Slug || m.Kind != meta.Kind {
			continue
		}
		d := haversineKM(meta.CentroidLat, meta.CentroidLon, m.CentroidLat, m.CentroidLon)
		candidates = append(candidates, candidate{m, d})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].dist != candidates[j].dist {
			return candidates[i].dist < candidates[j].dist
		}
		return candidates[i].meta.Slug < candidates[j].meta.Slug
	})
	if len(candidates) > n {
		candidates = candidates[:n]
	}
	out := make([]AreaRow, len(candidates))
	for i, c := range candidates {
		out[i] = rr.rowFrom(c.meta, lang)
	}
	return out
}

// childrenOf returns every area whose ParentSlug is meta.Slug, sorted by
// name — the inverse of ParentChain, built from KnownSlugs rather than a
// second query, per §0 item 1 of the SEO6 plan.
func (rr *Renderer) childrenOf(meta snapshot.AreaMeta, snap *snapshot.Snapshot, lang string) []AreaRow {
	var out []AreaRow
	for _, m := range snap.KnownSlugs {
		if m.ParentSlug == meta.Slug {
			out = append(out, rr.rowFrom(m, lang))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// nearestLabelKey names the "Nearest" block by kind — a district's nearest
// list is other districts, an oblast's is other oblasti, and the three read
// differently in both languages.
func nearestLabelKey(kind string) string {
	switch kind {
	case "oblast":
		return "area.nearest.oblast"
	case "city":
		return "area.nearest.city"
	default:
		return "area.nearest.neighbourhood"
	}
}

// nearestCount is how many same-kind areas the "Nearest" block lists (§1).
const nearestCount = 4

// buildAreaLinks fills in an area page's breadcrumb and the three
// nav.area-links blocks (Part of / Children / Nearest), per the per-kind
// table in §1 of the SEO6 plan:
//
//   - oblast: no parent block; children is the one областен град (omitted
//     for sofiyska-oblast, which contains no city); nearest is 4 oblasti.
//   - city: parent is its oblast; children is the 24 Sofia districts, city
//     "sofia" only; nearest is 4 cities.
//   - neighbourhood (a Sofia district): parent is the chain София, Област
//     София-град; no children; nearest is 4 districts.
func (rr *Renderer) buildAreaLinks(p *PageData, meta snapshot.AreaMeta, row AreaRow, snap *snapshot.Snapshot, lang string) {
	chain := rr.ParentChain(meta, snap, lang)
	p.AreaCrumbs = p.Breadcrumbs(row, chain)

	if len(chain) > 0 {
		p.AreaParentBlock = &AreaLinkBlock{Label: p.T("area.parent"), Items: chain}
	}

	if children := rr.childrenOf(meta, snap, lang); len(children) > 0 {
		label := p.T("area.children.oblast")
		if meta.Slug == "sofia" {
			label = p.T("area.children.districts")
		}
		p.AreaChildrenBlock = &AreaLinkBlock{Label: label, Items: children}
	}

	if nearest := rr.nearestSameKind(meta, snap, lang, nearestCount); len(nearest) > 0 {
		p.AreaNearestBlock = &AreaLinkBlock{Label: p.T(nearestLabelKey(meta.Kind)), Items: nearest}
	}
}

// DirGroup is one /areas directory entry: an oblast, its own label form, the
// cities inside it, and — for the group containing "sofia" only — the 24
// Sofia districts nested under that city (§3 of the SEO6 plan).
type DirGroup struct {
	Oblast    AreaRow
	Label     string
	Cities    []AreaRow
	Districts []AreaRow
}

// buildDirectory is the /areas directory: every oblast, its cities, and
// Sofia's districts nested under it — 28 + 27 + 24 = 79 slugs linked from
// server HTML, in name order (Go's byte order sorts the Bulgarian alphabet
// correctly, so no collation table is needed here).
func (rr *Renderer) buildDirectory(snap *snapshot.Snapshot, lang string) []DirGroup {
	oblasts := rr.areaRows(snap, lang, "oblast")
	sort.Slice(oblasts, func(i, j int) bool { return oblasts[i].Name < oblasts[j].Name })

	groups := make([]DirGroup, 0, len(oblasts))
	for _, ob := range oblasts {
		g := DirGroup{Oblast: ob, Label: PageData{Lang: lang, cat: rr.cat}.oblastFormRow(ob)}

		var cities []AreaRow
		for _, m := range snap.KnownSlugs {
			if m.Kind == "city" && m.ParentSlug == ob.Slug {
				cities = append(cities, rr.rowFrom(m, lang))
			}
		}
		sort.Slice(cities, func(i, j int) bool { return cities[i].Name < cities[j].Name })
		g.Cities = cities

		for _, c := range cities {
			if c.Slug != "sofia" {
				continue
			}
			var districts []AreaRow
			for _, m := range snap.KnownSlugs {
				if m.Kind == "neighbourhood" && m.ParentSlug == "sofia" {
					districts = append(districts, rr.rowFrom(m, lang))
				}
			}
			sort.Slice(districts, func(i, j int) bool { return districts[i].Name < districts[j].Name })
			g.Districts = districts
		}
		groups = append(groups, g)
	}
	return groups
}
