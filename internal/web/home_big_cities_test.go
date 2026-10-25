package web_test

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/web"
)

var liRe = regexp.MustCompile(`(?s)<li>.*?</li>`)

// bigCitiesFixture holds every production city and six Sofia districts with
// 10..60 sensors. Areas in noValue have no reading.
func bigCitiesFixture(noValue map[string]bool, generated time.Time) *snapshot.Snapshot {
	known := map[string]snapshot.AreaMeta{}
	add := func(slug, kind, parent, bg, en string, sensors int, pm float64) {
		m := snapshot.AreaMeta{Slug: slug, Kind: kind, ParentSlug: parent, NameBG: bg, NameEN: en,
			Covered: true, SensorCount: sensors}
		if !noValue[slug] {
			m.Values = map[string]float64{"P2": pm}
		}
		known[slug] = m
	}
	for i, s := range web.BigCitySlugs {
		add(s, "city", "", "Град-"+s, "City-"+s, 100+i, 12.5+float64(i))
	}
	for i := 1; i <= 6; i++ {
		add(fmt.Sprintf("d%d", i), "neighbourhood", "sofia", fmt.Sprintf("Район%d", i), fmt.Sprintf("District%d", i), i*10, 20)
	}
	return &snapshot.Snapshot{GeneratedAt: generated, KnownSlugs: known}
}

// bigRows returns the block's rows (the main list only) for a page.
func bigRows(t *testing.T, rr *web.Renderer, path string) []string {
	t.Helper()
	sec := section(t, fetch(t, rr, path).Body.String(), "home-big-cities")
	list := sec[strings.Index(sec, `home-big-cities__list`):]
	list = list[:strings.Index(list, "</ul>")]
	return liRe.FindAllString(list, -1)
}

// rowSlugs is the area slug each row links to.
func rowSlugs(rows []string, pre string) []string {
	var out []string
	for _, r := range rows {
		out = append(out, strings.TrimPrefix(hrefsIn(r)[0], pre+"/area/"))
	}
	return out
}

func TestHomeBigCitiesRows(t *testing.T) {
	rr := renderer(t, bigCitiesFixture(nil, time.Now()))
	for _, c := range []struct{ path, pre, heading, name, value string }{
		{"/", "", "Големите градове сега", "Град-", "13,5"},
		{"/en/", "/en", "Big cities now", "City-", "13.5"},
	} {
		sec := section(t, fetch(t, rr, c.path).Body.String(), "home-big-cities")
		if !strings.Contains(sec, ">"+c.heading+"</h2>") {
			t.Errorf("%s: heading %q missing", c.path, c.heading)
		}
		rows := bigRows(t, rr, c.path)
		got := rowSlugs(rows, c.pre)
		want := append(append([]string{}, web.BigCitySlugs...), "d6", "d5", "d4", "d3")
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s: rows %v, want %v", c.path, got, want)
		}
		if got[0] != "sofia" {
			t.Errorf("%s: sofia is not first", c.path)
		}
		plovdiv := rows[1]
		for _, frag := range []string{c.name + "plovdiv</a>", c.value, `class="chip"`, `class="chip__swatch"`, "µg/m³"} {
			if !strings.Contains(plovdiv, frag) {
				t.Errorf("%s: plovdiv row lacks %q: %s", c.path, frag, plovdiv)
			}
		}
		hrefs := hrefsIn(sec)
		for _, h := range hrefs {
			if code := fetch(t, rr, h).Code; code != http.StatusOK {
				t.Errorf("%s -> %d", h, code)
			}
		}
		// Tail links: Sofia districts and all areas.
		if n := len(hrefs); hrefs[n-2] != c.pre+"/area/sofia" || hrefs[n-1] != c.pre+"/areas" {
			t.Errorf("%s: tail links %v", c.path, hrefs[n-2:])
		}
	}
}

