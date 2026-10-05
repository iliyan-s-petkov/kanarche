package web_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"airbg.org/internal/config"
	"airbg.org/internal/i18n"
	"airbg.org/internal/snapshot"
	"airbg.org/internal/web"
)

// cityFixture adds prod-style transliterated city slugs: five cities plus an
// oblast whose sensor count is the highest, which the footer must not list.
func cityFixture(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	snap := fixture(t)
	add := func(slug, kind, bg, en string, sensors int) {
		snap.KnownSlugs[slug] = snapshot.AreaMeta{Slug: slug, Kind: kind, NameBG: bg, NameEN: en,
			CentroidLon: 24, CentroidLat: 42, DefaultZoom: 11, Covered: true, SensorCount: sensors}
	}
	add("sofiya", "city", "София", "Sofia", 90)
	add("plovdiv", "city", "Пловдив", "Plovdiv", 40)
	add("varna", "city", "Варна", "Varna", 30)
	add("burgas", "city", "Бургас", "Burgas", 20)
	add("ruse", "city", "Русе", "Ruse", 5)
	add("plovdiv-oblast", "oblast", "Пловдив (област)", "Plovdiv Province", 200)
	return snap
}

// rendererWithSocial builds a renderer whose footer social URLs are configured.
func rendererWithSocial(t *testing.T, facebook, linkedin string) *web.Renderer {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	cfg := testConfig(t)
	cfg.Social = config.Social{FacebookURL: facebook, LinkedInURL: linkedin}
	h := snapshot.NewHolder(cfg.Series, config.Wind{})
	h.Store(cityFixture(t))
	rr, err := web.NewRenderer(cat, h, cfg)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	return rr
}

// footerOf returns the rendered <footer>...</footer> block of a page.
func footerOf(t *testing.T, body string) string {
	t.Helper()
	open := strings.Index(body, `<footer class="footer">`)
	shut := strings.Index(body, "</footer>")
	if open < 0 || shut < open {
		t.Fatalf("no footer in page (open=%d shut=%d)", open, shut)
	}
	return body[open : shut+len("</footer>")]
}

func TestFooterHasThreeLabelledNavsEachWithAHeading(t *testing.T) {
	rr := renderer(t, cityFixture(t))
	// Each nav is named by its own h2 (aria-labelledby), not by a duplicate aria-label.
	navRe := regexp.MustCompile(`<nav [^>]*aria-labelledby="(footer-[a-z]+-h)">\s*<h2 [^>]*id="footer-[a-z]+-h"`)
	for _, path := range []string{"/", "/en/", "/about"} {
		foot := footerOf(t, fetch(t, rr, path).Body.String())
		if n := len(navRe.FindAllString(foot, -1)); n != 3 {
			t.Errorf("%s: footer has %d labelled navs with an h2, want 3", path, n)
		}
		for _, m := range navRe.FindAllStringSubmatch(foot, -1) {
			if !strings.Contains(m[0], `id="`+m[1]+`"`) {
				t.Errorf("%s: nav labelledby %s does not point at its h2", path, m[1])
			}
		}
		if strings.Contains(foot, "<nav class=\"footer__col\" aria-label=") {
			t.Errorf("%s: a footer nav duplicates its h2 in aria-label", path)
		}
		if strings.Count(foot, "<nav") != 3 {
			t.Errorf("%s: footer has %d navs, want 3", path, strings.Count(foot, "<nav"))
		}
	}
}

func TestFooterKeepsTheLanguagePickerInTheMasthead(t *testing.T) {
	foot := footerOf(t, fetch(t, renderer(t, cityFixture(t)), "/en/").Body.String())
	if strings.Contains(foot, "langpick") || strings.Contains(foot, `hreflang=`) {
		t.Error("the footer carries a language picker; it belongs to the masthead only")
	}
}

