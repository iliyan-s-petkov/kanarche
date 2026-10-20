package web_test

import (
	"html"
	"strings"
	"testing"

	"kanarche.eu/internal/i18n"
)

// The PM pages must not present a WHO guideline as a safe level or a legal
// norm: they state that the WHO found no level without harm.
func TestPMPagesStateNoHarmlessLevelAndDropSafeLevelWording(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range pmPages {
		body := fetch(t, rr, c.path).Body.String()
		want := "no level below which there is no harm to health"
		if strings.HasPrefix(c.path, "/en/") == false {
			want = "СЗО не е открила ниво, под което няма вреда за здравето"
		}
		if !strings.Contains(body, want) {
			t.Errorf("%s: missing %q", c.path, want)
		}
		for _, banned := range []string{"safe level", "каква е нормата", "нормата на СЗО"} {
			if strings.Contains(body, banned) {
				t.Errorf("%s: still contains %q", c.path, banned)
			}
		}
	}
}

// Kanarche grants no licence over the RHI (РЗИ) data, so no page may claim a
// CC BY grant for it.
func TestRZIDataCarriesNoCCBYGrant(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := rziRenderer(t, true)
	for _, lang := range []string{"bg", "en"} {
		for _, key := range []string{"licences.rzi.derived", "about.sources.rzi", "licences.rzi.name", "licences.rzi.what", "licences.rzi.unofficial"} {
			if strings.Contains(cat.T(lang, key), "CC BY") {
				t.Errorf("%s %s mentions CC BY", lang, key)
			}
		}
	}
	for path, lang := range rziLangs {
		body := fetch(t, rr, path).Body.String()
		start := strings.Index(body, "<li><strong>"+html.EscapeString(cat.T(lang, "licences.rzi.name")))
		if start < 0 {
			t.Fatalf("%s: no RZI list item", path)
		}
		item := body[start : start+strings.Index(body[start:], "</li>")]
		if strings.Contains(item, "CC BY") {
			t.Errorf("%s: RZI item grants CC BY: %q", path, item)
		}
		if !strings.Contains(item, html.EscapeString(cat.T(lang, "licences.rzi.derived"))) {
			t.Errorf("%s: RZI item lacks the no-licence sentence", path)
		}
	}
	for path, lang := range map[string]string{"/about-the-data": "bg", "/en/about-the-data": "en"} {
		body := fetch(t, rr, path).Body.String()
		sentence := html.EscapeString(cat.T(lang, "about.sources.rzi"))
		i := strings.Index(body, sentence)
		if i < 0 {
			t.Fatalf("%s: no RHI sentence", path)
		}
		if strings.Contains(sentence, "CC BY") {
			t.Errorf("%s: RHI sentence grants CC BY", path)
		}
	}
}

// The footer credits the code author and says the data belongs to its sources,
// and the sensor.community licence label names the contents licence too.
func TestFooterCopyrightAndSensorCommunityLicencePair(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := renderer(t, fixture(t))
	for path, lang := range map[string]string{"/": "bg", "/en/": "en"} {
		foot := footerOf(t, fetch(t, rr, path).Body.String())
		want := html.EscapeString(cat.T(lang, "footer.copyright"))
		if want == "" || !strings.Contains(foot, want) {
			t.Errorf("%s: footer lacks footer.copyright %q", path, want)
		}
		if !strings.Contains(foot, `<span class="footer__lic">ODbL, DbCL</span>`) {
			t.Errorf("%s: footer lacks the ODbL, DbCL label", path)
		}
		if !strings.Contains(foot, `href="`+map[string]string{"bg": "", "en": "/en"}[lang]+`/terms#contact"`) {
			t.Errorf("%s: footer contact does not point at the terms contact section", path)
		}
	}
}

// Address search goes to Nominatim server-side, so the privacy notice says so.
func TestPrivacyPageDescribesAddressSearch(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := renderer(t, fixture(t))
	for path, lang := range map[string]string{"/privacy": "bg", "/en/privacy": "en"} {
		body := fetch(t, rr, path).Body.String()
		for _, key := range []string{"privacy.search.heading", "privacy.search.body"} {
			if want := html.EscapeString(cat.T(lang, key)); !strings.Contains(body, want) {
				t.Errorf("%s: lacks %s", path, key)
			}
		}
		if !strings.Contains(body, "Nominatim") {
			t.Errorf("%s: does not name Nominatim", path)
		}
	}
}

// The licences page lists the DbCL pair, the vector schema, Natural Earth and
// the map font.
func TestLicencesListsDbCLAndMapAssets(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := renderer(t, fixture(t))
	for path, lang := range map[string]string{"/licences": "bg", "/en/licences": "en"} {
		body := fetch(t, rr, path).Body.String()
		for _, want := range []string{
			`href="https://opendatacommons.org/licenses/dbcl/1-0/"`,
			`href="https://openmaptiles.org/"`,
			`href="https://www.naturalearthdata.com/"`,
			`href="https://fonts.google.com/noto/specimen/Noto+Sans"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: lacks %s", path, want)
			}
		}
		for _, key := range []string{"licences.openmaptiles.what", "licences.naturalearth.what", "licences.noto.what"} {
			if want := html.EscapeString(cat.T(lang, key)); !strings.Contains(body, want) {
				t.Errorf("%s: lacks %s", path, key)
			}
		}
	}
}
