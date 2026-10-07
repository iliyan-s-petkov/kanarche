package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"airbg.org/internal/httpx"
)

func TestWWWToApex(t *testing.T) {
	const base = "https://kanarche.eu"
	cases := []struct {
		name, host, target string
		wantLoc            string // empty means "passes through"
	}{
		{"www page", "www.kanarche.eu", "/", base + "/"},
		{"www path and query", "www.kanarche.eu", "/area/sofia?lang=bg&x=1", base + "/area/sofia?lang=bg&x=1"},
		{"www api", "www.kanarche.eu", "/api/v1/overview?a=b", base + "/api/v1/overview?a=b"},
		{"uppercase", "WWW.Kanarche.EU", "/about", base + "/about"},
		{"port 443", "www.kanarche.eu:443", "/about", base + "/about"},
		{"apex", "kanarche.eu", "/about", ""},
		{"apex with port", "kanarche.eu:8080", "/about", ""},
		{"localhost", "localhost:8080", "/", ""},
		{"origin ip", "203.0.113.7", "/healthz", ""},
		{"unrelated www", "www.example.com", "/", ""},
		{"suffix lookalike", "www.kanarche.eu.evil.test", "/", ""},
		{"subdomain", "tiles.kanarche.eu", "/", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
			h := httpx.WWWToApex(next, base+"/")
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			req.Host = tc.host
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if tc.wantLoc == "" {
				if rec.Code != http.StatusTeapot {
					t.Fatalf("status = %d, want passthrough", rec.Code)
				}
				return
			}
			if rec.Code != http.StatusMovedPermanently {
				t.Fatalf("status = %d, want 301", rec.Code)
			}
			if got := rec.Header().Get("Location"); got != tc.wantLoc {
				t.Fatalf("Location = %q, want %q", got, tc.wantLoc)
			}
		})
	}
}

func TestWWWToApexIsInertWhenBaseIsUnusable(t *testing.T) {
	for _, base := range []string{"https://www.kanarche.eu", "not-a-url", ""} {
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "www.kanarche.eu"
		rec := httptest.NewRecorder()
		httpx.WWWToApex(next, base).ServeHTTP(rec, req)
		if rec.Code != http.StatusTeapot {
			t.Fatalf("base %q: status = %d, want passthrough", base, rec.Code)
		}
	}
}