func TestFooterExploreColumnUsesTheSnapshotsCitySlugs(t *testing.T) {
	rr := renderer(t, cityFixture(t))
	for _, tc := range []struct{ path, prefix string }{{"/", ""}, {"/en/", "/en"}} {
		foot := footerOf(t, fetch(t, rr, tc.path).Body.String())
		for _, slug := range []string{"sofiya", "plovdiv", "varna", "burgas"} {
			if want := `href="` + tc.prefix + `/area/` + slug + `"`; !strings.Contains(foot, want) {
				t.Errorf("%s: footer lacks %s", tc.path, want)
			}
		}
		for _, slug := range []string{"ruse", "plovdiv-oblast"} {
			if strings.Contains(foot, "/area/"+slug+`"`) {
				t.Errorf("%s: footer lists %s, which is not a top-four city", tc.path, slug)
			}
		}
		if !strings.Contains(foot, `href="`+tc.prefix+`/areas"`) {
			t.Errorf("%s: footer lacks the All areas link", tc.path)
		}
	}
	// Names follow the page language.
	if foot := footerOf(t, fetch(t, rr, "/en/").Body.String()); !strings.Contains(foot, ">Sofia<") {
		t.Error("English footer does not name the city in English")
	}
	if foot := footerOf(t, fetch(t, rr, "/").Body.String()); !strings.Contains(foot, ">София<") {
		t.Error("Bulgarian footer does not name the city in Bulgarian")
	}
}

func TestFooterWithoutASnapshotStillRenders(t *testing.T) {
	foot := footerOf(t, fetch(t, renderer(t, nil), "/about-the-data").Body.String())
	if strings.Contains(foot, "/area/") {
		t.Error("footer lists areas without a snapshot")
	}
	if !strings.Contains(foot, `href="/areas"`) {
		t.Error("footer lost the All areas link without a snapshot")
	}
}

func TestFooterCarriesTheRequiredAttributions(t *testing.T) {
	rr := renderer(t, cityFixture(t))
	for _, path := range []string{"/", "/en/", "/areas", "/area/sofiya", "/about", "/privacy", "/licences", "/en/licences"} {
		foot := footerOf(t, fetch(t, rr, path).Body.String())
		for _, want := range []string{
			"© OpenStreetMap contributors",
			`href="https://www.openstreetmap.org/copyright"`,
			`href="https://maps.sensor.community/"`,
			`href="https://eea.government.bg/kav/"`,
			`href="https://open-meteo.com/"`,
			`href="https://www.eea.europa.eu/en/topics/in-depth/water/bathing-water"`,
			`href="https://hostellation.com/"`,
			"ODbL",
			"CC BY 4.0",
		} {
			if !strings.Contains(foot, want) {
				t.Errorf("%s: footer lacks %q", path, want)
			}
		}
	}
}

func TestFooterExternalLinksSeverTheOpenerAndStayInTheTab(t *testing.T) {
	rr := rendererWithSocial(t, "https://www.facebook.com/airbg", "https://www.linkedin.com/company/airbg")
	foot := footerOf(t, fetch(t, rr, "/").Body.String())
	anchors := regexp.MustCompile(`<a [^>]*href="https?://[^"]+"[^>]*>`).FindAllString(foot, -1)
	if len(anchors) < 8 {
		t.Fatalf("found only %d external anchors in the footer", len(anchors))
	}
	for _, a := range anchors {
		if !strings.Contains(a, `rel="noopener noreferrer"`) {
			t.Errorf("external footer link without noopener noreferrer: %s", a)
		}
		if strings.Contains(a, "target=") {
			t.Errorf("footer link opens a new tab: %s", a)
		}
	}
}

func TestFooterProjectColumnLinks(t *testing.T) {
	foot := footerOf(t, fetch(t, renderer(t, cityFixture(t)), "/en/").Body.String())
	for _, want := range []string{
		`href="/en/about"`, `href="/en/about-the-data"`, `href="/en/privacy"`, `href="/en/licences"`,
		`href="https://github.com/iliyan-s-petkov/kanarche"`,
		`href="https://github.com/iliyan-s-petkov/kanarche/issues"`,
	} {
		if !strings.Contains(foot, want) {
			t.Errorf("footer lacks %s", want)
		}
	}
}

