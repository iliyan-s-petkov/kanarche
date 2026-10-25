package web_test

import (
	"html"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/web"
)

// seoFixture carries one area of every kind the SEO copy spec (seo-copy.md)
// calls out by name, including its worst-case (longest name) examples, so a
// length regression on the longest real name is caught rather than only on
// the fixtures render_test.go already uses for other things.
func seoFixture(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	return &snapshot.Snapshot{
		GeneratedAt: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
		KnownSlugs: map[string]snapshot.AreaMeta{
			"smolyan-oblast": {Slug: "smolyan-oblast", Kind: "oblast", NameBG: "Смолян", NameEN: "Smolyan",
				DefaultZoom: 9, Covered: true, SensorCount: 11},
			"smolyan": {Slug: "smolyan", Kind: "city", NameBG: "Смолян", NameEN: "Smolyan",
				DefaultZoom: 11, Covered: true, SensorCount: 6},
			"sofiyska-oblast": {Slug: "sofiyska-oblast", Kind: "oblast", NameBG: "Софийска", NameEN: "Sofia",
				DefaultZoom: 9, Covered: true, SensorCount: 40},
			"sofia-grad-oblast": {Slug: "sofia-grad-oblast", Kind: "oblast", NameBG: "София-град", NameEN: "Sofia-City",
				DefaultZoom: 9, Covered: true, SensorCount: 412},
			"veliko-tarnovo-oblast": {Slug: "veliko-tarnovo-oblast", Kind: "oblast", NameBG: "Велико Търново", NameEN: "Veliko Tarnovo",
				DefaultZoom: 9, Covered: true, SensorCount: 1024},
			"veliko-tarnovo": {Slug: "veliko-tarnovo", Kind: "city", NameBG: "Велико Търново", NameEN: "Veliko Tarnovo",
				DefaultZoom: 11, Covered: true, SensorCount: 1024},
			"plovdiv": {Slug: "plovdiv", Kind: "city", NameBG: "Пловдив", NameEN: "Plovdiv",
				DefaultZoom: 11, Covered: true, SensorCount: 137},
			"mladost": {Slug: "mladost", Kind: "neighbourhood", NameBG: "Младост", NameEN: "Mladost",
				DefaultZoom: 13, Covered: true, SensorCount: 38},
			"krasna-polyana": {Slug: "krasna-polyana", Kind: "neighbourhood", NameBG: "Красна поляна", NameEN: "Krasna poliana",
				DefaultZoom: 13, Covered: true, SensorCount: 1024},
		},
	}
}

// tagAttr extracts the value of a single-quoted-with-double-quotes attribute
// following marker (a whole opening fragment like `<meta name="description"
// content="`), so a test can pull one tag's value out of the full rendered
// head without narrowing to an element first.
func tagAttr(t *testing.T, body, marker string) string {
	t.Helper()
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("marker %q not found in body:\n%s", marker, body)
	}
	start += len(marker)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		t.Fatalf("unterminated value after %q", marker)
	}
	return body[start : start+end]
}

func hasMarker(body, marker string) bool { return strings.Contains(body, marker) }

func pageTitle(t *testing.T, body string) string {
	t.Helper()
	const open = "<title>"
	start := strings.Index(body, open)
	if start < 0 {
		t.Fatalf("no <title> in body:\n%s", body)
	}
	start += len(open)
	end := strings.Index(body[start:], "</title>")
	if end < 0 {
		t.Fatalf("unterminated <title> in body:\n%s", body)
	}
	return body[start : start+end]
}

func pageDescription(t *testing.T, body string) string {
	t.Helper()
	return tagAttr(t, body, `<meta name="description" content="`)
}

// TestEveryPageTypeHasADistinctTitleAndDescription covers every page kind the
// spec calls out — home, /areas, about-the-data, and all three area kinds —
// in both languages, and proves no two of them collide. /areas sharing
// handleIndex with / used to render the identical title for both; this is the
// mutation-proof for that regression (OpenProject #602 audit finding a).
func TestEveryPageTypeHasADistinctTitleAndDescription(t *testing.T) {
	rr := renderer(t, seoFixture(t))

	type page struct{ name, path string }
	pages := []page{
		{"home", "/"}, {"areas", "/areas"}, {"about", "/about-the-data"},
		{"oblast", "/area/smolyan-oblast"}, {"city", "/area/smolyan"},
		{"district", "/area/mladost"},
	}

	for _, lang := range []string{"", "/en"} {
		seenTitles := map[string]string{}
		for _, p := range pages {
			body := fetch(t, rr, lang+p.path).Body.String()
			title := pageTitle(t, body)
			desc := pageDescription(t, body)

			if title == "" {
				t.Errorf("lang=%q %s: empty title", lang, p.name)
			}
			if desc == "" {
				t.Errorf("lang=%q %s: empty description", lang, p.name)
			}
			if prior, ok := seenTitles[title]; ok {
				t.Errorf("lang=%q %s and %s share the title %q", lang, p.name, prior, title)
			}
			seenTitles[title] = p.name
		}
	}
}

