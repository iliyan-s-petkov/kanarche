// Package geocode proxies address search to a Nominatim-compatible upstream.
//
// The proxy exists so the visitor's IP never reaches the upstream and so the
// upstream's usage policy (identifying User-Agent, at most one request per
// second, no autocomplete) is enforced in one place. Queries are personal data
// (home addresses): nothing in this package logs one, and no error it returns
// carries one.
package geocode

import (
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"airbg.org/internal/config"
	"airbg.org/internal/ratelimit"
)

// Query length bounds, in runes after trimming.
const (
	MinQueryRunes = 3
	MaxQueryRunes = 120
)

// maxResults is what the caller gets back. upstreamLimit asks for a few more
// rows so merging same-label rows still leaves up to maxResults; it is still
// one upstream request.
const (
	maxResults    = 5
	upstreamLimit = 8
)

// maxBodyBytes caps what is read from the upstream.
const maxBodyBytes = 1 << 20

var (
	// ErrBusy: the global upstream budget is spent, or the upstream itself
	// answered 429. The caller should tell the visitor to retry shortly.
	ErrBusy = errors.New("geocode: busy")
	// ErrTimeout: the upstream did not answer within the configured timeout.
	ErrTimeout = errors.New("geocode: upstream timeout")
	// ErrUpstream: the upstream failed or answered something unusable.
	ErrUpstream = errors.New("geocode: upstream failure")
)

// Result is one address match. BBox is [west, south, east, north], the order
// MapLibre's fitBounds takes; all zeros means the upstream gave none.
type Result struct {
	Label string     `json:"label"`
	Lat   float64    `json:"lat"`
	Lon   float64    `json:"lon"`
	BBox  [4]float64 `json:"bbox"`
}

// Clean trims q, replaces control characters with spaces, collapses runs of
// whitespace and enforces the length bounds. ok is false for a query that is
// too short, too long or not valid UTF-8.
func Clean(q string) (string, bool) {
	if !utf8.ValidString(q) {
		return "", false
	}
	q = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, q)
	q = strings.Join(strings.Fields(q), " ")
	if n := utf8.RuneCountInString(q); n < MinQueryRunes || n > MaxQueryRunes {
		return "", false
	}
	return q, true
}

// Service answers searches from a cache, then from the upstream within a
// global rate budget.
type Service struct {
	baseURL   string
	userAgent string
	client    *http.Client
	cache     *lru
	// bucket is a one-key token bucket: the upstream budget is global, not per
	// visitor.
	bucket *ratelimit.Limiter

	mu  sync.RWMutex
	now func() time.Time
}

// upstreamKey is the single bucket key.
const upstreamKey = "upstream"

func New(cfg config.Geocoder) *Service {
	// Burst is one request for the policy's 1 req/s, and scales with the rate
	// so a configured higher rate is not throttled by a burst of one.
	bucket := ratelimit.New(config.Bucket{
		PerSecond:     cfg.UpstreamPerSecond,
		Burst:         math.Max(1, cfg.UpstreamPerSecond),
		TTL:           time.Hour,
		EvictInterval: time.Hour,
	}, 1)
	s := &Service{
		baseURL:   strings.TrimSuffix(cfg.URL, "/"),
		userAgent: cfg.UserAgent,
		// No redirect following: the upstream URL is operator config and a
		// redirect would be an unreviewed second destination.
		client: &http.Client{
			Timeout:       cfg.RequestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		bucket: bucket,
		now:    time.Now,
	}
	s.cache = newLRU(cfg.CacheMaxEntries, cfg.CacheTTL, s.clock)
	return s
}

// SetClockForTesting drives the cache expiry and the token bucket from one fake clock.
func (s *Service) SetClockForTesting(now func() time.Time) {
	s.mu.Lock()
	s.now = now
	s.mu.Unlock()
	s.bucket.SetClockForTesting(now)
}

func (s *Service) clock() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.now()
}

// Search returns matches for q, which must already have passed Clean. lang is
// "bg" or "en".
func (s *Service) Search(ctx context.Context, q, lang string) ([]Result, error) {
	// The v2 prefix keeps entries cached in the old label shape from being served.
	key := "v2\x00" + lang + "\x00" + strings.ToLower(strings.Join(strings.Fields(q), " "))
	if res, ok := s.cache.get(key); ok {
		return res, nil
	}
	// A cache hit costs no token; only a real upstream call spends one.
	if ok, _ := s.bucket.Allow(upstreamKey); !ok {
		return nil, ErrBusy
	}
	res, err := s.fetch(ctx, q, lang)
	if err != nil {
		return nil, err
	}
	s.cache.put(key, res)
	return res, nil
}

// fetch performs the upstream call. Every error it returns is one of the
// package sentinels: net/http's own errors embed the request URL, and the URL
// carries the query.
func (s *Service) fetch(ctx context.Context, q, lang string) ([]Result, error) {
	v := url.Values{}
	v.Set("q", q)
	v.Set("format", "jsonv2")
	v.Set("countrycodes", "bg")
	v.Set("limit", strconv.Itoa(upstreamLimit))
	v.Set("addressdetails", "1")
	v.Set("accept-language", lang)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/search?"+v.Encode(), nil)
	if err != nil {
		return nil, ErrUpstream
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		var ne net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
			return nil, ErrTimeout
		}
		return nil, ErrUpstream
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, ErrBusy
	case resp.StatusCode != http.StatusOK:
		return nil, ErrUpstream
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, ErrUpstream
	}
	return reduce(body)
}

