package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/web"
)

var aiCrawlerAgents = []string{
	"GPTBot", "OAI-SearchBot", "ChatGPT-User",
	"ClaudeBot", "Claude-SearchBot", "Claude-User",
	"PerplexityBot", "Perplexity-User",
	"Google-Extended", "CCBot", "Applebot-Extended", "Bingbot",
}

// robotsGroups maps each User-agent to the rule lines under it. A group is
// the User-agent lines before its first rule, up to the next blank line.
func robotsGroups(body string) map[string][]string {
	groups := map[string][]string{}
	var agents []string
	sawRule := false
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			if line == "" {
				agents, sawRule = nil, false
			}
		case strings.HasPrefix(line, "User-agent:"):
			if sawRule {
				agents, sawRule = nil, false
			}
			ua := strings.TrimSpace(strings.TrimPrefix(line, "User-agent:"))
			agents = append(agents, ua)
			if _, ok := groups[ua]; !ok {
				groups[ua] = nil
			}
		default:
			sawRule = true
			for _, a := range agents {
				groups[a] = append(groups[a], line)
			}
		}
	}
	return groups
}

func TestRobotsGivesEachAICrawlerItsOwnGroupWithAPIDisallow(t *testing.T) {
	rec := fetch(t, renderer(t, fixture(t)), "/robots.txt")
	groups := robotsGroups(rec.Body.String())

	for _, bot := range append([]string{"*"}, aiCrawlerAgents...) {
		rules, ok := groups[bot]
		if !ok {
			t.Errorf("robots.txt has no User-agent group for %s", bot)
			continue
		}
		// A named group replaces `*`, so each one must repeat the /api/ rule.
		if !slices.Contains(rules, "Disallow: /api/") {
			t.Errorf("%s group lacks Disallow: /api/: %q", bot, rules)
		}
	}
}

// Parsers that take the first match need each group's dataset Allow lines
// before its Disallow line.
func TestRobotsDatasetAllowsPrecedeDisallowInEveryGroup(t *testing.T) {
	groups := robotsGroups(fetch(t, renderer(t, fixture(t)), "/robots.txt").Body.String())
	for agent, rules := range groups {
		disallow := slices.Index(rules, "Disallow: /api/")
		if disallow < 0 {
			t.Errorf("%s group has no Disallow: /api/: %q", agent, rules)
			continue
		}
		for _, p := range []string{"/api/v1/areas$", "/api/v1/overview$"} {
			i := slices.Index(rules, "Allow: "+p)
			if i < 0 {
				t.Errorf("%s group lacks Allow: %s: %q", agent, p, rules)
			} else if i > disallow {
				t.Errorf("%s group has Allow: %s after its Disallow: %q", agent, p, rules)
			}
		}
	}
}

// Both text indexes change only on deploy, so they share one hour-long edge cache.
func TestRobotsAndLLMSTxtAreEdgeCacheable(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, path := range []string{"/robots.txt", "/llms.txt"} {
		if got := fetch(t, rr, path).Header().Get("Cache-Control"); got != "public, max-age=3600" {
			t.Errorf("GET %s Cache-Control = %q, want public, max-age=3600", path, got)
		}
	}
}

var contentURLRe = regexp.MustCompile(`"contentUrl":"([^"]+)"`)

// The Dataset distribution URLs sit under /api/, so every group must re-allow
// them. The paths come from the rendered JSON-LD, not a copy of it.
func TestRobotsAllowsEveryDatasetDistributionURL(t *testing.T) {
	rr := renderer(t, fixture(t))
	page := fetch(t, rr, "/about-the-data").Body.String()
	matches := contentURLRe.FindAllStringSubmatch(page, -1)
	if len(matches) == 0 {
		t.Fatal("no contentUrl in the Dataset JSON-LD on /about-the-data")
	}

	groups := robotsGroups(fetch(t, rr, "/robots.txt").Body.String())
	for _, m := range matches {
		u, err := url.Parse(m[1])
		if err != nil {
			t.Fatalf("contentUrl %q does not parse: %v", m[1], err)
		}
		for agent, rules := range groups {
			if !slices.Contains(rules, "Allow: "+u.Path+"$") {
				t.Errorf("%s group does not allow the distribution path %s: %q", agent, u.Path, rules)
			}
		}
	}
}

func TestLLMSTxtIsServedAsPlainText(t *testing.T) {
	rec := fetch(t, renderer(t, fixture(t)), "/llms.txt")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /llms.txt = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/plain; charset=utf-8", ct)
	}
}