// TestAreaTitlesDoNotCollideBetweenOblastAndCity: Смолян names both an oblast
// and a city with the same name_bg/name_en, which is exactly the collision
// seo-copy.md §0 identifies as the reason oblast titles carry "Област".
func TestAreaTitlesDoNotCollideBetweenOblastAndCity(t *testing.T) {
	rr := renderer(t, seoFixture(t))
	for _, lang := range []string{"", "/en"} {
		oblast := pageTitle(t, fetch(t, rr, lang+"/area/smolyan-oblast").Body.String())
		city := pageTitle(t, fetch(t, rr, lang+"/area/smolyan").Body.String())
		if oblast == city {
			t.Errorf("lang=%q: oblast and city titles both render %q", lang, oblast)
		}
	}
}

// TestRenderedTitlesAndDescriptionsMatchApprovedCopy pins the exact rendered
// strings from the approved seo-copy.md examples (§3), not just their length —
// a wrong but short string would pass a length-only check.
func TestRenderedTitlesAndDescriptionsMatchApprovedCopy(t *testing.T) {
	rr := renderer(t, seoFixture(t))

	type want struct {
		path                   string
		bgTitle, bgDescription string
		enTitle, enDescription string
	}
	cases := []want{
		{
			path:          "/",
			bgTitle:       "Качество на въздуха в България: карта на живо | Канарче",
			bgDescription: "Карта на живо на замърсяването на въздуха в България: фини прахови частици ФПЧ2.5 и ФПЧ10 от граждански сензори, обновявана на около 5 минути.",
			enTitle:       "Bulgaria air quality map: PM2.5 and PM10 now | Kanarche",
			enDescription: "Live map of air pollution in Bulgaria: PM2.5 and PM10 from sensor.community citizen sensors, updated about every 5 minutes.",
		},
		{
			path:          "/areas",
			bgTitle:       "Замърсяване на въздуха по области и градове | Канарче",
			bgDescription: "Областите в България, подредени по ориентировъчната медиана на ФПЧ2.5. С връзки към 27-те областни града и 24-те района на София.",
			enTitle:       "Air pollution in Bulgaria by province and city | Kanarche",
			enDescription: "Bulgaria's provinces ordered by indicative PM2.5 median, with links to all 27 provincial capitals and 24 Sofia districts.",
		},
		{
			path:          "/about-the-data",
			bgTitle:       "За данните: сензори, граници и точност | Канарче",
			bgDescription: "Колко точни са данните: нискобюджетни сензори срещу референтни станции, различни граници на градовете и неравномерно покритие.",
			enTitle:       "About the data: sensors, boundaries, accuracy | Kanarche",
			enDescription: "How accurate the data is: low-cost sensors versus reference stations, city boundaries of different kinds, and uneven coverage.",
		},
		{
			path:          "/area/plovdiv",
			bgTitle:       "Индекс за качество на въздуха в Пловдив (AQI) сега | Канарче",
			bgDescription: "Пловдив: индекс за качество на въздуха (AQI) по ФПЧ2.5 и ФПЧ10. Медиана от граждански сензори, оценка по Европейския индекс и графика за 24 часа.",
			enTitle:       "Plovdiv air quality (AQI) now: PM2.5 and PM10 | Kanarche",
			enDescription: "Plovdiv AQI and air pollution now: PM2.5 and PM10 from citizen sensors on a live map, with the European index band and 24-hour history.",
		},
		{
			path:          "/area/smolyan",
			bgTitle:       "Индекс за качество на въздуха в Смолян (AQI) сега | Канарче",
			bgDescription: "Смолян: индекс за качество на въздуха (AQI) по ФПЧ2.5 и ФПЧ10. Медиана от граждански сензори, оценка по Европейския индекс и графика за 24 часа.",
			enTitle:       "Smolyan air quality (AQI) now: PM2.5 and PM10 | Kanarche",
			enDescription: "Smolyan AQI and air pollution now: PM2.5 and PM10 from citizen sensors on a live map, with the European index band and 24-hour history.",
		},
		{
			path:          "/area/mladost",
			bgTitle:       "Младост, София: индекс за качество на въздуха (AQI)",
			bgDescription: "Индекс за качество на въздуха (AQI) в район Младост, София: ФПЧ2.5 и ФПЧ10. Медиана от граждански сензори и оценка по Европейския индекс.",
			enTitle:       "Mladost, Sofia: air quality index (AQI) today | Kanarche",
			enDescription: "Mladost, Sofia: air quality index (AQI) by district. PM2.5 and PM10 from citizen sensors, with the European index band and 24-hour history.",
		},
		{
			path:          "/area/smolyan-oblast",
			bgTitle:       "Област Смолян: индекс за качество на въздуха (AQI) | Канарче",
			bgDescription: "Индекс за качество на въздуха (AQI) в област Смолян: ФПЧ2.5 и ФПЧ10. Медиана от граждански сензори и оценка по Европейския индекс.",
			enTitle:       "Smolyan Province: air quality index (AQI) today | Kanarche",
			enDescription: "Air quality index (AQI) in Smolyan Province, Bulgaria: PM2.5 and PM10 from citizen sensors, with the European index band and 24-hour history.",
		},
		{
			path:          "/area/sofiyska-oblast",
			bgTitle:       "Софийска област: индекс за качество на въздуха (AQI)",
			bgDescription: "Индекс за качество на въздуха (AQI) в Софийска област: ФПЧ2.5 и ФПЧ10. Медиана от граждански сензори и оценка по Европейския индекс.",
			enTitle:       "Sofia Province: air quality index (AQI) today | Kanarche",
			enDescription: "Air quality index (AQI) in Sofia Province, Bulgaria: PM2.5 and PM10 from citizen sensors, with the European index band and 24-hour history.",
		},
		{
			path:          "/area/sofia-grad-oblast",
			bgTitle:       "Област София-град: индекс за качество на въздуха (AQI)",
			bgDescription: "Индекс за качество на въздуха (AQI) в област София-град: ФПЧ2.5 и ФПЧ10. Медиана от граждански сензори и оценка по Европейския индекс.",
			enTitle:       "Sofia City Province: air quality index (AQI) today",
			enDescription: "Air quality index (AQI) in Sofia City Province, Bulgaria: PM2.5 and PM10 from citizen sensors, with the European index band and 24-hour history.",
		},
		// Longest names: both go over 60 runes with the suffix, so it is dropped.
		{
			path:          "/area/veliko-tarnovo-oblast",
			bgTitle:       "Област Велико Търново: индекс за качество на въздуха (AQI)",
			bgDescription: "Индекс за качество на въздуха (AQI) в област Велико Търново: ФПЧ2.5 и ФПЧ10. Медиана от граждански сензори и оценка по Европейския индекс.",
			enTitle:       "Veliko Tarnovo Province: air quality index (AQI) today",
			enDescription: "Air quality index (AQI) in Veliko Tarnovo Province, Bulgaria: PM2.5 and PM10 from citizen sensors, with the European index band and 24-hour history.",
		},
	}

	for _, c := range cases {
		bg := fetch(t, rr, c.path).Body.String()
		if got := pageTitle(t, bg); got != c.bgTitle {
			t.Errorf("%s bg title = %q, want %q", c.path, got, c.bgTitle)
		}
		if got := pageDescription(t, bg); got != c.bgDescription {
			t.Errorf("%s bg description = %q, want %q", c.path, got, c.bgDescription)
		}

		en := fetch(t, rr, "/en"+c.path).Body.String()
		if got := pageTitle(t, en); got != c.enTitle {
			t.Errorf("%s en title = %q, want %q", c.path, got, c.enTitle)
		}
		if got := html.UnescapeString(pageDescription(t, en)); got != c.enDescription {
			t.Errorf("%s en description = %q, want %q", c.path, got, c.enDescription)
		}
	}
}

