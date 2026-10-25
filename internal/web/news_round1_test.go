package web_test

import (
	"encoding/json"
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"kanarche.eu/content"
	"kanarche.eu/internal/news"
	"kanarche.eu/internal/web"
)

// rendererWithPosts serves exactly the given posts at newsClock.
func rendererWithPosts(t *testing.T, fsys fstest.MapFS) *web.Renderer {
	t.Helper()
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

func TestNewsFeedContentHasNoRelativeLinks(t *testing.T) {
	rr := rendererWithPosts(t, fstest.MapFS{
		"2026-09-rel.en.md": newsFile("Relative", "2026-09-01", "", "published", "See [Sofia](/area/sofia) and ![map](/static/x.png) and [out](https://example.org/a)."),
	})
	body := fetch(t, rr, "/en/news/feed.xml").Body.String()
	var f atomFeed
	if err := xml.Unmarshal([]byte(body), &f); err != nil {
		t.Fatalf("not Atom: %v", err)
	}
	html := f.Entries[0].Content.Text
	for _, bad := range []string{`href="/`, `src="/`} {
		if strings.Contains(html, bad) {
			t.Errorf("feed content has a relative link %q: %s", bad, html)
		}
	}
	if !strings.Contains(html, `href="https://airbg.org/area/sofia"`) || !strings.Contains(html, `href="https://example.org/a"`) {
		t.Errorf("feed content links = %s", html)
	}
}

// A post can carry an updated date after today. It must not reach any output.
func futureUpdatedFS() fstest.MapFS {
	return fstest.MapFS{
		"2026-09-fut.en.md": newsFile("Future update", "2026-09-01", "updated: 2027-05-01\n", "published", "Body."),
	}
}

func TestNewsFutureUpdatedNeverLeaks(t *testing.T) {
	rr := rendererWithPosts(t, futureUpdatedFS())
	post := fetch(t, rr, "/en/news/2026-09-fut").Body.String()
	if strings.Contains(post, "2027") {
		t.Errorf("post page leaks the future updated date")
	}
	for name, path := range map[string]string{"feed": "/en/news/feed.xml", "sitemap": "/sitemap.xml", "list": "/en/news"} {
		if body := fetch(t, rr, path).Body.String(); strings.Contains(body, "2027") {
			t.Errorf("%s leaks the future updated date", name)
		}
	}
	if !strings.Contains(post, `"dateModified":"2026-10-25`) {
		t.Errorf("dateModified not clamped to the clock day: %s", regexp.MustCompile(`"dateModified":"[^"]*"`).FindString(post))
	}
	sm := fetch(t, rr, "/sitemap.xml").Body.String()
	if !regexp.MustCompile(`2026-09-fut</loc>\s*<lastmod>2026-10-25</lastmod>`).MatchString(sm) {
		t.Errorf("sitemap lastmod not clamped to 2026-10-25")
	}
}

func TestNewsEmptyFeedUpdatedIsStable(t *testing.T) {
	updated := func(now time.Time) string {
		rr := rendererWithPosts(t, fstest.MapFS{})
		rr.SetNews(mustEmptySet(t), func() time.Time { return now })
		var f atomFeed
		if err := xml.Unmarshal(fetch(t, rr, "/news/feed.xml").Body.Bytes(), &f); err != nil {
			t.Fatalf("not Atom: %v", err)
		}
		return f.Updated
	}
	a, b := updated(newsClock()), updated(newsClock().Add(48*time.Hour))
	if a == "" || a != b {
		t.Errorf("empty feed updated changes with the clock: %q vs %q", a, b)
	}
}

func mustEmptySet(t *testing.T) *news.Set {
	t.Helper()
	set, err := news.Load(fstest.MapFS{}, news.Options{})
	if err != nil {
		t.Fatalf("news.Load: %v", err)
	}
	return set
}

func TestLLMSTxtListsNewsOnlyForLanguagesWithPublicPosts(t *testing.T) {
	cases := map[string]struct {
		fsys           fstest.MapFS
		wantBG, wantEN bool
	}{
		"none":    {fstest.MapFS{}, false, false},
		"bg only": {fstest.MapFS{"2026-07-a.bg.md": newsFile("A", "2026-07-01", "", "published", "x")}, true, false},
		"en only": {fstest.MapFS{"2026-07-a.en.md": newsFile("A", "2026-07-01", "", "published", "x")}, false, true},
		"draft":   {fstest.MapFS{"2026-07-a.bg.md": newsFile("A", "2026-07-01", "", "draft", "x")}, false, false},
		"both": {fstest.MapFS{
			"2026-07-a.bg.md": newsFile("A", "2026-07-01", "", "published", "x"),
			"2026-07-a.en.md": newsFile("A", "2026-07-01", "", "published", "x"),
		}, true, true},
	}
	for name, c := range cases {
		body := fetch(t, rendererWithPosts(t, c.fsys), "/llms.txt").Body.String()
		if got := strings.Contains(body, "(https://airbg.org/news)"); got != c.wantBG {
			t.Errorf("%s: /news listed = %v, want %v", name, got, c.wantBG)
		}
		if got := strings.Contains(body, "(https://airbg.org/en/news)"); got != c.wantEN {
			t.Errorf("%s: /en/news listed = %v, want %v", name, got, c.wantEN)
		}
	}
}

// Loads the embedded posts the way NewRenderer does, so a bad post fails here
// and not at boot.
func TestEmbeddedNewsContentLoads(t *testing.T) {
	sub, err := fs.Sub(content.News, "news")
	if err != nil {
		t.Fatal(err)
	}
	sofia, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := news.Load(sub, news.Options{Location: sofia}); err != nil {
		t.Fatalf("embedded news does not load: %v", err)
	}
	if _, err := fs.Stat(sub, "_drafts"); err == nil {
		t.Error("_drafts must not be embedded")
	}
}

// boundarySlugs is every area slug the site can serve, read from the committed
// boundary files, so no database is needed.
func boundarySlugs(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("../../data/boundaries/*.geojson")
	if err != nil || len(files) == 0 {
		t.Fatalf("no boundary files: %v", err)
	}
	slugs := map[string]bool{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var fc struct {
			Features []struct {
				Properties struct {
					Slug string `json:"slug"`
				} `json:"properties"`
			} `json:"features"`
		}
		if err := json.Unmarshal(raw, &fc); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, ft := range fc.Features {
			slugs[ft.Properties.Slug] = true
		}
	}
	return slugs
}

func TestNewsAreasExistInTheBoundaryData(t *testing.T) {
	known := boundarySlugs(t)
	if !known["varna"] || !known["sofia"] {
		t.Fatalf("boundary slugs look wrong, varna=%v sofia=%v", known["varna"], known["sofia"])
	}
	sofia, err := time.LoadLocation("Europe/Sofia")
	if err != nil {
		t.Fatal(err)
	}
	opt := news.Options{Location: sofia, KnownArea: func(s string) bool { return known[s] }}
	for _, dir := range []string{"../../content/news", "../../content/news/_drafts"} {
		if _, err := news.Load(os.DirFS(dir), opt); err != nil {
			t.Errorf("%s: %v", dir, err)
		}
		files, _ := filepath.Glob(dir + "/*.md")
		if dir != "../../content/news" && len(files) == 0 {
			t.Errorf("%s: no posts found, the check proves nothing", dir)
		}
	}
}
