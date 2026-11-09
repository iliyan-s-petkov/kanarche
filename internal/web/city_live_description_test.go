package web_test

import (
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
)

// liveCityFixture holds one city per case: в/във names, a plain name, and cities without full readings.
func liveCityFixture(generatedAt time.Time) *snapshot.Snapshot {
	city := func(slug, bg, en string, values map[string]float64) snapshot.AreaMeta {
		return snapshot.AreaMeta{Slug: slug, Kind: "city", NameBG: bg, NameEN: en,
			DefaultZoom: 11, Covered: true, SensorCount: 9, Values: values}
	}
	both := map[string]float64{"P2": 12.44, "P1": 21.06}
	return &snapshot.Snapshot{
		GeneratedAt: generatedAt,
		KnownSlugs: map[string]snapshot.AreaMeta{
			"plovdiv":  city("plovdiv", "Пловдив", "Plovdiv", both),
			"varna":    city("varna", "Варна", "Varna", both),
			"vidin":    city("vidin", "Видин", "Vidin", both),
			"smolyan":  city("smolyan", "Смолян", "Smolyan", nil),
			"pm25only": city("pm25only", "Русе", "Ruse", map[string]float64{"P2": 12.4}),
			"mladost": {Slug: "mladost", Kind: "neighbourhood", NameBG: "Младост", NameEN: "Mladost",
				DefaultZoom: 13, Covered: true, SensorCount: 5, Values: both},
		},
	}
}

func cityDescriptions(t *testing.T, snap *snapshot.Snapshot, path string) (meta, og, tw string) {
	t.Helper()
	body := fetch(t, renderer(t, snap), path).Body.String()
	return pageDescription(t, body),
		tagAttr(t, body, `<meta property="og:description" content="`),
		tagAttr(t, body, `<meta name="twitter:description" content="`)
}

// staticDescription is the catalogue's seo.city.description for a name, the fallback text.
func staticDescription(t *testing.T, lang, name string) string {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(cat.T(lang, "seo.city.description"), "{name}", name)
}

func TestCityDescriptionLiveBG(t *testing.T) {
	got, _, _ := cityDescriptions(t, liveCityFixture(time.Now()), "/area/plovdiv")
	want := "В Пловдив ФПЧ2.5 е 12,4 µg/m³ (задоволително), ФПЧ10 е 21,1 µg/m³. Индекс за качество на въздуха (AQI) от граждански сензори и станции на ИАОС, с графика за 24 часа."
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestCityDescriptionLiveEN(t *testing.T) {
	got, _, _ := cityDescriptions(t, liveCityFixture(time.Now()), "/en/area/plovdiv")
	want := "In Plovdiv PM2.5 is 12.4 µg/m³ (fair) and PM10 is 21.1 µg/m³. Air quality index (AQI) from citizen sensors and IAOS stations, with a 24-hour chart."
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestCityDescriptionUsesVavForVAndF(t *testing.T) {
	for _, c := range []struct{ path, prefix string }{
		{"/area/varna", "Във Варна ФПЧ2.5 е "},
		{"/area/vidin", "Във Видин ФПЧ2.5 е "},
	} {
		got, _, _ := cityDescriptions(t, liveCityFixture(time.Now()), c.path)
		if !strings.HasPrefix(got, c.prefix) {
			t.Errorf("%s: got %q, want prefix %q", c.path, got, c.prefix)
		}
	}
}

func TestCityDescriptionFallsBackWithoutData(t *testing.T) {
	snap := liveCityFixture(time.Now())
	for _, c := range []struct{ path, lang, name string }{
		{"/area/smolyan", i18n.DefaultLang, "Смолян"},
		{"/area/pm25only", i18n.DefaultLang, "Русе"},
		{"/en/area/smolyan", "en", "Smolyan"},
		{"/en/area/pm25only", "en", "Ruse"},
	} {
		got, _, _ := cityDescriptions(t, snap, c.path)
		if want := staticDescription(t, c.lang, c.name); got != want {
			t.Errorf("%s: got %q, want static %q", c.path, got, want)
		}
	}
}

func TestCityDescriptionFallsBackWhenStale(t *testing.T) {
	snap := liveCityFixture(time.Now().Add(-time.Hour))
	for _, c := range []struct{ path, lang, name string }{
		{"/area/plovdiv", i18n.DefaultLang, "Пловдив"},
		{"/en/area/plovdiv", "en", "Plovdiv"},
	} {
		got, _, _ := cityDescriptions(t, snap, c.path)
		if want := staticDescription(t, c.lang, c.name); got != want {
			t.Errorf("%s: stale snapshot rendered %q, want %q", c.path, got, want)
		}
	}
}

func TestCityDescriptionOGAndTwitterMatchMeta(t *testing.T) {
	for _, path := range []string{"/area/varna", "/en/area/varna", "/area/smolyan"} {
		meta, og, tw := cityDescriptions(t, liveCityFixture(time.Now()), path)
		if og != meta || tw != meta {
			t.Errorf("%s: meta %q og %q twitter %q", path, meta, og, tw)
		}
	}
}

func TestDistrictDescriptionStaysStatic(t *testing.T) {
	got, _, _ := cityDescriptions(t, liveCityFixture(time.Now()), "/en/area/mladost")
	if strings.Contains(got, "PM2.5 is") {
		t.Errorf("district description turned live: %q", got)
	}
}