// llmsPages are the paths the llms.txt file must link, in its own format.
var llmsPages = []string{"/", "/en/", "/areas", "/about", "/about-the-data", "/licences", "/privacy", "/sitemap.xml"}

func TestLLMSTxtHasH1AndEveryPageURL(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/llms.txt").Body.String()
	if !strings.HasPrefix(body, "# Kanarche\n") {
		t.Errorf("llms.txt does not open with the H1 %q", "# Kanarche")
	}
	if !strings.Contains(body, "\n> ") {
		t.Error("llms.txt has no blockquote summary")
	}
	for _, p := range llmsPages {
		if want := "(" + testBaseURL + p + ")"; !strings.Contains(body, want) {
			t.Errorf("llms.txt does not link %s", want)
		}
	}
}

// The intro, between the summary and the first H2, needs its Bulgarian paragraph.
// The Data section also carries Bulgarian, so checking the whole file would not prove this.
func TestLLMSTxtHasBulgarianParagraph(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/llms.txt").Body.String()
	intro, _, _ := strings.Cut(body, "\n## ")
	if strings.IndexFunc(intro, func(r rune) bool { return unicode.Is(unicode.Cyrillic, r) }) < 0 {
		t.Errorf("llms.txt intro has no Bulgarian paragraph before the first H2:\n%s", intro)
	}
}

// Site copy rule: no semicolons and no en or em dashes.
func TestLLMSTxtCopyHasNoSemicolonOrDash(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/llms.txt").Body.String()
	for _, bad := range []string{";", "–", "—"} {
		if strings.Contains(body, bad) {
			t.Errorf("llms.txt contains %q, which the copy rules forbid", bad)
		}
	}
}

// Catalogue bodies go into llms.txt as plain text. Markup from them would
// leak into the file, and a stray `#` line would read as a heading. The one
// `>` allowed is the blockquote marker that opens the summary line.
func TestLLMSTxtBodiesAreMarkupAndHeadingFree(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/llms.txt").Body.String()

	// Strip only the blockquote marker, then no angle bracket may remain.
	plain := strings.Replace(body, "\n> ", "\n", 1)
	for _, bad := range []string{"<", ">"} {
		if strings.Contains(plain, bad) {
			t.Errorf("llms.txt contains %q outside the blockquote marker", bad)
		}
	}

	for i, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		if line != "# Kanarche" && !strings.HasPrefix(line, "## ") {
			t.Errorf("llms.txt line %d is a stray heading: %q", i+1, line)
		}
	}
}

func TestTwitterTagsMirrorOpenGraph(t *testing.T) {
	rr := renderer(t, seoFixture(t))
	for _, path := range []string{"/", "/en/", "/area/plovdiv"} {
		body := fetch(t, rr, path).Body.String()
		for _, pair := range [][2]string{
			{"twitter:title", "og:title"},
			{"twitter:description", "og:description"},
			{"twitter:image", "og:image"},
		} {
			og := tagAttr(t, body, `<meta property="`+pair[1]+`" content="`)
			tw := tagAttr(t, body, `<meta name="`+pair[0]+`" content="`)
			if tw != og {
				t.Errorf("%s: %s = %q, want the %s value %q", path, pair[0], tw, pair[1], og)
			}
		}
	}
}

// rendererWithTiles is renderer with a basemap configured, so the tiles
// origin is known to the template.
func rendererWithTiles(t *testing.T, snap *snapshot.Snapshot) *web.Renderer {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	cfg := testConfig(t)
	cfg.Tiles = config.Tiles{
		Addr:      "127.0.0.1:0",
		Dir:       t.TempDir(),
		PublicURL: "https://tiles.kanarche.eu/",
		Archive:   "basemap.pmtiles",
	}
	h := snapshot.NewHolder(cfg.Series, config.Wind{})
	if snap != nil {
		h.Store(snap)
	}
	rr, err := web.NewRenderer(cat, h, cfg)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	return rr
}

func TestTilesPreconnectOnlyWhenBasemapConfigured(t *testing.T) {
	const preconnect = `<link rel="preconnect" href="https://tiles.kanarche.eu" crossorigin>`

	without := fetch(t, renderer(t, fixture(t)), "/").Body.String()
	if strings.Contains(without, `rel="preconnect"`) {
		t.Error("preconnect rendered with no basemap configured")
	}

	with := fetch(t, rendererWithTiles(t, fixture(t)), "/").Body.String()
	if !strings.Contains(with, preconnect) {
		t.Errorf("basemap configured but %s is missing", preconnect)
	}
}
