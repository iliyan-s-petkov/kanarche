package web

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
)

// aiCrawlers are allowed on purpose, training crawlers included: the data is open.
// Each named group repeats the /api/ rule, because a named group replaces `*`.
var aiCrawlers = []string{
	"GPTBot", "OAI-SearchBot", "ChatGPT-User",
	"ClaudeBot", "Claude-SearchBot", "Claude-User",
	"PerplexityBot", "Perplexity-User",
	"Google-Extended", "CCBot", "Applebot-Extended", "Bingbot",
}

// datasetPaths are the Dataset distribution URLs in jsonld.go, each one exact
// file. The `$` anchors each Allow to that URL alone, so siblings under /api/
// stay disallowed. Each group lists them before its Disallow, for parsers that
// take the first match.
var datasetPaths = []string{"/api/v1/areas", "/api/v1/overview"}

// robotsTxt: /embed is not disallowed, because a crawl block would hide its X-Robots-Tag noindex.
func robotsTxt(baseURL string) string {
	var b strings.Builder
	group := func(agent string) {
		fmt.Fprintf(&b, "User-agent: %s\n", agent)
		for _, p := range datasetPaths {
			fmt.Fprintf(&b, "Allow: %s$\n", p)
		}
		b.WriteString("Disallow: /api/\n\n")
	}
	group("*")
	for _, bot := range aiCrawlers {
		group(bot)
	}
	fmt.Fprintf(&b, "Sitemap: %s/sitemap.xml\n", baseURL)
	return b.String()
}

// textCacheControl is for the two text indexes, which change only on deploy,
// so an hour at the edge is safe.
const textCacheControl = "public, max-age=3600"

func (rr *Renderer) handleRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", textCacheControl)
	fmt.Fprint(w, robotsTxt(rr.baseURL))
}

// handleLLMSTxt serves the llmstxt.org index.
func (rr *Renderer) handleLLMSTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", textCacheControl)
	fmt.Fprint(w, rr.llmsTxt())
}

// llmsTxt builds the file from the catalogue, so its copy is the page copy.
func (rr *Renderer) llmsTxt() string {
	en := func(key string) string { return rr.cat.T("en", key) }
	bg := func(key string) string { return rr.cat.T("bg", key) }
	base := rr.baseURL

	var b strings.Builder
	b.WriteString("# Kanarche\n\n")
	fmt.Fprintf(&b, "> %s\n\n", en("seo.about.description"))
	fmt.Fprintf(&b, "%s\n\n", en("terms.what.body"))
	fmt.Fprintf(&b, "%s\n\n", bg("terms.what.body"))

	b.WriteString("## Pages\n\n")
	pages := []struct{ path, title, desc string }{
		{"/", "Map (Bulgarian)", en("seo.home.description")},
		{"/en/", "Map (English)", en("seo.home.description")},
		{"/areas", "Areas", en("seo.areas.description")},
		{"/about", "About", en("seo.about.description")},
		{"/about-the-data", "About the data", en("seo.about_data.description")},
		{"/licences", "Licences", en("seo.licences.description")},
		{"/privacy", "Privacy", en("seo.privacy.description")},
	}
	for _, p := range pages {
		fmt.Fprintf(&b, "- [%s](%s%s): %s\n", p.title, base, p.path, p.desc)
	}

	b.WriteString("\n## Data\n\n")
	fmt.Fprintf(&b, "%s\n\n", en("about.sources.body"))
	fmt.Fprintf(&b, "%s\n\n", bg("about.sources.body"))
	fmt.Fprintf(&b, "%s\n\n", en("licences.intro"))

	b.WriteString("## Optional\n\n")
	fmt.Fprintf(&b, "- [Sitemap](%s/sitemap.xml): every page in both languages, with hreflang alternates.\n", base)
	return b.String()
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
	snap := rr.holder.Load()

	body, err := rr.sitemapBytes(snap)
	if err != nil {
		http.Error(w, "sitemap unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write(body)
}

// sitemapBytes builds from one snapshot throughout, so the slugs, the lastmods
// and the cache key cannot come from different loads.
func (rr *Renderer) sitemapBytes(snap *snapshot.Snapshot) ([]byte, error) {
	var generatedAt time.Time
	if snap != nil {
		generatedAt = snap.GeneratedAt
	}
	rr.sitemapMu.Lock()
	defer rr.sitemapMu.Unlock()

	if rr.sitemapBody != nil && rr.sitemapAt.Equal(generatedAt) {
		return rr.sitemapBody, nil
	}

	// Only covered areas: an uncovered page is noindex, so it stays out of the sitemap too.
	var slugs []string
	if snap != nil {
		for slug, meta := range snap.KnownSlugs {
			if meta.Covered {
				slugs = append(slugs, slug)
			}
		}
		sort.Strings(slugs)
	}

	var day string
	if !generatedAt.IsZero() {
		day = generatedAt.UTC().Format("2006-01-02")
	}
	// lastmod only where the page body follows the data. The home and the
	// directory follow every snapshot. An area carries the day its own figures
	// last changed, and none when that is unknown. Static pages get none
	// rather than a date nothing supports.
	type entry struct{ path, lastMod string }
	entries := make([]entry, 0, len(slugs)+6)
	entries = append(entries, entry{"/", day}, entry{"/areas", day}, entry{"/about", ""}, entry{"/about-the-data", ""}, entry{"/privacy", ""}, entry{"/licences", ""}, entry{"/terms", ""}, entry{"/pm25", ""}, entry{"/pm10", ""}, entry{"/air-pollution", ""})
	for _, slug := range slugs {
		var changed string
		if at := snap.KnownSlugs[slug].ChangedAt; !at.IsZero() {
			changed = at.UTC().Format("2006-01-02")
		}
		entries = append(entries, entry{"/area/" + slug, changed})
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
