package rzi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestDiscoverRefusesUserinfoAndPort(t *testing.T) {
	for _, href := range []string{
		"https://user:pw@www.rzi-varna.com/health/mvodi-2026.pdf",
		"https://user@www.rzi-varna.com/health/mvodi-2026.pdf",
		"https://www.rzi-varna.com:8443/health/mvodi-2026.pdf",
	} {
		base, _ := url.Parse("https://rzi-varna.com/?page_id=2635")
		_, err := discover([]byte(`<a href="`+href+`">x</a>`), base, mustSpec(t, SourceVarna), "2026")
		if !errors.Is(err, ErrHost) {
			t.Errorf("%s: err = %v, want ErrHost", href, err)
		}
	}
}

func TestFetchRefusesUserinfoRedirect(t *testing.T) {
	for _, to := range []string{
		"https://user:pw@www.rzi-varna.com/health/other.pdf",
		"https://www.rzi-varna.com:8443/health/other.pdf",
	} {
		r := newRig(t, hostHandlers{
			"rzi-varna.com/": servePage(t, "varna-section.html"),
			"www.rzi-varna.com/health/mvodi-2026.pdf": func(w http.ResponseWriter, req *http.Request) {
				http.Redirect(w, req, to, http.StatusFound)
			},
			"www.rzi-varna.com/health/other.pdf": serveBytes("application/pdf", []byte(pdfBody)),
		})
		_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
		if !errors.Is(err, ErrRedirect) {
			t.Errorf("%s: err = %v, want ErrRedirect", to, err)
		}
		if r.hit("www.rzi-varna.com/health/other.pdf") {
			t.Errorf("%s: target was requested", to)
		}
	}
}

func TestNewRefusesUserinfoPageURL(t *testing.T) {
	r := varnaRig(t, nil)
	for _, raw := range []string{
		"https://user:pw@rzi-varna.com/?page_id=2635",
		"https://rzi-varna.com:8443/?page_id=2635",
	} {
		r.cfg.RZI.Varna.PageURL = raw
		if _, err := New(r.cfg, Options{Transport: r.tr}); !errors.Is(err, ErrHost) {
			t.Errorf("%s: err = %v, want ErrHost", raw, err)
		}
	}
}

// stall blocks until cleanup releases it. The 2s backstop only matters when the
// client timeout is missing, so the test fails instead of hanging.
func stall(release <-chan struct{}) {
	select {
	case <-release:
	case <-time.After(2 * time.Second):
	}
}

// The timeout must stop a stalled response, headers only or mid-body, while
// the handler is still blocked. The handler waits on a channel that only
// cleanup closes: if it also returned on the request context, the server would
// finish the response the moment the client gave up, and the client could see
// that clean end instead of its own timeout.
func TestFetchTimeoutBounded(t *testing.T) {
	tests := []struct {
		name string
		file func(w http.ResponseWriter, release <-chan struct{})
	}{
		{"no response", func(_ http.ResponseWriter, release <-chan struct{}) { stall(release) }},
		{"stalls mid-body", func(w http.ResponseWriter, release <-chan struct{}) {
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-"))
			w.(http.Flusher).Flush()
			stall(release)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release := make(chan struct{})
			r := varnaRig(t, func(w http.ResponseWriter, _ *http.Request) { tt.file(w, release) })
			// Cleanups run last-in first-out: release the handler before the
			// server closes, or Close would wait for it.
			t.Cleanup(func() { close(release) })
			r.cfg.RZI.RequestTimeout = 150 * time.Millisecond
			start := time.Now()
			_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
			if !errors.Is(err, ErrRequest) {
				t.Fatalf("err = %v, want ErrRequest", err)
			}
			if d := time.Since(start); d > time.Second {
				t.Errorf("took %v, timeout not applied", d)
			}
		})
	}
}