func TestFooterSocialIconsFollowTheConfig(t *testing.T) {
	// Unconfigured: GitHub only.
	foot := footerOf(t, fetch(t, renderer(t, cityFixture(t)), "/en/").Body.String())
	if !strings.Contains(foot, `aria-label="Kanarche on GitHub"`) {
		t.Error("GitHub icon missing or unnamed")
	}
	for _, absent := range []string{"Kanarche on Facebook", "Kanarche on LinkedIn", "facebook.com", "linkedin.com"} {
		if strings.Contains(foot, absent) {
			t.Errorf("unconfigured footer renders %q", absent)
		}
	}

	// Configured: all three, each an inline SVG with an accessible name.
	rr := rendererWithSocial(t, "https://www.facebook.com/airbg", "https://www.linkedin.com/company/airbg")
	foot = footerOf(t, fetch(t, rr, "/en/").Body.String())
	for _, want := range []string{
		`href="https://www.facebook.com/airbg"`, `aria-label="Kanarche on Facebook"`,
		`href="https://www.linkedin.com/company/airbg"`, `aria-label="Kanarche on LinkedIn"`,
	} {
		if !strings.Contains(foot, want) {
			t.Errorf("configured footer lacks %s", want)
		}
	}
	if n := strings.Count(foot, `class="footer__social-link"`); n != 3 {
		t.Errorf("footer has %d social links, want 3", n)
	}

	// Only one configured: only that one renders.
	only := rendererWithSocial(t, "", "https://www.linkedin.com/company/airbg")
	foot = footerOf(t, fetch(t, only, "/en/").Body.String())
	if strings.Contains(foot, "Facebook") || !strings.Contains(foot, "LinkedIn") {
		t.Error("a single configured network must render alone")
	}
}

func TestFooterBottomBar(t *testing.T) {
	foot := footerOf(t, fetch(t, renderer(t, cityFixture(t)), "/en/").Body.String())
	year := fmt.Sprint(time.Now().Year())
	for _, want := range []string{"© " + year + " Kanarche", `href="/en/privacy"`, `href="/en/licences"`, "Hostellation"} {
		if !strings.Contains(foot, want) {
			t.Errorf("bottom bar lacks %q", want)
		}
	}
	if n := strings.Count(foot, `href="/en/licences"`); n != 1 {
		t.Errorf("Licences is linked %d times in the footer, want once, in the bottom bar", n)
	}
}

func TestFooterStatesTheShortDisclaimerAndTagline(t *testing.T) {
	foot := footerOf(t, fetch(t, renderer(t, cityFixture(t)), "/en/").Body.String())
	for _, want := range []string{"footer__brand", "Air quality in Bulgaria, live.", "not reference-method"} {
		if !strings.Contains(foot, want) {
			t.Errorf("brand column lacks %q", want)
		}
	}
}

func TestPrivacyAndLicencesRoutes(t *testing.T) {
	rr := renderer(t, cityFixture(t))
	for _, path := range []string{"/privacy", "/en/privacy", "/licences", "/en/licences"} {
		rec := fetch(t, rr, path)
		if rec.Code != 200 {
			t.Errorf("%s status = %d, want 200", path, rec.Code)
		}
		body := rec.Body.String()
		if strings.Count(body, "<h1") != 1 {
			t.Errorf("%s has %d h1, want 1", path, strings.Count(body, "<h1"))
		}
		if !strings.Contains(body, `<link rel="canonical"`) || !strings.Contains(body, `hreflang="en"`) {
			t.Errorf("%s lacks canonical or hreflang", path)
		}
	}
	// No snapshot: the pages are static prose and must still render.
	for _, path := range []string{"/privacy", "/en/licences"} {
		if rec := fetch(t, renderer(t, nil), path); rec.Code != 200 {
			t.Errorf("%s without a snapshot = %d, want 200", path, rec.Code)
		}
	}
}

func TestLicencesPageListsEverySourceAndTheCodeLicence(t *testing.T) {
	body := fetch(t, renderer(t, cityFixture(t)), "/en/licences").Body.String()
	for _, want := range []string{
		"sensor.community", "ODbL 1.0", "https://opendatacommons.org/licenses/odbl/1-0/",
		"https://maps.sensor.community/",
		"https://eea.government.bg/kav/",
		"Open-Meteo", "CC BY 4.0", "https://creativecommons.org/licenses/by/4.0/",
		"https://www.eea.europa.eu/en/topics/in-depth/water/bathing-water",
		"© OpenStreetMap contributors", "https://www.openstreetmap.org/copyright",
		"MIT", "https://github.com/iliyan-s-petkov/kanarche/blob/master/LICENSE",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("licences page lacks %q", want)
		}
	}
	licence, err := os.ReadFile(filepath.Join("..", "..", "LICENSE"))
	if err != nil || !strings.HasPrefix(string(licence), "MIT License") {
		t.Fatalf("LICENSE is no longer MIT (%v); the licences page says MIT", err)
	}
}

