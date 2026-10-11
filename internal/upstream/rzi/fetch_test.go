package rzi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kanarche.eu/internal/config"
)

// hostHandlers maps "host/path" (host without port) to a handler.
type hostHandlers map[string]http.HandlerFunc

// rig is one TLS server that stands in for every RZI host. Its certificate
// names every production host and node2.invalid, and the transport trusts it,
// so certificate checks still run.
type rig struct {
	t        *testing.T
	handlers hostHandlers
	mu       sync.Mutex
	hits     []string
	tr       *http.Transport
	inbox    string
	cfg      config.Sea
}

const (
	pdfBody  = "%PDF-1.5\n1 0 obj\n<<>>\nendobj\n"
	xlsMagic = "\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1"
)

func newRig(t *testing.T, h hostHandlers) *rig {
	t.Helper()
	hosts := []string{"rzi-varna.com", "www.rzi-varna.com", "rzi-burgas.bg", "rzi-dobrich.egov.bg", "node2.invalid", "evil.example"}
	cert, pool := selfSigned(t, hosts)
	r := &rig{t: t, handlers: h}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			host = req.Host
		}
		key := host + req.URL.Path
		r.mu.Lock()
		r.hits = append(r.hits, key)
		r.mu.Unlock()
		hd, ok := r.handlers[key]
		if !ok {
			http.NotFound(w, req)
			return
		}
		hd(w, req)
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	addr := srv.Listener.Addr().String()
	r.tr = &http.Transport{
		// Every hostname dials the one test server, through the loopback guard.
		DialContext:     func(ctx context.Context, network, _ string) (net.Conn, error) { return guardDial(ctx, network, addr) },
		TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
	}
	t.Cleanup(r.tr.CloseIdleConnections)
	r.inbox = filepath.Join(t.TempDir(), "inbox")
	r.cfg = config.Sea{
		UserAgent: "kanarche-test (+https://kanarche.eu)",
		RZI: config.SeaRZI{
			InboxDir:       r.inbox,
			OutboxDir:      filepath.Join(t.TempDir(), "outbox"),
			MaxFileBytes:   1024,
			RequestTimeout: 5 * time.Second,
			Varna:          config.SeaRZISource{PageURL: "https://rzi-varna.com/?page_id=2635"},
			Burgas:         config.SeaRZISource{PageURL: "https://rzi-burgas.bg/kachestvo-na-vodite/"},
			Dobrich:        config.SeaRZISource{PageURL: "https://rzi-dobrich.egov.bg/wps/portal/rzi.dobrich/main-activities/public-health/swimming-zone"},
		},
	}
	return r
}

