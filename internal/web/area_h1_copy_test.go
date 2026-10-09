package web_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"kanarche.eu/internal/api"
	"kanarche.eu/internal/snapshot"
)

// h1Fixture adds the areas the h1 cases need to the shared area fixture: a
// city whose BG name starts with в (Варна's neighbour Враца takes во), the
// в/ф cases (Варна, Враца, Велико Търново) and a Sofia district.
func h1Fixture(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	snap := areaPageFixture(t)
	oblast := func(slug, name, nameEN string, lon, lat float64) snapshot.AreaMeta {
		return snapshot.AreaMeta{Slug: slug, Kind: "oblast", NameBG: name, NameEN: nameEN,
			CentroidLon: lon, CentroidLat: lat, DefaultZoom: 9, Covered: true, SensorCount: 10}
	}
	city := func(slug, name, nameEN, parent string, lon, lat float64, covered bool) snapshot.AreaMeta {
		return snapshot.AreaMeta{Slug: slug, Kind: "city", NameBG: name, NameEN: nameEN,
			ParentSlug: parent, CentroidLon: lon, CentroidLat: lat, DefaultZoom: 11,
			Covered: covered, SensorCount: 5, Values: map[string]float64{"P2": 11.0}}
	}
	snap.KnownSlugs["varna-oblast"] = oblast("varna-oblast", "Варна", "Varna", 27.9, 43.2)
	snap.KnownSlugs["varna"] = city("varna", "Варна", "Varna", "varna-oblast", 27.91, 43.21, true)
	snap.KnownSlugs["vratsa-oblast"] = oblast("vratsa-oblast", "Враца", "Vratsa", 23.55, 43.2)
	snap.KnownSlugs["vratsa"] = city("vratsa", "Враца", "Vratsa", "vratsa-oblast", 23.55, 43.2, true)
	snap.KnownSlugs["veliko-tarnovo-oblast"] = oblast("veliko-tarnovo-oblast", "Велико Търново", "Veliko Tarnovo", 25.6, 43.08)
	snap.KnownSlugs["veliko-tarnovo"] = city("veliko-tarnovo", "Велико Търново", "Veliko Tarnovo", "veliko-tarnovo-oblast", 25.62, 43.08, true)
	snap.KnownSlugs["vazrazhdane"] = snapshot.AreaMeta{Slug: "vazrazhdane", Kind: "neighbourhood",
		NameBG: "Възраждане", NameEN: "Vazrazhdane", ParentSlug: "sofiya",
		CentroidLon: 23.3, CentroidLat: 42.69, DefaultZoom: 13, Covered: true, SensorCount: 3,
		Values: map[string]float64{"P2": 7.0}}
	return snap
}

// h1Of returns the text of the page's h1, or "" when there is none.
func h1Of(body string) string {
	m := regexp.MustCompile(`<h1 class="t-title">([^<]*)</h1>`).FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return m[1]
}

// TestAreaH1IsDescriptive: the h1 names the air quality of the area in both
// languages. "во" before В/Ф names is the one grammar case that needs code; the
// rest is a catalogue form per kind.
func TestAreaH1IsDescriptive(t *testing.T) {
	rr := renderer(t, h1Fixture(t))
	cases := []struct {
		path, want string
	}{
		{"/area/varna", "Качество на въздуха във Варна"},
		{"/area/vratsa", "Качество на въздуха във Враца"},
		{"/area/veliko-tarnovo", "Качество на въздуха във Велико Търново"},
		{"/area/plovdiv", "Качество на въздуха в Пловдив"},
		{"/area/varna-oblast", "Качество на въздуха в област Варна"},
		{"/area/sofiyska-oblast", "Качество на въздуха в Софийска област"},
		{"/area/vazrazhdane", "Качество на въздуха в район Възраждане"},
		{"/en/area/varna", "Air quality in Varna"},
		{"/en/area/vratsa", "Air quality in Vratsa"},
		{"/en/area/veliko-tarnovo", "Air quality in Veliko Tarnovo"},
		{"/en/area/plovdiv", "Air quality in Plovdiv"},
		{"/en/area/varna-oblast", "Air quality in Varna Province"},
		{"/en/area/sofiyska-oblast", "Air quality in Sofia Province"},
		{"/en/area/vazrazhdane", "Air quality in Vazrazhdane district, Sofia"},
	}
	for _, c := range cases {
		body := fetch(t, rr, c.path).Body.String()
		if got := h1Of(body); got != c.want {
			t.Errorf("%s: h1 = %q, want %q", c.path, got, c.want)
		}
	}

	// The breadcrumb's current crumb keeps the bare name: only the h1 got the sentence.
	body := fetch(t, rr, "/area/varna").Body.String()
	if !strings.Contains(body, `<span aria-current="page">Варна</span>`) {
		t.Error("/area/varna: the current breadcrumb crumb is no longer the bare name")
	}
}

