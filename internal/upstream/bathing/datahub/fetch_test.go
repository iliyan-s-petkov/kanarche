package datahub

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"airbg.org/internal/xlsx/xlsxtest"
)

const xlsxType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// fixture is a TLS server serving body, plus a config pinned to it.
type fixture struct {
	srv  *httptest.Server
	cfg  FetchConfig
	hits atomic.Int32
	body []byte
	dir  string
}

func newFixture(t *testing.T, h func(f *fixture, w http.ResponseWriter, r *http.Request)) *fixture {
	t.Helper()
	body, err := xlsxtest.TableBook([][]string{{"a"}, {"b"}})
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{body: body, dir: t.TempDir()}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		if h == nil {
			w.Header().Set("Content-Type", xlsxType)
			w.Header().Set("ETag", `"abc"`)
			w.Header().Set("Last-Modified", "Mon, 01 Jun 2026 07:55:58 GMT")
			w.Write(f.body)
			return
		}
		h(f, w, r)
	}))
	t.Cleanup(f.srv.Close)
	f.cfg = FetchConfig{
		URL:          f.srv.URL + "/datastore/public/x/file.xlsx",
		SHA256:       sum(body),
		Size:         int64(len(body)),
		AllowedHosts: []string{"127.0.0.1"},
		Timeout:      5 * time.Second,
		MaxBytes:     1 << 20,
		Client:       f.srv.Client(),
	}
	return f
}

func (f *fixture) fetch() (Meta, error) {
	return Fetch(context.Background(), f.cfg, f.dir)
}

// leftovers lists what Fetch left in the output dir.
func (f *fixture) leftovers(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

func wantFail(t *testing.T, f *fixture, want error) {
	t.Helper()
	m, err := f.fetch()
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if m.Path != "" {
		t.Fatalf("path returned on failure: %q", m.Path)
	}
	if l := f.leftovers(t); len(l) != 0 {
		t.Fatalf("files left behind: %v", l)
	}
}

func TestFetchOK(t *testing.T) {
	f := newFixture(t, nil)
	m, err := f.fetch()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(m.Path)
	if err != nil || !bytes.Equal(got, f.body) {
		t.Fatalf("body mismatch, err=%v", err)
	}
	st, _ := os.Stat(m.Path)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", st.Mode().Perm())
	}
	if filepath.Dir(m.Path) != f.dir || filepath.Base(m.Path) != "file.xlsx" {
		t.Fatalf("path = %s", m.Path)
	}
	if m.SHA256 != f.cfg.SHA256 || m.Size != int64(len(f.body)) || m.ETag != `"abc"` || m.LastModified == "" {
		t.Fatalf("meta = %+v", m)
	}
	if l := f.leftovers(t); len(l) != 1 {
		t.Fatalf("expected only the final file, got %v", l)
	}
}

func TestFetchRejectsHTTP(t *testing.T) {
	f := newFixture(t, nil)
	f.cfg.URL = "http://" + f.srv.Listener.Addr().String() + "/f.xlsx"
	wantFail(t, f, ErrNotHTTPS)
	if f.hits.Load() != 0 {
		t.Fatal("request was sent")
	}
}

func TestFetchRejectsHostNotAllowed(t *testing.T) {
	f := newFixture(t, nil)
	f.cfg.AllowedHosts = []string{"sdi.eea.europa.eu"}
	wantFail(t, f, ErrHostNotAllowed)
	if f.hits.Load() != 0 {
		t.Fatal("request was sent")
	}
}

func TestFetchRejectsBadPath(t *testing.T) {
	for _, p := range []string{"/f.xlsm", "/f.xltm", "/f.xlsb", "/f.xls", "/f", "/"} {
		t.Run(p, func(t *testing.T) {
			f := newFixture(t, nil)
			f.cfg.URL = f.srv.URL + p
			wantFail(t, f, ErrNotXLSX)
			if f.hits.Load() != 0 {
				t.Fatal("request was sent")
			}
		})
	}
}

func TestFetchRejectsOffHostRedirect(t *testing.T) {
	other := newFixture(t, nil)
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.srv.URL+"/file.xlsx", http.StatusFound)
	})
	wantFail(t, f, ErrRedirect)
	if other.hits.Load() != 0 {
		t.Fatal("redirect target was contacted")
	}
}

func TestFetchRejectsRedirectToHTTP(t *testing.T) {
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://"+r.Host+"/file.xlsx", http.StatusFound)
	})
	wantFail(t, f, ErrRedirect)
}

func TestFetchFollowsSameHostRedirect(t *testing.T) {
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/moved/file.xlsx" {
			http.Redirect(w, r, "/moved/file.xlsx", http.StatusMovedPermanently)
			return
		}
		w.Header().Set("Content-Type", xlsxType)
		w.Write(f.body)
	})
	if _, err := f.fetch(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchRejectsNon200(t *testing.T) {
	for _, code := range []int{403, 404, 500, 204} {
		f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", xlsxType)
			w.WriteHeader(code)
			w.Write(f.body)
		})
		wantFail(t, f, ErrStatus)
	}
}

func TestFetchRejectsContentType(t *testing.T) {
	for _, ct := range []string{"text/html", "application/json", "application/xml", ""} {
		f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
			w.Header()["Content-Type"] = []string{ct}
			if ct == "" {
				w.Header()["Content-Type"] = nil
			}
			w.Write(f.body)
		})
		wantFail(t, f, ErrContentType)
	}
}

