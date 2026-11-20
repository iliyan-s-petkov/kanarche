package web_test

import (
	"html"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"kanarche.eu/internal/snapshot"
)

var guidePages = []struct {
	name, path, prefix, h1 string
	faq                    int
}{
	{"build", "/build-a-sensor", "", "Как да си направите сензор за въздуха", 4},
	{"build", "/en/build-a-sensor", "/en", "Build your own air quality sensor", 4},
	{"bathing", "/bathing-water", "", "Качество на водата за къпане в Черно море", 5},
	{"bathing", "/en/bathing-water", "/en", "Bathing water quality on the Black Sea", 5},
}

func TestGuidePagesRenderInBothLanguages(t *testing.T) {
	for _, snap := range []*snapshot.Snapshot{fixture(t), nil} {
		rr := renderer(t, snap)
		for _, c := range guidePages {
			rec := fetch(t, rr, c.path)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", c.path, rec.Code)
			}
			body := rec.Body.String()
			if !strings.Contains(body, `<h1 class="t-title">`+c.h1+`</h1>`) {
				t.Errorf("%s: h1 %q missing", c.path, c.h1)
			}
			for _, bad := range []string{"{minutes}", "_link}", "{adopt", "{map", "!build.", "!bathing.", "!seo."} {
				if strings.Contains(body, bad) {
					t.Errorf("%s: unfilled placeholder or missing key %s", c.path, bad)
				}
			}
		}
	}
}

func TestGuidePagesTitleAndMetaLimits(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range guidePages {
		body := fetch(t, rr, c.path).Body.String()
		title, desc := pageTitle(t, body), pageDescription(t, body)
		if n := utf8.RuneCountInString(title); n < 1 || n > 60 {
			t.Errorf("%s: title %q is %d runes, want 1..60", c.path, title, n)
		}
		if n := utf8.RuneCountInString(desc); n < 1 || n > 155 {
			t.Errorf("%s: description is %d runes, want 1..155", c.path, n)
		}
		if !strings.HasSuffix(title, " | "+testBrand(c.prefix)) {
			t.Errorf("%s: title %q lacks the brand suffix", c.path, title)
		}
	}
}

