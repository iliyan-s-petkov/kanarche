package web_test

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

const nb = "\u00a0"

var factsSectionRE = regexp.MustCompile(`(?s)<section class="area-facts"[^>]*>.*?</section>`)

// factsText is the visible text of the facts section, one paragraph per line.
func factsText(t *testing.T, body string) string {
	t.Helper()
	sec := factsSectionRE.FindString(body)
	if sec == "" {
		t.Fatalf("no facts section in page")
	}
	sec = strings.ReplaceAll(sec, "</p>", "\n")
	return html.UnescapeString(tagRE.ReplaceAllString(sec, ""))
}

// factsSnapshot is 27 cities, 28 oblasts and 24 Sofia districts with known values, sizes and two stations.
// Shumen is 9th of 27 by PM2.5 with the median 19.0. Razgrad is 24th of 28 oblasts by size, Sredets 22nd of 24.
func factsSnapshot() *snapshot.Snapshot {
	areas := map[string]snapshot.AreaMeta{}
	add := func(m snapshot.AreaMeta) { areas[m.Slug] = m }
	val := func(v float64) map[string]float64 { return map[string]float64{"P2": v} }

	add(snapshot.AreaMeta{Slug: "shumen", Kind: "city", NameBG: "Шумен", NameEN: "Shumen", CentroidLon: 26.93, CentroidLat: 43.27,
		Covered: true, SensorCount: 12, Values: val(14.2), ParentSlug: "shumen-oblast", Km2: 66,
		BySource: map[string]snapshot.SourceEntry{"sensor.community": {N: 11}, "eea": {N: 1}}, Stations: []string{"BG0038A"}})
	add(snapshot.AreaMeta{Slug: "pleven", Kind: "city", NameBG: "Плевен", NameEN: "Pleven", CentroidLon: 24.62, CentroidLat: 43.41,
		Covered: true, SensorCount: 9, Values: val(15), ParentSlug: "pleven-oblast", Km2: 1044, Source: "sensor.community"})
	for i := 1; i <= 25; i++ {
		v := float64(5 + i)
		if i > 8 {
			v = float64(7 + i)
		}
		add(snapshot.AreaMeta{Slug: fmt.Sprintf("city%02d", i), Kind: "city", NameBG: fmt.Sprintf("Град%02d", i), NameEN: fmt.Sprintf("Town%02d", i),
			Covered: true, SensorCount: 6, Values: val(v), Source: "sensor.community"})
	}

	add(snapshot.AreaMeta{Slug: "razgrad-oblast", Kind: "oblast", NameBG: "Разград", NameEN: "Razgrad", CentroidLon: 26.549, CentroidLat: 43.547,
		Covered: true, SensorCount: 9, Values: val(20), Km2: 2640, Source: "sensor.community"})
	add(snapshot.AreaMeta{Slug: "shumen-oblast", Kind: "oblast", NameBG: "Шумен", NameEN: "Shumen", Covered: true, SensorCount: 12, Values: val(11), Km2: 1000,
		BySource: map[string]snapshot.SourceEntry{"sensor.community": {N: 11}, "eea": {N: 1}}, Stations: []string{"BG0038A"}})
	add(snapshot.AreaMeta{Slug: "pleven-oblast", Kind: "oblast", NameBG: "Плевен", NameEN: "Pleven", Covered: true, SensorCount: 9, Values: val(12), Km2: 1000, Source: "sensor.community"})
	for i := 1; i <= 25; i++ {
		km := 3000 + float64(i)
		if i > 23 {
			km = 500
		}
		add(snapshot.AreaMeta{Slug: fmt.Sprintf("obl%02d", i), Kind: "oblast", NameBG: fmt.Sprintf("Обл%02d", i), NameEN: fmt.Sprintf("Obl%02d", i),
			Covered: true, SensorCount: 5, Values: val(10 + float64(i)), Km2: km, Source: "sensor.community"})
	}

	add(snapshot.AreaMeta{Slug: "sredets", Kind: "neighbourhood", NameBG: "Средец", NameEN: "Sredets", CentroidLon: 23.33, CentroidLat: 42.69,
		Covered: true, SensorCount: 14, Values: val(11.8), Km2: 3.1, Source: "sensor.community"})
	for i := 1; i <= 23; i++ {
		km := 10 + float64(i)
		if i > 21 {
			km = 1
		}
		add(snapshot.AreaMeta{Slug: fmt.Sprintf("dist%02d", i), Kind: "neighbourhood", NameBG: fmt.Sprintf("Рай%02d", i), NameEN: fmt.Sprintf("Dis%02d", i),
			Covered: true, SensorCount: 4, Values: val(9 + float64(i)/10), Km2: km, Source: "sensor.community"})
	}

	return &snapshot.Snapshot{
		GeneratedAt: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
		KnownSlugs:  areas,
		Stations: map[string]snapshot.OfficialStation{
			"BG0038A": {Code: "BG0038A", Lon: 26.9333, Lat: 43.2667},
			"BG0050A": {Code: "BG0050A", Lon: 23.30, Lat: 42.68},
		},
	}
}

