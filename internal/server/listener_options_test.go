package server_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/server"
	"kanarche.eu/internal/snapshot"
)

// TestRunServesOnSuppliedListeners proves Run uses Options.PublicListener and
// PrivateListener when set, rather than binding Config.Listen itself. Mutate
// serveCapped/listen to ignore a non-nil ln and this fails on timeout.
func TestRunServesOnSuppliedListeners(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	cfg := testConfig(t)
	// Left blank: if Run ignored the supplied listeners it would bind these
	// instead, and nothing would answer on the addresses below.
	cfg.Listen.Addr = ""
	cfg.Listen.MetricsAddr = ""
	holder := snapshot.NewHolder(cfg.Series, config.Wind{})
	holder.Store(&snapshot.Snapshot{
		GeneratedAt: time.Now().UTC(),
		KnownSlugs:  map[string]snapshot.AreaMeta{},
		Overview:    snapshot.Body{JSON: []byte(`{"areas":[]}`), ETag: `"t"`},
	})

	publicLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	privateLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	cfg.Listen.BaseURL = "http://" + publicLn.Addr().String()

	srv, err := server.New(server.Options{
		Config: cfg, Catalogue: cat, Snapshots: holder,
		PublicListener: publicLn, PrivateListener: privateLn,
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.Run(ctx) }()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + privateLn.Addr().String() + "/healthz")
	if err != nil {
		t.Fatalf("GET %s/healthz: %v", privateLn.Addr(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