// TestAreaVavOnlyForInitialVAndF: the във form is chosen by the first letter
// of a city's name, and only в/В/ф/Ф take it. Пловдив (П) keeps в.
func TestAreaVavOnlyForInitialVAndF(t *testing.T) {
	snap := h1Fixture(t)
	snap.KnownSlugs["fakhria"] = snapshot.AreaMeta{Slug: "fakhria", Kind: "city", NameBG: "Фахрия", NameEN: "Fakhriya",
		ParentSlug: "plovdiv-oblast", CentroidLon: 24.7, CentroidLat: 42.1, DefaultZoom: 11,
		Covered: true, SensorCount: 2, Values: map[string]float64{"P2": 4.0}}
	rr := renderer(t, snap)
	if got, want := h1Of(fetch(t, rr, "/area/fakhria").Body.String()), "Качество на въздуха във Фахрия"; got != want {
		t.Errorf("h1 = %q, want %q", got, want)
	}
}

// TestHomeAboutPMUsesScaleEdges: the map home carries the PM explainer, and its
// PM2.5 thresholds are the EAQI upper edges from api.Scales(), not typed in.
func TestHomeAboutPMUsesScaleEdges(t *testing.T) {
	rr := renderer(t, areaPageFixture(t))

	var edges []string
	for _, s := range api.Scales() {
		if s.Metric != "P2" || s.Name != "eaqi" {
			continue
		}
		for _, b := range s.Bands {
			if b.Upper == nil {
				break
			}
			edges = append(edges, strconv.FormatFloat(*b.Upper, 'f', -1, 64))
		}
		break
	}
	if len(edges) != 5 {
		t.Fatalf("PM2.5 EAQI scale has %d finite edges, want 5", len(edges))
	}

	bgThresholds := fmt.Sprintf("За ФПЧ2.5 доброто е до %s, задоволителното до %s, умереното до %s, лошото до %s, много лошото до %s, а над това е изключително лошо.",
		edges[0], edges[1], edges[2], edges[3], edges[4])
	enThresholds := fmt.Sprintf("For PM2.5, good is up to %s, fair up to %s, moderate up to %s, poor up to %s, very poor up to %s, and above that extremely poor.",
		edges[0], edges[1], edges[2], edges[3], edges[4])

	const bgPara = `<p class="t-body">ФПЧ2.5 и ФПЧ10 са фини прахови частици във въздуха, с диаметър до 2,5 и до 10 микрона. Стойностите са в µg/m³. Повечето данни идват от сензори на доброволци от мрежата sensor.community, а до тях са показани и станциите на ИАОС. Цветовете следват класовете на Европейския индекс за качество на въздуха. ` + "За ФПЧ2.5 доброто е до 5, задоволителното до 15, умереното до 50, лошото до 90, много лошото до 140, а над това е изключително лошо." + `</p>`
	const enPara = `<p class="t-body">PM2.5 and PM10 are fine particles in the air, up to 2.5 and up to 10 microns across. Values are in µg/m³. Most readings come from volunteer sensors in the sensor.community network, and the official stations of the Executive Environment Agency are shown alongside. Colours follow the European Air Quality Index bands. For PM2.5, good is up to 5, fair up to 15, moderate up to 50, poor up to 90, very poor up to 140, and above that extremely poor.</p>`

	bg := fetch(t, rr, "/").Body.String()
	if !strings.Contains(bg, bgPara) {
		t.Error("BG home: the PM explainer paragraph is missing or its text differs")
	}
	if !strings.Contains(bg, bgThresholds) {
		t.Errorf("BG home: thresholds differ from api.Scales() edges, want %q", bgThresholds)
	}
	en := fetch(t, rr, "/en/").Body.String()
	if !strings.Contains(en, enPara) {
		t.Error("EN home: the PM explainer paragraph is missing or its text differs")
	}
	if !strings.Contains(en, enThresholds) {
		t.Errorf("EN home: thresholds differ from api.Scales() edges, want %q", enThresholds)
	}
	if strings.Contains(fetch(t, rr, "/areas").Body.String(), "За ФПЧ2.5") {
		t.Error("/areas: the map-home PM explainer must not appear on the directory")
	}
}

