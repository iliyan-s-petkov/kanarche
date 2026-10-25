package web

import (
	"encoding/xml"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"kanarche.eu/content"
	"kanarche.eu/internal/news"
)

const newsPath = "/news"

// emptyFeedUpdated is the feed updated time while no post is public.
var emptyFeedUpdated = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// relativeURLAttr matches a root-relative href or src in rendered post HTML.
// goldmark always writes double quotes, and "//" is left alone.
var relativeURLAttr = regexp.MustCompile(`\b(href|src)="/([^/])`)

// absolutizeLinks makes root-relative links absolute, since a feed reader has no page to resolve them against.
func absolutizeLinks(html, base string) string {
	return relativeURLAttr.ReplaceAllString(html, `$1="`+base+`/$2`)
}

// loadEmbeddedNews reads the published posts compiled into the binary.
func (rr *Renderer) loadEmbeddedNews() error {
	sub, err := fs.Sub(content.News, "news")
	if err != nil {
		return fmt.Errorf("web: news content: %w", err)
	}
	set, err := news.Load(sub, news.Options{Location: rr.sofiaLoc})
	if err != nil {
		return fmt.Errorf("web: news content: %w", err)
	}
	rr.news = set
	return nil
}

// SetNews swaps the post set and the clock. For tests, before serving.
func (rr *Renderer) SetNews(set *news.Set, now func() time.Time) {
	rr.news, rr.newsNow = set, now
	rr.sitemapMu.Lock()
	rr.sitemapBody = nil
	rr.sitemapMu.Unlock()
}

// NewsItem is one row of the /news list.
type NewsItem struct {
	Title, Description, URL string
	DateISO, DateHuman      string
}

// NewsArea is a related-area link under a post.
type NewsArea struct{ Name, URL string }

// NewsPostView is what the post template reads.
type NewsPostView struct {
	Title                   string
	HTML                    template.HTML
	DateISO, DateHuman      string
	HasUpdate               bool
	UpdatedISO, UpdatedText string
	Areas                   []NewsArea
}

// OnNews is true on the list and on every post, so the masthead tab stays marked.
func (p PageData) OnNews() bool {
	return p.RequestPath == newsPath || strings.HasPrefix(p.RequestPath, newsPath+"/")
}

// newsDate prints a calendar day as "30 September 2026" in the page language.
func (p PageData) newsDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), p.T("news.month."+strconv.Itoa(int(t.Month()))), t.Year())
}

func newsDay(t time.Time) string { return t.Format("2006-01-02") }

// newsLangs is the served languages the slug has a public post in.
func (rr *Renderer) newsLangs(now time.Time, slug string) []string {
	var out []string
	for _, lang := range rr.news.Langs(now, slug) {
		if rr.cat.Has(lang, "lang.name") {
			out = append(out, lang)
		}
	}
	return out
}

// newsListLangs is the served languages that have at least one public post.
func (rr *Renderer) newsListLangs(now time.Time) []string {
	out := []string{}
	for _, lang := range rr.cat.Languages() {
		if len(rr.news.ForLang(now, lang)) > 0 {
			out = append(out, lang)
		}
	}
	return out
}

func (rr *Renderer) newsPageData(r *http.Request) PageData {
	var generatedAt time.Time
	if snap := rr.holder.Load(); snap != nil {
		generatedAt = snap.GeneratedAt
	}
	lang, path := rr.cat.LangFromPath(r.URL.Path)
	data := rr.newPageData(lang, path, generatedAt)
	data.FeedURL = data.BaseURL + data.Path(newsPath+"/feed.xml")
	data.AltFallbackPath = newsPath
	return data
}

func (rr *Renderer) handleNewsList(w http.ResponseWriter, r *http.Request) {
	now := rr.newsNow()
	data := rr.newsPageData(r)
	data.Title = composeTitle(data.T("seo.news.title"), data.T("seo.title_brand"))
	data.Description = data.T("seo.news.description")
	data.AltLangs = rr.newsListLangs(now)

	posts := rr.news.ForLang(now, data.Lang)
	// An empty list is a page nobody should land on from search.
	data.NoIndex = len(posts) == 0
	for _, p := range posts {
		data.NewsPosts = append(data.NewsPosts, NewsItem{
			Title: p.Title, Description: p.Description, URL: data.Path(newsPath + "/" + p.Slug),
			DateISO: newsDay(p.Date), DateHuman: data.newsDate(p.Date),
		})
	}
	rr.render(w, r, http.StatusOK, "news_list", data)
}

func (rr *Renderer) handleNewsPost(w http.ResponseWriter, r *http.Request) {
	now := rr.newsNow()
	data := rr.newsPageData(r)
	post, ok := rr.news.Lookup(now, data.Lang, r.PathValue("slug"))
	if !ok {
		rr.RenderError(w, r, http.StatusNotFound, "not_found")
		return
	}
	data.Title = composeTitle(post.Title, data.T("seo.title_brand"))
	data.Description = post.Description
	data.AltLangs = rr.newsLangs(now, post.Slug)

	view := &NewsPostView{
		Title: post.Title, HTML: post.HTML,
		DateISO: newsDay(post.Date), DateHuman: data.newsDate(post.Date),
		HasUpdate: post.HasUpdate(), UpdatedISO: newsDay(post.Updated), UpdatedText: data.newsDate(post.Updated),
	}
	// Names come from the snapshot, so a slug it does not know gets no link.
	if snap := rr.holder.Load(); snap != nil {
		for _, slug := range post.Areas {
			if meta, ok := snap.KnownSlugs[slug]; ok {
				view.Areas = append(view.Areas, NewsArea{Name: rr.rowFrom(meta, data.Lang).Name, URL: data.Path("/area/" + slug)})
			}
		}
	}
	data.NewsPost = view
	data.JSONLD = rr.mustJSONLD(data.newsPostJSONLD(post))
	rr.render(w, r, http.StatusOK, "news_post", data)
}

