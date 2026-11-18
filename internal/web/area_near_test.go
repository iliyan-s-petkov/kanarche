package web_test

import (
	"fmt"
	"strings"
	"testing"

	"kanarche.eu/internal/snapshot"
)

// nearSnapshot gives the 27 cities a size and Shumen three known neighbours.
// Shumen has 11 citizen sensors on 66 km2, which is 16.7 per 100 km2.
// Ten cities are denser (6 sensors on 30 km2 is 20) and the rest are sparser, so Shumen is 11th of 27.
// Town02 is about 40 km east of Shumen, Town01 and Town03 about 56 km north and south.
func nearSnapshot() *snapshot.Snapshot {
	snap := factsSnapshot()
	for i := 1; i <= 25; i++ {
		slug := fmt.Sprintf("city%02d", i)
		m := snap.KnownSlugs[slug]
		m.Km2 = 60
		if i <= 10 {
			m.Km2 = 30
		}
		snap.KnownSlugs[slug] = m
	}
	place := func(slug string, lon, lat float64) {
		m := snap.KnownSlugs[slug]
		m.CentroidLon, m.CentroidLat = lon, lat
		snap.KnownSlugs[slug] = m
	}
	place("city01", 26.93, 43.77)
	place("city02", 27.43, 43.27)
	place("city03", 26.93, 42.77)
	return snap
}

func TestDensityLineAndRank(t *testing.T) {
	rr := renderer(t, nearSnapshot())
	bg := factsText(t, fetch(t, rr, "/area/shumen").Body.String())
	for _, want := range []string{
		"Шумен има 17 граждански сензора на 100 км². Шумен е на 11-то място от 27 областни центъра по брой граждански сензори на 100 км².",
	} {
		if !hasLine(bg, want) {
			t.Errorf("missing %q in:\n%s", want, bg)
		}
	}
	en := factsText(t, fetch(t, rr, "/en/area/shumen").Body.String())
	for _, want := range []string{
		"Shumen has 17 citizen sensors per 100 km². Shumen ranks 11th of 27 provincial capitals by citizen sensors per 100 km².",
	} {
		if !hasLine(en, want) {
			t.Errorf("missing %q in:\n%s", want, en)
		}
	}
}

// A BG district page starts its rank sentence with the capitalised subject, not the lowercase "район" of the plain name.
func TestDensityRankSentenceStartsCapitalisedOnBGDistrict(t *testing.T) {
	rr := renderer(t, sofiaOnlyFixture())
	bg := factsText(t, fetch(t, rr, "/area/mladost").Body.String())
	want := "Район Младост има 80 граждански сензора на 100 км². Район Младост е на 1-во място от 6 района по брой граждански сензори на 100 км²."
	if !hasLine(bg, want) {
		t.Errorf("missing %q in:\n%s", want, bg)
	}
	if strings.Contains(bg, "район Младост е на") {
		t.Errorf("rank sentence starts lowercase:\n%s", bg)
	}
}

func TestDensityKeepsTheLineWithoutEnoughPeers(t *testing.T) {
	rr := renderer(t, factsSnapshot())
	bg := factsText(t, fetch(t, rr, "/area/shumen").Body.String())
	if !hasLine(bg, "Шумен има 17 граждански сензора на 100 км².") {
		t.Errorf("density line missing:\n%s", bg)
	}
	if strings.Contains(bg, "По гъстота") {
		t.Errorf("density rank shown with too few sized peers:\n%s", bg)
	}
}

func TestDensityHiddenWithoutSizeOrCitizenSensors(t *testing.T) {
	snap := nearSnapshot()
	m := snap.KnownSlugs["shumen"]
	m.Km2 = 0
	snap.KnownSlugs["shumen"] = m
	rr := renderer(t, snap)
	if got := factsText(t, fetch(t, rr, "/area/shumen").Body.String()); strings.Contains(got, "на 100 км²") {
		t.Errorf("density shown without a size:\n%s", got)
	}
	snap = nearSnapshot()
	m = snap.KnownSlugs["shumen"]
	m.SensorCount, m.Stations = 1, []string{"BG0038A"}
	m.BySource = map[string]snapshot.SourceEntry{"eea": {N: 1}}
	snap.KnownSlugs["shumen"] = m
	rr = renderer(t, snap)
	if got := factsText(t, fetch(t, rr, "/area/shumen").Body.String()); strings.Contains(got, "на 100 км²") {
		t.Errorf("density shown without citizen sensors:\n%s", got)
	}
}

