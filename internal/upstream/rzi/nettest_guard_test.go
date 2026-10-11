package rzi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// errNetGuard marks a dial the test guard refused.
var errNetGuard = errors.New("nettest guard: non-loopback dial blocked")

// guardDial refuses every address that is not a loopback IP.
// Hostnames are refused too, so no DNS lookup can leave the test.
func guardDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("%w: %s", errNetGuard, addr)
	}
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

func TestMain(m *testing.M) {
	// Replace the default transport so every net/http client in this
	// package goes through the guard. Proxy nil keeps env proxies out.
	http.DefaultTransport = &http.Transport{DialContext: guardDial, Proxy: nil}
	os.Exit(m.Run())
}

// TestNetGuardBlocksExternalDial makes a real request to a TEST-NET-3
// address. The guard must refuse it before any packet leaves.
func TestNetGuardBlocksExternalDial(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://203.0.113.1:443/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if !errors.Is(err, errNetGuard) {
		t.Fatalf("external dial not blocked by guard, err = %v", err)
	}
}

// TestNetGuardAllowsLoopback checks the guard does not block httptest servers.
func TestNetGuardAllowsLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("loopback request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
