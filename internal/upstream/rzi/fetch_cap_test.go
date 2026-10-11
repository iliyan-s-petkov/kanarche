package rzi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
)

// streamJunk writes n bytes in 64 KiB flushed writes, starting with prefix,
// and counts how many went out before the client hung up.
func streamJunk(w http.ResponseWriter, prefix string, n int, sent *int64) {
	chunk := bytes.Repeat([]byte("x"), 64<<10)
	first := true
	for left := n; left > 0; left -= len(chunk) {
		buf := chunk
		if first {
			buf, first = append([]byte(prefix), chunk...), false
		}
		m, err := w.Write(buf)
		atomic.AddInt64(sent, int64(m))
		if err != nil {
			return
		}
		w.(http.Flusher).Flush()
	}
}

// The cap bounds memory, not only the verdict: a body that never ends must
// be abandoned soon after cap+1 bytes (I5).
func TestFetchStopsReadingAtCap(t *testing.T) {
	var sent int64
	r := varnaRig(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		streamJunk(w, "%PDF-", 64<<20, &sent)
	})
	_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("err = %v, want ErrFileTooLarge", err)
	}
	if got := atomic.LoadInt64(&sent); got > 16<<20 {
		t.Errorf("server sent %d bytes: the read is not bounded", got)
	}
}

func TestFetchStopsReadingPageAtCap(t *testing.T) {
	var sent int64
	r := newRig(t, hostHandlers{"rzi-varna.com/": func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		streamJunk(w, "", 64<<20, &sent)
	}})
	_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
	if !errors.Is(err, ErrPageTooLarge) {
		t.Fatalf("err = %v, want ErrPageTooLarge", err)
	}
	if got := atomic.LoadInt64(&sent); got > 16<<20 {
		t.Errorf("server sent %d bytes: the page read is not bounded", got)
	}
}

// A declared length over the cap is refused before any body is read.
func TestFetchRefusesDeclaredLengthOverCap(t *testing.T) {
	r := varnaRig(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", "1073741824")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-req.Context().Done()
	})
	_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("err = %v, want ErrFileTooLarge", err)
	}
}
