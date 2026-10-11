package rzi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"time"

	"kanarche.eu/internal/config"
)

// MaxPageBytes caps a discovery page.
const MaxPageBytes = 2 << 20

// maxRedirects bounds one request chain.
const maxRedirects = 5

// maxValidatorBytes bounds a stored ETag or Last-Modified value.
const maxValidatorBytes = 256

// Fetch errors are fixed strings: nothing from a page or a body is echoed.
var (
	ErrRequest      = errors.New("rzi: request failed")
	ErrRedirect     = errors.New("rzi: redirect refused")
	ErrStatus       = errors.New("rzi: unexpected HTTP status")
	ErrPageTooLarge = errors.New("rzi: discovery page over size cap")
	ErrFileTooLarge = errors.New("rzi: document over size cap")
	ErrContentType  = errors.New("rzi: unexpected Content-Type")
	ErrMagic        = errors.New("rzi: content does not match the expected type")
	ErrWrite        = errors.New("rzi: inbox write failed")
)

// FetchStatus says what a fetch did to the inbox.
type FetchStatus string

const (
	StatusWritten     FetchStatus = "written"
	StatusNotModified FetchStatus = "not_modified"
	// StatusUnchanged is a 200 whose bytes are already known: same sha as the
	// caller's last good, or already in the inbox.
	StatusUnchanged FetchStatus = "unchanged"
)

// Validators are what the caller remembers about the last fetch of a source.
type Validators struct {
	ETag         string
	LastModified string
	SHA256       string
}

// Result is one fetch. Path is set when Status is StatusWritten or when the
// file was already in the inbox.
type Result struct {
	Source       string
	Status       FetchStatus
	URL          string
	SHA256       string
	Path         string
	ETag         string
	LastModified string
}

// Options injects what the host controls. Transport is the only way out to
// the network, so the host run (or a test) decides where connections go.
type Options struct {
	Transport *http.Transport
	Now       func() time.Time
}

// Fetcher downloads the current-year document of each configured source.
type Fetcher struct {
	tr        *http.Transport
	now       func() time.Time
	userAgent string
	timeout   time.Duration
	maxBytes  int64
	inbox     string
	pages     map[string]*url.URL
}

// New checks every page URL against its source's host allow-list.
func New(cfg config.Sea, opt Options) (*Fetcher, error) {
	f := &Fetcher{
		tr:        opt.Transport,
		now:       opt.Now,
		userAgent: cfg.UserAgent,
		timeout:   cfg.RZI.RequestTimeout,
		maxBytes:  cfg.RZI.MaxFileBytes,
		inbox:     cfg.RZI.InboxDir,
		pages:     map[string]*url.URL{},
	}
	if f.tr == nil {
		f.tr = defaultTransport()
	}
	if f.now == nil {
		f.now = time.Now
	}
	for id, raw := range map[string]string{
		SourceVarna:   cfg.RZI.Varna.PageURL,
		SourceBurgas:  cfg.RZI.Burgas.PageURL,
		SourceDobrich: cfg.RZI.Dobrich.PageURL,
	} {
		spec, _ := specFor(id)
		u, err := url.Parse(raw)
		if err != nil || !spec.allows(u) {
			return nil, fmt.Errorf("%w: %s page_url", ErrHost, id)
		}
		f.pages[id] = u
	}
	return f, nil
}

// defaultTransport has no proxy and no shared state with http.DefaultTransport.
func defaultTransport() *http.Transport {
	return &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout: 10 * time.Second,
		MaxIdleConns:        2,
		IdleConnTimeout:     30 * time.Second,
	}
}

// Sources lists the source ids in a fixed order.
func Sources() []string {
	ids := make([]string, len(specs))
	for i, s := range specs {
		ids[i] = s.ID
	}
	return ids
}

// checkRedirect lets a request follow only https redirects to hosts on the
// source's own allow-list (I4).
func (s Spec) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > maxRedirects || !s.allows(req.URL) {
		return ErrRedirect
	}
	return nil
}

