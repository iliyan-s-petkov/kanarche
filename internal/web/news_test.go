package web_test

import (
	"encoding/xml"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"kanarche.eu/internal/news"
	"kanarche.eu/internal/web"
)

var newsClock = func() time.Time { return time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC) }

func newsFile(title, date, extra, status, body string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte("---\ntitle: " + title + "\ndescription: Desc of " + title + ".\ndate: " + date +
		"\ntype: monthly\nstatus: " + status + "\n" + extra + "---\n" + body + "\n")}
}

// newsRenderer serves a fixed post set: one post in both languages (the
// Bulgarian one edited later), one English only, one Bulgarian only, one draft
// and one dated after newsClock.
func newsRenderer(t *testing.T) *web.Renderer {
	t.Helper()
	fsys := fstest.MapFS{
		"2026-09-sofia.bg.md": newsFile("Въздухът в София", "2026-09-30", "updated: 2026-10-02\nareas: [sofia]\n", "published", "Текст на поста.\n\n## Подзаглавие\n\nОще текст."),
		"2026-09-sofia.en.md": newsFile("Air in Sofia", "2026-09-30", "areas: [sofia]\n", "published", "Body of the post.\n\n## Subheading\n\nMore text."),
		"2026-08-note.en.md":  newsFile("English only note", "2026-08-31", "", "published", "English only body."),
		"2026-07-nota.bg.md":  newsFile("Само на български", "2026-07-31", "", "published", "Само български."),
		"2026-10-draft.bg.md": newsFile("Чернова", "2026-10-01", "", "draft", "Чернова."),
		"2027-01-bud.bg.md":   newsFile("Бъдеща", "2027-01-05", "", "published", "Бъдеща."),
	}
	sofia, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Fatalf("tzdata: %v", err)
	}
	set, err := news.Load(fsys, news.Options{Location: sofia})
	if err != nil {
		t.Fatalf("news.Load: %v", err)
	}
	rr := renderer(t, fixture(t))
	rr.SetNews(set, newsClock)
	return rr
}

func TestNewsListBGShowsOnlyBulgarianPublishedPostsNewestFirst(t *testing.T) {
	rec := fetch(t, newsRenderer(t), "/news")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /news = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<h1") || !strings.Contains(body, ">Новини</h1>") {
		t.Errorf("no Новини heading")
	}
	sofia := strings.Index(body, `href="/news/2026-09-sofia"`)
	nota := strings.Index(body, `href="/news/2026-07-nota"`)
	if sofia < 0 || nota < 0 || sofia > nota {
		t.Errorf("links missing or not newest first: sofia=%d nota=%d", sofia, nota)
	}
	for _, bad := range []string{"English only", "Чернова", "Бъдеща", "2026-08-note", "2026-10-draft", "2027-01-bud"} {
		if strings.Contains(body, bad) {
			t.Errorf("list contains %q", bad)
		}
	}
	if !strings.Contains(body, `<time datetime="2026-09-30">30 септември 2026</time>`) {
		t.Errorf("no localized date for the Sofia post")
	}
}

func TestNewsListENShowsOnlyEnglishPosts(t *testing.T) {
	body := fetch(t, newsRenderer(t), "/en/news").Body.String()
	if !strings.Contains(body, ">News</h1>") {
		t.Errorf("no News heading")
	}
	if !strings.Contains(body, `href="/en/news/2026-09-sofia"`) || !strings.Contains(body, `href="/en/news/2026-08-note"`) {
		t.Errorf("English links missing")
	}
	if strings.Contains(body, "Само на български") || strings.Contains(body, "2026-07-nota") {
		t.Errorf("Bulgarian-only post listed under /en/news")
	}
	if !strings.Contains(body, `<time datetime="2026-09-30">30 September 2026</time>`) {
		t.Errorf("no localized English date")
	}
}

func TestNewsListWithNoPostsIsAnEmptyNoindexPage(t *testing.T) {
	rr := renderer(t, fixture(t))
	set, _ := news.Load(fstest.MapFS{}, news.Options{})
	rr.SetNews(set, newsClock)
	rec := fetch(t, rr, "/news")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /news = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<meta name="robots" content="noindex">`) {
		t.Errorf("empty list is indexable")
	}
	if strings.Contains(body, "news-list") {
		t.Errorf("empty list renders a list")
	}
	if strings.Contains(body, "feed.xml") {
		t.Errorf("empty list advertises the feed")
	}
}

func TestNewsListAdvertisesFeedOnlyWithPosts(t *testing.T) {
	body := fetch(t, newsRenderer(t), "/news").Body.String()
	if !strings.Contains(body, `<link rel="alternate" type="application/atom+xml"`) || !strings.Contains(body, `news/feed.xml" type="application/atom+xml">`) {
		t.Errorf("list with posts misses the feed link")
	}
}

