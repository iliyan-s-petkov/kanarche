package web_test

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

// The two cities below carry districts: Sofia (the golden, byte-identical case) and the fixture city Testgrad.
// Testgrad is test data only, so no real boundary file is involved.

const goldenDir = "testdata"

// sofiaOnlyFixture is Sofia with its 6 districts and nothing else, the shape prod has today.
func sofiaOnlyFixture() *snapshot.Snapshot {
	snap := districtFixture()
	for slug, m := range snap.KnownSlugs {
		if strings.HasPrefix(m.ParentSlug, "testgrad") || strings.HasPrefix(slug, "testgrad") {
			delete(snap.KnownSlugs, slug)
		}
	}
	return snap
}

// districtFixture is Sofia with 6 districts and Testgrad with 5, so a rank that mixes them reads 11.
func districtFixture() *snapshot.Snapshot {
	areas := map[string]snapshot.AreaMeta{}
	add := func(m snapshot.AreaMeta) { areas[m.Slug] = m }
	val := func(v float64) map[string]float64 { return map[string]float64{"P2": v, "P1": v * 1.5} }

	add(snapshot.AreaMeta{Slug: "sofia-grad-oblast", Kind: "oblast", NameBG: "София-град", NameEN: "Sofia-City",
		CentroidLon: 23.32, CentroidLat: 42.69, DefaultZoom: 9, Covered: true, SensorCount: 90, Values: val(9), Km2: 1300, Source: "sensor.community"})
	add(snapshot.AreaMeta{Slug: "sofia", Kind: "city", NameBG: "София", NameEN: "Sofia", ParentSlug: "sofia-grad-oblast",
		CentroidLon: 23.32, CentroidLat: 42.69, DefaultZoom: 11, Covered: true, SensorCount: 80, Values: val(9), Km2: 1300, Source: "sensor.community"})
	sofia := []struct{ slug, bg, en string }{
		{"mladost", "Младост", "Mladost"}, {"lozenets", "Лозенец", "Lozenets"}, {"sredets", "Средец", "Sredets"},
		{"vitosha", "Витоша", "Vitosha"}, {"lyulin", "Люлин", "Lyulin"}, {"nadezhda", "Надежда", "Nadezhda"},
	}
	for i, d := range sofia {
		add(snapshot.AreaMeta{Slug: d.slug, Kind: "neighbourhood", NameBG: d.bg, NameEN: d.en, ParentSlug: "sofia",
			CentroidLon: 23.30 + float64(i)*0.02, CentroidLat: 42.65 + float64(i)*0.01, DefaultZoom: 13, Covered: true,
			SensorCount: 4 + i, Values: val(6 + float64(i)*1.5), Km2: 5 + float64(i)*3, Source: "sensor.community"})
	}

	add(snapshot.AreaMeta{Slug: "testgrad-oblast", Kind: "oblast", NameBG: "Тестоград", NameEN: "Testgrad",
		CentroidLon: 25, CentroidLat: 42, DefaultZoom: 9, Covered: true, SensorCount: 30, Values: val(12), Km2: 5000, Source: "sensor.community"})
	add(snapshot.AreaMeta{Slug: "testgrad", Kind: "city", NameBG: "Тестоград", NameEN: "Testgrad", ParentSlug: "testgrad-oblast",
		CentroidLon: 25, CentroidLat: 42, DefaultZoom: 11, Covered: true, SensorCount: 25, Values: val(12), Km2: 100, Source: "sensor.community"})
	test := []struct{ slug, bg, en string }{
		{"tsentralen", "Централен", "Tsentralen"}, {"severen", "Северен", "Severen"}, {"iztochen", "Източен", "Iztochen"},
		{"zapaden", "Западен", "Zapaden"}, {"yuzhen", "Южен", "Yuzhen"},
	}
	for i, d := range test {
		add(snapshot.AreaMeta{Slug: d.slug, Kind: "neighbourhood", NameBG: d.bg, NameEN: d.en, ParentSlug: "testgrad",
			CentroidLon: 25 + float64(i)*0.02, CentroidLat: 42 + float64(i)*0.01, DefaultZoom: 13, Covered: true,
			SensorCount: 3 + i, Values: val(10 + float64(i)*2), Km2: 8 + float64(i)*4, Source: "sensor.community"})
	}
	return &snapshot.Snapshot{GeneratedAt: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC), KnownSlugs: areas}
}

