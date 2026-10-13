package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"kanarche.eu/internal/httpx"
)

func TestLicenceLinkIsSetOnceUnderAPIv1Only(t *testing.T) {
	inner := httpx.LicenceLink(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	// A stale Link written earlier must be replaced, not joined.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Link", "<x>; rel=\"other\"")
		inner.ServeHTTP(w, r)
	})
	for path, want := range map[string][]string{
		"/api/v1/overview": {`<https://kanarche.eu/licences>; rel="license"`},
		"/api/v1/nothing":  {`<https://kanarche.eu/licences>; rel="license"`},
		"/api/partner/v1/": {`<x>; rel="other"`},
		"/":                {`<x>; rel="other"`},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Values("Link"); !slices.Equal(got, want) {
			t.Errorf("%s: Link = %q, want %q", path, got, want)
		}
	}
}

func TestCORSExposesLinkToAllowedOrigins(t *testing.T) {
	h := httpx.CORS(okHandler(), allowlist(t, false))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	req.Header.Set("Origin", siteOrigin)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Values("Access-Control-Expose-Headers"); !slices.Equal(got, []string{"Link"}) {
		t.Errorf("Access-Control-Expose-Headers = %q, want [Link]", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	req.Header.Set("Origin", "https://evil.test")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Values("Access-Control-Expose-Headers"); len(got) != 0 {
		t.Errorf("refused origin got Access-Control-Expose-Headers = %q", got)
	}
}