func TestPrivacyPageStatesOnlyWhatTheCodeDoes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("static", "storage-keys.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Only the keys the app writes; "legacy" ones are read once for migration.
	var keys struct {
		LocalStorage []string `json:"localStorage"`
	}
	if err := json.Unmarshal(raw, &keys); err != nil || len(keys.LocalStorage) == 0 {
		t.Fatalf("storage-keys.json: no localStorage keys (%v)", err)
	}
	for _, path := range []string{"/privacy", "/en/privacy"} {
		body := fetch(t, renderer(t, cityFixture(t)), path).Body.String()
		for _, key := range keys.LocalStorage {
			if !strings.Contains(body, "<code>"+key+"</code>") {
				t.Errorf("%s does not list storage key %s", path, key)
			}
		}
		// The raster basemap is the whole map's ground at every zoom, over Bulgaria too.
		for _, bad := range []string{"outside Bulgaria", "извън България"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s says the OSM tiles load only %q; the raster is the ground everywhere", path, bad)
			}
		}
		view := "the part of the map you are viewing"
		if path == "/privacy" {
			view = "частта от картата, която разглеждате"
		}
		for _, want := range []string{"tile.openstreetmap.org", "Cloudflare", `data-island="clearsettings"`, view} {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %q", path, want)
			}
		}
	}
}

func TestSitemapListsPrivacyAndLicencesWithHreflang(t *testing.T) {
	doc := fetchSitemap(t)
	for _, loc := range []string{
		"https://airbg.org/privacy", "https://airbg.org/en/privacy",
		"https://airbg.org/licences", "https://airbg.org/en/licences",
	} {
		found := false
		for _, u := range doc.URLs {
			if u.Loc != loc {
				continue
			}
			found = true
			langs := map[string]bool{}
			for _, l := range u.Links {
				langs[l.Hreflang] = true
			}
			for _, want := range []string{"bg", "en", "x-default"} {
				if !langs[want] {
					t.Errorf("%s lacks hreflang %s", loc, want)
				}
			}
		}
		if !found {
			t.Errorf("sitemap is missing %s", loc)
		}
	}
}

// airbg.info is the Bulgarian sensor.community volunteers' site: independent,
// so the pages link to it and claim no affiliation.
func TestAirbgInfoIsLinkedFromTheFooterAndAboutAsIndependent(t *testing.T) {
	rr := renderer(t, cityFixture(t))
	for _, path := range []string{"/", "/en/"} {
		foot := footerOf(t, fetch(t, rr, path).Body.String())
		if !strings.Contains(foot, `href="https://airbg.info/" rel="noopener noreferrer"`) {
			t.Errorf("%s: footer lacks the airbg.info link", path)
		}
	}
	for _, tc := range []struct{ path, note string }{
		{"/about", "Канарче не е свързано с airbg.info"},
		{"/en/about", "Kanarche is not affiliated with airbg.info"},
	} {
		body := fetch(t, rr, tc.path).Body.String()
		for _, href := range []string{
			"https://airbg.info/naprawi-si-stancia",
			"https://airbg.info/zaqwi-uchastie",
			"https://maps.sensor.community",
		} {
			if !strings.Contains(body, `href="`+href+`" rel="noopener noreferrer"`) {
				t.Errorf("%s lacks the %s link", tc.path, href)
			}
		}
		if i, j := strings.Index(body, `id="station"`), strings.Index(body, `id="more"`); i < 0 || j < i || strings.Contains(stationProse(body[i:j]), "sensor.community") {
			t.Errorf("%s: the station intro and note must not tie airbg.info to sensor.community", tc.path)
		}
		if !strings.Contains(body, `id="station"`) || !strings.Contains(body, tc.note) {
			t.Errorf("%s lacks the station section or its independence note %q", tc.path, tc.note)
		}
	}
}

// The title and description may only claim what stays true: OSM tile servers see the IP.
func TestPrivacyMetaDoesNotClaimNoIPAddresses(t *testing.T) {
	for _, path := range []string{"/privacy", "/en/privacy"} {
		body := fetch(t, renderer(t, cityFixture(t)), path).Body.String()
		head := body[:strings.Index(body, "</head>")]
		for _, bad := range []string{"IP addresses", "IP адреси"} {
			if strings.Contains(head, bad) {
				t.Errorf("%s: title or description claims %q", path, bad)
			}
		}
	}
}

// stationProse is the station section minus its cards: the sensor.community
// map is a card of its own, but the prose must not tie airbg.info to it.
func stationProse(section string) string {
	var out strings.Builder
	for _, p := range regexp.MustCompile(`(?s)<p(?: [^>]*)?>.*?</p>`).FindAllString(section, -1) {
		out.WriteString(p)
	}
	return out.String()
}