func hasLine(text, want string) bool {
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}

func TestFactsBlocksBulgarianCityWithOwnBoundary(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	text := factsText(t, fetch(t, rr, "/area/shumen").Body.String())
	for _, want := range []string{
		"За въздуха в Шумен",
		"Шумен е областен център в област Шумен. Границата на града, по която отнасяме сензорите, обхваща 66 км².",
		"В Шумен има 11 граждански сензора и 1 официална станция на ИАОС: АИС Шумен. АИС Шумен е градска фонова станция.",
		"Шумен сега е на 9-то място от 27 областни центъра по ФПЧ2.5, считано от най-чистия въздух. Стойността е 14,2 µg/m³, а медианата за всички 27 е 19,0 µg/m³. Подредбата се променя с всяко обновяване на данните.",
	} {
		if !hasLine(text, want) {
			t.Errorf("missing line %q in:\n%s", want, text)
		}
	}
}

func TestFactsBlocksEnglishCityWithOwnBoundary(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	text := factsText(t, fetch(t, rr, "/en/area/shumen").Body.String())
	for _, want := range []string{
		"About the air in Shumen",
		"Shumen is a provincial capital in Shumen Province. The city boundary we use to assign sensors covers 66 km².",
		"Shumen has 11 citizen sensors and 1 official station of the Executive Environment Agency: AIS Shumen. AIS Shumen is an urban background station.",
		"Shumen currently ranks 9th of 27 provincial capitals for PM2.5, counting from the cleanest air. The value is 14.2 µg/m³ and the median for all 27 is 19.0 µg/m³. The ranking changes with every data update.",
	} {
		if !hasLine(text, want) {
			t.Errorf("missing line %q in:\n%s", want, text)
		}
	}
}

func TestFactsPlaceForMunicipalityCity(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	bg := factsText(t, fetch(t, rr, "/area/pleven").Body.String())
	want := "Плевен е областен център в област Плевен. За него използваме границата на цялата община. Тя обхваща 1" + nb + "044 км² и е по-широка от самия град."
	if !hasLine(bg, want) {
		t.Errorf("missing %q in:\n%s", want, bg)
	}
	en := factsText(t, fetch(t, rr, "/en/area/pleven").Body.String())
	want = "Pleven is a provincial capital in Pleven Province. We use the boundary of the whole municipality. It covers 1,044 km² and is wider than the city itself."
	if !hasLine(en, want) {
		t.Errorf("missing %q in:\n%s", want, en)
	}
}

func TestFactsPlaceForDistrictAndOblast(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	cases := []struct{ path, want string }{
		{"/area/sredets", "Район Средец е един от 24 района на София. Обхваща 3,1 км². По площ е на 22-ро място, считано от най-големия."},
		{"/en/area/sredets", "Sredets district is one of the 24 districts of Sofia. It covers 3.1 km². It ranks 22nd by size, counting from the largest."},
		{"/area/razgrad-oblast", "Област Разград е една от 28 области в България. Обхваща 2" + nb + "640 км². По площ е на 24-то място, считано от най-голямата."},
		{"/en/area/razgrad-oblast", "Razgrad Province is one of the 28 provinces of Bulgaria. It covers 2,640 km². It ranks 24th by size, counting from the largest."},
	}
	for _, c := range cases {
		text := factsText(t, fetch(t, rr, c.path).Body.String())
		if !hasLine(text, c.want) {
			t.Errorf("%s: missing %q in:\n%s", c.path, c.want, text)
		}
	}
}

