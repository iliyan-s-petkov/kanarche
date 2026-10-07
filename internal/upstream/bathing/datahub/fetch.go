package datahub

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"airbg.org/internal/xlsx"
)

var (
	ErrBadPin         = errors.New("datahub: sha256 pin must be 64 lowercase hex characters")
	ErrBadLimits      = errors.New("datahub: timeout and max bytes must be positive")
	ErrNotHTTPS       = errors.New("datahub: url must be https")
	ErrHostNotAllowed = errors.New("datahub: host not in allowed_hosts")
	ErrNotXLSX        = errors.New("datahub: url path must end in .xlsx")
	ErrRedirect       = errors.New("datahub: redirect refused")
	ErrStatus         = errors.New("datahub: status is not 200")
	ErrContentType    = errors.New("datahub: unexpected content type")
	ErrTooLarge       = errors.New("datahub: download over size cap")
	ErrSize           = errors.New("datahub: size differs from the pin")
	ErrSHA256         = errors.New("datahub: sha256 differs from the pin")
	ErrNotZip         = xlsx.ErrNotZip
	ErrOLE            = xlsx.ErrOLE
)

var (
	pinPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	zipMagic   = []byte("PK\x03\x04")
	oleMagic   = []byte{0xD0, 0xCF, 0x11, 0xE0}
	// A real error page is HTML or JSON, so only these types may carry the file.
	okTypes = []string{
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/octet-stream",
	}
)

const maxRedirects = 10

// FetchConfig pins one download. Task 5 maps config.SeaDatahub onto it.
type FetchConfig struct {
	URL          string
	SHA256       string        // expected digest, lowercase hex
	Size         int64         // expected byte count, 0 skips the check
	AllowedHosts []string      // bare hostnames
	Timeout      time.Duration // whole request, body included
	MaxBytes     int64         // streaming cap
	Client       *http.Client  // optional, for the transport only (tests inject TLS trust)
}

// Meta describes a verified download. Path is empty on every error.
type Meta struct {
	Path         string
	SHA256       string
	Size         int64
	ETag         string
	LastModified string
}

// Fetch downloads cfg.URL into dir and returns the file only after the size
// and sha256 pins match. Until then the bytes sit in a 0600 temp file that is
// removed on any failure, so no caller can parse an unverified body.
func Fetch(ctx context.Context, cfg FetchConfig, dir string) (Meta, error) {
	u, err := checkURL(cfg)
	if err != nil {
		return Meta{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Meta{}, err
	}
	resp, err := client(cfg).Do(req)
	if err != nil {
		return Meta{}, fmt.Errorf("datahub: download: %w", err)
	}
	defer resp.Body.Close()
	if err := checkResponse(resp, cfg); err != nil {
		return Meta{}, err
	}
	tmp, err := os.CreateTemp(dir, ".download-*.part") // mode 0600
	if err != nil {
		return Meta{}, err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	sum, n, err := copyChecked(tmp, resp.Body, cfg)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return Meta{}, err
	}
	if cfg.Size > 0 && n != cfg.Size {
		return Meta{}, fmt.Errorf("%w: got %d bytes, pinned %d", ErrSize, n, cfg.Size)
	}
	if sum != cfg.SHA256 {
		return Meta{}, fmt.Errorf("%w: observed %s, pinned %s", ErrSHA256, sum, cfg.SHA256)
	}
	final := filepath.Join(dir, path.Base(u.Path))
	if err := os.Rename(tmp.Name(), final); err != nil {
		return Meta{}, err
	}
	return Meta{
		Path: final, SHA256: sum, Size: n,
		ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified"),
	}, nil
}

// checkURL runs every check that needs no network.
func checkURL(cfg FetchConfig) (*url.URL, error) {
	if !pinPattern.MatchString(cfg.SHA256) {
		return nil, ErrBadPin
	}
	if cfg.Timeout <= 0 || cfg.MaxBytes <= 0 {
		return nil, ErrBadLimits
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("datahub: url: %w", err)
	}
	if u.Scheme != "https" {
		return nil, ErrNotHTTPS
	}
	if !slices.Contains(cfg.AllowedHosts, u.Hostname()) {
		return nil, fmt.Errorf("%w: %q", ErrHostNotAllowed, u.Hostname())
	}
	// Lower-cased, so .XLSM cannot slip past either way.
	if !strings.HasSuffix(strings.ToLower(u.Path), ".xlsx") {
		return nil, ErrNotXLSX
	}
	return u, nil
}

// client copies the caller's client so the redirect policy and timeout always apply.
func client(cfg FetchConfig) *http.Client {
	c := http.Client{}
	if cfg.Client != nil {
		c = *cfg.Client
	}
	c.Timeout = cfg.Timeout
	c.CheckRedirect = sameOriginOnly
	return &c
}

// sameOriginOnly follows a redirect only to the same scheme and host:port.
func sameOriginOnly(req *http.Request, via []*http.Request) error {
	first := via[0].URL
	if req.URL.Scheme != first.Scheme || req.URL.Host != first.Host {
		return fmt.Errorf("%w: %s://%s to %s://%s", ErrRedirect, first.Scheme, first.Host, req.URL.Scheme, req.URL.Host)
	}
	if len(via) >= maxRedirects {
		return fmt.Errorf("%w: more than %d redirects", ErrRedirect, maxRedirects)
	}
	return nil
}

func checkResponse(resp *http.Response, cfg FetchConfig) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrStatus, resp.StatusCode)
	}
	mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !slices.Contains(okTypes, mt) {
		return fmt.Errorf("%w: %q", ErrContentType, resp.Header.Get("Content-Type"))
	}
	if resp.ContentLength > cfg.MaxBytes {
		return fmt.Errorf("%w: declared %d, cap %d", ErrTooLarge, resp.ContentLength, cfg.MaxBytes)
	}
	return nil
}

// copyChecked streams body to w, hashing as it goes. It reads at most
// MaxBytes+1 bytes, so an endless body costs one byte over the cap.
func copyChecked(w io.Writer, body io.Reader, cfg FetchConfig) (string, int64, error) {
	body = io.LimitReader(body, cfg.MaxBytes+1)
	magic := make([]byte, 4)
	if _, err := io.ReadFull(body, magic); err != nil {
		return "", 0, fmt.Errorf("%w: body shorter than a zip header", ErrNotZip)
	}
	if bytes.Equal(magic, oleMagic) {
		return "", 0, ErrOLE
	}
	if !bytes.Equal(magic, zipMagic) {
		return "", 0, ErrNotZip
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), io.MultiReader(bytes.NewReader(magic), body))
	if err != nil {
		return "", 0, fmt.Errorf("datahub: download: %w", err)
	}
	if n > cfg.MaxBytes {
		return "", 0, fmt.Errorf("%w: over %d bytes", ErrTooLarge, cfg.MaxBytes)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
