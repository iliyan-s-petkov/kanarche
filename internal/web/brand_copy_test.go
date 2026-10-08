package web_test

import (
	"regexp"
	"strings"
	"testing"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/web"
)

var (
	brandScriptRe = regexp.MustCompile(`(?s)<script\b([^>]*)>(.*?)</script>`)
	brandStyleRe  = regexp.MustCompile(`(?s)<style\b.*?</style>`)
	// URL-valued attributes carry hosts and repo paths, not copy.
	brandURLAttrRe = regexp.MustCompile(`\s(?:href|src|action|srcset|data-[a-z-]*(?:url|href))="[^"]*"`)
	brandRepoURLRe = regexp.MustCompile(`https://github\.com/iliyan-s-petkov/kanarche\S*?"`)
	// The clear-settings button lists the pre-rename storage keys it must wipe.
	brandKeysAttrRe = regexp.MustCompile(`\sdata-keys="[^"]*"`)
	// The one reference that stays: the other project's own name.
	brandAllowedRe = regexp.MustCompile(`(?i)airbg\.info`)
)

// brandCopy keeps what a reader or crawler can see: markup text, attribute
// copy and JSON-LD. Other scripts, styles and URL-valued attributes are dropped.
func brandCopy(body string) string {
	body = brandScriptRe.ReplaceAllStringFunc(body, func(s string) string {
		if strings.Contains(s, "application/ld+json") {
			return brandRepoURLRe.ReplaceAllString(s, `"`)
		}
		return ""
	})
	body = brandStyleRe.ReplaceAllString(body, "")
	body = brandURLAttrRe.ReplaceAllString(body, "")
	body = brandKeysAttrRe.ReplaceAllString(body, "")
	return brandAllowedRe.ReplaceAllString(body, "")
}

// Every page and the embed, in both languages, name the product Kanarche /
// Канарче and never airbg. The base URL is neutral so host strings cannot
// hide a copy leak.
func TestPagesCarryNoAirbgBrandCopy(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	cfg := testConfig(t)
	cfg.Listen.BaseURL = "https://example.test"
	cfg.Social = config.Social{FacebookURL: "https://www.facebook.com/x", LinkedInURL: "https://www.linkedin.com/company/x"}
	h := snapshot.NewHolder(cfg.Series, config.Wind{})
	h.Store(cityFixture(t))
	rr, err := web.NewRenderer(cat, h, cfg)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	paths := []string{"/", "/areas", "/area/sofiya", "/area/plovdiv-oblast", "/about", "/about-the-data", "/privacy", "/licences", "/embed", "/no-such-page"}
	for _, lang := range []string{"", "/en"} {
		for _, p := range paths {
			path := lang + p
			rec := fetch(t, rr, path)
			if rec.Code != 200 && p != "/no-such-page" {
				t.Errorf("GET %s = %d, want 200", path, rec.Code)
				continue
			}
			copy := brandCopy(rec.Body.String())
			if i := strings.Index(strings.ToLower(copy), "airbg"); i >= 0 {
				t.Errorf("GET %s carries the old brand: ...%s...", path, copy[max(0, i-60):min(len(copy), i+60)])
			}
		}
	}
}

// testBrand is the product name in the language a path prefix selects.
func testBrand(langPrefix string) string {
	if langPrefix == "/en" {
		return "Kanarche"
	}
	return "Канарче"
}