// TestTitlesAndDescriptionsFitTheirBudget: ≤60 runes for a title, ≤155 for a
// description, for every area in the fixture, in both languages — the
// mutation-proof for "remove the length guard" (composeTitle's <=60 check).
func TestTitlesAndDescriptionsFitTheirBudget(t *testing.T) {
	rr := renderer(t, seoFixture(t))
	snap := seoFixture(t)

	paths := []string{"/", "/areas", "/about-the-data", "/air-pollution"}
	for slug := range snap.KnownSlugs {
		paths = append(paths, "/area/"+slug)
	}

	for _, lang := range []string{"", "/en"} {
		for _, path := range paths {
			body := fetch(t, rr, lang+path).Body.String()
			title := pageTitle(t, body)
			desc := pageDescription(t, body)
			if n := utf8.RuneCountInString(title); n > 60 {
				t.Errorf("lang=%q %s: title is %d runes (%q), want <=60", lang, path, n, title)
			}
			if n := utf8.RuneCountInString(desc); n > 155 {
				t.Errorf("lang=%q %s: description is %d runes (%q), want <=155", lang, path, n, desc)
			}
		}
	}
}

// TestBrandSuffixDroppedOnlyWhenOverBudget: the exact-string assertions above
// already pin the two worst-case titles with the suffix dropped; this proves
// the general rule both ways — present when it fits, absent only when it
// would push the title over 60.
func TestBrandSuffixDroppedOnlyWhenOverBudget(t *testing.T) {
	rr := renderer(t, seoFixture(t))
	for _, lang := range []string{"", "/en"} {
		brand := testBrand(lang)
		for _, path := range []string{"/", "/areas", "/about-the-data", "/area/plovdiv", "/area/veliko-tarnovo-oblast"} {
			title := pageTitle(t, fetch(t, rr, lang+path).Body.String())
			hasSuffix := strings.HasSuffix(title, " | "+brand)
			fits := utf8.RuneCountInString(title) <= 60
			// The budget gates core+suffix, not the bare core: a title that
			// dropped the suffix must prove it would NOT have fit had it kept it.
			wouldFitWithSuffix := utf8.RuneCountInString(title+" | "+brand) <= 60
			if !hasSuffix && wouldFitWithSuffix {
				t.Errorf("lang=%q %s: title %q dropped the brand suffix but core+suffix fits in 60 runes", lang, path, title)
			}
			// A title carrying the suffix must itself still fit the budget —
			// covered by TestTitlesAndDescriptionsFitTheirBudget, restated here
			// as the other half of the same rule.
			if hasSuffix && !fits {
				t.Errorf("lang=%q %s: title %q carries the brand suffix and exceeds 60 runes", lang, path, title)
			}
		}
	}
}

