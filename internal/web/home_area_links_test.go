package web_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

// areaLinksFixture holds 24 Sofia districts, four cities and one oblast.
func areaLinksFixture() *snapshot.Snapshot {
	known := map[string]snapshot.AreaMeta{}
	add := func(slug, kind, parent, bg, en string) {
		known[slug] = snapshot.AreaMeta{Slug: slug, Kind: kind, ParentSlug: parent, NameBG: bg, NameEN: en,
			Covered: true, SensorCount: 5, Values: map[string]float64{"P2": 10}}
	}
	for i := 1; i <= 23; i++ {
		add(fmt.Sprintf("d%02d", 24-i), "neighbourhood", "sofia", fmt.Sprintf("Район%02d", 24-i), fmt.Sprintf("District%02d", 24-i))
	}
	add("mladost", "neighbourhood", "sofia", "Младост", "Mladost")
	add("sofia", "city", "sofia-oblast", "София", "Sofia")
	add("vratsa", "city", "vratsa-oblast", "Враца", "Vratsa")
	add("varna", "city", "varna-oblast", "Варна", "Varna")
	add("plovdiv", "city", "plovdiv-oblast", "Пловдив", "Plovdiv")
	add("vratsa-oblast", "oblast", "", "Област Враца", "Vratsa Province")
	return &snapshot.Snapshot{GeneratedAt: time.Now(), KnownSlugs: known}
}

// listSlugs is the area slugs linked from one list of the home area-links block.
func listSlugs(t *testing.T, sec, list, pre string) []string {
	t.Helper()
	i := strings.Index(sec, `home-area-links__`+list)
	if i < 0 {
		t.Fatalf("no %s list", list)
	}
	l := sec[i:]
	l = l[:strings.Index(l, "</ul>")]
	var out []string
	for _, h := range hrefsIn(l) {
		out = append(out, strings.TrimPrefix(h, pre+"/area/"))
	}
	return out
}

// The home page links every Sofia district and every provincial capital, by name, in both languages.
func TestHomeLinksAllDistrictsAndCapitals(t *testing.T) {
	rr := renderer(t, areaLinksFixture())
	for _, c := range []struct {
		path, pre, heading  string
		firstCity, lastCity string
	}{
		{"/", "", "Градове и райони по области", "varna", "sofia"},
		{"/en/", "/en", "Cities and districts by province", "plovdiv", "vratsa"},
	} {
		sec := section(t, fetch(t, rr, c.path).Body.String(), "home-area-links")
		if !strings.Contains(sec, "<h2 id=\"home-area-links-h\">"+c.heading+"</h2>") {
			t.Errorf("%s: want heading %q", c.path, c.heading)
		}
		districts := listSlugs(t, sec, "districts", c.pre)
		if len(districts) != 24 {
			t.Errorf("%s: %d district links, want 24", c.path, len(districts))
		}
		if !strings.Contains(strings.Join(districts, " "), "mladost") {
			t.Errorf("%s: no mladost link", c.path)
		}
		cities := listSlugs(t, sec, "cities", c.pre)
		if len(cities) != 4 || cities[0] != c.firstCity || cities[3] != c.lastCity {
			t.Errorf("%s: cities %v, want 4 from %s to %s", c.path, cities, c.firstCity, c.lastCity)
		}
		if !strings.Contains(strings.Join(cities, " "), "vratsa") || strings.Contains(sec, "vratsa-oblast") {
			t.Errorf("%s: want the vratsa city link and no oblast link", c.path)
		}
		if c.pre != "" && strings.Contains(strings.ReplaceAll(sec, c.pre+"/area/", ""), `href="/area/`) {
			t.Errorf("%s: link without the language prefix", c.path)
		}
	}
}

// Districts are ordered by display name in the page language.
func TestHomeAreaLinksDistrictOrder(t *testing.T) {
	rr := renderer(t, areaLinksFixture())
	sec := section(t, fetch(t, rr, "/en/").Body.String(), "home-area-links")
	d := listSlugs(t, sec, "districts", "/en")
	if d[0] != "d01" || d[22] != "d23" || d[23] != "mladost" {
		t.Errorf("district order %v", d)
	}
}

// The directory page already lists everything, so it does not repeat the block.
func TestAreasPageHasNoHomeAreaLinks(t *testing.T) {
	rr := renderer(t, areaLinksFixture())
	if strings.Contains(fetch(t, rr, "/areas").Body.String(), "home-area-links") {
		t.Error("/areas renders the home block")
	}
}
