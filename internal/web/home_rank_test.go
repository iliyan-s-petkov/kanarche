package web_test

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

var hrefRe = regexp.MustCompile(`href="([^"]+)"`)

// rankFixture holds n provinces o1..on with PM2.5 falling from 10*n, so o1 is
// the most polluted and on the cleanest. Twelve cities hang off Sofia and a
// province with no reading is added when silent is true.
func rankFixture(n int, silent bool, generated time.Time) *snapshot.Snapshot {
	known := map[string]snapshot.AreaMeta{}
	for i := 1; i <= n; i++ {
		slug := fmt.Sprintf("o%d-oblast", i)
		known[slug] = snapshot.AreaMeta{Slug: slug, Kind: "oblast",
			NameBG: fmt.Sprintf("Област%d", i), NameEN: fmt.Sprintf("Prov%d", i),
			Covered: true, SensorCount: 5, Values: map[string]float64{"P2": float64(10 * (n - i + 1))}}
	}
	if silent {
		known["silent-oblast"] = snapshot.AreaMeta{Slug: "silent-oblast", Kind: "oblast",
			NameBG: "Тих", NameEN: "Silent", Covered: false, SensorCount: 1}
	}
	known["sofia"] = snapshot.AreaMeta{Slug: "sofia", Kind: "city", NameBG: "София", NameEN: "Sofia",
		Covered: true, SensorCount: 100, Values: map[string]float64{"P2": 20}}
	for i := 1; i <= 12; i++ {
		slug := fmt.Sprintf("city%02d", i)
		known[slug] = snapshot.AreaMeta{Slug: slug, Kind: "city",
			NameBG: fmt.Sprintf("Град%02d", i), NameEN: fmt.Sprintf("City%02d", i),
			Covered: true, SensorCount: 50 - i, Values: map[string]float64{"P2": 15}}
	}
	return &snapshot.Snapshot{GeneratedAt: generated, KnownSlugs: known}
}

// section returns the HTML of the <section> whose class is cls.
func section(t *testing.T, body, cls string) string {
	t.Helper()
	start := strings.Index(body, `class="`+cls+`"`)
	if start < 0 {
		t.Fatalf("no %s section", cls)
	}
	sec := body[start:]
	return sec[:strings.Index(sec, "</section>")]
}

func hrefsIn(s string) []string {
	var out []string
	for _, m := range hrefRe.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

// rankHrefs returns the hrefs in the section's two lists, split on the h3s.
func rankHrefs(t *testing.T, body string) (worst, best []string) {
	t.Helper()
	parts := strings.Split(section(t, body, "home-rank"), "<h3")
	if len(parts) > 1 {
		worst = hrefsIn(parts[1])
	}
	if len(parts) > 2 {
		best = hrefsIn(parts[2])
	}
	return worst, best
}

func TestHomeRankListsThreeAndThreeInOrder(t *testing.T) {
	rr := renderer(t, rankFixture(8, true, time.Now()))
	for _, c := range []struct{ path, pre, h2, worst, best string }{
		{"/", "", "Области с най-ниски и най-високи стойности сега", "С най-високи стойности", "С най-ниски стойности"},
		{"/en/", "/en", "Provinces with the lowest and highest readings now", "Highest readings", "Lowest readings"},
	} {
		body := fetch(t, rr, c.path).Body.String()
		for _, want := range []string{"<h2", c.h2, "<h3", c.worst, c.best} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: missing %q", c.path, want)
			}
		}
		if n := strings.Count(body, "<h1"); n != 1 {
			t.Errorf("%s: %d h1, want 1", c.path, n)
		}
		worst, best := rankHrefs(t, body)
		wantWorst := []string{c.pre + "/area/o1-oblast", c.pre + "/area/o2-oblast", c.pre + "/area/o3-oblast"}
		wantBest := []string{c.pre + "/area/o8-oblast", c.pre + "/area/o7-oblast", c.pre + "/area/o6-oblast"}
		if strings.Join(worst, ",") != strings.Join(wantWorst, ",") {
			t.Errorf("%s: worst = %v, want %v", c.path, worst, wantWorst)
		}
		if strings.Join(best, ",") != strings.Join(wantBest, ",") {
			t.Errorf("%s: best = %v, want %v", c.path, best, wantBest)
		}
		if strings.Contains(strings.Join(append(worst, best...), ","), "silent") {
			t.Errorf("%s: province without a value was ranked", c.path)
		}
	}
	en := fetch(t, rr, "/en/").Body.String()
	if !strings.Contains(en, "Prov1 Province") || !strings.Contains(en, "PM2.5 80.0") {
		t.Errorf("EN item text missing label or figure")
	}
	if strings.Contains(en, "The latest data is from") {
		t.Errorf("fresh data shows the stale sentence")
	}
}