func TestGuidePagesHeadTags(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range guidePages {
		slug := strings.TrimPrefix(c.path, c.prefix)
		body := fetch(t, rr, c.path).Body.String()
		for _, want := range []string{
			`<link rel="canonical" href="https://airbg.org` + c.path + `"`,
			`hreflang="bg" href="https://airbg.org` + slug + `"`,
			`hreflang="en" href="https://airbg.org/en` + slug + `"`,
			`hreflang="x-default" href="https://airbg.org` + slug + `"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: head lacks %q", c.path, want)
			}
		}
	}
}

func TestGuidePagesFAQPageMatchesVisibleDetails(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range guidePages {
		body := fetch(t, rr, c.path).Body.String()
		var visibleQ, visibleA []string
		section := faqSectionRE.FindString(body)
		for _, m := range summaryRe.FindAllStringSubmatch(section, -1) {
			visibleQ = append(visibleQ, html.UnescapeString(m[1]))
		}
		for _, m := range answerRe.FindAllStringSubmatch(section, -1) {
			// Answers with RHI links carry anchors on the page and plain URLs in the JSON-LD, which TestBathingFAQPageText checks.
			if strings.Contains(m[1], "<a ") {
				visibleA = append(visibleA, "")
				continue
			}
			visibleA = append(visibleA, html.UnescapeString(m[1]))
		}
		var ldQ, ldA []string
		crumbs := 0
		for _, n := range ldGraph(t, body) {
			switch n.Type {
			case "BreadcrumbList":
				crumbs++
			case "FAQPage":
				for _, q := range n.MainEntity {
					ldQ = append(ldQ, q.Name)
					ldA = append(ldA, q.AcceptedAnswer.Text)
				}
			}
		}
		if crumbs != 1 {
			t.Errorf("%s: %d BreadcrumbList nodes, want 1", c.path, crumbs)
		}
		if len(ldQ) != c.faq {
			t.Fatalf("%s: FAQPage has %d questions, want %d", c.path, len(ldQ), c.faq)
		}
		if strings.Join(ldQ, "|") != strings.Join(visibleQ, "|") {
			t.Errorf("%s: FAQPage questions %q differ from visible %q", c.path, ldQ, visibleQ)
		}
		for i := range visibleA {
			if visibleA[i] == "" && i < len(ldA) {
				ldA[i] = ""
			}
		}
		if strings.Join(ldA, "|") != strings.Join(visibleA, "|") {
			t.Errorf("%s: FAQPage answers differ from visible answers", c.path)
		}
	}
}

func TestFooterStationLinkIsTheBuildPage(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range []struct{ path, want string }{
		{"/", `<a href="/build-a-sensor">`},
		{"/en/", `<a href="/en/build-a-sensor">`},
		{"/about", `<a href="/build-a-sensor">`},
		{"/en/about", `<a href="/en/build-a-sensor">`},
	} {
		body := fetch(t, rr, c.path).Body.String()
		if !strings.Contains(body, c.want) {
			t.Errorf("%s: footer lacks %s", c.path, c.want)
		}
		if strings.Contains(body, `href="https://airbg.info/"`) {
			t.Errorf("%s: footer still links airbg.info", c.path)
		}
	}
}

func TestBuildPageAdoptLinkIsExternalAndSafe(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, path := range []string{"/build-a-sensor", "/en/build-a-sensor"} {
		body := fetch(t, rr, path).Body.String()
		if !strings.Contains(body, `href="https://airbg.info/zaqwi-uchastie/" rel="noopener noreferrer"`) {
			t.Errorf("%s: adopt link missing or without rel", path)
		}
	}
}

var guideImgRE = regexp.MustCompile(`<img [^>]*>`)

func TestBuildPageImagesHaveSizeLazyAndAlt(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, path := range []string{"/build-a-sensor", "/en/build-a-sensor"} {
		body := fetch(t, rr, path).Body.String()
		var guide []string
		for _, img := range guideImgRE.FindAllString(body, -1) {
			if strings.Contains(img, "build-a-sensor/") {
				guide = append(guide, img)
			}
		}
		if len(guide) != 19 {
			t.Fatalf("%s: %d guide images, want 19 (12 guide, 6 station, 1 usb board)", path, len(guide))
		}
		for _, img := range guide {
			for _, want := range []string{`width="`, `height="`, `loading="lazy"`, `alt="`} {
				if !strings.Contains(img, want) {
					t.Errorf("%s: %s lacks %s", path, img, want)
				}
			}
			if strings.Contains(img, `alt=""`) {
				t.Errorf("%s: empty alt on %s", path, img)
			}
			if strings.Contains(img, "kit") {
				t.Errorf("%s: %s still carries kit in its name", path, img)
			}
		}
	}
}

func TestBathingPageLinksMapWithSeaLayer(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range []struct{ path, want string }{
		{"/bathing-water", `href="/#layers=sea"`},
		{"/en/bathing-water", `href="/en/#layers=sea"`},
	} {
		if body := fetch(t, rr, c.path).Body.String(); !strings.Contains(body, c.want) {
			t.Errorf("%s: lacks %s", c.path, c.want)
		}
	}
}

func TestBathingPageCarriesRZILinksInFAQ(t *testing.T) {
	rr := rziRenderer(t, true)
	for _, path := range []string{"/bathing-water", "/en/bathing-water"} {
		sec := faqSectionRE.FindString(fetch(t, rr, path).Body.String())
		for _, u := range rziFAQURLs {
			if got := strings.Count(sec, `<a href="`+u+`" rel="noopener noreferrer">`); got != 2 {
				t.Errorf("%s: %s linked %d times in the FAQ, want 2 (a10 and a14)", path, u, got)
			}
		}
	}
}

