package web_test

import (
	"strings"
	"testing"

	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/web"
)

// rendererAt is renderer() with a different public base URL.
func rendererAt(t *testing.T, baseURL string) *web.Renderer {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	cfg := testConfig(t)
	cfg.Listen.BaseURL = baseURL
	rr, err := web.NewRenderer(cat, snapshot.NewHolder(cfg.Series, cfg.Wind), cfg)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	return rr
}

// The copy-paste iframe must point at the host this deployment is configured
// to serve from, not at a hard-coded one.
func TestAboutEmbedSnippetFollowsBaseURL(t *testing.T) {
	for _, p := range aboutPaths() {
		body := fetch(t, rendererAt(t, "https://example.test"), p).Body.String()
		if !strings.Contains(body, `&lt;iframe src="https://example.test/embed?area=plovdiv-oblast"`) {
			t.Errorf("%s: embed snippet does not use the configured base URL", p)
		}
		if !strings.Contains(body, `title="`+testBrand(map[bool]string{true: "/en", false: ""}[strings.HasPrefix(p, "/en")])+`"`) {
			t.Errorf("%s: embed snippet title is not the product name", p)
		}
		if strings.Contains(body, "https://airbg.org/embed") {
			t.Errorf("%s: embed snippet still carries the literal airbg.org host", p)
		}
	}
}

// With the production base URL the snippet is what it always was.
func TestAboutEmbedSnippetIsUnchangedOnTheCurrentHost(t *testing.T) {
	body := fetch(t, rendererAt(t, testBaseURL), "/en/about").Body.String()
	want := `&lt;iframe src="https://airbg.org/embed?area=plovdiv-oblast"
        title="Kanarche" width="100%" height="480"`
	if !strings.Contains(body, want) {
		t.Errorf("embed snippet changed on the current host; want %q", want)
	}
}
