package api_test

import (
	"net/http"
	"testing"
)

const licenceLink = `<https://kanarche.eu/licences>; rel="license"`

func TestEveryAPIResponseCarriesTheLicenceLink(t *testing.T) {
	paths := []string{
		"/api/v1/overview",
		"/api/v1/hexes",
		"/api/v1/meta",
		"/api/v1/sensor/42/series?metric=P2&period=24h",
		"/api/v1/sea/sites",
		"/api/v1/sea/sites/BG0000000000000000",
		"/api/v1/sea/sites/bg'--",
		"/api/v1/hexes?bbox=nonsense",
	}
	for _, p := range paths {
		rec := serve(t, visitorsDeps(t, seaSource()), get(p, "203.0.113.90"))
		if got := rec.Header().Get("Link"); got != licenceLink {
			t.Errorf("GET %s (%d): Link = %q, want %q", p, rec.Code, got, licenceLink)
		}
	}
}

func TestLicenceLinkRidesOnAnErrorResponse(t *testing.T) {
	rec := serve(t, visitorsDeps(t, seaSource()), get("/api/v1/sea/sites/BG0000000000000000", "203.0.113.91"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if got := rec.Header().Get("Link"); got != licenceLink {
		t.Errorf("Link = %q, want %q", got, licenceLink)
	}
}