func TestBathingFAQMovedOffAboutTheData(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, c := range []struct{ path, moved, link string }{
		{"/about-the-data", "Откъде идват данните за чистотата на водата за къпане?", `href="/bathing-water"`},
		{"/en/about-the-data", "Where does the data on bathing water cleanliness come from?", `href="/en/bathing-water"`},
	} {
		body := fetch(t, rr, c.path).Body.String()
		if strings.Contains(body, html.EscapeString(c.moved)) {
			t.Errorf("%s: moved question is still on the page", c.path)
		}
		if !strings.Contains(body, c.link) {
			t.Errorf("%s: no link left behind to %s", c.path, c.link)
		}
		bathing := fetch(t, rr, strings.Replace(c.path, "about-the-data", "bathing-water", 1)).Body.String()
		if !strings.Contains(bathing, html.EscapeString(c.moved)) {
			t.Errorf("%s: moved question missing from the bathing page", c.path)
		}
	}
}

var coastalSlugs = []string{"varna", "varna-oblast", "burgas", "burgas-oblast", "dobrich", "dobrich-oblast"}

func TestCoastalAreaPagesLinkBathingPage(t *testing.T) {
	snap := fixture(t)
	for _, s := range coastalSlugs {
		snap.KnownSlugs[s] = snapshot.AreaMeta{Slug: s, Kind: "oblast", NameBG: s, NameEN: s,
			CentroidLon: 27.9, CentroidLat: 43.2, DefaultZoom: 9, Covered: true, SensorCount: 3}
	}
	rr := renderer(t, snap)
	for _, s := range coastalSlugs {
		for _, c := range []struct{ prefix, want string }{{"", `href="/bathing-water"`}, {"/en", `href="/en/bathing-water"`}} {
			rec := fetch(t, rr, c.prefix+"/area/"+s)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s/area/%s = %d", c.prefix, s, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), c.want) {
				t.Errorf("%s/area/%s lacks %s", c.prefix, s, c.want)
			}
		}
	}
	for _, c := range []struct{ prefix, want string }{{"", `href="/bathing-water"`}, {"/en", `href="/en/bathing-water"`}} {
		if strings.Contains(fetch(t, rr, c.prefix+"/area/sofia").Body.String(), c.want) {
			t.Errorf("%s/area/sofia: an inland area links the bathing page", c.prefix)
		}
	}
}

func TestGuidePagesInSitemap(t *testing.T) {
	doc := fetchSitemap(t)
	for _, c := range guidePages {
		found := false
		for _, u := range doc.URLs {
			found = found || u.Loc == "https://airbg.org"+c.path
		}
		if !found {
			t.Errorf("sitemap lacks %s", c.path)
		}
	}
}

// The FAQPage text for the bathing answers that end in links is plain: the sentence, then the three URLs.
func TestBathingFAQPageBathingAnswersCarryPlainURLs(t *testing.T) {
	rr := rziRenderer(t, true)
	for _, path := range []string{"/bathing-water", "/en/bathing-water"} {
		checked := false
		for _, n := range ldGraph(t, fetch(t, rr, path).Body.String()) {
			if n.Type != "FAQPage" {
				continue
			}
			if len(n.MainEntity) != 5 {
				t.Fatalf("%s: %d FAQ entries, want 5", path, len(n.MainEntity))
			}
			for _, i := range []int{0, 4} {
				text := n.MainEntity[i].AcceptedAnswer.Text
				if strings.Contains(text, "<") || strings.Contains(text, "[links]") {
					t.Errorf("%s: answer %d is not plain text: %q", path, i+1, text)
				}
				if !strings.HasSuffix(text, ": "+strings.Join(rziFAQURLs, " ")) {
					t.Errorf("%s: answer %d does not end with the three URLs: %q", path, i+1, text)
				}
			}
			checked = true
		}
		if !checked {
			t.Errorf("%s: no FAQPage", path)
		}
	}
}