var (
	titleRE    = regexp.MustCompile(`<title>([^<]*)</title>`)
	descRE     = regexp.MustCompile(`<meta name="description" content="([^"]*)">`)
	navLinksRE = regexp.MustCompile(`(?s)<nav class="area-links".*?</nav>`)
	ldRE       = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	hrefRE     = regexp.MustCompile(`href="([^"]*)"`)
)

// faqLD is the FAQPage JSON-LD of the page as question and answer pairs.
func faqLD(t *testing.T, body string) []faqEntry {
	t.Helper()
	type node struct {
		Type       string `json:"@type"`
		MainEntity []struct {
			Name           string `json:"name"`
			AcceptedAnswer struct {
				Text string `json:"text"`
			} `json:"acceptedAnswer"`
		} `json:"mainEntity"`
	}
	var out []faqEntry
	for _, m := range ldRE.FindAllStringSubmatch(body, -1) {
		var doc struct {
			Graph []node `json:"@graph"`
		}
		if json.Unmarshal([]byte(m[1]), &doc) != nil {
			continue
		}
		for _, n := range doc.Graph {
			if n.Type != "FAQPage" {
				continue
			}
			for _, e := range n.MainEntity {
				out = append(out, faqEntry{Q: e.Name, A: e.AcceptedAnswer.Text})
			}
		}
	}
	return out
}

// districtView is the text of every part of a page that names a city: SEO, heading, facts, FAQ in both forms, link blocks.
func districtView(t *testing.T, body string) string {
	t.Helper()
	var b strings.Builder
	one := func(label string, re *regexp.Regexp) {
		m := re.FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("no %s in page", label)
		}
		fmt.Fprintf(&b, "%s: %s\n", label, html.UnescapeString(m[1]))
	}
	one("title", titleRE)
	one("description", descRE)
	fmt.Fprintf(&b, "h1: %s\n", h1Of(body))
	if factsSectionRE.FindString(body) != "" {
		b.WriteString("facts:\n" + factsText(t, body))
	}
	for _, e := range faqVisible(t, body) {
		fmt.Fprintf(&b, "faq visible: %s | %s\n", e.Q, e.A)
	}
	for _, e := range faqLD(t, body) {
		fmt.Fprintf(&b, "faq jsonld: %s | %s\n", e.Q, e.A)
	}
	if nav := navLinksRE.FindString(body); nav != "" {
		text := nav
		for _, end := range []string{"</div>", "</li>", "</h2>", "</p>"} {
			text = strings.ReplaceAll(text, end, "\n")
		}
		fmt.Fprintf(&b, "links:\n%s\n", html.UnescapeString(tagRE.ReplaceAllString(text, "")))
		for _, m := range hrefRE.FindAllStringSubmatch(nav, -1) {
			fmt.Fprintf(&b, "href %s\n", m[1])
		}
	}
	return b.String()
}

// The Sofia output is frozen in testdata. Regenerate with UPDATE_GOLDEN=1 only for an intended Sofia change.
func TestSofiaDistrictPagesAreByteIdentical(t *testing.T) {
	rr := renderer(t, sofiaOnlyFixture())
	paths := []string{"/area/mladost", "/en/area/mladost", "/area/sredets", "/en/area/sredets",
		"/area/nadezhda", "/en/area/nadezhda", "/area/sofia", "/en/area/sofia"}
	var got strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&got, "=== %s\n%s\n", p, districtView(t, fetch(t, rr, p).Body.String()))
	}
	file := filepath.Join(goldenDir, "sofia_districts.golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(got.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Errorf("Sofia district output changed.\n--- got ---\n%s\n--- want ---\n%s", got.String(), want)
	}
}