func (r *rig) fetcher() *Fetcher {
	r.t.Helper()
	f, err := New(r.cfg, Options{
		Transport: r.tr,
		Now:       func() time.Time { return time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		r.t.Fatal(err)
	}
	return f
}

func (r *rig) hit(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, h := range r.hits {
		if h == key {
			return true
		}
	}
	return false
}

func selfSigned(t *testing.T, hosts []string) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "rzi test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              hosts,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

func serveBytes(ctype string, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", ctype)
		_, _ = w.Write(body)
	}
}

func servePage(t *testing.T, name string) http.HandlerFunc {
	return serveBytes("text/html; charset=utf-8", page(t, name))
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func inboxNames(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

// varnaRig serves the Varna page and a file handler for mvodi-2026.pdf.
func varnaRig(t *testing.T, file http.HandlerFunc) *rig {
	return newRig(t, hostHandlers{
		"rzi-varna.com/": servePage(t, "varna-section.html"),
		"www.rzi-varna.com/health/mvodi-2026.pdf": file,
	})
}

func TestFetchWritesNamedBySHA(t *testing.T) {
	body := []byte(pdfBody)
	r := varnaRig(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Last-Modified", "Wed, 23 Sep 2026 10:00:00 GMT")
		if got := req.Header.Get("User-Agent"); got != "kanarche-test (+https://kanarche.eu)" {
			t.Errorf("User-Agent = %q", got)
		}
		_, _ = w.Write(body)
	})
	res, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
	if err != nil {
		t.Fatal(err)
	}
	want := sum(body)
	if res.Status != StatusWritten || res.SHA256 != want || res.ETag != `"v1"` || res.LastModified == "" {
		t.Fatalf("res = %+v", res)
	}
	path := filepath.Join(r.inbox, want+".pdf")
	if res.Path != path {
		t.Errorf("Path = %q, want %q", res.Path, path)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("inbox content: %v", err)
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm()&^0o640 != 0 {
		t.Errorf("mode = %v, want 0640 or tighter", st.Mode().Perm())
	}
	if names := inboxNames(t, r.inbox); len(names) != 1 {
		t.Errorf("inbox = %v, want exactly the file (no temp leftovers)", names)
	}
}

func TestFetchSameSHAWritesNothing(t *testing.T) {
	body := []byte(pdfBody)
	r := varnaRig(t, serveBytes("application/pdf", body))
	res, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{SHA256: sum(body)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusUnchanged {
		t.Errorf("Status = %v, want unchanged", res.Status)
	}
	if names := inboxNames(t, r.inbox); len(names) != 0 {
		t.Errorf("inbox = %v, want empty", names)
	}
}

func TestFetchExistingInboxFileIsNotRewritten(t *testing.T) {
	body := []byte(pdfBody)
	r := varnaRig(t, serveBytes("application/pdf", body))
	if err := os.MkdirAll(r.inbox, 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(r.inbox, sum(body)+".pdf")
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.WriteFile(path, body, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	res, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
	if err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(path)
	if res.Status != StatusUnchanged || !st.ModTime().Equal(old) {
		t.Errorf("Status = %v, mtime = %v: file was rewritten", res.Status, st.ModTime())
	}
}

func TestFetchNotModifiedWritesNothing(t *testing.T) {
	var gotINM, gotIMS string
	r := varnaRig(t, func(w http.ResponseWriter, req *http.Request) {
		gotINM, gotIMS = req.Header.Get("If-None-Match"), req.Header.Get("If-Modified-Since")
		w.WriteHeader(http.StatusNotModified)
	})
	res, err := r.fetcher().Fetch(context.Background(), SourceVarna,
		Validators{ETag: `"v1"`, LastModified: "Wed, 23 Sep 2026 10:00:00 GMT", SHA256: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusNotModified || res.SHA256 != strings.Repeat("a", 64) {
		t.Errorf("res = %+v", res)
	}
	if gotINM != `"v1"` || gotIMS != "Wed, 23 Sep 2026 10:00:00 GMT" {
		t.Errorf("conditional headers = %q, %q", gotINM, gotIMS)
	}
	if names := inboxNames(t, r.inbox); len(names) != 0 {
		t.Errorf("inbox = %v, want empty", names)
	}
}

func TestFetchNoValidatorsSendsNoConditionalHeaders(t *testing.T) {
	r := varnaRig(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("If-None-Match") != "" || req.Header.Get("If-Modified-Since") != "" {
			t.Error("conditional header sent without validators")
		}
		serveBytes("application/pdf", []byte(pdfBody))(w, req)
	})
	if _, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{}); err != nil {
		t.Fatal(err)
	}
}

// I5: one byte over the cap is refused and nothing reaches the inbox.
func TestFetchRefusesCapPlusOne(t *testing.T) {
	over := append([]byte("%PDF-"), bytes.Repeat([]byte("x"), 1024-5+1)...) // 1025 bytes
	exact := over[:1024]
	for _, tt := range []struct {
		name    string
		body    []byte
		chunked bool
		wantErr error
	}{
		{"exact cap", exact, false, nil},
		{"cap plus one with length", over, false, ErrFileTooLarge},
		// No Content-Length: only the bounded read can catch it.
		{"cap plus one chunked", over, true, ErrFileTooLarge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := varnaRig(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/pdf")
				if tt.chunked {
					w.(http.Flusher).Flush()
				}
				_, _ = w.Write(tt.body)
			})
			_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if names := inboxNames(t, r.inbox); len(names) != 0 {
					t.Errorf("inbox = %v, want empty", names)
				}
			}
		})
	}
}

// I6: the expected type decides, not what the server says.
func TestFetchRefusesWrongContent(t *testing.T) {
	html := []byte("<!DOCTYPE html><html><body>Access denied</body></html>")
	tests := []struct {
		name    string
		ctype   string
		body    []byte
		wantErr error
	}{
		{"html served as html", "text/html", html, ErrContentType},
		{"html served as pdf", "application/pdf", html, ErrMagic},
		{"xls served as pdf", "application/pdf", append([]byte(xlsMagic), 0, 0), ErrMagic},
		{"pdf served as octet-stream", "application/octet-stream", []byte(pdfBody), ErrContentType},
		{"missing content type", "", []byte(pdfBody), ErrContentType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := varnaRig(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header()["Content-Type"] = nil
				if tt.ctype != "" {
					w.Header().Set("Content-Type", tt.ctype)
				}
				_, _ = w.Write(tt.body)
			})
			_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if names := inboxNames(t, r.inbox); len(names) != 0 {
				t.Errorf("inbox = %v, want empty", names)
			}
		})
	}
}

func TestFetchMagicPerKind(t *testing.T) {
	zip := []byte("PK\x03\x04rest")
	burgasHandlers := func(t *testing.T, ctype string, body []byte) hostHandlers {
		return hostHandlers{
			"rzi-burgas.bg/kachestvo-na-vodite/":                                           servePage(t, "burgas-section.html"),
			"rzi-burgas.bg/wp-content/uploads/2026/09/tablici-monitoring-more-mz-2026.xls": serveBytes(ctype, body),
		}
	}
	t.Run("xls ok", func(t *testing.T) {
		r := newRig(t, burgasHandlers(t, "application/vnd.ms-excel", append([]byte(xlsMagic), 1, 2, 3)))
		res, err := r.fetcher().Fetch(context.Background(), SourceBurgas, Validators{})
		if err != nil || !strings.HasSuffix(res.Path, ".xls") {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
	})
	t.Run("zip as xls refused", func(t *testing.T) {
		r := newRig(t, burgasHandlers(t, "application/vnd.ms-excel", zip))
		if _, err := r.fetcher().Fetch(context.Background(), SourceBurgas, Validators{}); !errors.Is(err, ErrMagic) {
			t.Fatalf("err = %v, want ErrMagic", err)
		}
	})
	t.Run("html as xls refused", func(t *testing.T) {
		r := newRig(t, burgasHandlers(t, "application/vnd.ms-excel", []byte("<html>")))
		if _, err := r.fetcher().Fetch(context.Background(), SourceBurgas, Validators{}); !errors.Is(err, ErrMagic) {
			t.Fatalf("err = %v, want ErrMagic", err)
		}
	})
}

// Dobrich: the page links /wcm/myconnect/, which redirects to an internal
// host. The fetcher must ask /wcm/connect/ and never touch myconnect.
func TestFetchDobrichRewritesMyconnect(t *testing.T) {
	const dir = "rzi-dobrich.egov.bg/wps/wcm/"
	const tail = "rzi-dobrich.egov.bg-3026/d340442c-83a4-4708-b547-0143ce079330/RZI-Dobrich-morski+vodi+2026-Prilojenie+2.xlsx"
	var gotQuery string
	r := newRig(t, hostHandlers{
		"rzi-dobrich.egov.bg/wps/portal/rzi.dobrich/main-activities/public-health/swimming-zone": servePage(t, "dobrich-section.html"),
		dir + "connect/" + tail: func(w http.ResponseWriter, req *http.Request) {
			gotQuery = req.URL.RawQuery
			serveBytes("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []byte("PK\x03\x04zz"))(w, req)
		},
		dir + "myconnect/" + tail: func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "https://node2.invalid:10059/login", http.StatusFound)
		},
	})
	res, err := r.fetcher().Fetch(context.Background(), SourceDobrich, Validators{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(res.Path, ".xlsx") || gotQuery != "MOD=AJPERES" {
		t.Errorf("res = %+v, query = %q", res, gotQuery)
	}
	if r.hit(dir+"myconnect/"+tail) || r.hit("node2.invalid/login") {
		t.Error("myconnect or node2 was requested")
	}
}

// I4: a redirect off the source's allow-list is refused and not followed.
func TestFetchRefusesRedirectOffAllowList(t *testing.T) {
	tests := []struct {
		name, to string
		hit      string
	}{
		{"internal host", "https://node2.invalid:10059/login", "node2.invalid/login"},
		{"other host", "https://evil.example/mvodi-2026.pdf", "evil.example/mvodi-2026.pdf"},
		// Allowed for Burgas, not for Varna: the list is per source.
		{"other source host", "https://rzi-burgas.bg/x.pdf", "rzi-burgas.bg/x.pdf"},
		{"plain http", "http://www.rzi-varna.com/health/mvodi-2026.pdf", "www.rzi-varna.com/health/mvodi-2026.pdf-http"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := hostHandlers{
				"rzi-varna.com/": servePage(t, "varna-section.html"),
				"www.rzi-varna.com/health/mvodi-2026.pdf": func(w http.ResponseWriter, req *http.Request) {
					http.Redirect(w, req, tt.to, http.StatusFound)
				},
				tt.hit: serveBytes("application/pdf", []byte(pdfBody)),
			}
			r := newRig(t, h)
			_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
			if !errors.Is(err, ErrRedirect) {
				t.Fatalf("err = %v, want ErrRedirect", err)
			}
			if r.hit(tt.hit) {
				t.Error("redirect target was requested")
			}
			if names := inboxNames(t, r.inbox); len(names) != 0 {
				t.Errorf("inbox = %v, want empty", names)
			}
		})
	}
}

func TestFetchRefusesRedirectFromPage(t *testing.T) {
	r := newRig(t, hostHandlers{
		"rzi-varna.com/": func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "https://node2.invalid:10059/", http.StatusFound)
		},
	})
	_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
	if !errors.Is(err, ErrRedirect) {
		t.Fatalf("err = %v, want ErrRedirect", err)
	}
}

