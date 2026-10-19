package api_test

import (
	"net/http"
	"testing"
)

// Callers of the retired sofiya slugs get a 301 to the same endpoint under the
// new slug, query kept. A current slug is not redirected.
func TestRetiredAreaSlugsRedirectOnEveryAreaEndpoint(t *testing.T) {
	for from, to := range map[string]string{
		"/api/v1/area/sofiya/sensors":                     "/api/v1/area/sofia/sensors",
		"/api/v1/area/sofiya/series?metric=P2&period=24h": "/api/v1/area/sofia/series?metric=P2&period=24h",
		"/api/v1/area/sofiya-grad-oblast/pollen":          "/api/v1/area/sofia-grad-oblast/pollen",
	} {
		rec := serve(t, deps(t, fixture(t)), get(from, "203.0.113.50"))
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("GET %s = %d, want 301", from, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != to {
			t.Errorf("GET %s Location = %q, want %q", from, got, to)
		}
	}
}

func TestCurrentAreaSlugIsNotRedirected(t *testing.T) {
	rec := serve(t, deps(t, fixture(t)), get("/api/v1/area/sofia/sensors", "203.0.113.51"))
	if rec.Code == http.StatusMovedPermanently {
		t.Error("a current slug was redirected")
	}
}
