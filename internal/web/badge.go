package web

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"strings"

	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
)

const (
	badgeMetric   = "P2"
	badgeCache    = "public, max-age=300, s-maxage=300"
	badgeNoData   = "public, max-age=60"
	badgeSuffix   = ".svg"
	badgeMaxName  = 16
	badgeExample  = "plovdiv-oblast"
	badgeGreyFill = "#9e9e9e"
)

// badgeLook is everything the SVG shows; Name and Band are empty on the no-data badge.
type badgeLook struct {
	Name, Band, Value, Colour string
}

// handleBadge serves /badge/{slug}.svg. ServeMux has no wildcard-plus-suffix, so the
// segment is {file} and the suffix is checked here.
func (rr *Renderer) handleBadge(w http.ResponseWriter, r *http.Request) {
	slug, ok := strings.CutSuffix(r.PathValue("file"), badgeSuffix)
	if !ok || slug == "" {
		rr.RenderError(w, r, http.StatusNotFound, "not_found")
		return
	}
	lang, _ := rr.cat.LangFromPath(r.URL.Path)
	snap := rr.holder.Load()
	if snap == nil {
		rr.writeBadge(w, lang, badgeNoData, badgeLook{})
		return
	}
	meta, known := snap.KnownSlugs[slug]
	if !known {
		if target, found := areaAliasTarget(snap, slug); found {
			dest := "/badge/" + target + badgeSuffix
			if lang != i18n.DefaultLang {
				dest = "/" + lang + dest
			}
			http.Redirect(w, r, dest, http.StatusMovedPermanently)
			return
		}
		rr.RenderError(w, r, http.StatusNotFound, "not_found")
		return
	}
	look, has := rr.badgeFor(snap, slug, lang)
	if !has {
		rr.writeBadge(w, lang, badgeNoData, badgeLook{Name: rr.rowFrom(meta, lang).Name})
		return
	}
	rr.writeBadge(w, lang, badgeCache, look)
}

// badgeAlt is badge.alt filled for one reading, shared by the SVG title and the page snippet.
func (rr *Renderer) badgeAlt(lang string, look badgeLook) string {
	return strings.NewReplacer("{name}", look.Name, "{band}", look.Band, "{value}", look.Value).
		Replace(rr.cat.T(lang, rr.vavKey(lang, "badge.alt", look.Name)))
}

// vavKey is key's _vav variant for в/ф names (startsWithVav), when the catalogue has one.
func (rr *Renderer) vavKey(lang, key, name string) string {
	if startsWithVav(name) && rr.cat.Has(lang, key+"_vav") {
		return key + "_vav"
	}
	return key
}

// badgeFor is the look of one area's current reading; ok is false without a PM2.5 value.
func (rr *Renderer) badgeFor(snap *snapshot.Snapshot, slug, lang string) (badgeLook, bool) {
	if snap == nil {
		return badgeLook{}, false
	}
	meta, known := snap.KnownSlugs[slug]
	if !known {
		return badgeLook{}, false
	}
	value, has := meta.Values[badgeMetric]
	if !has {
		return badgeLook{}, false
	}
	return badgeLook{
		Name:   rr.rowFrom(meta, lang).Name,
		Band:   rr.cat.T(lang, "band."+bandLabel(badgeMetric, value)),
		Value:  formatValue(value, lang),
		Colour: bandColour(badgeMetric, value),
	}, true
}

// writeBadge sets the image headers and writes the SVG. CORP is cross-origin on this route only.
func (rr *Renderer) writeBadge(w http.ResponseWriter, lang, cacheControl string, look badgeLook) {
	h := w.Header()
	h.Set("Content-Type", "image/svg+xml; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cross-Origin-Resource-Policy", "cross-origin")
	h.Set("Cache-Control", cacheControl)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(rr.badgeSVG(lang, look))
}

func xmlEsc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// clipName shortens a long area name so it stays inside the 200 px badge.
func clipName(name string) string {
	runes := []rune(name)
	if len(runes) <= badgeMaxName {
		return name
	}
	return string(runes[:badgeMaxName-1]) + "…"
}

// badgeSVG composes the 200x40 image from catalogue strings only. Every interpolated
// string goes through xmlEsc; the colour is a constant from the band table.
func (rr *Renderer) badgeSVG(lang string, look badgeLook) []byte {
	brand := xmlEsc(rr.cat.T(lang, "seo.title_brand"))
	var b strings.Builder
	head := func(alt, colour string) {
		b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="200" height="40" viewBox="0 0 200 40" role="img" aria-label="` +
			xmlEsc(alt) + `"><title>` + xmlEsc(alt) + `</title>`)
		b.WriteString(`<rect x="0.5" y="0.5" width="199" height="39" rx="4" fill="#ffffff" stroke="#c4c4c4"/>`)
		b.WriteString(`<rect x="1" y="1" width="8" height="38" rx="3" fill="` + colour + `"/>`)
	}
	const font = `font-family="system-ui,-apple-system,'Segoe UI',Roboto,Arial,sans-serif"`
	if look.Band == "" {
		noData := rr.cat.T(lang, "badge.no_data")
		head(noData, badgeGreyFill)
		if look.Name != "" {
			b.WriteString(`<text x="16" y="16" ` + font + ` font-size="12" font-weight="700" fill="#1a1a1a">` + xmlEsc(clipName(look.Name)) + `</text>`)
		}
		b.WriteString(`<text x="16" y="31" ` + font + ` font-size="11" fill="#4a4a4a">` + xmlEsc(noData) + `</text>`)
	} else {
		// Three rows: name, reading, band word. The band word has its own row because the
		// longest Bulgarian word does not fit beside the reading in 176 px.
		head(rr.badgeAlt(lang, look), look.Colour)
		b.WriteString(`<text x="16" y="12" ` + font + ` font-size="11" font-weight="700" fill="#1a1a1a">` + xmlEsc(clipName(look.Name)) + `</text>`)
		reading := rr.cat.T(lang, "metric."+badgeMetric) + " " + look.Value + " " + rr.cat.T(lang, "unit."+badgeMetric)
		b.WriteString(`<text x="16" y="25" ` + font + ` font-size="10" fill="#1a1a1a">` + xmlEsc(reading) + `</text>`)
		b.WriteString(`<text x="16" y="37" ` + font + ` font-size="10" font-weight="700" fill="#1a1a1a">` + xmlEsc(look.Band) + `</text>`)
	}
	b.WriteString(`<text x="194" y="11" text-anchor="end" ` + font + ` font-size="8" fill="#4a4a4a">` + brand + `</text></svg>`)
	return []byte(b.String())
}

// badgeExampleAlt is the alt text for the page snippet: the example area's current
// badge.alt, or badge.link_text when the area has no reading yet.
func (rr *Renderer) badgeExampleAlt(lang string) string {
	look, ok := rr.badgeFor(rr.holder.Load(), badgeExample, lang)
	if ok {
		return rr.badgeAlt(lang, look)
	}
	name := badgeExample
	if snap := rr.holder.Load(); snap != nil {
		if meta, known := snap.KnownSlugs[badgeExample]; known {
			name = rr.rowFrom(meta, lang).Name
		}
	}
	return strings.ReplaceAll(rr.cat.T(lang, rr.vavKey(lang, "badge.link_text", name)), "{name}", name)
}
