package server_test

import (
	"io"
	"net/http"
	"slices"
	"testing"
)

const wantLicenceLink = `<https://kanarche.eu/licences>; rel="license"`

// The wrapper sits at the /api/ mount, so mux-generated 404 and 405 answers
// carry the header too, not only the handlers.
func TestEveryAPIv1ResponseCarriesTheLicenceLink(t *testing.T) {
	public, _ := running(t)
	for _, tc := range []struct {
		name, method, path string
	}{
		{"overview", http.MethodGet, "/api/v1/overview"},
		{"hexes", http.MethodGet, "/api/v1/hexes"},
		{"meta", http.MethodGet, "/api/v1/meta"},
		{"sensor series", http.MethodGet, "/api/v1/sensor/42/series?metric=P2&period=24h"},
		{"sea sites", http.MethodGet, "/api/v1/sea/sites"},
		{"unknown sea site", http.MethodGet, "/api/v1/sea/sites/BG0000000000000000"},
		{"unknown path", http.MethodGet, "/api/v1/nothing-here"},
		{"POST to a GET route", http.MethodPost, "/api/v1/overview"},
		{"HEAD", http.MethodHead, "/api/v1/overview"},
	} {
		req, err := http.NewRequest(tc.method, "http://"+public+tc.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if got := resp.Header.Values("Link"); !slices.Equal(got, []string{wantLicenceLink}) {
			t.Errorf("%s (%s %s -> %d): Link = %q, want exactly [%q]", tc.name, tc.method, tc.path, resp.StatusCode, got, wantLicenceLink)
		}
	}
}

func TestPagesDoNotCarryTheLicenceLink(t *testing.T) {
	public, _ := running(t)
	if got := get(t, public, "/").Header.Values("Link"); len(got) != 0 {
		t.Errorf("GET / Link = %q, want none", got)
	}
}
