package web_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestAboutPageRendersInEveryLanguage. The page exists to carry the data
// caveats to a reader, so the assertions are about the caveats being present
// in that reader's language — not about the page merely returning 200. The
// municipality split is checked specifically because it is the one that
// silently changes how a city's number should be read.
func TestAboutPageRendersInEveryLanguage(t *testing.T) {
	rr := renderer(t, fixture(t))

	for _, tc := range []struct {
		path string
		want []string
	}{
		{"/about-the-data", []string{"За данните", "община", "sensor.community", "OpenStreetMap"}},
		{"/en/about-the-data", []string{"About the data", "municipality", "sensor.community", "OpenStreetMap"}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rec := fetch(t, rr, tc.path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			body := rec.Body.String()
			for _, want := range tc.want {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q", want)
				}
			}
			// Catalogue.T renders "!key!" for a key it does not hold. The
			// key-parity test covers templates as a class; this catches the
			// same failure through the actual rendered response, which is
			// what a reader would see.
			if strings.Contains(body, "!about.") {
				t.Errorf("body contains an untranslated key marker:\n%s", body)
			}
		})
	}
}

// TestAboutPageRendersWithoutASnapshot. Every other page 503s when the
// snapshot holder is empty, and this one deliberately does not: the content is
// static prose that needs no snapshot, and the moment a reader goes looking for
// "can I trust this site" is exactly the moment the data is not loading. A
// future refactor that gives this handler the same snapshot guard as the others
// would look like consistency and would remove the page precisely when it is
// most wanted.
func TestAboutPageRendersWithoutASnapshot(t *testing.T) {
	rec := fetch(t, renderer(t, nil), "/about-the-data")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — the about page must not depend on a snapshot", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "За данните") {
		t.Errorf("body is missing the page title:\n%s", rec.Body)
	}
}

// TestEveryPageFootersLinkToTheAboutPage, in the reader's own language. The
// link is the only route to this page — there is no nav entry — so a page
// whose footer lost it has silently unpublished the caveats. The
// language-prefix half matters just as much: a bare href="/about-the-data" in
// base.gohtml would render identically on both pages, pass a naive existence
// check, and drop every English reader back into Bulgarian.
func TestEveryPageFootersLinkToTheAboutPage(t *testing.T) {
	rr := renderer(t, fixture(t))

	for _, tc := range []struct{ page, wantHref string }{
		{"/", `href="/about-the-data"`},
		{"/areas", `href="/about-the-data"`},
		{"/area/sofia", `href="/about-the-data"`},
		{"/en/", `href="/en/about-the-data"`},
		{"/en/areas", `href="/en/about-the-data"`},
		{"/en/area/sofia", `href="/en/about-the-data"`},
		// The error page renders through the same base template; a reader who
		// mistyped a slug is still owed the link.
		{"/en/area/no-such-place", `href="/en/about-the-data"`},
	} {
		t.Run(tc.page, func(t *testing.T) {
			body := fetch(t, rr, tc.page).Body.String()
			if !strings.Contains(body, tc.wantHref) {
				t.Errorf("footer does not link to the about page with %s", tc.wantHref)
			}
		})
	}
}

// TestCityBoundaryNoteAppearsOnCityPagesOnly. The city page is where the
// municipality/city-proper split actually misleads someone — it is the number
// a reader compares against another city's. The note is deliberately not shown
// on oblast or district pages, where it would be simply false: those tiers have
// one consistent geometry.
func TestCityBoundaryNoteAppearsOnCityPagesOnly(t *testing.T) {
	snap := fixture(t)
	// "sofia" in the fixture is an oblast; give the snapshot a city too, so
	// both branches of the template are exercised against real routing rather
	// than against a hand-built PageData.
	meta := snap.KnownSlugs["sofia"]
	meta.Slug, meta.Kind, meta.NameBG, meta.NameEN = "plovdiv", "city", "Пловдив", "Plovdiv"
	snap.KnownSlugs["plovdiv"] = meta
	rr := renderer(t, snap)

	const bgNote = "цялата община"
	if body := fetch(t, rr, "/area/plovdiv").Body.String(); !strings.Contains(body, bgNote) {
		t.Errorf("city page does not carry the boundary note")
	}
	if body := fetch(t, rr, "/area/sofia").Body.String(); strings.Contains(body, bgNote) {
		t.Errorf("oblast page carries the city boundary note, which is not true of oblast geometry")
	}
	if body := fetch(t, rr, "/en/area/plovdiv").Body.String(); !strings.Contains(body, "whole municipality") {
		t.Errorf("English city page does not carry the boundary note")
	}
}