// A district of another city never says Sofia, in either language, in any part that names a city.
func TestNewCityDistrictPageHasNoSofia(t *testing.T) {
	rr := renderer(t, districtFixture())
	for _, p := range []string{"/area/severen", "/en/area/severen", "/area/yuzhen", "/en/area/yuzhen"} {
		view := districtView(t, fetch(t, rr, p).Body.String())
		for _, bad := range []string{"Sofia", "София", "Софи", "Mladost", "Младост"} {
			if strings.Contains(view, bad) {
				t.Errorf("%s names %q:\n%s", p, bad, view)
			}
		}
		if strings.Contains(view, "{") {
			t.Errorf("%s leaves a raw placeholder:\n%s", p, view)
		}
	}
}

// The title, heading and facts of a new-city district carry the parent city.
func TestNewCityDistrictCarriesItsCity(t *testing.T) {
	rr := renderer(t, districtFixture())
	cases := []struct {
		path string
		want []string
	}{
		{"/area/severen", []string{"Северен, Тестоград", "един от 5 района на Тестоград"}},
		{"/en/area/severen", []string{"Severen, Testgrad", "one of the 5 districts of Testgrad"}},
	}
	for _, c := range cases {
		view := districtView(t, fetch(t, rr, c.path).Body.String())
		for _, w := range c.want {
			if !strings.Contains(view, w) {
				t.Errorf("%s: missing %q in:\n%s", c.path, w, view)
			}
		}
	}
}

// Siblings are the district's own peers, and the rank counts only them.
func TestNewCityDistrictSiblingsAndRankStayInTheCity(t *testing.T) {
	rr := renderer(t, districtFixture())
	for _, prefix := range []string{"", "/en"} {
		body := fetch(t, rr, prefix+"/area/severen").Body.String()
		nav := navLinksRE.FindString(body)
		for _, peer := range []string{"tsentralen", "iztochen", "zapaden", "yuzhen"} {
			if !strings.Contains(nav, `href="`+prefix+`/area/`+peer+`"`) {
				t.Errorf("%s: sibling %s missing", prefix, peer)
			}
		}
		for _, other := range []string{"mladost", "lozenets", "sredets", "vitosha", "lyulin", "nadezhda"} {
			if strings.Contains(nav, "/area/"+other+`"`) {
				t.Errorf("%s: foreign district %s listed as a sibling", prefix, other)
			}
		}
		items := faqVisible(t, body)
		if len(items) == 0 {
			t.Fatalf("%s: no FAQ", prefix)
		}
		rank := items[0].A
		if !strings.Contains(rank, "5") || strings.Contains(rank, "11") {
			t.Errorf("%s: rank denominator is not the city's own 5: %q", prefix, rank)
		}
		if !strings.Contains(factsText(t, body), " 5 ") {
			t.Errorf("%s: facts do not count 5 districts", prefix)
		}
	}
}

// The FAQ JSON-LD of every district page says what the visible FAQ says.
func TestDistrictFAQJSONLDMatchesVisible(t *testing.T) {
	rr := renderer(t, districtFixture())
	for _, p := range []string{"/area/severen", "/en/area/severen", "/area/mladost", "/en/area/mladost"} {
		body := fetch(t, rr, p).Body.String()
		vis, ld := faqVisible(t, body), faqLD(t, body)
		if len(vis) == 0 || len(vis) != len(ld) {
			t.Fatalf("%s: %d visible, %d JSON-LD", p, len(vis), len(ld))
		}
		for i := range vis {
			if vis[i] != ld[i] {
				t.Errorf("%s item %d: visible %+v, JSON-LD %+v", p, i, vis[i], ld[i])
			}
		}
	}
}

// A city with districts labels its children block with its own name.
func TestCityPageChildrenBlockNamesItsCity(t *testing.T) {
	rr := renderer(t, districtFixture())
	cases := map[string]string{
		"/area/testgrad": "<h2>Райони на Тестоград</h2>", "/en/area/testgrad": "<h2>Testgrad districts</h2>",
	}
	for p, want := range cases {
		nav := navLinksRE.FindString(fetch(t, rr, p).Body.String())
		if !strings.Contains(nav, want) {
			t.Errorf("%s: missing %s in %s", p, want, nav)
		}
		// The nearest-cities block may name Sofia as a neighbour; the children block may not.
		children := nav[strings.Index(nav, want):]
		children = children[:strings.Index(children, "</ul>")]
		if strings.Contains(children, "Sofia") || strings.Contains(children, "София") {
			t.Errorf("%s: children block names Sofia", p)
		}
	}
}