func TestFactsNetworkWithoutInsideStationNamesTheNearest(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	bg := factsText(t, fetch(t, rr, "/area/razgrad-oblast").Body.String())
	wantBG := "В област Разград има 9 граждански сензора и няма официална станция на ИАОС. Най-близката официална станция е АИС Шумен. Тя е на 44 км югоизточно от средата на област Разград. Стойностите ѝ може да се различават от тези в област Разград. АИС Шумен е градска фонова станция."
	if !hasLine(bg, wantBG) {
		t.Errorf("missing %q in:\n%s", wantBG, bg)
	}
	en := factsText(t, fetch(t, rr, "/en/area/razgrad-oblast").Body.String())
	wantEN := "Razgrad Province has 9 citizen sensors and no official station of the Executive Environment Agency. The nearest official station is AIS Shumen. It is 44 km south-east of the centre of Razgrad Province. Its values may differ from those in Razgrad Province. AIS Shumen is an urban background station."
	if !hasLine(en, wantEN) {
		t.Errorf("missing %q in:\n%s", wantEN, en)
	}
}

func TestFactsNearestStationNameIsTransliteratedInEnglish(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	en := factsText(t, fetch(t, rr, "/en/area/sredets").Body.String())
	if !strings.Contains(en, "The nearest official station is Sofia - AIS Hipodruma.") {
		t.Errorf("EN nearest station not transliterated:\n%s", en)
	}
	bg := factsText(t, fetch(t, rr, "/area/sredets").Body.String())
	if !strings.Contains(bg, "Най-близката официална станция е София - АИС Хиподрума.") {
		t.Errorf("BG nearest station name changed:\n%s", bg)
	}
}

// networkCase mutates Shumen so the four count variants and the one-station and many-stations lists show up.
func networkSnapshot(sensors int, stations ...string) *snapshot.Snapshot {
	snap := factsSnapshot()
	m := snap.KnownSlugs["shumen"]
	m.SensorCount = sensors + len(stations)
	m.BySource = nil
	m.Source = "sensor.community"
	if len(stations) > 0 {
		m.BySource = map[string]snapshot.SourceEntry{"sensor.community": {N: sensors}, "eea": {N: len(stations)}}
		m.Source = ""
	}
	m.Stations = stations
	snap.KnownSlugs["shumen"] = m
	return snap
}

func TestFactsNetworkCountVariants(t *testing.T) {
	cases := []struct {
		name     string
		snap     *snapshot.Snapshot
		path     string
		wantLine string
	}{
		{"k=1 n=1", networkSnapshot(1, "BG0038A"), "/area/shumen",
			"В Шумен има 1 граждански сензор и 1 официална станция на ИАОС: АИС Шумен. АИС Шумен е градска фонова станция."},
		{"k=2", networkSnapshot(3, "BG0038A", "BG0050A"), "/area/shumen",
			"В Шумен има 3 граждански сензора и 2 официални станции на ИАОС: АИС Шумен и София - АИС Хиподрума."},
		{"k=0 n=3", networkSnapshot(3), "/area/shumen",
			"В Шумен има 3 граждански сензора и няма официална станция на ИАОС. Най-близката официална станция е АИС Шумен."},
		{"k=0 n=1", networkSnapshot(1), "/en/area/shumen",
			"Shumen has 1 citizen sensor and no official station of the Executive Environment Agency. The nearest official station is AIS Shumen."},
		{"n=0 k=1", networkSnapshot(0, "BG0038A"), "/area/shumen",
			"В Шумен няма граждански сензори. Има 1 официална станция на ИАОС: АИС Шумен. АИС Шумен е градска фонова станция."},
		{"n=0 k=2 en", networkSnapshot(0, "BG0038A", "BG0050A"), "/en/area/shumen",
			"Shumen has no citizen sensors. It has 2 official stations of the Executive Environment Agency: AIS Shumen and Sofia - AIS Hipodruma."},
		{"n=0 k=0", networkSnapshot(0), "/area/shumen",
			"В Шумен няма граждански сензори и няма официална станция на ИАОС."},
	}
	for _, c := range cases {
		rr := renderer(t, c.snap)
		text := factsText(t, fetch(t, rr, c.path).Body.String())
		found := false
		for _, l := range strings.Split(text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), c.wantLine) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no line starts with %q in:\n%s", c.name, c.wantLine, text)
		}
	}
}

