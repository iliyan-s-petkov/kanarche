package main

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// listenLoopback starts serve on a loopback port and returns its address.
func listenLoopback(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go serve(ln)
	return ln.Addr().String()
}

func TestDialReachesOwnListener(t *testing.T) {
	addr := listenLoopback(t)
	var out bytes.Buffer
	if code := run([]string{"-dial", addr}, &out); code != 0 {
		t.Fatalf("dial to own listener: exit %d, out %q", code, out.String())
	}
	if out.String() != "reached\n" {
		t.Fatalf("out = %q", out.String())
	}
}

func TestDialFailsWithoutListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	var out bytes.Buffer
	if code := run([]string{"-dial", addr}, &out); code == 0 {
		t.Fatal("dial to a closed port exited 0")
	}
}

// A listener that does not send the token is not ours: no success.
func TestDialRejectsForeignListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		c.Write([]byte("HTTP/1.1 200 OK\r\n"))
		c.Close()
	}()
	var out bytes.Buffer
	if code := run([]string{"-dial", ln.Addr().String()}, &out); code == 0 {
		t.Fatal("foreign listener counted as reached")
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"-dial", "a:1", "-listen", ":1"}, {"-bogus"}} {
		var out bytes.Buffer
		if code := run(args, &out); code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
	}
}

func TestDialTimeoutIsShort(t *testing.T) {
	if dialTimeout > 5*time.Second {
		t.Fatalf("dialTimeout = %v; the sandbox run must end well inside 20 s", dialTimeout)
	}
}