func TestFetchAcceptsOctetStream(t *testing.T) {
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream; charset=binary")
		w.Write(f.body)
	})
	if _, err := f.fetch(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchRejectsNonZipMagic(t *testing.T) {
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", xlsxType)
		w.Write([]byte("<html>You can't view this record</html>"))
	})
	wantFail(t, f, ErrNotZip)
}

func TestFetchRejectsOLEHeader(t *testing.T) {
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", xlsxType)
		w.Write(append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 64)...))
	})
	wantFail(t, f, ErrOLE)
}

func TestFetchRejectsShortBody(t *testing.T) {
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", xlsxType)
		w.Write([]byte("PK"))
	})
	wantFail(t, f, ErrNotZip)
}

func TestFetchRejectsOversize(t *testing.T) {
	// No Content-Length, so only the streaming cap can stop it.
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", xlsxType)
		w.Write(f.body[:4])
		fl := w.(http.Flusher)
		chunk := make([]byte, 4096)
		for i := 0; i < 1024; i++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
			fl.Flush()
		}
	})
	f.cfg.MaxBytes = 64 << 10
	f.cfg.Size = 0
	wantFail(t, f, ErrTooLarge)
}

func TestFetchStopsStreamingAtCap(t *testing.T) {
	var sent atomic.Int64
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", xlsxType)
		w.Write(f.body[:4])
		fl := w.(http.Flusher)
		chunk := make([]byte, 4096)
		for i := 0; i < 100000; i++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
			sent.Add(int64(len(chunk)))
			fl.Flush()
		}
	})
	f.cfg.MaxBytes = 64 << 10
	f.cfg.Size = 0
	wantFail(t, f, ErrTooLarge)
	if sent.Load() > 8<<20 {
		t.Fatalf("server sent %d bytes before the client hung up", sent.Load())
	}
}

func TestFetchRejectsDeclaredOversize(t *testing.T) {
	f := newFixture(t, nil)
	f.cfg.MaxBytes = int64(len(f.body)) - 1
	wantFail(t, f, ErrTooLarge)
}

func TestFetchRejectsSizeMismatch(t *testing.T) {
	f := newFixture(t, nil)
	f.cfg.Size = int64(len(f.body)) + 1
	wantFail(t, f, ErrSize)
	f = newFixture(t, nil)
	f.cfg.Size = int64(len(f.body)) - 1
	wantFail(t, f, ErrSize)
}

func TestFetchRejectsSHA256Mismatch(t *testing.T) {
	f := newFixture(t, nil)
	f.cfg.SHA256 = sum([]byte("something else"))
	m, err := f.fetch()
	if !errors.Is(err, ErrSHA256) {
		t.Fatalf("err = %v", err)
	}
	// The observed hash is reported so a new edition can be pinned on purpose.
	if want := sum(f.body); !bytes.Contains([]byte(err.Error()), []byte(want)) {
		t.Fatalf("error lacks observed hash: %v", err)
	}
	// No path means no caller can hand the bytes to the parser.
	if m.Path != "" || len(f.leftovers(t)) != 0 {
		t.Fatalf("unverified file escaped: %+v %v", m, f.leftovers(t))
	}
}

func TestFetchRejectsBadPin(t *testing.T) {
	for _, s := range []string{"", "abc", "XX" + sum(nil)[2:], sum(nil) + "0"} {
		f := newFixture(t, nil)
		f.cfg.SHA256 = s
		wantFail(t, f, ErrBadPin)
		if f.hits.Load() != 0 {
			t.Fatal("request was sent")
		}
	}
}

func TestFetchTimeout(t *testing.T) {
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", xlsxType)
		w.Write(f.body[:4])
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	f.cfg.Timeout = 150 * time.Millisecond
	start := time.Now()
	m, err := f.fetch()
	if err == nil || m.Path != "" {
		t.Fatalf("err=%v meta=%+v", err, m)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("took %v", time.Since(start))
	}
	if l := f.leftovers(t); len(l) != 0 {
		t.Fatalf("files left: %v", l)
	}
}

func TestFetchHonoursCallerCancel(t *testing.T) {
	f := newFixture(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Fetch(ctx, f.cfg, f.dir); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchRejectsNonPositiveLimits(t *testing.T) {
	f := newFixture(t, nil)
	f.cfg.Timeout = 0
	wantFail(t, f, ErrBadLimits)
	f = newFixture(t, nil)
	f.cfg.MaxBytes = 0
	wantFail(t, f, ErrBadLimits)
	if f.hits.Load() != 0 {
		t.Fatal("request was sent")
	}
}

func TestFetchStopsRedirectLoop(t *testing.T) {
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path+"x.xlsx", http.StatusFound)
	})
	wantFail(t, f, ErrRedirect)
	if f.hits.Load() > 11 {
		t.Fatalf("followed %d redirects", f.hits.Load())
	}
}

func TestFetchRejectsDeclaredLengthBeforeReading(t *testing.T) {
	f := newFixture(t, func(f *fixture, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", xlsxType)
		w.Header().Set("Content-Length", "1073741824")
		w.Write(f.body[:4])
		w.(http.Flusher).Flush()
		<-r.Context().Done() // never sends the rest
	})
	f.cfg.Size = 0
	start := time.Now()
	wantFail(t, f, ErrTooLarge)
	if time.Since(start) > 2*time.Second {
		t.Fatal("waited on the body instead of refusing the header")
	}
}