// TestUncoveredAreaSEOSaysNoDataYet: an area with no covered sensors gets the
// honest "no data yet" title and description, stays noindex, and still links
// its nearby areas, which the description promises.
func TestUncoveredAreaSEOSaysNoDataYet(t *testing.T) {
	rr := renderer(t, h1Fixture(t))
	metaTitle := regexp.MustCompile(`<title>([^<]*)</title>`)
	metaDesc := regexp.MustCompile(`<meta name="description" content="([^"]*)">`)
	cases := []struct {
		path, title, desc string
	}{
		{"/area/silistra",
			"Силистра: още няма данни за въздуха",
			"Силистра: засега няма сензори с надеждни данни. Вижте картата и съседните райони за най-близките измервания на ФПЧ2.5 и ФПЧ10."},
		{"/area/silistra-oblast",
			"Област Силистра: още няма данни за въздуха",
			"Област Силистра: засега няма сензори с надеждни данни. Вижте картата и съседните райони за най-близките измервания на ФПЧ2.5 и ФПЧ10."},
		{"/en/area/silistra",
			"Silistra: no air quality data yet",
			"Silistra: no sensors are reporting usable air quality data yet. See the map and nearby areas for the closest PM2.5 and PM10 readings."},
		{"/en/area/silistra-oblast",
			"Silistra Province: no air quality data yet",
			"Silistra Province: no sensors are reporting usable air quality data yet. See the map and nearby areas for the closest PM2.5 and PM10 readings."},
	}
	for _, c := range cases {
		body := fetch(t, rr, c.path).Body.String()
		if m := metaTitle.FindStringSubmatch(body); m == nil || !strings.HasPrefix(m[1], c.title) {
			t.Errorf("%s: title = %v, want prefix %q", c.path, m, c.title)
		}
		if m := metaDesc.FindStringSubmatch(body); m == nil || m[1] != c.desc {
			t.Errorf("%s: description = %v, want %q", c.path, m, c.desc)
		}
		if !strings.Contains(body, `<meta name="robots" content="noindex">`) {
			t.Errorf("%s: uncovered area lost its noindex", c.path)
		}
		// Links carry the /en prefix on the English pages, so match the slug tail only.
		if !strings.Contains(body, `<nav class="area-links"`) || !strings.Contains(body, `area/varna"`) {
			t.Errorf("%s: nearby areas are not rendered, but the description promises them", c.path)
		}
	}

	// A covered area keeps its descriptive copy and is not told there is no data.
	if body := fetch(t, rr, "/area/varna").Body.String(); strings.Contains(body, "още няма данни") {
		t.Error("/area/varna: covered area carries the no-data title")
	}
}