// Fetch reads the source page, finds the current-year document and stores it
// in the inbox as <sha256>.<ext> when it is new.
func (f *Fetcher) Fetch(ctx context.Context, id string, prior Validators) (Result, error) {
	spec, ok := specFor(id)
	if !ok {
		return Result{}, ErrSource
	}
	client := &http.Client{Transport: f.tr, Timeout: f.timeout, CheckRedirect: spec.checkRedirect}

	pageResp, err := f.get(ctx, client, f.pages[id], nil)
	if err != nil {
		return Result{}, err
	}
	defer pageResp.Body.Close()
	if pageResp.StatusCode != http.StatusOK {
		return Result{}, ErrStatus
	}
	page, err := io.ReadAll(io.LimitReader(pageResp.Body, MaxPageBytes+1))
	if err != nil {
		return Result{}, ErrRequest
	}
	if len(page) > MaxPageBytes {
		return Result{}, ErrPageTooLarge
	}
	target, err := discover(page, pageResp.Request.URL, spec, fmt.Sprint(f.now().UTC().Year()))
	if err != nil {
		return Result{}, err
	}

	hdr := http.Header{}
	if prior.ETag != "" {
		hdr.Set("If-None-Match", prior.ETag)
	}
	if prior.LastModified != "" {
		hdr.Set("If-Modified-Since", prior.LastModified)
	}
	conditional := len(hdr) > 0
	resp, err := f.get(ctx, client, target, hdr)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	res := Result{Source: id, URL: target.String(), SHA256: prior.SHA256,
		ETag: prior.ETag, LastModified: prior.LastModified}
	if resp.StatusCode == http.StatusNotModified && conditional {
		res.Status = StatusNotModified
		return res, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, ErrStatus
	}
	res.ETag, res.LastModified = headerValue(resp.Header.Get("ETag")), headerValue(resp.Header.Get("Last-Modified"))

	mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !slices.Contains(spec.Kind.ContentTypes, mt) {
		return Result{}, ErrContentType
	}
	if resp.ContentLength > f.maxBytes {
		return Result{}, ErrFileTooLarge
	}
	// One byte past the cap tells a too-large body from an exact fit (I5).
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes+1))
	if err != nil {
		return Result{}, ErrRequest
	}
	if int64(len(body)) > f.maxBytes {
		return Result{}, ErrFileTooLarge
	}
	if err := checkMagic(spec.Kind, body); err != nil {
		return Result{}, err
	}

	digest := sha256.Sum256(body)
	res.SHA256 = hex.EncodeToString(digest[:])
	if res.SHA256 == prior.SHA256 {
		res.Status = StatusUnchanged
		return res, nil
	}
	res.Path = filepath.Join(f.inbox, res.SHA256+"."+spec.Kind.Ext)
	if _, err := os.Stat(res.Path); err == nil {
		res.Status = StatusUnchanged
		return res, nil
	}
	if err := f.write(res.Path, body); err != nil {
		return Result{}, err
	}
	res.Status = StatusWritten
	return res, nil
}

// get issues one GET. Redirect refusals and the context error keep their
// identity; every other failure is the fixed ErrRequest.
func (f *Fetcher) get(ctx context.Context, c *http.Client, u *url.URL, hdr http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrRequest
	}
	for k, v := range hdr {
		req.Header[k] = v
	}
	req.Header.Set("User-Agent", f.userAgent)
	resp, err := c.Do(req)
	switch {
	case err == nil:
		return resp, nil
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case errors.Is(err, ErrRedirect):
		return nil, ErrRedirect
	}
	return nil, ErrRequest
}

// checkMagic is the content check that does not trust the server (I6).
func checkMagic(k Kind, body []byte) error {
	if !bytes.HasPrefix(body, k.Magic) {
		return ErrMagic
	}
	return nil
}

// headerValue keeps a header value only if it is short printable ASCII.
func headerValue(v string) string {
	if len(v) > maxValidatorBytes {
		return ""
	}
	for i := 0; i < len(v); i++ {
		if v[i] < 0x20 || v[i] > 0x7e {
			return ""
		}
	}
	return v
}

// write puts the bytes in a temp file inside the inbox, then renames it, so a
// reader never sees a partial file. Mode 0640.
func (f *Fetcher) write(path string, body []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("%w: %w", ErrWrite, err)
	}
	tmp, err := os.CreateTemp(dir, ".fetch-*")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWrite, err)
	}
	name := tmp.Name()
	fail := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(name)
		return fmt.Errorf("%w: %w", ErrWrite, err)
	}
	if err := tmp.Chmod(0o640); err != nil {
		return fail(err)
	}
	if _, err := tmp.Write(body); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		return fail(err)
	}
	if err := os.Rename(name, path); err != nil {
		return fail(err)
	}
	return nil
}