// municipalityRe pulls the "Whole municipality" row out of the limitations doc.
// Permissive on the count in the header cell so that fixing the doc's number
// alone cannot make this stop matching silently.
var municipalityRe = regexp.MustCompile(`\*\*Whole municipality\*\*[^|]*\|([^|]*)\|`)

// TestAboutPageListsTheSameCitiesAsTheLimitationsDoc. The 14 municipality-
// boundary cities are stated in three places — data/boundaries/README.md,
// docs/known-limitations.md, and now user-facing copy in two catalogues. Copy
// is where drift is least likely to be noticed: nothing about a wrong city name
// in a translation string looks wrong, and the page reads perfectly either way.
//
// Compared as a set, not a sequence: the doc's order and the sentence's order
// are both arbitrary and neither should force a change in the other. The
// Bulgarian list is checked by count only — the names are Cyrillic and have no
// counterpart in the doc, so a name-level check there would need a
// transliteration table this test has no business owning.
func TestAboutPageListsTheSameCitiesAsTheLimitationsDoc(t *testing.T) {
	const wantCities = 14

	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "known-limitations.md"))
	if err != nil {
		t.Fatalf("reading known-limitations.md: %v", err)
	}
	m := municipalityRe.FindSubmatch(raw)
	if m == nil {
		t.Fatal("no `**Whole municipality**` row found in docs/known-limitations.md — " +
			"the table changed shape and this test is comparing nothing")
	}
	fromDoc := splitCityList(string(m[1]))
	if len(fromDoc) != wantCities {
		t.Fatalf("doc lists %d municipality cities, want %d", len(fromDoc), wantCities)
	}

	rr := renderer(t, fixture(t))

	fromPage := splitCityList(cityListSentence(t, fetch(t, rr, "/en/about-the-data").Body.String(), "Whole municipality:"))
	if len(fromPage) != wantCities {
		t.Fatalf("the English page lists %d municipality cities, want %d: %v", len(fromPage), wantCities, fromPage)
	}
	if strings.Join(fromDoc, "|") != strings.Join(fromPage, "|") {
		t.Errorf("the English page and docs/known-limitations.md disagree about which cities use a municipality boundary\npage: %v\ndoc:  %v", fromPage, fromDoc)
	}

	bg := splitCityList(cityListSentence(t, fetch(t, rr, "/about-the-data").Body.String(), "С граница на общината:"))
	if len(bg) != wantCities {
		t.Errorf("the Bulgarian page lists %d municipality cities, want %d: %v", len(bg), wantCities, bg)
	}
}

// cityListSentence returns the comma-separated remainder of the rendered
// sentence introduced by prefix.
func cityListSentence(t *testing.T, body, prefix string) string {
	t.Helper()
	_, rest, ok := strings.Cut(body, prefix)
	if !ok {
		t.Fatalf("rendered page has no sentence beginning %q", prefix)
	}
	sentence, _, ok := strings.Cut(rest, "<")
	if !ok {
		t.Fatalf("no element end after %q; the template changed shape", prefix)
	}
	return sentence
}

