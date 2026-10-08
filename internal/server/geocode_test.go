package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"kanarche.eu/internal/config"
)

// safeBuffer: the server logs from its own goroutines.
type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// TestGeocodeThroughTheFullChainReachesTheStubAndLogsNoQuery runs the real
// server against a stub upstream: the route is mounted, answers the reduced
// shape, and the whole middleware chain (limiter, metrics, recovery) leaves the
// typed address out of the log.
func TestGeocodeThroughTheFullChainReachesTheStubAndLogsNoQuery(t *testing.T) {
	const distinctive = "Qwzxv-Home-Address-42"

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"display_name":"ул. Тест 1","lat":"42.7","lon":"23.3","boundingbox":["42.6","42.8","23.2","23.4"]}]`))
	}))
	defer up.Close()

	var logs safeBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	public, _, _ := start(t, "", func(c *config.Config) { c.Geocoder.URL = up.URL })

	resp := get(t, public, "/api/v1/geocode?lang=bg&q="+distinctive)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body %s", resp.StatusCode, body)
	}
	var rows []struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(body, &rows); err != nil || len(rows) != 1 || rows[0].Label != "ул. Тест 1" {
		t.Errorf("body = %s (err %v), want one reduced row", body, err)
	}
	// A bad request and a bad language go through the same chain.
	get(t, public, "/api/v1/geocode?q=ab&x="+distinctive)

	if strings.Contains(logs.String(), "Qwzxv") {
		t.Errorf("the query text reached the log:\n%s", logs.String())
	}
}
