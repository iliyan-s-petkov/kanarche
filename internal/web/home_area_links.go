package web

import (
	"html/template"
	"sort"
	"strings"

	"kanarche.eu/internal/snapshot"
)

// areaLinkRows is the areas of one kind sorted by display name; a non-empty parent filters by ParentSlug.
func (rr *Renderer) areaLinkRows(snap *snapshot.Snapshot, lang, kind, parent string) []AreaRow {
	var rows []AreaRow
	for _, meta := range snap.KnownSlugs {
		if meta.Kind == kind && (parent == "" || meta.ParentSlug == parent) {
			rows = append(rows, rr.rowFrom(meta, lang))
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows
}

// HomeAreaLinksHTML is the crawlable list of every Sofia district and provincial capital under the map.
func (p PageData) HomeAreaLinksHTML() template.HTML {
	if len(p.HomeDistricts) == 0 && len(p.HomeCapitals) == 0 {
		return ""
	}
	list := func(class string, rows []AreaRow) string {
		var b strings.Builder
		b.WriteString(`<ul class="home-area-links__list home-area-links__` + class + `">`)
		for _, a := range rows {
			b.WriteString(`<li><a class="link" href="` + htmlEscape(p.Path("/area/"+a.Slug)) + `">` + htmlEscape(a.Name) + `</a></li>`)
		}
		b.WriteString(`</ul>`)
		return b.String()
	}
	return template.HTML(`<section class="home-area-links" aria-labelledby="home-area-links-h"><h2 id="home-area-links-h">` +
		htmlEscape(p.T("areas.directory.title")) + `</h2>` +
		list("districts", p.HomeDistricts) + list("cities", p.HomeCapitals) + `</section>`)
}
