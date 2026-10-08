package wind_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"kanarche.eu/internal/wind"
)

func TestFetchSendsTheConfiguredUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.URL = srv.URL
	cfg.UserAgent = "kanarche.eu collector (+https://kanarche.eu)"
	// The empty payload is a parse error; only the request header matters here.
	_, _ = wind.New(cfg).Fetch(context.Background(), points())
	if got != cfg.UserAgent {
		t.Errorf("User-Agent = %q, want %q", got, cfg.UserAgent)
	}
}