// splitCityList normalises either source into a sorted set of city names.
func splitCityList(s string) []string {
	out := make([]string, 0, 16)
	for _, name := range strings.Split(strings.TrimSuffix(strings.TrimSpace(s), "."), ",") {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// TestAboutStartTiltCard. The Getting started section explains tilting and
// turning the map; the card must render its title and both theme images in
// each language, and every image it points at must be served.
func TestAboutStartTiltCard(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, tc := range []struct{ path, lang, title string }{
		{"/en/about", "en", "Tilt and turn the map"},
		{"/about", "bg", "Наклонете и завъртете картата"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			body := fetch(t, rr, tc.path).Body.String()
			if !strings.Contains(body, "<h4>"+tc.title+"</h4>") {
				t.Fatalf("no card titled %q", tc.title)
			}
			for _, theme := range []string{"light", "dark"} {
				re := regexp.MustCompile(`src="(/static/about/start-tilt-` + tc.lang + `-` + theme + `[^"]*\.webp[^"]*)"`)
				m := re.FindStringSubmatch(body)
				if m == nil {
					t.Fatalf("no %s tilt image in the page", theme)
				}
				if rec := fetch(t, rr, m[1]); rec.Code != http.StatusOK {
					t.Errorf("GET %s = %d, want 200", m[1], rec.Code)
				}
			}
		})
	}
}

// aboutGuide is the Getting started layout: groups in page order, cards in
// order inside each group. The template is checked against this list, so a
// card dropped from a group or a group moved fails here.
var aboutGuide = []struct {
	group string
	cards []string
}{
	{"read", []string{"layers", "metrics", "legend", "window", "inactive"}},
	{"find", []string{"search", "areas", "below", "phone"}},
	{"move", []string{"tilt", "locate"}},
	{"layers", []string{"pollen", "sea", "wind", "official"}},
	{"history", []string{"replay", "charts"}},
	{"keep", []string{"favourite", "share", "embed", "table"}},
}

// catalogueKey reads one key straight from the catalogue file, so the tests
// compare the page against the copy and not against a literal.
func catalogueKey(t *testing.T, lang, key string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "i18n", lang+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	v, ok := m[key]
	if !ok {
		t.Fatalf("%s.json has no key %q", lang, key)
	}
	return v
}

// TestAboutGuideCards. Each card renders its h4 title and four images
// (en/bg x light/dark), and each image file exists on disk.
func TestAboutGuideCards(t *testing.T) {
	rr := renderer(t, fixture(t))
	pages := map[string]string{
		"en": fetch(t, rr, "/en/about").Body.String(),
		"bg": fetch(t, rr, "/about").Body.String(),
	}
	for _, g := range aboutGuide {
		for _, name := range g.cards {
			t.Run(name, func(t *testing.T) {
				for lang, body := range pages {
					title := catalogueKey(t, lang, "about.start."+name+".title")
					if !strings.Contains(body, "<h4>"+title+"</h4>") {
						t.Errorf("%s page has no <h4> for card %q (%q)", lang, name, title)
					}
				}
				for _, lang := range []string{"en", "bg"} {
					for _, theme := range []string{"light", "dark"} {
						file := "start-" + name + "-" + lang + "-" + theme
						re := regexp.MustCompile(`src="/static/about/` + file + `[^"]*\.webp[^"]*"`)
						if !re.MatchString(pages[lang]) {
							t.Errorf("%s page has no src for %s", lang, file)
						}
						t.Run("file/"+file, func(t *testing.T) {
							_, err := os.Stat(filepath.Join("static", "about", file+".webp"))
							if err == nil {
								return
							}
							t.Errorf("image missing: %v", err)
						})
					}
				}
			})
		}
	}
}

// TestAboutGuideGroups. Group headings render in the specified order, each
// carries the anchor the table of contents points at, and each group holds
// exactly its cards between its heading and the next one.
func TestAboutGuideGroups(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, lang := range []struct{ path, code string }{{"/en/about", "en"}, {"/about", "bg"}} {
		t.Run(lang.code, func(t *testing.T) {
			body := fetch(t, rr, lang.path).Body.String()
			last := strings.Index(body, `id="start"`)
			if last < 0 {
				t.Fatal("no #start section")
			}
			// The privacy section follows the guide and bounds the last group.
			guideEnd := strings.Index(body, `id="privacy"`)
			for i, g := range aboutGuide {
				marker := `<h3 id="start-` + g.group + `" class="about-group">` + catalogueKey(t, lang.code, "about.start.group."+g.group+".title") + `</h3>`
				at := strings.Index(body, marker)
				if at < 0 {
					t.Fatalf("no group heading %s", marker)
				}
				if at < last {
					t.Errorf("group %q is out of order", g.group)
				}
				end := guideEnd
				if i+1 < len(aboutGuide) {
					end = strings.Index(body, `id="start-`+aboutGuide[i+1].group+`"`)
				}
				if n := strings.Count(body[at:end], "<h4>"); n != len(g.cards) {
					t.Errorf("group %q has %d cards, want %d", g.group, n, len(g.cards))
				}
				last = at
			}
		})
	}
}

// TestAboutTOCLinksToGroupsAndInvolved. The nav lists each group anchor and
// the involved section, and the page carries the involved target.
func TestAboutTOCLinksToGroupsAndInvolved(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/en/about").Body.String()
	nav := body[strings.Index(body, `class="about-toc"`):]
	nav = nav[:strings.Index(nav, "</nav>")]
	for _, g := range aboutGuide {
		if !strings.Contains(nav, `href="#start-`+g.group+`"`) {
			t.Errorf("TOC has no link to #start-%s", g.group)
		}
	}
	if !strings.Contains(nav, `href="#involved"`) {
		t.Error("TOC has no link to #involved")
	}
	if !strings.Contains(body, `id="involved"`) {
		t.Error("page has no #involved section")
	}
}

// TestAboutInvolvedLinks. The section links to the repository and its issue
// tracker, both derived from SourceRepoURL, and sits before #more.
func TestAboutInvolvedLinks(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/en/about").Body.String()
	at := strings.Index(body, `id="involved"`)
	more := strings.Index(body, `id="more"`)
	if at < 0 || more < 0 || at > more {
		t.Fatalf("#involved must exist and come before #more (involved=%d more=%d)", at, more)
	}
	hrefs := regexp.MustCompile(`<a href="([^"]+)"[^>]*>`).FindAllStringSubmatch(body[at:more], -1)
	if len(hrefs) != 2 {
		t.Fatalf("#involved has %d links, want 2", len(hrefs))
	}
	repo, issues := hrefs[0][1], hrefs[1][1]
	if !strings.HasPrefix(repo, "https://") || strings.HasSuffix(repo, "/") {
		t.Errorf("repo link %q is not a bare repository URL", repo)
	}
	if issues != repo+"/issues" {
		t.Errorf("issues link = %q, want %q", issues, repo+"/issues")
	}
	if !strings.Contains(body[more:], `<a href="`+repo+`"`) {
		t.Errorf("repo link %q differs from the SourceRepoURL used in #more", repo)
	}
}

// TestAboutValuesIncludePhone. The values strip gains a phone item.
func TestAboutValuesIncludePhone(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/en/about").Body.String()
	for _, k := range []string{"title", "body"} {
		if v := catalogueKey(t, "en", "about.values.phone."+k); !strings.Contains(body, v) {
			t.Errorf("values strip lacks about.values.phone.%s", k)
		}
	}
}

// TestAboutInvolvedBodyRendersInlineLinks. The sentence carries {repo} and
// {issues}; they must become the two anchors with the catalogue's link text,
// the surrounding words must be present, and no placeholder may leak.
func TestAboutInvolvedBodyRendersInlineLinks(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, lang := range []struct{ path, code string }{{"/en/about", "en"}, {"/about", "bg"}} {
		t.Run(lang.code, func(t *testing.T) {
			body := fetch(t, rr, lang.path).Body.String()
			sec := body[strings.Index(body, `id="involved"`):strings.Index(body, `id="more"`)]
			if strings.Contains(sec, "{repo}") || strings.Contains(sec, "{issues}") {
				t.Errorf("placeholder left in the section:\n%s", sec)
			}
			for _, key := range []string{"repo", "issues"} {
				label := catalogueKey(t, lang.code, "about.involved."+key)
				if !strings.Contains(sec, `rel="noopener noreferrer">`+label+`</a>`) {
					t.Errorf("no anchor with text %q", label)
				}
			}
			// The opening words of the sentence, up to its first placeholder.
			lead, _, _ := strings.Cut(catalogueKey(t, lang.code, "about.involved.body"), "{")
			if !strings.Contains(sec, lead) {
				t.Errorf("section lacks the sentence's opening text %q", lead)
			}
		})
	}
}
