package web

import (
	"html/template"
	"sort"
	"strconv"
	"strings"

	"kanarche.eu/internal/snapshot"
)

// bigCitySlugs are the city pages the home block always names, in this order.
var bigCitySlugs = []string{"sofia", "plovdiv", "varna", "burgas", "ruse", "stara-zagora", "pleven"}

// bigCityDistricts is how many Sofia districts follow the cities.
const bigCityDistricts = 4

// bigCityRows is the seven cities, then the Sofia districts with the most
// sensors in the snapshot (ties by slug). Slugs the snapshot lacks are skipped.
func (rr *Renderer) bigCityRows(snap *snapshot.Snapshot, lang string) []AreaRow {
	var rows []AreaRow
	for _, slug := range bigCitySlugs {
		if meta, ok := snap.KnownSlugs[slug]; ok && meta.Kind == "city" {
			rows = append(rows, rr.rowFrom(meta, lang))
		}
	}
	var districts []snapshot.AreaMeta
	for _, meta := range snap.KnownSlugs {
		if meta.Kind == "neighbourhood" && meta.ParentSlug == sofiaHubSlug {
			districts = append(districts, meta)
		}
	}
	sort.Slice(districts, func(i, j int) bool {
		if districts[i].SensorCount != districts[j].SensorCount {
			return districts[i].SensorCount > districts[j].SensorCount
		}
		return districts[i].Slug < districts[j].Slug
	})
	if len(districts) > bigCityDistricts {
		districts = districts[:bigCityDistricts]
	}
	for _, meta := range districts {
		rows = append(rows, rr.rowFrom(meta, lang))
	}
	return rows
}

// bigCityItem is one row: the area link, then its figure and band, or the
// no-data text when it has no reading.
func (p PageData) bigCityItem(a AreaRow) string {
	name := a.Name
	// Districts carry their city, the one the block lists them under.
	if a.Kind == "neighbourhood" {
		name = strings.ReplaceAll(p.T("home.bigcities.district"), "{name}", name)
	}
	link := `<a class="link" href="` + htmlEscape(p.Path("/area/"+a.Slug)) + `">` + htmlEscape(name) + `</a>`
	if !a.HasValue {
		return `<li>` + link + `: ` + htmlEscape(p.T("map.legend.no_data")) + `</li>`
	}
	chip := `<span class="chip">`
	if a.Colour != "" {
		chip += `<svg class="chip__swatch" data-class="` + strconv.Itoa(a.Class) + `" viewBox="0 0 12 14" aria-hidden="true"><polygon class="class-outline" points="6,0 12,3.5 12,10.5 6,14 0,10.5 0,3.5" fill="` + htmlEscape(a.Colour) + `" stroke="` + htmlEscape(p.MarkerLabelColour) + `"></polygon></svg>`
	}
	chip += htmlEscape(a.ValueText) + `</span>`
	return `<li>` + strings.NewReplacer(
		"{area}", link,
		"{metric}", htmlEscape(p.T("metric."+p.DefaultMetric)),
		"{value}", chip,
		"{unit}", p.unitMarkup(p.DefaultMetric),
		"{band}", htmlEscape(p.T("band."+bandLabel(p.DefaultMetric, a.Value))),
	).Replace(p.T("home.rank.item")) + `</li>`
}

// hasSofiaHub reports whether the snapshot knows the Sofia city page.
func hasSofiaHub(snap *snapshot.Snapshot) bool {
	meta, ok := snap.KnownSlugs[sofiaHubSlug]
	return ok && meta.Kind == "city"
}

// HomeBigCitiesHTML is the crawlable big-cities block under the map.
func (p PageData) HomeBigCitiesHTML() template.HTML {
	if len(p.HomeBigCities) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="home-big-cities" aria-labelledby="home-big-cities-h"><h2 id="home-big-cities-h">` + htmlEscape(p.T("home.bigcities.heading")) + `</h2>`)
	// Same staleness rule and sentence as the province ranking.
	if p.stale() {
		b.WriteString(p.staleSentence())
	}
	b.WriteString(`<ul class="home-big-cities__list">`)
	for _, a := range p.HomeBigCities {
		b.WriteString(p.bigCityItem(a))
	}
	b.WriteString(`</ul><ul class="home-big-cities__more">`)
	if p.HomeSofia {
		b.WriteString(`<li><a class="link" href="` + htmlEscape(p.Path("/area/"+sofiaHubSlug)) + `">` + htmlEscape(p.T("home.cities.sofia")) + `</a></li>`)
	}
	b.WriteString(`<li><a class="link" href="` + htmlEscape(p.Path("/areas")) + `">` + htmlEscape(p.T("home.cities.all")) + `</a></li></ul></section>`)
	return template.HTML(b.String())
}