func TestFactsRankHiddenWithoutValueOrPeers(t *testing.T) {
	// No current value for Shumen.
	snap := factsSnapshot()
	m := snap.KnownSlugs["shumen"]
	m.Values = nil
	snap.KnownSlugs["shumen"] = m
	text := factsText(t, fetch(t, renderer(t, snap), "/area/shumen").Body.String())
	if strings.Contains(text, "място от") {
		t.Errorf("rank shown without a value:\n%s", text)
	}

	// Four cities with a value is below the five-peer floor, five is enough.
	for _, c := range []struct {
		withValue int
		want      bool
	}{{4, false}, {5, true}} {
		snap := factsSnapshot()
		n := 0
		for slug, m := range snap.KnownSlugs {
			if m.Kind != "city" || slug == "shumen" {
				continue
			}
			n++
			if n > c.withValue-1 {
				m.Values = nil
				snap.KnownSlugs[slug] = m
			}
		}
		text := factsText(t, fetch(t, renderer(t, snap), "/area/shumen").Body.String())
		if got := strings.Contains(text, "място от"); got != c.want {
			t.Errorf("%d cities with a value: rank shown = %v, want %v", c.withValue, got, c.want)
		}
	}
}

func TestFactsRankFirstPlaceVariant(t *testing.T) {
	snap := factsSnapshot()
	m := snap.KnownSlugs["shumen"]
	m.Values = map[string]float64{"P2": 1.5}
	snap.KnownSlugs["shumen"] = m
	rr := renderer(t, snap)
	bg := factsText(t, fetch(t, rr, "/area/shumen").Body.String())
	if want := "Шумен сега е на 1-во място от 27 областни центъра по ФПЧ2.5. Това е най-чистият въздух сред тях."; !strings.Contains(bg, want) {
		t.Errorf("BG first place: missing %q in:\n%s", want, bg)
	}
	en := factsText(t, fetch(t, rr, "/en/area/shumen").Body.String())
	if want := "Shumen currently ranks 1st of 27 provincial capitals for PM2.5. This is the cleanest air among them."; !strings.Contains(en, want) {
		t.Errorf("EN first place: missing %q in:\n%s", want, en)
	}
	if strings.Contains(bg, "считано от най-чистия") {
		t.Errorf("first place still says counting from the cleanest:\n%s", bg)
	}
}

func TestFactsPlaceDropsSizeWhenUnknown(t *testing.T) {
	snap := factsSnapshot()
	for _, slug := range []string{"shumen", "razgrad-oblast"} {
		m := snap.KnownSlugs[slug]
		m.Km2 = 0
		snap.KnownSlugs[slug] = m
	}
	rr := renderer(t, snap)
	text := factsText(t, fetch(t, rr, "/area/shumen").Body.String())
	if !hasLine(text, "Шумен е областен център в област Шумен.") || strings.Contains(text, "км²") {
		t.Errorf("city without size:\n%s", text)
	}
	text = factsText(t, fetch(t, rr, "/area/razgrad-oblast").Body.String())
	if !hasLine(text, "Област Разград е една от 28 области в България.") || strings.Contains(text, "км²") || strings.Contains(text, "По площ") {
		t.Errorf("oblast without size:\n%s", text)
	}
}

func TestFactsDifferBetweenAreas(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	seen := map[string]string{}
	for _, slug := range []string{"shumen", "pleven", "sredets", "razgrad-oblast", "city01"} {
		for _, lang := range []string{"", "/en"} {
			text := factsText(t, fetch(t, rr, lang+"/area/"+slug).Body.String())
			if other, ok := seen[text]; ok {
				t.Errorf("%s%s has the same facts as %s", lang, slug, other)
			}
			seen[text] = lang + slug
		}
	}
}

func TestFAQHasThreeAreaQuestionsAndMatchesJSONLD(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	body := fetch(t, rr, "/area/shumen").Body.String()
	items := faqVisible(t, body)
	wantQ := []string{
		"Колко чист е въздухът в Шумен в сравнение с другите областни центрове?",
		"Има ли официална станция в Шумен?",
		"Каква територия обхваща страницата за Шумен?",
	}
	if len(items) != 3 {
		t.Fatalf("%d FAQ items, want 3", len(items))
	}
	for i, q := range wantQ {
		if items[i].Q != q {
			t.Errorf("q%d = %q, want %q", i+1, items[i].Q, q)
		}
	}
	wantA1 := "Шумен сега е на 9-то място от 27 областни центъра по ФПЧ2.5, считано от най-чистия въздух. Стойността е 14,2 µg/m³, а медианата за всички 27 е 19,0 µg/m³. Стойностите са от граждански сензори."
	if items[0].A != wantA1 {
		t.Errorf("a1 = %q, want %q", items[0].A, wantA1)
	}
	if want := "В Шумен има 11 граждански сензора и 1 официална станция на ИАОС: АИС Шумен. АИС Шумен е градска фонова станция."; items[1].A != want {
		t.Errorf("a2 = %q, want %q", items[1].A, want)
	}
	if want := "Шумен е областен център в област Шумен. Границата на града, по която отнасяме сензорите, обхваща 66 км²."; items[2].A != want {
		t.Errorf("a3 = %q, want %q", items[2].A, want)
	}
}

