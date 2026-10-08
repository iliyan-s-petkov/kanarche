package server_test

import (
	"net/http"
	"strings"
	"testing"

	"kanarche.eu/internal/api"
	"kanarche.eu/internal/config"
	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/server"
	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/web"
)

// TestNoRouteSetsCookies is OpenProject #584's origin-side privacy guard: the
// site stores no state in a cookie, on any route, on either listener.
// Cloudflare sits in front in production and is out of our control; this
// covers only what this process itself sends.
//
// The route list comes from api.RoutePatterns, (*web.Renderer).RoutePatterns
// and server.PrivateRoutePatterns — the same maps NewRouter/Routes/privateMux
// register from — so a route added there is a route this test hits without
// anyone updating a second, hand-written list here.
func TestNoRouteSetsCookies(t *testing.T) {
	public, private := running(t)

	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	cfg := testConfig(t)
	holder := snapshot.NewHolder(cfg.Series, config.Wind{})
	renderer, err := web.NewRenderer(cat, holder, cfg)
	if err != nil {
		t.Fatalf("web.NewRenderer: %v", err)
	}

	pagePatterns := append(api.RoutePatterns(api.Deps{}), renderer.RoutePatterns()...)

	for _, pattern := range pagePatterns {
		path := concretize(pattern)
		assertNoSetCookie(t, public, path)
	}
	for _, pattern := range server.PrivateRoutePatterns(server.Options{}) {
		path := concretize(pattern)
		assertNoSetCookie(t, private, path)
	}

	// Error paths: a 404 off the page router, a 400 off an API route, and a
	// method the mux rejects with 405 — none of them are exempt from the
	// no-cookie rule.
	assertNoSetCookie(t, public, "/this-path-does-not-exist")
	assertNoSetCookie(t, public, "/api/v1/sensor/1/series") // missing required query params -> 400
	assertNoSetCookiePost(t, public, "/api/v1/overview")    // GET-only route -> 405
}

// concretize turns a registered ServeMux pattern into a request path: strips
// the leading method, fills {slug}/{id} wildcards with a harmless sample
// value, and turns "/{$}" (exact match) and a bare trailing "/" (subtree
// match) into a real path a client would actually send.
func concretize(pattern string) string {
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		pattern = pattern[i+1:]
	}
	pattern = strings.ReplaceAll(pattern, "{slug}", "sofia")
	pattern = strings.ReplaceAll(pattern, "{id}", "1")
	pattern = strings.ReplaceAll(pattern, "/{$}", "/")
	if pattern == "" {
		pattern = "/"
	}
	if strings.HasSuffix(pattern, "/") && pattern != "/" {
		pattern += "probe"
	}
	return pattern
}

func assertNoSetCookie(t *testing.T, addr, path string) {
	t.Helper()
	resp := get(t, addr, path)
	if sc := resp.Header.Values("Set-Cookie"); len(sc) > 0 {
		t.Errorf("GET %s (status %d) set a cookie: %v", path, resp.StatusCode, sc)
	}
}

func assertNoSetCookiePost(t *testing.T, addr, path string) {
	t.Helper()
	resp, err := http.Post("http://"+addr+path, "text/plain", nil)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if sc := resp.Header.Values("Set-Cookie"); len(sc) > 0 {
		t.Errorf("POST %s (status %d) set a cookie: %v", path, resp.StatusCode, sc)
	}
}