func TestNearbyListsThreeNearestWithDistanceDirectionAndValue(t *testing.T) {
	rr := renderer(t, nearSnapshot())
	bg := factsText(t, fetch(t, rr, "/area/shumen").Body.String())
	wantBG := "Най-близките областни центрове до Шумен са Град02 (40 км източно, 7,0 µg/m³), Град01 (56 км северно, 6,0 µg/m³) и Град03 (56 км южно, 8,0 µg/m³). По ФПЧ2.5 сега всички те имат по-чист въздух от Шумен."
	if !hasLine(bg, wantBG) {
		t.Errorf("missing %q in:\n%s", wantBG, bg)
	}
	en := factsText(t, fetch(t, rr, "/en/area/shumen").Body.String())
	wantEN := "The nearest provincial capitals to Shumen are Town02 (40 km east, 7.0 µg/m³), Town01 (56 km north, 6.0 µg/m³) and Town03 (56 km south, 8.0 µg/m³). On PM2.5 all of them have cleaner air than Shumen right now."
	if !hasLine(en, wantEN) {
		t.Errorf("missing %q in:\n%s", wantEN, en)
	}
}

func TestNearbyCleanerCountVariants(t *testing.T) {
	cases := []struct {
		name         string
		v1, v2, v3   float64
		bgEnd, enEnd string
	}{
		{"none", 20, 21, 22, "По ФПЧ2.5 сега никой от тях няма по-чист въздух от Шумен.", "On PM2.5 none of them has cleaner air than Shumen right now."},
		{"one", 20, 10, 22, "По ФПЧ2.5 сега един от тях има по-чист въздух от Шумен.", "On PM2.5 one of them has cleaner air than Shumen right now."},
		{"two", 20, 10, 11, "По ФПЧ2.5 сега 2 от тях имат по-чист въздух от Шумен.", "On PM2.5 2 of them have cleaner air than Shumen right now."},
	}
	for _, c := range cases {
		snap := nearSnapshot()
		for slug, v := range map[string]float64{"city01": c.v1, "city02": c.v2, "city03": c.v3} {
			m := snap.KnownSlugs[slug]
			m.Values = map[string]float64{"P2": v}
			snap.KnownSlugs[slug] = m
		}
		rr := renderer(t, snap)
		if got := factsText(t, fetch(t, rr, "/area/shumen").Body.String()); !strings.Contains(got, c.bgEnd+"\n") {
			t.Errorf("%s bg: missing %q in:\n%s", c.name, c.bgEnd, got)
		}
		if got := factsText(t, fetch(t, rr, "/en/area/shumen").Body.String()); !strings.Contains(got, c.enEnd+"\n") {
			t.Errorf("%s en: missing %q in:\n%s", c.name, c.enEnd, got)
		}
	}
}

func TestNearbySkipsNeighboursWithoutAValueAndHidesUnderTwo(t *testing.T) {
	snap := nearSnapshot()
	for i := 1; i <= 25; i++ {
		slug := fmt.Sprintf("city%02d", i)
		if i == 2 {
			continue
		}
		m := snap.KnownSlugs[slug]
		m.Values = nil
		snap.KnownSlugs[slug] = m
	}
	pl := snap.KnownSlugs["pleven"]
	pl.Values = nil
	snap.KnownSlugs["pleven"] = pl
	rr := renderer(t, snap)
	if got := factsText(t, fetch(t, rr, "/area/shumen").Body.String()); strings.Contains(got, "Най-близките") {
		t.Errorf("nearby shown with one neighbour value:\n%s", got)
	}
}

func TestNearbyForDistrictNamesDistricts(t *testing.T) {
	rr := renderer(t, nearSnapshot())
	bg := factsText(t, fetch(t, rr, "/area/sredets").Body.String())
	if !strings.Contains(bg, "Най-близките райони до район Средец са ") {
		t.Errorf("district nearby missing:\n%s", bg)
	}
}

func TestNearAndDensityCopyHasNoForbiddenPunctuation(t *testing.T) {
	rr := renderer(t, nearSnapshot())
	for _, path := range []string{"/area/shumen", "/en/area/shumen", "/area/sredets", "/en/area/razgrad-oblast"} {
		text := factsText(t, fetch(t, rr, path).Body.String())
		for _, bad := range []string{";", "—", "–", "{", "}"} {
			if strings.Contains(text, bad) {
				t.Errorf("%s: facts contain %q:\n%s", path, bad, text)
			}
		}
	}
}