func TestFAQDropsRankQuestionWhenRankIsHidden(t *testing.T) {
	snap := factsSnapshot()
	m := snap.KnownSlugs["shumen"]
	m.Values = nil
	snap.KnownSlugs["shumen"] = m
	rr := renderer(t, snap)
	for _, lang := range []string{"", "/en"} {
		body := fetch(t, rr, lang+"/area/shumen").Body.String()
		items := faqVisible(t, body)
		if len(items) != 2 {
			t.Errorf("%s: %d FAQ items, want 2 without the rank", lang, len(items))
		}
		if strings.Contains(body, "compared with other") || strings.Contains(body, "в сравнение с другите") {
			t.Errorf("%s: rank question still rendered", lang)
		}
	}
}

func TestFAQEnglishQuestions(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	items := faqVisible(t, fetch(t, rr, "/en/area/shumen").Body.String())
	want := []string{
		"How clean is the air in Shumen compared with other provincial capitals?",
		"Is there an official monitoring station in Shumen?",
		"What area does the page for Shumen cover?",
	}
	for i, q := range want {
		if items[i].Q != q {
			t.Errorf("q%d = %q, want %q", i+1, items[i].Q, q)
		}
	}
	if !strings.HasSuffix(items[0].A, "The values come from citizen sensors.") {
		t.Errorf("a1 = %q", items[0].A)
	}
}

func TestFAQLinksToTheSharedPages(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	cases := []struct{ path, about, air, line string }{
		{"/area/shumen", "/about-the-data", "/air-pollution", "Общите въпроси за данните, цветовете и обновяването са в За данните."},
		{"/en/area/shumen", "/en/about-the-data", "/en/air-pollution", "The general questions about the data, the colours and the update times are in About the data."},
	}
	for _, c := range cases {
		body := fetch(t, rr, c.path).Body.String()
		for _, href := range []string{c.about, c.air} {
			if !strings.Contains(body, `href="`+href+`"`) {
				t.Errorf("%s: no link to %s", c.path, href)
			}
			if rec := fetch(t, rr, href); rec.Code != 200 {
				t.Errorf("%s answers %d", href, rec.Code)
			}
		}
		text := html.UnescapeString(tagRE.ReplaceAllString(body, ""))
		if !strings.Contains(text, c.line) {
			t.Errorf("%s: missing line %q", c.path, c.line)
		}
	}
}

func TestFactsSectionComesBeforeTheFAQ(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	body := fetch(t, rr, "/area/shumen").Body.String()
	facts := strings.Index(body, `<section class="area-facts"`)
	faq := strings.Index(body, `<section class="faq"`)
	if facts < 0 || faq < 0 || facts > faq {
		t.Fatalf("facts at %d, faq at %d", facts, faq)
	}
	text := factsText(t, body)
	order := []string{"Шумен е областен център", "В Шумен има", "Шумен сега е"}
	last := -1
	for _, s := range order {
		i := strings.Index(text, s)
		if i < 0 || i < last {
			t.Errorf("block %q out of order (at %d after %d)", s, i, last)
		}
		last = i
	}
}

func TestFactsLeaveNoRawPlaceholderOrForbiddenPunctuation(t *testing.T) {
	snap := factsSnapshot()
	rr := renderer(t, snap)
	for slug := range snap.KnownSlugs {
		if strings.Contains(slug, "city") && slug != "city01" || strings.HasPrefix(slug, "obl") && slug != "obl01" || strings.HasPrefix(slug, "dist") && slug != "dist01" {
			continue
		}
		for _, lang := range []string{"", "/en"} {
			text := factsText(t, fetch(t, rr, lang+"/area/"+slug).Body.String())
			if strings.ContainsAny(text, "{}") {
				t.Errorf("%s%s: raw placeholder in facts: %q", lang, slug, text)
			}
			if strings.ContainsAny(text, ";—–") {
				t.Errorf("%s%s: semicolon or dash in facts: %q", lang, slug, text)
			}
		}
	}
}