func TestNewsPostRendersFullTextWithoutScript(t *testing.T) {
	rec := fetch(t, newsRenderer(t), "/news/2026-09-sofia")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<title>Въздухът в София",
		">Въздухът в София</h1>",
		"Текст на поста.",
		`<h2 id="`,
		`<link rel="canonical" href="https://airbg.org/news/2026-09-sofia">`,
		`<time datetime="2026-09-30">30 септември 2026</time>`,
		`<time datetime="2026-10-02">2 октомври 2026</time>`, // updated
		`href="/area/sofia"`, // related area
		`href="/news"`,       // breadcrumb back to the list
	} {
		if !strings.Contains(body, want) {
			t.Errorf("post page missing %q", want)
		}
	}
	// The post body must be in the HTML itself, not injected by a script.
	if strings.Contains(body, "Chernova") || strings.Contains(body, "Чернова") {
		t.Errorf("draft text leaked")
	}
}

func TestNewsPostRelatedAreaUsesTheSnapshotName(t *testing.T) {
	body := fetch(t, newsRenderer(t), "/en/news/2026-09-sofia").Body.String()
	if !regexp.MustCompile(`<a [^>]*href="/en/area/sofia"[^>]*>Sofia</a>`).MatchString(body) {
		t.Errorf("related area link with the English name missing")
	}
}

func TestNewsPostMissingForLanguageDraftFutureAndUnknownAre404(t *testing.T) {
	rr := newsRenderer(t)
	for _, path := range []string{
		"/news/2026-08-note",       // English only, asked in Bulgarian
		"/en/news/2026-07-nota",    // Bulgarian only, asked in English
		"/news/2026-10-draft",      // status draft
		"/news/2027-01-bud",        // future date
		"/news/nope",               // unknown
		"/news/..%2Fetc",           // not a slug
		"/en/news/2026-10-draft",   // draft under another prefix
		"/news/2026-09-sofia/more", // no such route
	} {
		if got := fetch(t, rr, path).Code; got != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, got)
		}
	}
}

var hreflangRe = regexp.MustCompile(`<link rel="alternate" hreflang="([^"]+)" href="([^"]+)">`)

func TestNewsPostHreflangListsOnlyExistingLanguages(t *testing.T) {
	rr := newsRenderer(t)

	both := map[string]string{}
	for _, m := range hreflangRe.FindAllStringSubmatch(fetch(t, rr, "/news/2026-09-sofia").Body.String(), -1) {
		both[m[1]] = m[2]
	}
	if both["bg"] != "https://airbg.org/news/2026-09-sofia" || both["en"] != "https://airbg.org/en/news/2026-09-sofia" ||
		both["x-default"] != both["bg"] {
		t.Errorf("two-language post alternates = %v", both)
	}

	for _, path := range []string{"/news/2026-07-nota", "/en/news/2026-08-note"} {
		body := fetch(t, rr, path).Body.String()
		if m := hreflangRe.FindAllStringSubmatch(body, -1); len(m) != 0 {
			t.Errorf("%s: single-language post has hreflang links %v", path, m)
		}
		if !strings.Contains(body, `<link rel="canonical" href="https://airbg.org`+path+`">`) {
			t.Errorf("%s: not self-canonical", path)
		}
	}
}

func TestNewsPostLanguageSwitcherFallsBackToTheList(t *testing.T) {
	rr := newsRenderer(t)
	picker := func(path string) string {
		b := fetch(t, rr, path).Body.String()
		i := strings.Index(b, `class="langpick__list"`)
		return b[i : i+strings.Index(b[i:], "</ul>")]
	}
	if p := picker("/news/2026-09-sofia"); !strings.Contains(p, `href="https://airbg.org/en/news/2026-09-sofia"`) {
		t.Errorf("switcher does not link the English version: %s", p)
	}
	if p := picker("/news/2026-07-nota"); !strings.Contains(p, `href="https://airbg.org/en/news"`) || strings.Contains(p, "/en/news/2026-07-nota") {
		t.Errorf("switcher for a one-language post should link the English list: %s", p)
	}
}

