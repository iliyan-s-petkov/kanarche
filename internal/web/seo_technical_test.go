package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Trailing-slash URLs that name a real route 301 to the no-slash form; the
// language homes keep their slash.
func TestTrailingSlashRedirectsToCanonical(t *testing.T) {
	rr := renderer(t, fixture(t))
	for from, to := range map[string]string{
		"/area/sofia/":     "/area/sofia",
		"/about/":          "/about",
		"/about-the-data/": "/about-the-data",
		"/areas/":          "/areas",
		"/en/about/":       "/en/about",
		"/en/area/sofia/":  "/en/area/sofia",
		"/en/areas/":       "/en/areas",
		"/en":              "/en/",
	} {
		rec := fetch(t, rr, from)
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("GET %s = %d, want 301", from, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != to {
			t.Errorf("GET %s Location = %q, want %q", from, got, to)
		}
	}
}

func TestTrailingSlashRedirectKeepsQueryAndNoLoops(t *testing.T) {
	rr := renderer(t, fixture(t))
	rec := fetch(t, rr, "/areas/?x=1")
	if got := rec.Header().Get("Location"); got != "/areas?x=1" {
		t.Errorf("Location = %q, want /areas?x=1", got)
	}
	// Every redirect target must itself be a final answer, never another redirect.
	for _, from := range []string{"/areas/", "/en/about/", "/en", "/area/sofia/"} {
		to := fetch(t, rr, from).Header().Get("Location")
		if rec := fetch(t, rr, to); rec.Code != http.StatusOK {
			t.Errorf("redirect target %s of %s = %d, want 200", to, from, rec.Code)
		}
	}
	for _, path := range []string{"/", "/en/"} {
		if rec := fetch(t, rr, path); rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200 (language homes keep their slash)", path, rec.Code)
		}
	}
}

// A slash on a path that is not a page stays a 404, and static directories
// are not redirected into the file server.
func TestTrailingSlashOnUnknownOrStaticPathStays404(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, path := range []string{"/nonsense/", "/xx/", "/static/", "/static/build/"} {
		rec := fetch(t, rr, path)
		if rec.Code == http.StatusMovedPermanently {
			t.Errorf("GET %s = 301 to %s, want no redirect", path, rec.Header().Get("Location"))
		}
	}
	if rec := fetch(t, rr, "/nonsense/"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /nonsense/ = %d, want 404", rec.Code)
	}
}

func TestFaviconICORedirectsToTheSVG(t *testing.T) {
	rr := renderer(t, fixture(t))
	rec := fetch(t, rr, "/favicon.ico")
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("GET /favicon.ico = %d, want 301", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/static/favicon.svg") {
		t.Fatalf("Location = %q, want the static favicon.svg", loc)
	}
	if got := fetch(t, rr, loc); got.Code != http.StatusOK {
		t.Errorf("GET %s = %d, want 200", loc, got.Code)
	}
}

// lastmod is the snapshot day only where the page content follows the data:
// covered areas, the home and the directory. Static pages and areas without
// data carry none, rather than a date nothing supports.
func TestSitemapLastModOnlyWhereDataSupportsIt(t *testing.T) {
	doc := fetchSitemap(t)
	want := map[string]string{
		"https://airbg.org/":                  "2026-08-09",
		"https://airbg.org/areas":             "2026-08-09",
		"https://airbg.org/area/sofia":        "2026-08-09",
		"https://airbg.org/en/area/sofia":     "2026-08-09",
		"https://airbg.org/about":             "",
		"https://airbg.org/about-the-data":    "",
		"https://airbg.org/en/about":          "",
		"https://airbg.org/en/about-the-data": "",
	}
	seen := 0
	for _, u := range doc.URLs {
		if w, ok := want[u.Loc]; ok {
			seen++
			if u.LastMod != w {
				t.Errorf("%s: lastmod = %q, want %q", u.Loc, u.LastMod, w)
			}
		}
	}
	if seen != len(want) {
		t.Errorf("checked %d of %d expected sitemap URLs", seen, len(want))
	}
}

func TestAreasPageHasItsOwnH1(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path, h1 := range map[string]string{
		"/areas":    "Области и градове",
		"/en/areas": "Areas and cities",
	} {
		rec := httptest.NewRecorder()
		rr.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		want := `<h1 class="t-title">` + h1 + `</h1>`
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: want %s", path, want)
		}
	}
}

func TestWebSiteNameIsTheBrand(t *testing.T) {
	rr := renderer(t, jsonLDFixture(t))
	for path, want := range map[string][2]string{"/": {"Канарче", "канарче"}, "/en/": {"Kanarche", "kanarche"}} {
		site := nodeOfType(jsonLD(t, path, fetch(t, rr, path).Body.String()), "WebSite")
		if site == nil || site.Name != want[0] || site.AlternateName != want[1] {
			t.Errorf("%s: WebSite name/alternateName = %+v, want %v", path, site, want)
		}
	}
}