// TestNotFoundPageHasNoIndexAndNoCanonical is the mutation-proof for "restore
// canonical on 404": a rendered error page must not be indexed or claim a
// canonical URL for a page that does not exist.
func TestNotFoundPageHasNoIndexAndNoCanonical(t *testing.T) {
	rr := renderer(t, seoFixture(t))
	for _, path := range []string{"/area/does-not-exist", "/en/area/does-not-exist"} {
		rec := fetch(t, rr, path)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", path, rec.Code)
		}
		body := rec.Body.String()
		if !hasMarker(body, `<meta name="robots" content="noindex">`) {
			t.Errorf("%s: 404 page does not carry robots noindex:\n%s", path, body)
		}
		if hasMarker(body, `rel="canonical"`) {
			t.Errorf("%s: 404 page carries a canonical link for a URL that does not exist", path)
		}
		if hasMarker(body, `rel="alternate"`) {
			t.Errorf("%s: 404 page carries hreflang alternates for a URL that does not exist", path)
		}
	}
}

// TestAreaTitlesNameTheAQI pins the SEO decision that every area page kind
// carries the AQI term in its title, in both languages.
func TestAreaTitlesNameTheAQI(t *testing.T) {
	rr := renderer(t, seoFixture(t))
	for _, path := range []string{"/area/plovdiv", "/area/mladost", "/area/smolyan-oblast"} {
		for _, prefix := range []string{"", "/en"} {
			if got := pageTitle(t, fetch(t, rr, prefix+path).Body.String()); !strings.Contains(got, "AQI") {
				t.Errorf("%s%s title %q lacks AQI", prefix, path, got)
			}
		}
	}
}

// pageDataForCompile keeps web.PageData's Title/Description/NoIndex fields
// referenced so a future rename fails this package's build rather than only
// failing at template execution time.
var _ = web.PageData{Title: "", Description: "", NoIndex: false}