func TestNewsListHreflangOnlyForLanguagesWithPosts(t *testing.T) {
	rr := renderer(t, fixture(t))
	set, err := news.Load(fstest.MapFS{"2026-07-nota.bg.md": newsFile("Само на български", "2026-07-31", "", "published", "Текст.")}, news.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rr.SetNews(set, newsClock)
	if m := hreflangRe.FindAllStringSubmatch(fetch(t, rr, "/news").Body.String(), -1); len(m) != 0 {
		t.Errorf("list with posts in one language has hreflang links %v", m)
	}
	// The language with no post at all serves an empty page nobody should index.
	if !strings.Contains(fetch(t, rr, "/en/news").Body.String(), `content="noindex"`) {
		t.Errorf("/en/news with no English post is indexable")
	}
}

func TestNewsNavLinkInMastheadAndMarkedCurrent(t *testing.T) {
	rr := newsRenderer(t)
	for path, want := range map[string]string{
		"/about":   `<a class="masthead__link masthead__link--news" href="/news">Новини</a>`,
		"/news":    `<a class="masthead__link masthead__link--news" href="/news" aria-current="page">Новини</a>`,
		"/en/news": `<a class="masthead__link masthead__link--news" href="/en/news" aria-current="page">News</a>`,
	} {
		if body := fetch(t, rr, path).Body.String(); !strings.Contains(body, want) {
			t.Errorf("%s: masthead missing %s", path, want)
		}
	}
	// A post is inside the section, so the tab stays marked.
	if body := fetch(t, rr, "/news/2026-09-sofia").Body.String(); !strings.Contains(body, `href="/news" aria-current="page"`) {
		t.Errorf("post page does not mark the Новини tab")
	}
}

func TestNewsRoutesAreListed(t *testing.T) {
	got := strings.Join(renderer(t, fixture(t)).RoutePatterns(), "\n")
	for _, want := range []string{"GET /news", "GET /news/{slug}", "GET /news/feed.xml", "GET /en/news", "GET /en/news/{slug}", "GET /en/news/feed.xml"} {
		if !strings.Contains(got, want+"\n") && !strings.HasSuffix(got, want) {
			t.Errorf("RoutePatterns missing %q", want)
		}
	}
}

func newsSitemap(t *testing.T) map[string]struct {
	LastMod string
	Links   map[string]string
} {
	t.Helper()
	rec := fetch(t, newsRenderer(t), "/sitemap.xml")
	var doc sitemapXML
	if err := xml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("sitemap: %v", err)
	}
	out := map[string]struct {
		LastMod string
		Links   map[string]string
	}{}
	for _, u := range doc.URLs {
		links := map[string]string{}
		for _, l := range u.Links {
			links[l.Hreflang] = l.Href
		}
		out[u.Loc] = struct {
			LastMod string
			Links   map[string]string
		}{u.LastMod, links}
	}
	return out
}

func TestSitemapListsNewsPostsPerExistingLanguageWithRealLastmod(t *testing.T) {
	sm := newsSitemap(t)

	bg, en := sm["https://airbg.org/news/2026-09-sofia"], sm["https://airbg.org/en/news/2026-09-sofia"]
	if bg.LastMod != "2026-10-02" || en.LastMod != "2026-09-30" {
		t.Errorf("lastmod bg=%q en=%q, want updated 2026-10-02 and date 2026-09-30", bg.LastMod, en.LastMod)
	}
	if bg.Links["en"] != "https://airbg.org/en/news/2026-09-sofia" || bg.Links["bg"] != "https://airbg.org/news/2026-09-sofia" ||
		bg.Links["x-default"] != bg.Links["bg"] {
		t.Errorf("two-language post links = %v", bg.Links)
	}

	for _, loc := range []string{"https://airbg.org/news/2026-07-nota", "https://airbg.org/en/news/2026-08-note"} {
		e, ok := sm[loc]
		if !ok {
			t.Errorf("sitemap missing %s", loc)
			continue
		}
		if len(e.Links) != 0 {
			t.Errorf("%s: single-language post carries alternates %v", loc, e.Links)
		}
	}
	// Each one-language post is listed only under the language that has it.
	for _, loc := range []string{"https://airbg.org/en/news/2026-07-nota", "https://airbg.org/news/2026-08-note"} {
		if _, ok := sm[loc]; ok {
			t.Errorf("sitemap lists %s, which does not exist", loc)
		}
	}
}

func TestSitemapOmitsDraftAndFutureNewsAndUsesNewestPostForTheList(t *testing.T) {
	sm := newsSitemap(t)
	for loc := range sm {
		if strings.Contains(loc, "2026-10-draft") || strings.Contains(loc, "2027-01-bud") {
			t.Errorf("sitemap lists %s", loc)
		}
	}
	if got := sm["https://airbg.org/news"].LastMod; got != "2026-10-02" {
		t.Errorf("/news lastmod = %q, want the newest bg post update 2026-10-02", got)
	}
	if got := sm["https://airbg.org/en/news"].LastMod; got != "2026-09-30" {
		t.Errorf("/en/news lastmod = %q, want 2026-09-30", got)
	}
}

