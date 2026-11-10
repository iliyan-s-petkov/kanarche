package web_test

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/httpx"
	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
)

// badgeSVG is the subset of the SVG the tests read back.
type badgeSVG struct {
	XMLName xml.Name `xml:"svg"`
	Width   string   `xml:"width,attr"`
	Height  string   `xml:"height,attr"`
	Role    string   `xml:"role,attr"`
	Label   string   `xml:"aria-label,attr"`
	Title   string   `xml:"title"`
	Texts   []string `xml:"text"`
}

func badgeSnapshot() *snapshot.Snapshot {
	return &snapshot.Snapshot{
		GeneratedAt: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
		KnownSlugs: map[string]snapshot.AreaMeta{
			"sofia": {Slug: "sofia", Kind: "oblast", NameBG: "София", NameEN: "Sofia",
				Covered: true, SensorCount: 12, Values: map[string]float64{"P2": 12.4}},
			"vidin": {Slug: "vidin", Kind: "oblast", NameBG: "Видин", NameEN: "Vidin",
				Covered: true, SensorCount: 3, Values: map[string]float64{"P1": 30}},
			"odd": {Slug: "odd", Kind: "oblast", NameBG: "A&B <x>", NameEN: "A&B <x>",
				Covered: true, SensorCount: 3, Values: map[string]float64{"P2": 3}},
			"sofia-grad-oblast": {Slug: "sofia-grad-oblast", Kind: "oblast", NameBG: "София-град", NameEN: "Sofia City",
				Covered: true, SensorCount: 3, Values: map[string]float64{"P2": 20}},
		},
	}
}

func badgeFetch(t *testing.T, snap *snapshot.Snapshot, path string) *httptest.ResponseRecorder {
	t.Helper()
	return fetch(t, renderer(t, snap), path)
}

func parseBadge(t *testing.T, rec *httptest.ResponseRecorder) badgeSVG {
	t.Helper()
	var s badgeSVG
	if err := xml.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("badge is not valid XML: %v\n%s", err, rec.Body.String())
	}
	return s
}

func TestBadgeIsValidSVGWithBandWordAndValueInBothLanguages(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ path, lang, name, value string }{
		{"/badge/sofia.svg", "bg", "София", "12,4"},
		{"/en/badge/sofia.svg", "en", "Sofia", "12.4"},
	}
	for _, c := range cases {
		rec := badgeFetch(t, badgeSnapshot(), c.path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d, want 200", c.path, rec.Code)
		}
		s := parseBadge(t, rec)
		if s.Width != "200" || s.Height != "40" || s.Role != "img" {
			t.Errorf("%s: svg attrs = %q x %q role %q", c.path, s.Width, s.Height, s.Role)
		}
		band := cat.T(c.lang, "band.fair")
		alt := strings.NewReplacer("{name}", c.name, "{band}", band, "{value}", c.value).
			Replace(cat.T(c.lang, "badge.alt"))
		if s.Title != alt || s.Label != alt {
			t.Errorf("%s: title %q / aria-label %q, want %q", c.path, s.Title, s.Label, alt)
		}
		all := strings.Join(s.Texts, "|")
		for _, want := range []string{c.name, c.value, band, cat.T(c.lang, "metric.P2"), "µg/m³", cat.T(c.lang, "seo.title_brand")} {
			if !strings.Contains(all, want) {
				t.Errorf("%s: texts %q lack %q", c.path, all, want)
			}
		}
	}
}

// badgeStack serves the routes behind the same security middleware production uses.
func badgeStack(t *testing.T, snap *snapshot.Snapshot) http.Handler {
	t.Helper()
	cfg := testConfig(t)
	return httpx.SecurityHeaders(renderer(t, snap).Routes(), cfg.Listen.CSP, cfg.Listen.PermissionsPolicy)
}

func TestBadgeHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	badgeStack(t, badgeSnapshot()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/badge/sofia.svg", nil))
	want := map[string]string{
		"Content-Type":                 "image/svg+xml",
		"X-Content-Type-Options":       "nosniff",
		"Cross-Origin-Resource-Policy": "cross-origin",
		"Cache-Control":                "public, max-age=300, s-maxage=300",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); !strings.HasPrefix(got, v) {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

// The relaxation is for the image only: every other route, and a badge miss, keeps same-origin.
func TestBadgeCORPIsCrossOriginOnlyOnTheBadge(t *testing.T) {
	h := badgeStack(t, badgeSnapshot())
	for _, path := range []string{"/", "/en/", "/area/sofia", "/open-data", "/embed", "/api/v1/areas", "/badge/nope.svg", "/en/badge/nope.svg", "/badge/sofia.png"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("Cross-Origin-Resource-Policy"); got != "same-origin" {
			t.Errorf("%s: CORP = %q, want same-origin", path, got)
		}
	}
	for _, path := range []string{"/badge/sofia.svg", "/en/badge/sofia.svg", "/badge/vidin.svg"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("Cross-Origin-Resource-Policy"); got != "cross-origin" {
			t.Errorf("%s: CORP = %q, want cross-origin", path, got)
		}
	}
}

func TestBadgeUnknownSlugIs404AndNotSVG(t *testing.T) {
	for _, path := range []string{"/badge/nope.svg", "/en/badge/nope.svg", "/badge/sofia", "/badge/sofia.png"} {
		rec := badgeFetch(t, badgeSnapshot(), path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, rec.Code)
		}
		if strings.HasPrefix(rec.Header().Get("Content-Type"), "image/svg+xml") {
			t.Errorf("%s: 404 must not be an SVG", path)
		}
	}
}

func TestBadgeRetiredSlugRedirects(t *testing.T) {
	for path, dest := range map[string]string{
		"/badge/sofiya-grad-oblast.svg":    "/badge/sofia-grad-oblast.svg",
		"/en/badge/sofiya-grad-oblast.svg": "/en/badge/sofia-grad-oblast.svg",
	} {
		rec := badgeFetch(t, badgeSnapshot(), path)
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != dest {
			t.Errorf("%s = %d to %q, want 301 to %q", path, rec.Code, rec.Header().Get("Location"), dest)
		}
	}
}

func TestBadgeNoDataFallbackIsSVGWithShortCache(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		snap *snapshot.Snapshot
		path string
		lang string
	}{
		{"no snapshot", nil, "/badge/sofia.svg", "bg"},
		{"no snapshot en", nil, "/en/badge/sofia.svg", "en"},
		{"no P2 value", badgeSnapshot(), "/badge/vidin.svg", "bg"},
		{"no P2 value en", badgeSnapshot(), "/en/badge/vidin.svg", "en"},
	}
	for _, c := range cases {
		rec := badgeFetch(t, c.snap, c.path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", c.name, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/svg+xml") {
			t.Errorf("%s: Content-Type = %q", c.name, ct)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=60" {
			t.Errorf("%s: Cache-Control = %q, want public, max-age=60", c.name, cc)
		}
		if got := rec.Header().Get("Cross-Origin-Resource-Policy"); got != "cross-origin" {
			t.Errorf("%s: CORP = %q", c.name, got)
		}
		s := parseBadge(t, rec)
		noData := cat.T(c.lang, "badge.no_data")
		if !strings.Contains(strings.Join(s.Texts, "|"), noData) || s.Title != noData || s.Label != noData {
			t.Errorf("%s: texts %v title %q label %q, want %q", c.name, s.Texts, s.Title, s.Label, noData)
		}
	}
}

func TestBadgeEscapesTheAreaName(t *testing.T) {
	rec := badgeFetch(t, badgeSnapshot(), "/en/badge/odd.svg")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "A&B <x>") || strings.Contains(body, "<x>") {
		t.Errorf("name written unescaped:\n%s", body)
	}
	s := parseBadge(t, rec)
	if !strings.Contains(strings.Join(s.Texts, "|"), "A&B <x>") || !strings.Contains(s.Title, "A&B <x>") {
		t.Errorf("escaped name does not round-trip: %v / %q", s.Texts, s.Title)
	}
}

func TestBadgeIsNotInTheSitemap(t *testing.T) {
	rec := badgeFetch(t, badgeSnapshot(), "/sitemap.xml")
	if strings.Contains(rec.Body.String(), "/badge/") {
		t.Error("sitemap lists a badge")
	}
}

func TestOpenDataShowsBadgeSectionAndSnippet(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	snap := badgeSnapshot()
	snap.KnownSlugs["plovdiv-oblast"] = snapshot.AreaMeta{Slug: "plovdiv-oblast", Kind: "oblast",
		NameBG: "Пловдив", NameEN: "Plovdiv", Covered: true, SensorCount: 5, Values: map[string]float64{"P2": 12.4}}
	rr := renderer(t, snap)
	cfg := testConfig(t)
	cases := []struct{ path, lang, prefix, name, value string }{
		{"/open-data", "bg", "", "Пловдив", "12,4"},
		{"/en/open-data", "en", "/en", "Plovdiv", "12.4"},
	}
	for _, c := range cases {
		body := fetch(t, rr, c.path).Body.String()
		for _, key := range []string{"opendata.badge.heading", "opendata.badge.body"} {
			if !strings.Contains(body, htmlEscape(cat.T(c.lang, key))) {
				t.Errorf("%s: %s not rendered", c.path, key)
			}
		}
		alt := strings.NewReplacer("{name}", c.name, "{band}", cat.T(c.lang, "band.fair"), "{value}", c.value).
			Replace(cat.T(c.lang, "badge.alt"))
		want := `&lt;a href="` + cfg.Listen.BaseURL + c.prefix + `/area/plovdiv-oblast"&gt;&lt;img src="` +
			cfg.Listen.BaseURL + c.prefix + `/badge/plovdiv-oblast.svg" alt="` + htmlEscape(alt) + `" width="200" height="40"&gt;&lt;/a&gt;`
		if !strings.Contains(body, want) {
			t.Errorf("%s: snippet missing, want %q", c.path, want)
		}
	}
}

func htmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;").Replace(s)
}
