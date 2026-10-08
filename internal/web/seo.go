package web

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"time"

	"kanarche.eu/internal/i18n"
)

// /embed is not disallowed: a crawl block would hide its X-Robots-Tag noindex.
const robotsBody = "User-agent: *\n" +
	"Allow: /\n" +
	"Disallow: /api/\n" +
	"Sitemap: %s/sitemap.xml\n"

func (rr *Renderer) handleRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, robotsBody, rr.baseURL)
}

// Sitemap protocol plus the xhtml:link hreflang extension.
type sitemapURLSet struct {
	XMLName    xml.Name     `xml:"urlset"`
	Xmlns      string       `xml:"xmlns,attr"`
	XmlnsXhtml string       `xml:"xmlns:xhtml,attr"`
	URLs       []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string        `xml:"loc"`
	LastMod string        `xml:"lastmod,omitempty"`
	Links   []sitemapLink `xml:"xhtml:link"`
}

type sitemapLink struct {
	Rel      string `xml:"rel,attr"`
	Hreflang string `xml:"hreflang,attr"`
	Href     string `xml:"href,attr"`
}

// handleSitemap lists the static pages and every area in every language; cached per snapshot.
func (rr *Renderer) handleSitemap(w http.ResponseWriter, r *http.Request) {
	var generatedAt time.Time
	if snap := rr.holder.Load(); snap != nil {
		generatedAt = snap.GeneratedAt
	}

	body, err := rr.sitemapBytes(generatedAt)
	if err != nil {
		http.Error(w, "sitemap unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write(body)
}

func (rr *Renderer) sitemapBytes(generatedAt time.Time) ([]byte, error) {
	rr.sitemapMu.Lock()
	defer rr.sitemapMu.Unlock()

	if rr.sitemapBody != nil && rr.sitemapAt.Equal(generatedAt) {
		return rr.sitemapBody, nil
	}

	var slugs []string
	covered := map[string]bool{}
	if snap := rr.holder.Load(); snap != nil {
		for slug, meta := range snap.KnownSlugs {
			slugs = append(slugs, slug)
			covered[slug] = meta.Covered
		}
		sort.Strings(slugs)
	}

	var day string
	if !generatedAt.IsZero() {
		day = generatedAt.UTC().Format("2006-01-02")
	}
	// lastmod only where the page body follows the data: the home, the
	// directory and areas with coverage. Static pages and areas without data
	// get none rather than a date nothing supports.
	type entry struct{ path, lastMod string }
	entries := make([]entry, 0, len(slugs)+6)
	entries = append(entries, entry{"/", day}, entry{"/areas", day}, entry{"/about", ""}, entry{"/about-the-data", ""}, entry{"/privacy", ""}, entry{"/licences", ""})
	for _, slug := range slugs {
		lm := ""
		if covered[slug] {
			lm = day
		}
		entries = append(entries, entry{"/area/" + slug, lm})
	}

	urls := make([]sitemapURL, 0, len(entries)*len(rr.cat.Languages()))
	for _, e := range entries {
		// One <url> per language variant, each carrying the full alternate set.
		links := rr.sitemapLinks(e.path)
		for _, a := range rr.pageAlternates(e.path) {
			urls = append(urls, sitemapURL{Loc: a.URL, LastMod: e.lastMod, Links: links})
		}
	}

	set := sitemapURLSet{
		Xmlns:      "http://www.sitemaps.org/schemas/sitemap/0.9",
		XmlnsXhtml: "http://www.w3.org/1999/xhtml",
		URLs:       urls,
	}

	out, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, err
	}
	body := append([]byte(xml.Header), out...)

	rr.sitemapAt = generatedAt
	rr.sitemapBody = body
	return body, nil
}

// pageAlternates is the same list the page <head> renders for this path.
func (rr *Renderer) pageAlternates(path string) []alternate {
	data := PageData{cat: rr.cat, BaseURL: rr.baseURL, RequestPath: path}
	return data.Alternates()
}

// sitemapLinks reuses the <head> Alternates so the two never disagree, plus x-default.
func (rr *Renderer) sitemapLinks(path string) []sitemapLink {
	alts := rr.pageAlternates(path)

	links := make([]sitemapLink, 0, len(alts)+1)
	for _, a := range alts {
		links = append(links, sitemapLink{Rel: "alternate", Hreflang: a.Lang, Href: a.URL})
	}
	for _, a := range alts {
		if a.Lang == i18n.DefaultLang {
			links = append(links, sitemapLink{Rel: "alternate", Hreflang: "x-default", Href: a.URL})
			break
		}
	}
	return links
}