func TestLLMSTxtLinksTheNewsSectionInBothLanguagesWithPosts(t *testing.T) {
	for name, rr := range map[string]*web.Renderer{"with posts": newsRenderer(t)} {
		body := fetch(t, rr, "/llms.txt").Body.String()
		for _, want := range []string{"(https://airbg.org/news)", "(https://airbg.org/en/news)"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: llms.txt missing %s", name, want)
			}
		}
	}
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
	Href string `xml:"href,attr"`
}

type atomText struct {
	Type string `xml:"type,attr"`
	Text string `xml:",chardata"`
}

type atomEntry struct {
	ID        string   `xml:"id"`
	Title     string   `xml:"title"`
	Updated   string   `xml:"updated"`
	Published string   `xml:"published"`
	Link      atomLink `xml:"link"`
	Summary   string   `xml:"summary"`
	Content   atomText `xml:"content"`
	Author    struct {
		Name string `xml:"name"`
	} `xml:"author"`
}

type atomFeed struct {
	XMLName xml.Name   `xml:"http://www.w3.org/2005/Atom feed"`
	Lang    string     `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
	ID      string     `xml:"id"`
	Title   string     `xml:"title"`
	Updated string     `xml:"updated"`
	Links   []atomLink `xml:"link"`
	Author  struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Entries []atomEntry `xml:"entry"`
}

func fetchFeed(t *testing.T, path string) (atomFeed, *http.Response) {
	t.Helper()
	rec := fetch(t, newsRenderer(t), path)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d", path, rec.Code)
	}
	var f atomFeed
	if err := xml.Unmarshal(rec.Body.Bytes(), &f); err != nil {
		t.Fatalf("%s is not Atom: %v\n%s", path, err, rec.Body.String())
	}
	return f, rec.Result()
}

func TestNewsFeedIsValidAtomForOneLanguage(t *testing.T) {
	f, res := fetchFeed(t, "/news/feed.xml")
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/atom+xml") {
		t.Errorf("Content-Type = %q", ct)
	}
	if f.Lang != "bg" || f.ID == "" || f.Title == "" || f.Author.Name == "" {
		t.Errorf("feed header = %+v", f)
	}
	if _, err := time.Parse(time.RFC3339, f.Updated); err != nil {
		t.Errorf("feed updated %q: %v", f.Updated, err)
	}
	var self bool
	for _, l := range f.Links {
		if l.Rel == "self" && l.Href == "https://airbg.org/news/feed.xml" {
			self = true
		}
	}
	if !self {
		t.Errorf("no rel=self link: %+v", f.Links)
	}
	if len(f.Entries) != 2 {
		t.Fatalf("entries = %d, want the 2 Bulgarian public posts", len(f.Entries))
	}
	first := f.Entries[0]
	if first.ID != "https://airbg.org/news/2026-09-sofia" || first.Link.Href != first.ID {
		t.Errorf("first entry id/link = %q / %q", first.ID, first.Link.Href)
	}
	if first.Title != "Въздухът в София" || first.Summary == "" {
		t.Errorf("first entry = %+v", first)
	}
	for name, v := range map[string]string{"published": first.Published, "updated": first.Updated, "feed updated": f.Updated} {
		if _, err := time.Parse(time.RFC3339, v); err != nil {
			t.Errorf("%s %q: %v", name, v, err)
		}
	}
	if !strings.HasPrefix(first.Updated, "2026-10-02") || !strings.HasPrefix(first.Published, "2026-09-30") {
		t.Errorf("dates published=%q updated=%q", first.Published, first.Updated)
	}
	if f.Updated != first.Updated {
		t.Errorf("feed updated %q != newest entry %q", f.Updated, first.Updated)
	}
	if first.Content.Type != "html" || !strings.Contains(first.Content.Text, "<h2") {
		t.Errorf("content = %+v", first.Content)
	}
	if first.Author.Name == "" {
		t.Errorf("entry has no author")
	}
}

func TestNewsFeedExcludesDraftFutureAndOtherLanguages(t *testing.T) {
	for path, want := range map[string][]string{
		"/news/feed.xml":    {"2026-09-sofia", "2026-07-nota"},
		"/en/news/feed.xml": {"2026-09-sofia", "2026-08-note"},
	} {
		f, _ := fetchFeed(t, path)
		var got []string
		for _, e := range f.Entries {
			got = append(got, e.ID[strings.LastIndex(e.ID, "/")+1:])
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s entries = %v, want %v", path, got, want)
		}
	}
}

func TestNewsFeedEntryIDsAreStableAcrossRequests(t *testing.T) {
	rr := newsRenderer(t)
	a, b := fetch(t, rr, "/news/feed.xml").Body.String(), fetch(t, rr, "/news/feed.xml").Body.String()
	if a != b {
		t.Error("feed differs between two requests with the same clock")
	}
}

func TestNewsPagesAdvertiseTheFeed(t *testing.T) {
	rr := newsRenderer(t)
	for path, href := range map[string]string{
		"/news":                 "https://airbg.org/news/feed.xml",
		"/news/2026-09-sofia":   "https://airbg.org/news/feed.xml",
		"/en/news":              "https://airbg.org/en/news/feed.xml",
		"/en/news/2026-08-note": "https://airbg.org/en/news/feed.xml",
	} {
		want := `<link rel="alternate" type="application/atom+xml" href="` + href + `"`
		if !strings.Contains(fetch(t, rr, path).Body.String(), want) {
			t.Errorf("%s: no feed autodiscovery link %s", path, href)
		}
	}
	if strings.Contains(fetch(t, rr, "/about").Body.String(), "application/atom+xml") {
		t.Errorf("/about advertises the feed")
	}
}

func TestNewsPostJSONLDIsAnArticleWithBreadcrumbs(t *testing.T) {
	body := fetch(t, newsRenderer(t), "/news/2026-09-sofia").Body.String()
	doc := jsonLD(t, "/news/2026-09-sofia", body)

	art := nodeOfType(doc, "Article")
	if art == nil {
		t.Fatal("no Article node")
	}
	if art.InLanguage != "bg" {
		t.Errorf("article = %+v", art)
	}
	if art.DatePublished != "2026-09-30T00:00:00+03:00" || art.DateModified != "2026-10-02T00:00:00+03:00" {
		t.Errorf("dates published=%q modified=%q", art.DatePublished, art.DateModified)
	}
	if art.Headline != "Въздухът в София" || art.MainEntityOfPage != "https://airbg.org/news/2026-09-sofia" {
		t.Errorf("headline/mainEntityOfPage = %q / %q", art.Headline, art.MainEntityOfPage)
	}
	if art.Author == nil || art.Author.ID != "https://airbg.org/#controller" || art.Publisher == nil || art.Publisher.ID != "https://airbg.org/#org" {
		t.Errorf("author/publisher = %+v / %+v", art.Author, art.Publisher)
	}
	if art.Image == "" {
		t.Errorf("article has no image")
	}
	if nodeOfType(doc, "Person") == nil || nodeOfType(doc, "Organization") == nil {
		t.Errorf("author and publisher nodes are not in the graph")
	}
	bc := nodeOfType(doc, "BreadcrumbList")
	if bc == nil || len(bc.ItemListElement) != 3 {
		t.Fatalf("breadcrumb = %+v", bc)
	}
	if bc.ItemListElement[1].Name != "Новини" || bc.ItemListElement[1].Item != "https://airbg.org/news" ||
		bc.ItemListElement[2].Item != "https://airbg.org/news/2026-09-sofia" {
		t.Errorf("breadcrumb items = %+v", bc.ItemListElement)
	}
}

func TestNewsPostJSONLDUsesTheRequestLanguage(t *testing.T) {
	body := fetch(t, newsRenderer(t), "/en/news/2026-09-sofia").Body.String()
	art := nodeOfType(jsonLD(t, "/en/news/2026-09-sofia", body), "Article")
	if art == nil || art.InLanguage != "en" || art.DateModified != "2026-09-30T00:00:00+03:00" {
		t.Errorf("article = %+v", art)
	}
}

func TestNewsPostEscapesTitleAndNeverRendersRawHTML(t *testing.T) {
	fsys := fstest.MapFS{"2026-09-x.bg.md": newsFile(`"<script>alert(1)</script>"`, "2026-09-01", "", "published", "<script>alert(2)</script>\n\nТекст.")}
	set, err := news.Load(fsys, news.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rr := renderer(t, fixture(t))
	rr.SetNews(set, newsClock)
	body := fetch(t, rr, "/news/2026-09-x").Body.String()
	if strings.Contains(body, "<script>alert") {
		t.Errorf("page contains an unescaped script")
	}
}