func TestFetchFollowsRedirectWithinAllowList(t *testing.T) {
	body := []byte(pdfBody)
	r := newRig(t, hostHandlers{
		"rzi-varna.com/": servePage(t, "varna-section.html"),
		"www.rzi-varna.com/health/mvodi-2026.pdf": func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "https://rzi-varna.com/files/mvodi-2026.pdf", http.StatusMovedPermanently)
		},
		"rzi-varna.com/files/mvodi-2026.pdf": serveBytes("application/pdf", body),
	})
	res, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
	if err != nil || res.SHA256 != sum(body) {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
}

func TestFetchRedirectLoopStops(t *testing.T) {
	r := newRig(t, hostHandlers{
		"rzi-varna.com/": servePage(t, "varna-section.html"),
		"www.rzi-varna.com/health/mvodi-2026.pdf": func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, req.URL.Path, http.StatusFound)
		},
	})
	_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
	if !errors.Is(err, ErrRedirect) {
		t.Fatalf("err = %v, want ErrRedirect", err)
	}
}

func TestFetchPageErrors(t *testing.T) {
	big := bytes.Repeat([]byte("a"), MaxPageBytes+1)
	tests := []struct {
		name    string
		page    http.HandlerFunc
		wantErr error
	}{
		{"status 500", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }, ErrStatus},
		{"page over cap", serveBytes("text/html", big), ErrPageTooLarge},
		{"no matching link", serveBytes("text/html", []byte("<html></html>")), ErrNoMatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRig(t, hostHandlers{"rzi-varna.com/": tt.page})
			_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestFetchFileStatusIsError(t *testing.T) {
	r := varnaRig(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(pdfBody))
	})
	_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
	if !errors.Is(err, ErrStatus) {
		t.Fatalf("err = %v, want ErrStatus", err)
	}
	if names := inboxNames(t, r.inbox); len(names) != 0 {
		t.Errorf("inbox = %v, want empty", names)
	}
}