// newsPostJSONLD is the breadcrumb trail and an Article, author and publisher as on the other prose pages.
func (p PageData) newsPostJSONLD(post news.Post) (template.JS, error) {
	person, org := p.controller(), p.organization()
	article := ldArticle{
		Type:             "Article",
		Headline:         post.Title,
		Description:      post.Description,
		InLanguage:       p.Lang,
		MainEntityOfPage: p.CanonicalURL(),
		Author:           ldRef{ID: person.ID},
		Publisher:        ldRef{ID: org.ID},
		DatePublished:    post.Date.Format(time.RFC3339),
		DateModified:     post.Updated.Format(time.RFC3339),
		Image:            p.OGImageURL(),
	}
	crumbs := ldBreadcrumbList{Type: "BreadcrumbList", ItemListElement: []ldListItem{
		{Type: "ListItem", Position: 1, Name: p.T("area.breadcrumb.home"), Item: p.BaseURL + p.Path("/")},
		{Type: "ListItem", Position: 2, Name: p.T("news.breadcrumb"), Item: p.BaseURL + p.Path(newsPath)},
		{Type: "ListItem", Position: 3, Name: post.Title, Item: p.CanonicalURL()},
	}}
	return encodeJSONLD(crumbs, article, org, person)
}

// Atom 1.0, https://www.rfc-editor.org/rfc/rfc4287.
type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Lang    string      `xml:"xml:lang,attr"`
	ID      string      `xml:"id"`
	Title   string      `xml:"title"`
	Updated string      `xml:"updated"`
	Links   []atomLink  `xml:"link"`
	Author  atomAuthor  `xml:"author"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr,omitempty"`
	Href string `xml:"href,attr"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomText struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type atomEntry struct {
	ID        string     `xml:"id"`
	Title     string     `xml:"title"`
	Link      atomLink   `xml:"link"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Author    atomAuthor `xml:"author"`
	Summary   string     `xml:"summary"`
	Content   atomText   `xml:"content"`
}

func (rr *Renderer) handleNewsFeed(w http.ResponseWriter, r *http.Request) {
	now := rr.newsNow()
	data := rr.newsPageData(r)
	author := atomAuthor{Name: data.T("seo.title_brand")}
	listURL := data.BaseURL + data.Path(newsPath)

	feed := atomFeed{
		Lang: data.Lang, ID: listURL, Title: data.T("news.feed_title"), Author: author,
		Links: []atomLink{
			{Rel: "self", Type: "application/atom+xml", Href: data.FeedURL},
			{Rel: "alternate", Type: "text/html", Href: listURL},
		},
	}
	var newest time.Time
	for _, p := range rr.news.ForLang(now, data.Lang) {
		url := data.BaseURL + data.Path(newsPath+"/"+p.Slug)
		feed.Entries = append(feed.Entries, atomEntry{
			ID: url, Title: p.Title, Link: atomLink{Rel: "alternate", Type: "text/html", Href: url},
			Published: p.Date.Format(time.RFC3339), Updated: p.Updated.Format(time.RFC3339),
			Author: author, Summary: p.Description, Content: atomText{Type: "html", Body: absolutizeLinks(string(p.HTML), data.BaseURL)},
		})
		if p.Updated.After(newest) {
			newest = p.Updated
		}
	}
	// A feed with no entries still needs an updated time, and a fixed one keeps it cacheable.
	if newest.IsZero() {
		newest = emptyFeedUpdated
	}
	feed.Updated = newest.Format(time.RFC3339)

	out, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.Header().Set("Cache-Control", textCacheControl)
	_, _ = w.Write(append([]byte(xml.Header), out...))
}

// newsSitemapEntry is one URL the sitemap lists for the news section.
type newsSitemapEntry struct {
	path, lastMod, lang string
	alts                []string
}

// newsSitemapEntries lists the list pages and the posts, one entry per
// language that exists, each with the languages it pairs with.
func (rr *Renderer) newsSitemapEntries(now time.Time) []newsSitemapEntry {
	listLangs := rr.newsListLangs(now)
	var out []newsSitemapEntry
	for _, lang := range listLangs {
		posts := rr.news.ForLang(now, lang)
		newest := slices.MaxFunc(posts, func(a, b news.Post) int { return a.Updated.Compare(b.Updated) })
		out = append(out, newsSitemapEntry{path: newsPath, lastMod: newsDay(newest.Updated), lang: lang, alts: listLangs})
	}
	for _, p := range rr.news.Visible(now) {
		if !rr.cat.Has(p.Lang, "lang.name") {
			continue
		}
		out = append(out, newsSitemapEntry{path: newsPath + "/" + p.Slug, lastMod: newsDay(p.Updated), lang: p.Lang, alts: rr.newsLangs(now, p.Slug)})
	}
	return out
}
