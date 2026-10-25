package web_test

import (
	"encoding/xml"
	"net/http"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

// The owner decision: sofiya was a typo, sofia is canonical. The old indexed
// slugs 301 to the new ones, both languages, query kept.
func oldSlugFixture(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	snap := fixture(t)
	snap.KnownSlugs["sofia-grad-oblast"] = snapshot.AreaMeta{Slug: "sofia-grad-oblast", Kind: "oblast",
		NameBG: "София-град", NameEN: "Sofia City", Covered: true, SensorCount: 3}
	return snap
}

func TestOldSofiyaSlugsRedirectToSofia(t *testing.T) {
	rr := renderer(t, oldSlugFixture(t))
	for from, to := range map[string]string{
		"/area/sofiya":                      "/area/sofia",
		"/en/area/sofiya":                   "/en/area/sofia",
		"/area/sofiya?metric=a":             "/area/sofia?metric=a",
		"/area/sofiya-grad-oblast":          "/area/sofia-grad-oblast",
		"/en/area/sofiya-grad-oblast?x=1&y": "/en/area/sofia-grad-oblast?x=1&y",
	} {
		rec := fetch(t, rr, from)
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("GET %s = %d, want 301", from, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != to {
			t.Errorf("GET %s Location = %q, want %q", from, got, to)
		}
	}
}

func TestNewSofiaSlugServesDirectly(t *testing.T) {
	rr := renderer(t, oldSlugFixture(t))
	for _, p := range []string{"/area/sofia", "/en/area/sofia-grad-oblast"} {
		if rec := fetch(t, rr, p); rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, rec.Code)
		}
	}
}

func TestOtherUnknownSlugsStill404(t *testing.T) {
	rr := renderer(t, oldSlugFixture(t))
	for _, p := range []string{"/area/sofiaa", "/area/sofia-city", "/area/plovdiv", "/area/sofiya-x"} {
		if rec := fetch(t, rr, p); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, rec.Code)
		}
	}
}

func lastMods(t *testing.T, snap *snapshot.Snapshot) map[string]string {
	t.Helper()
	rec := fetch(t, renderer(t, snap), "/sitemap.xml")
	var doc sitemapXML
	if err := xml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, u := range doc.URLs {
		out[u.Loc] = u.LastMod
	}
	return out
}

// Each area's lastmod is the day its own data last changed, not the snapshot day.
func TestSitemapAreaLastModIsPerArea(t *testing.T) {
	snap := fixture(t)
	snap.KnownSlugs["varna"] = snapshot.AreaMeta{Slug: "varna", Kind: "oblast", NameBG: "Варна", NameEN: "Varna",
		Covered: true, SensorCount: 5, ChangedAt: time.Date(2026, 8, 7, 23, 0, 0, 0, time.UTC)}
	m := snap.KnownSlugs["sofia"]
	m.ChangedAt = time.Date(2026, 8, 9, 3, 0, 0, 0, time.UTC)
	snap.KnownSlugs["sofia"] = m

	got := lastMods(t, snap)
	for loc, want := range map[string]string{
		"https://airbg.org/area/sofia":    "2026-08-09",
		"https://airbg.org/area/varna":    "2026-08-07",
		"https://airbg.org/en/area/varna": "2026-08-07",
		"https://airbg.org/":              "2026-08-09",
	} {
		if got[loc] != want {
			t.Errorf("%s: lastmod = %q, want %q", loc, got[loc], want)
		}
	}
}

// An area with no recorded change gets no lastmod rather than a guessed one.
func TestSitemapAreaWithoutChangeTimeHasNoLastMod(t *testing.T) {
	got := lastMods(t, fixture(t))
	if v, ok := got["https://airbg.org/area/sofia"]; !ok || v != "" {
		t.Errorf("lastmod = %q (present %v), want present and empty", v, ok)
	}
}

var staticPagePaths = []string{"/about", "/about-the-data", "/privacy", "/licences", "/terms", "/pm25", "/pm10", "/air-pollution"}

// Static pages carry a fixed release date: a valid day, the same in both languages, and not
// the snapshot day, so a new snapshot does not move it.
func TestSitemapStaticPagesHaveStableLastMod(t *testing.T) {
	first := fixture(t)
	second := fixture(t)
	second.GeneratedAt = second.GeneratedAt.AddDate(0, 3, 0)

	a, b := lastMods(t, first), lastMods(t, second)
	for _, p := range staticPagePaths {
		for _, prefix := range []string{"", "/en"} {
			loc := "https://airbg.org" + prefix + p
			if _, err := time.Parse("2006-01-02", a[loc]); err != nil {
				t.Errorf("%s: lastmod = %q, want a YYYY-MM-DD day", loc, a[loc])
			}
			if a[loc] != b[loc] {
				t.Errorf("%s: lastmod moved with the snapshot, %q then %q", loc, a[loc], b[loc])
			}
		}
	}
}

// An area whose data did not change keeps its lastmod when a later snapshot is generated.
func TestSitemapAreaLastModIgnoresSnapshotTime(t *testing.T) {
	changed := time.Date(2026, 8, 7, 23, 0, 0, 0, time.UTC)
	build := func(generated time.Time) map[string]string {
		snap := fixture(t)
		snap.GeneratedAt = generated
		m := snap.KnownSlugs["sofia"]
		m.ChangedAt = changed
		snap.KnownSlugs["sofia"] = m
		return lastMods(t, snap)
	}
	a := build(time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC))
	b := build(time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC))
	for _, loc := range []string{"https://airbg.org/area/sofia", "https://airbg.org/en/area/sofia"} {
		if a[loc] != "2026-08-07" || b[loc] != "2026-08-07" {
			t.Errorf("%s: lastmod = %q then %q, want 2026-08-07 both times", loc, a[loc], b[loc])
		}
	}
}

// A third-party iframe still carrying the retired slug lands on the same centroid.
func TestEmbedResolvesRetiredAreaSlug(t *testing.T) {
	rr := renderer(t, fixture(t))
	body := framed(t, rr, "/embed?area=sofiya").Body.String()
	if !strings.Contains(body, `data-lat="42.69"`) || !strings.Contains(body, `data-lon="23.32"`) {
		t.Error("area=sofiya did not move the map to Sofia's centroid")
	}
}
