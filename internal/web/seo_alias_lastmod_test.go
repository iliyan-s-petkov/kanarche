package web_test

import (
	"encoding/xml"
	"net/http"
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
