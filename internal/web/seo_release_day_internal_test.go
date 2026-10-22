package web

import (
	"encoding/xml"
	"path/filepath"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
)

// Static pages carry the build's release day as lastmod. Internal because
// releaseDay is stamped by -ldflags and only reachable from this package.
func TestStaticPagesCarryReleaseDayLastMod(t *testing.T) {
	old := releaseDay
	releaseDay = "2026-10-22"
	t.Cleanup(func() { releaseDay = old })

	t.Setenv(config.DatabaseURLEnv, "postgres://user:pass@localhost:5432/airbg")
	cfg, err := config.LoadFile(filepath.Join("..", "..", "airbg.yaml"))
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	cfg.Listen.BaseURL = "https://airbg.org"
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	rr, err := NewRenderer(cat, snapshot.NewHolder(cfg.Series, config.Wind{}), cfg)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	snap := &snapshot.Snapshot{
		GeneratedAt: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
		KnownSlugs: map[string]snapshot.AreaMeta{
			"sofia": {Slug: "sofia", Kind: "oblast", NameBG: "София", NameEN: "Sofia", Covered: true, SensorCount: 12},
		},
	}
	body, err := rr.sitemapBytes(snap)
	if err != nil {
		t.Fatalf("sitemapBytes: %v", err)
	}
	var doc struct {
		URLs []struct {
			Loc     string `xml:"loc"`
			LastMod string `xml:"lastmod"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("sitemap did not parse: %v", err)
	}

	lastMod := map[string]string{}
	for _, u := range doc.URLs {
		lastMod[u.Loc] = u.LastMod
	}
	for _, path := range []string{"/about", "/about-the-data", "/privacy", "/licences", "/terms", "/pm25", "/pm10"} {
		for _, loc := range []string{"https://airbg.org" + path, "https://airbg.org/en" + path} {
			got, ok := lastMod[loc]
			if !ok {
				t.Errorf("sitemap has no entry for %s", loc)
				continue
			}
			if got != "2026-10-22" {
				t.Errorf("%s lastmod = %q, want the release day 2026-10-22", loc, got)
			}
		}
	}
	// Areas keep their own change day rule: unknown here, so none.
	if got := lastMod["https://airbg.org/area/sofia"]; got != "" {
		t.Errorf("area lastmod = %q, want none when its change day is unknown", got)
	}
}