func TestHomeBigCitiesNoData(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	rr := renderer(t, bigCitiesFixture(map[string]bool{"varna": true}, time.Now()))
	for _, c := range []struct{ path, lang string }{{"/", "bg"}, {"/en/", "en"}} {
		varna := bigRows(t, rr, c.path)[2]
		if !strings.Contains(varna, "/area/varna") {
			t.Fatalf("%s: row 2 is not varna: %s", c.path, varna)
		}
		if strings.Contains(varna, "chip") || strings.Contains(varna, "µg") {
			t.Errorf("%s: varna row has a value or chip: %s", c.path, varna)
		}
		if !strings.Contains(varna, ": "+cat.T(c.lang, "map.legend.no_data")+"</li>") {
			t.Errorf("%s: varna row lacks the no-data text: %s", c.path, varna)
		}
	}
}

func TestHomeBigCitiesDistrictOrderAndTies(t *testing.T) {
	snap := bigCitiesFixture(nil, time.Now())
	// Two districts tie with d4 at 40 sensors: slug order breaks the tie.
	for _, s := range []string{"a-tie", "z-tie"} {
		snap.KnownSlugs[s] = snapshot.AreaMeta{Slug: s, Kind: "neighbourhood", ParentSlug: "sofia",
			NameBG: s, NameEN: s, Covered: true, SensorCount: 40, Values: map[string]float64{"P2": 20}}
	}
	got := rowSlugs(bigRows(t, renderer(t, snap), "/en/"), "/en")
	if tail := strings.Join(got[len(web.BigCitySlugs):], ","); tail != "d6,d5,a-tie,d4" {
		t.Errorf("districts %s, want d6,d5,a-tie,d4", tail)
	}
}

func TestHomeBigCitiesSkipsMissingAndWrongKind(t *testing.T) {
	snap := bigCitiesFixture(nil, time.Now())
	delete(snap.KnownSlugs, "burgas")
	ruse := snap.KnownSlugs["ruse"]
	ruse.Kind = "oblast"
	snap.KnownSlugs["ruse"] = ruse
	// A district of another city must not appear.
	snap.KnownSlugs["other"] = snapshot.AreaMeta{Slug: "other", Kind: "neighbourhood", ParentSlug: "varna",
		NameBG: "o", NameEN: "o", Covered: true, SensorCount: 999, Values: map[string]float64{"P2": 20}}
	got := rowSlugs(bigRows(t, renderer(t, snap), "/en/"), "/en")
	for _, s := range got {
		if s == "burgas" || s == "ruse" || s == "other" {
			t.Errorf("%s must not be listed: %v", s, got)
		}
	}
	if len(got) != len(web.BigCitySlugs)-2+4 {
		t.Errorf("rows %v", got)
	}
}

func TestHomeBigCitiesEmptyMeansNoBlock(t *testing.T) {
	snap := &snapshot.Snapshot{GeneratedAt: time.Now(), KnownSlugs: map[string]snapshot.AreaMeta{
		"x-oblast": {Slug: "x-oblast", Kind: "oblast", NameBG: "Х", NameEN: "X", Covered: true, SensorCount: 1}}}
	if strings.Contains(fetch(t, renderer(t, snap), "/").Body.String(), "home-big-cities") {
		t.Error("block rendered with no rows")
	}
}

func TestHomeBigCitiesNotOnAreas(t *testing.T) {
	rr := renderer(t, bigCitiesFixture(nil, time.Now()))
	if strings.Contains(fetch(t, rr, "/areas").Body.String(), "home-big-cities") {
		t.Error("/areas carries the big-cities block")
	}
	if !strings.Contains(fetch(t, rr, "/").Body.String(), "home-big-cities") {
		t.Error("/ lacks the block")
	}
}

func TestHomeBigCitiesStaleNote(t *testing.T) {
	for _, c := range []struct {
		name  string
		age   time.Duration
		stale bool
	}{{"fresh", time.Minute, false}, {"stale", 3 * time.Hour, true}} {
		rr := renderer(t, bigCitiesFixture(nil, time.Now().Add(-c.age)))
		for _, p := range []struct{ path, note string }{
			{"/", "Последните данни са от"}, {"/en/", "The latest data is from"},
		} {
			sec := section(t, fetch(t, rr, p.path).Body.String(), "home-big-cities")
			if got := strings.Contains(sec, p.note); got != c.stale {
				t.Errorf("%s %s: stale note present=%v, want %v", c.name, p.path, got, c.stale)
			}
		}
	}
}
