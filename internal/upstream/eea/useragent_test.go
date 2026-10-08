package eea_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kanarche.eu/internal/upstream/eea"
)

func TestEveryRequestSendsTheConfiguredUserAgent(t *testing.T) {
	var agents []string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agents = append(agents, r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte(srv.URL + "/a.parquet\n"))
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL, srv.URL)
	cfg.UserAgent = "kanarche.eu collector (+https://kanarche.eu)"
	c := eea.New(cfg)
	ctx := context.Background()
	// The bodies are not valid metadata or parquet; only the headers matter here.
	_, _, _ = c.FileURLs(ctx)
	_, _, _, _ = c.FetchFile(ctx, srv.URL+"/a.parquet", time.Time{})
	_, _ = c.FetchMetadata(ctx)

	if len(agents) != 3 {
		t.Fatalf("saw %d requests, want 3", len(agents))
	}
	for i, got := range agents {
		if got != cfg.UserAgent {
			t.Errorf("request %d User-Agent = %q, want %q", i, got, cfg.UserAgent)
		}
	}
}