// nominatimRow is the part of a jsonv2 search row this proxy keeps. Nominatim
// sends coordinates as strings.
type nominatimRow struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	BoundingBox []string          `json:"boundingbox"`
	Address     map[string]string `json:"address"`
}

// firstOf returns the first non-empty address value among keys.
func firstOf(addr map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(addr[k]); v != "" {
			return v
		}
	}
	return ""
}

// buildLabel makes a short label: house number and road (or the place name),
// the smallest sub-locality, the city/town/village, then the municipality or
// county only when it is not already contained in the label. Postcode and
// country are dropped (search is Bulgaria-only). Without address parts it
// falls back to display_name.
func buildLabel(row nominatimRow) string {
	addr := row.Address
	head := firstOf(addr, "road", "pedestrian", "footway", "path", "residential")
	if head != "" {
		if hn := firstOf(addr, "house_number"); hn != "" {
			head += " " + hn
		}
	} else {
		head = strings.TrimSpace(row.Name)
	}
	if head == "" {
		return row.DisplayName
	}
	parts := []string{head}
	// add skips a value already present in, or containing, an earlier part.
	add := func(v string) {
		if v == "" {
			return
		}
		lv := strings.ToLower(v)
		for _, p := range parts {
			lp := strings.ToLower(p)
			if strings.Contains(lp, lv) || strings.Contains(lv, lp) {
				return
			}
		}
		parts = append(parts, v)
	}
	add(firstOf(addr, "suburb", "quarter", "neighbourhood", "city_district"))
	add(firstOf(addr, "city", "town", "village", "hamlet"))
	add(firstOf(addr, "municipality", "county"))
	return strings.Join(parts, ", ")
}

// mergeBBox widens a to cover b; an all-zero box means none.
func mergeBBox(a, b [4]float64) [4]float64 {
	if b == ([4]float64{}) {
		return a
	}
	if a == ([4]float64{}) {
		return b
	}
	return [4]float64{math.Min(a[0], b[0]), math.Min(a[1], b[1]), math.Max(a[2], b[2]), math.Max(a[3], b[3])}
}

// reduce keeps the first-seen order and merges rows whose label is equal
// (OSM splits one street into ways with different postcodes): bboxes are
// united, the point stays the first row's, a real OSM point on the street where
// the union centre could fall beside it.
func reduce(body []byte) ([]Result, error) {
	var rows []nominatimRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, ErrUpstream
	}
	out := make([]Result, 0, maxResults)
	seen := make(map[string]int, len(rows))
	for _, row := range rows {
		lat, errLat := strconv.ParseFloat(row.Lat, 64)
		lon, errLon := strconv.ParseFloat(row.Lon, 64)
		label := buildLabel(row)
		if label == "" || errLat != nil || errLon != nil ||
			math.Abs(lat) > 90 || math.Abs(lon) > 180 {
			continue
		}
		var box [4]float64
		// Nominatim's order is [south, north, west, east].
		if len(row.BoundingBox) == 4 {
			var b [4]float64
			valid := true
			for i, s := range row.BoundingBox {
				f, err := strconv.ParseFloat(s, 64)
				if err != nil {
					valid = false
					break
				}
				b[i] = f
			}
			if valid {
				box = [4]float64{b[2], b[0], b[3], b[1]}
			}
		}
		key := strings.ToLower(label)
		if i, ok := seen[key]; ok {
			out[i].BBox = mergeBBox(out[i].BBox, box)
			continue
		}
		if len(out) == maxResults {
			continue
		}
		seen[key] = len(out)
		out = append(out, Result{Label: label, Lat: lat, Lon: lon, BBox: box})
	}
	return out, nil
}

// lru is a small mutex-guarded LRU with a TTL; no dependency needed for this.
type lru struct {
	mu    sync.Mutex
	max   int
	ttl   time.Duration
	now   func() time.Time
	order *list.List // front = most recently used
	items map[string]*list.Element
}

type lruEntry struct {
	key     string
	res     []Result
	expires time.Time
}

func newLRU(max int, ttl time.Duration, now func() time.Time) *lru {
	return &lru{max: max, ttl: ttl, now: now, order: list.New(), items: make(map[string]*list.Element)}
}

func (c *lru) get(key string) ([]Result, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	e := el.Value.(*lruEntry)
	if !c.now().Before(e.expires) {
		c.order.Remove(el)
		delete(c.items, key)
		return nil, false
	}
	c.order.MoveToFront(el)
	return e.res, true
}

func (c *lru) put(key string, res []Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	expires := c.now().Add(c.ttl)
	if el, ok := c.items[key]; ok {
		e := el.Value.(*lruEntry)
		e.res, e.expires = res, expires
		c.order.MoveToFront(el)
		return
	}
	c.items[key] = c.order.PushFront(&lruEntry{key: key, res: res, expires: expires})
	for c.order.Len() > c.max {
		last := c.order.Back()
		c.order.Remove(last)
		delete(c.items, last.Value.(*lruEntry).key)
	}
}
