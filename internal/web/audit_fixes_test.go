package web_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

var titleRe = regexp.MustCompile(`<title>([^<]*)</title>`)

// S01: the 404 carries the same brand suffix as every other page, no canonical
// and no hreflang alternates, and still answers 404.
func TestNotFoundPageUsesSiteBrandAndNoAlternates(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, path := range []string{"/area/atlantis", "/en/area/atlantis", "/nonexistent"} {
		rec := fetch(t, rr, path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, rec.Code)
		}
		body := rec.Body.String()
		m := titleRe.FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("%s: no <title>", path)
		}
		if brand := testBrand(map[bool]string{true: "/en", false: ""}[strings.HasPrefix(path, "/en")]); !strings.HasSuffix(m[1], " | "+brand) {
			t.Errorf("%s: title %q does not end with the site brand %q", path, m[1], brand)
		}
		for _, unwanted := range []string{`rel="canonical"`, `rel="alternate"`} {
			if strings.Contains(body, unwanted) {
				t.Errorf("%s: the 404 advertises %s", path, unwanted)
			}
		}
	}
}

// The embed's title uses the same brand as the rest of the site.
func TestEmbedTitleUsesSiteBrand(t *testing.T) {
	body := framed(t, renderer(t, fixture(t)), "/embed").Body.String()
	if m := titleRe.FindStringSubmatch(body); m == nil || m[1] != "Канарче" {
		t.Errorf("embed title = %v, want Канарче", m)
	}
}

// S02: the head names the default-language URL as x-default, as the sitemap does.
func TestHeadCarriesXDefaultPointingAtTheDefaultLanguage(t *testing.T) {
	rr := renderer(t, fixture(t))
	cases := map[string]string{
		"/":              "https://airbg.org/",
		"/en/":           "https://airbg.org/",
		"/area/sofia":    "https://airbg.org/area/sofia",
		"/en/area/sofia": "https://airbg.org/area/sofia",
		"/en/about":      "https://airbg.org/about",
	}
	for path, want := range cases {
		body := fetch(t, rr, path).Body.String()
		tag := `<link rel="alternate" hreflang="x-default" href="` + want + `">`
		if !strings.Contains(body, tag) {
			t.Errorf("%s: head is missing %s", path, tag)
		}
	}
}

// I01: every map island hands MapLibre the strings it would otherwise show in
// English, from the catalogue of the page's language.
func TestMapIslandCarriesMapLibreStrings(t *testing.T) {
	rr := renderer(t, fixture(t))
	cases := []struct {
		path, title, toggle string
		framed              bool
	}{
		{"/", "Карта на качеството на въздуха", "Показване на източниците", false},
		{"/en/", "Air quality map", "Toggle attribution", false},
		{"/area/sofia", "Карта на качеството на въздуха", "Показване на източниците", false},
		{"/embed", "Карта на качеството на въздуха", "Показване на източниците", true},
	}
	for _, c := range cases {
		var body string
		if c.framed {
			body = framed(t, rr, c.path).Body.String()
		} else {
			body = fetch(t, rr, c.path).Body.String()
		}
		for _, want := range []string{
			`data-t-map-title="` + c.title + `"`,
			`data-t-attribution-toggle="` + c.toggle + `"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: missing %s", c.path, want)
			}
		}
	}
}

// A03: the embed snippet scrolls sideways, so it has to be reachable and named.
func TestAboutEmbedCodeIsKeyboardScrollable(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path, label := range map[string]string{"/about": "Код за вграждане", "/en/about": "Embed code"} {
		body := fetch(t, rr, path).Body.String()
		want := `<pre class="about-code" dir="ltr" tabindex="0" role="region" aria-label="` + label + `">`
		if !strings.Contains(body, want) {
			t.Errorf("%s: embed <pre> is not focusable and named; want %s", path, want)
		}
	}
}

// S03, map-first: the home page carries one visually hidden product h1 and no visible hero.
func TestHomeHasOneHiddenH1AndNoVisibleHero(t *testing.T) {
	rr := renderer(t, fixture(t))
	cases := map[string]string{
		"/":    "Канарче, качество на въздуха в България: карта на живо",
		"/en/": "Kanarche, Bulgaria air quality map: PM2.5 and PM10 now",
	}
	for path, want := range cases {
		body := fetch(t, rr, path).Body.String()
		h1 := `<h1 class="visually-hidden">` + want + `</h1>`
		if !strings.Contains(body, h1) {
			t.Errorf("%s: want %s", path, h1)
		}
		if n := strings.Count(body, "<h1"); n != 1 {
			t.Errorf("%s: want exactly one h1, got %d", path, n)
		}
		for _, gone := range []string{`class="page-head`, `class="t-title"`, `class="t-sub"`, `class="toolbar"`} {
			if strings.Contains(body, gone) {
				t.Errorf("%s: the visible hero/toolbar is still rendered (%s)", path, gone)
			}
		}
	}
}

// The areas tab is not map-first: it keeps its visible head and toolbar.
func TestAreasTabKeepsVisibleHeadAndToolbar(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/en/areas").Body.String()
	for _, want := range []string{`<h1 class="t-title">Areas and cities</h1>`, `class="toolbar"`} {
		if !strings.Contains(body, want) {
			t.Errorf("/en/areas: want %s", want)
		}
	}
}

// The brand link shows the lowercase wordmark and is named by the product name.
func TestBrandLinkIsNamedByProductName(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/en/").Body.String()
	if !strings.Contains(body, `class="masthead__brand" href="/en/" aria-label="Kanarche">`) || !strings.Contains(body, `alt="">kanarche</a>`) {
		t.Error("brand link lacks aria-label=\"Kanarche\" and the lowercase wordmark")
	}
}

// U08: About and GitHub are hidden from the phone header, so the language menu carries them.
func TestLanguageMenuCarriesAboutAndSourceLinks(t *testing.T) {
	rr := renderer(t, fixture(t))
	cases := map[string]string{"/": `href="/about"`, "/en/": `href="/en/about"`}
	for path, about := range cases {
		body := fetch(t, rr, path).Body.String()
		start := strings.Index(body, `<ul class="langpick__list">`)
		if start < 0 {
			t.Fatalf("%s: no language list", path)
		}
		list := body[start : start+strings.Index(body[start:], "</ul>")]
		for _, want := range []string{about, `href="https://github.com/iliyan-s-petkov/kanarche"`, `rel="noopener noreferrer"`} {
			if !strings.Contains(list, want) {
				t.Errorf("%s: language menu is missing %s", path, want)
			}
		}
	}
}

// The footer's snapshot time keeps a UTC fallback for no-JS and marks itself for the local-time rewrite.
func TestFooterTimeCarriesUTCFallbackAndLocalMarker(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/en/").Body.String()
	want := `<time datetime="2026-08-09T12:00:00Z" data-local-time>2026-08-09 12:00 UTC</time>`
	if !strings.Contains(body, want) {
		t.Errorf("footer time: want %s", want)
	}
}