func TestHomeRankStaleVariant(t *testing.T) {
	rr := renderer(t, rankFixture(8, false, time.Now().Add(-2*time.Hour)))
	body := fetch(t, rr, "/en/").Body.String()
	if !strings.Contains(body, "The latest data is from") {
		t.Errorf("stale data lacks the stale sentence")
	}
	if worst, _ := rankHrefs(t, body); len(worst) != 3 {
		t.Errorf("stale with values still ranks, got %v", worst)
	}
}

func TestHomeRankNoneVariants(t *testing.T) {
	snap := &snapshot.Snapshot{GeneratedAt: time.Now(), KnownSlugs: map[string]snapshot.AreaMeta{
		"a-oblast": {Slug: "a-oblast", Kind: "oblast", NameBG: "А", NameEN: "A"},
	}}
	body := fetch(t, renderer(t, snap), "/en/").Body.String()
	if !strings.Contains(body, "No province has enough recent sensor readings to rank right now.") {
		t.Errorf("none variant missing")
	}
	if strings.Contains(body, "The latest data is from") || strings.Contains(section(t, body, "home-rank"), "<ol") {
		t.Errorf("fresh none variant shows stale text or a list")
	}

	snap.GeneratedAt = time.Now().Add(-2 * time.Hour)
	body = fetch(t, renderer(t, snap), "/en/").Body.String()
	none := strings.Index(body, "No province has enough")
	stale := strings.Index(body, "The latest data is from")
	if none < 0 || stale < 0 || stale < none {
		t.Errorf("stale none variant: none at %d, stale at %d, want none then stale", none, stale)
	}
}

func TestHomeRankNeverRepeatsAProvince(t *testing.T) {
	for _, c := range []struct{ n, worst, best int }{{2, 2, 0}, {3, 3, 0}, {4, 3, 1}, {5, 3, 2}, {6, 3, 3}} {
		rr := renderer(t, rankFixture(c.n, false, time.Now()))
		worst, best := rankHrefs(t, fetch(t, rr, "/en/").Body.String())
		if len(worst) != c.worst || len(best) != c.best {
			t.Errorf("n=%d: got %d+%d items, want %d+%d", c.n, len(worst), len(best), c.worst, c.best)
		}
		seen := map[string]bool{}
		for _, h := range append(worst, best...) {
			if seen[h] {
				t.Errorf("n=%d: %s listed twice", c.n, h)
			}
			seen[h] = true
		}
	}
}

func TestHomeCitiesLinkToLiveRoutes(t *testing.T) {
	rr := renderer(t, rankFixture(8, false, time.Now()))
	for _, pre := range []string{"", "/en"} {
		sec := section(t, fetch(t, rr, pre+"/").Body.String(), "home-cities")
		if !strings.Contains(sec, "<h2") {
			t.Errorf("%q: cities block has no h2", pre)
		}
		hrefs := hrefsIn(sec)
		// ten cities by sensor count (Sofia first), the Sofia hub, then all.
		if len(hrefs) != 12 {
			t.Fatalf("%q: %d links, want 12: %v", pre, len(hrefs), hrefs)
		}
		if hrefs[0] != pre+"/area/sofia" || hrefs[1] != pre+"/area/city01" || hrefs[9] != pre+"/area/city09" {
			t.Errorf("%q: city order wrong: %v", pre, hrefs)
		}
		if hrefs[10] != pre+"/area/sofia" || hrefs[11] != pre+"/areas" {
			t.Errorf("%q: tail links wrong: %v", pre, hrefs[10:])
		}
		for _, h := range hrefs {
			if code := fetch(t, rr, h).Code; code != http.StatusOK {
				t.Errorf("%s -> %d", h, code)
			}
		}
	}
}
