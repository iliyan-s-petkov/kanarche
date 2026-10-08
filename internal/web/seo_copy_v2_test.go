package web_test

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/snapshot"
)

// TestFreshnessCopyMatchesUpstreamPollInterval pins the "every 5 minutes"
// wording to the configured upstream.poll_interval: change one without the
// other and this fails.
func TestFreshnessCopyMatchesUpstreamPollInterval(t *testing.T) {
	t.Setenv(config.DatabaseURLEnv, "postgres://user:pass@localhost:5432/airbg")
	cfg, err := config.LoadFile(filepath.Join("..", "..", "airbg.yaml"))
	if err != nil {
		t.Fatalf("LoadFile error = %v, want nil", err)
	}
	interval := cfg.Upstream.PollInterval
	if interval%time.Minute != 0 {
		t.Fatalf("upstream.poll_interval = %v is not whole minutes; the SEO copy cannot state it", interval)
	}
	minutes := int(interval / time.Minute)

	rr := renderer(t, seoFixture(t))
	bg := pageDescription(t, fetch(t, rr, "/").Body.String())
	en := pageDescription(t, fetch(t, rr, "/en/").Body.String())

	if want := fmt.Sprintf("обновявана на всеки %d минути", minutes); !strings.Contains(bg, want) {
		t.Errorf("bg home description %q does not contain %q (upstream.poll_interval = %v)", bg, want, interval)
	}
	if want := fmt.Sprintf("updated every %d minutes", minutes); !strings.Contains(en, want) {
		t.Errorf("en home description %q does not contain %q (upstream.poll_interval = %v)", en, want, interval)
	}
}

// eeaFixture is seoFixture with official (eea) data on Plovdiv, Smolyan's
// oblast and Mladost, and a single-network citizen area elsewhere.
func eeaFixture(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	snap := seoFixture(t)
	both := map[string]snapshot.SourceEntry{"sensor.community": {N: 5}, "eea": {N: 1}}
	for _, slug := range []string{"plovdiv", "smolyan-oblast", "mladost"} {
		m := snap.KnownSlugs[slug]
		m.BySource = both
		snap.KnownSlugs[slug] = m
	}
	citizen := snap.KnownSlugs["smolyan"]
	citizen.Source = "sensor.community"
	snap.KnownSlugs["smolyan"] = citizen
	return snap
}

// Official stations are named only on city and oblast pages whose by_source
// contains eea.
func TestOfficialStationsMentionedOnlyWhenAreaHasEEA(t *testing.T) {
	rr := renderer(t, eeaFixture(t))
	official := regexp.MustCompile(`ИАОС|official|ExEA|EEA`)

	cases := []struct {
		path        string
		wantMention bool
	}{
		{"/area/plovdiv", true},
		{"/area/smolyan-oblast", true},
		{"/area/smolyan", false},
		{"/area/veliko-tarnovo", false},
		{"/area/krasna-polyana", false},
		{"/", false},
		{"/areas", false},
	}
	for _, c := range cases {
		for _, lang := range []string{"", "/en"} {
			desc := pageDescription(t, fetch(t, rr, lang+c.path).Body.String())
			if got := official.MatchString(desc); got != c.wantMention {
				t.Errorf("%s%s: official-station mention = %v, want %v in %q", lang, c.path, got, c.wantMention, desc)
			}
		}
	}

	bg := pageDescription(t, fetch(t, rr, "/area/plovdiv").Body.String())
	if want := "Граждански сензори и станции на ИАОС, графика за 24 часа."; !strings.Contains(bg, want) {
		t.Errorf("bg eea city description %q does not contain %q", bg, want)
	}
	en := pageDescription(t, fetch(t, rr, "/en/area/plovdiv").Body.String())
	want := "Plovdiv air pollution: PM2.5 and PM10 from citizen sensors and official stations on a live map, with the city median and 24-hour history."
	if en != want {
		t.Errorf("en eea city description = %q, want %q", en, want)
	}
}

// Copy uses ФПЧ (never ПМ) in Bulgarian and never says AQI or "индекс".
func TestSEOCopyWordingRules(t *testing.T) {
	snap := eeaFixture(t)
	rr := renderer(t, snap)
	paths := []string{"/", "/areas", "/about-the-data"}
	for slug := range snap.KnownSlugs {
		paths = append(paths, "/area/"+slug)
	}
	for _, lang := range []string{"", "/en"} {
		for _, path := range paths {
			body := fetch(t, rr, lang+path).Body.String()
			for name, text := range map[string]string{
				"title":       pageTitle(t, body),
				"description": pageDescription(t, body),
				"og:title":    tagAttr(t, body, `<meta property="og:title" content="`),
			} {
				if strings.Contains(strings.ToUpper(text), "AQI") || strings.Contains(strings.ToLower(text), "индекс") {
					t.Errorf("%s%s %s %q uses AQI/индекс wording", lang, path, name, text)
				}
				if lang == "" && strings.Contains(text, "ПМ") {
					t.Errorf("%s %s %q uses ПМ, want ФПЧ", path, name, text)
				}
				if strings.Contains(text, "Моят въздух") || strings.Contains(text, "My Air") {
					t.Errorf("%s%s %s %q still carries the old brand", lang, path, name, text)
				}
			}
		}
	}
}

// Every title ends with " | <brand>" and fits 60 runes; og:title is the
// same title without the suffix, so the two cannot drift apart.
func TestTitlesEndWithBrandAndOGTitleFollows(t *testing.T) {
	snap := seoFixture(t)
	rr := renderer(t, snap)
	paths := []string{"/", "/areas", "/about-the-data"}
	for slug := range snap.KnownSlugs {
		paths = append(paths, "/area/"+slug)
	}
	for _, lang := range []string{"", "/en"} {
		suffix := " | " + testBrand(lang)
		for _, path := range paths {
			body := fetch(t, rr, lang+path).Body.String()
			title := pageTitle(t, body)
			// The suffix is dropped only when it would push the title past 60.
			if !strings.HasSuffix(title, suffix) && utf8.RuneCountInString(title+suffix) <= 60 {
				t.Errorf("%s%s: title %q does not end with %q", lang, path, title, suffix)
			}
			if n := utf8.RuneCountInString(title); n > 60 {
				t.Errorf("%s%s: title %q is %d runes, want <=60", lang, path, title, n)
			}
			og := tagAttr(t, body, `<meta property="og:title" content="`)
			if want := strings.TrimSuffix(title, suffix); og != want {
				t.Errorf("%s%s: og:title = %q, want %q", lang, path, og, want)
			}
			if strings.Contains(body, `name="twitter:title"`) {
				if tw := tagAttr(t, body, `<meta name="twitter:title" content="`); tw != og {
					t.Errorf("%s%s: twitter:title = %q, want og:title %q", lang, path, tw, og)
				}
			}
		}
	}
}