func TestFetchErrorsAreFixedStrings(t *testing.T) {
	r := varnaRig(t, serveBytes("application/pdf", []byte("<html>SECRETTEXT</html>")))
	_, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{})
	if err == nil || strings.Contains(err.Error(), "SECRETTEXT") || strings.Contains(err.Error(), "html") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchUnknownSource(t *testing.T) {
	r := varnaRig(t, nil)
	if _, err := r.fetcher().Fetch(context.Background(), "rzi_nowhere", Validators{}); !errors.Is(err, ErrSource) {
		t.Fatalf("err = %v, want ErrSource", err)
	}
}

func TestFetchInboxUnwritable(t *testing.T) {
	r := varnaRig(t, serveBytes("application/pdf", []byte(pdfBody)))
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r.cfg.RZI.InboxDir = filepath.Join(blocker, "inbox")
	if _, err := r.fetcher().Fetch(context.Background(), SourceVarna, Validators{}); !errors.Is(err, ErrWrite) {
		t.Fatalf("err = %v, want ErrWrite", err)
	}
}

func TestNewRefusesPageURLOffAllowList(t *testing.T) {
	r := varnaRig(t, nil)
	r.cfg.RZI.Varna.PageURL = "https://evil.example/?page_id=1"
	if _, err := New(r.cfg, Options{Transport: r.tr}); !errors.Is(err, ErrHost) {
		t.Fatalf("err = %v, want ErrHost", err)
	}
	r.cfg.RZI.Varna.PageURL = "http://rzi-varna.com/?page_id=2635"
	if _, err := New(r.cfg, Options{Transport: r.tr}); !errors.Is(err, ErrHost) {
		t.Fatalf("http: err = %v, want ErrHost", err)
	}
}

// With no transport given the fetcher builds its own; it must not touch
// http.DefaultClient or http.DefaultTransport, which the test guard replaces.
func TestNewBuildsOwnTransport(t *testing.T) {
	r := varnaRig(t, nil)
	f, err := New(r.cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if f.tr == nil || http.RoundTripper(f.tr) == http.DefaultTransport {
		t.Error("fetcher uses the default transport")
	}
}
