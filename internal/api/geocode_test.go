package api_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"airbg.org/internal/api"
	"airbg.org/internal/config"
	"airbg.org/internal/geocode"
)

const upstreamRows = `[{"display_name":"бул. Витоша 1, София","lat":"42.6931","lon":"23.3201",
 "boundingbox":["42.6930","42.6932","23.3200","23.3202"],"osm_id":123,"extra":"dropped"}]`

type geoUp struct {
	*httptest.Server
	calls atomic.Int32
}

func newGeoUp(t *testing.T, status int, body string, delay time.Duration) *geoUp {
	t.Helper()
	u := &geoUp{}
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.calls.Add(1)
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(u.Close)
	return u
}

// geoDeps wires a real geocode.Service against the stub. upstreamRate is the
// global upstream budget per second.
func geoDeps(t *testing.T, up *geoUp, tweak func(*config.Geocoder)) api.Deps {
	t.Helper()
	d := deps(t, fixture(t))
	gc := config.Geocoder{
		URL:               up.URL,
		UserAgent:         "test-agent",
		RequestTimeout:    time.Second,
		CacheTTL:          time.Hour,
		CacheMaxEntries:   50,
		UpstreamPerSecond: 1000,
	}
	if tweak != nil {
		tweak(&gc)
	}
	d.Geocoder = geocode.New(gc)
	d.GeocodeLimiter = api.NewGeocodeLimiter(d.Config)
	return d
}

func geoGet(q, lang, ip string) *http.Request {
	v := url.Values{"q": {q}}
	if lang != "" {
		v.Set("lang", lang)
	}
	return get("/api/v1/geocode?"+v.Encode(), ip)
}

func TestGeocodeReturnsTheReducedShape(t *testing.T) {
	up := newGeoUp(t, 200, upstreamRows, 0)
	rec := serve(t, geoDeps(t, up, nil), geoGet("Витоша 1", "bg", "203.0.113.10"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.HasPrefix(cc, "private") {
		t.Errorf("Cache-Control = %q, want private", cc)
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not a JSON array: %v (%s)", err, rec.Body)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows", len(got))
	}
	if len(got[0]) != 4 {
		t.Errorf("row keys = %v, want exactly label, lat, lon, bbox", got[0])
	}
	if got[0]["label"] != "бул. Витоша 1, София" {
		t.Errorf("label = %v", got[0]["label"])
	}
	if bb, _ := got[0]["bbox"].([]any); len(bb) != 4 || bb[0] != 23.32 {
		t.Errorf("bbox = %v, want [west south east north]", got[0]["bbox"])
	}
}

func TestGeocodeEmptyResultIsAnEmptyArray(t *testing.T) {
	up := newGeoUp(t, 200, `[]`, 0)
	rec := serve(t, geoDeps(t, up, nil), geoGet("nowhere at all", "en", "203.0.113.11"))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("status %d body %q, want 200 and []", rec.Code, rec.Body)
	}
}

func TestGeocodeRejectsBadQueries(t *testing.T) {
	up := newGeoUp(t, 200, upstreamRows, 0)
	d := geoDeps(t, up, nil)
	// One client address per case: the per-client bucket would otherwise
	// answer 429 to the later ones.
	n := 0
	for name, target := range map[string]string{
		"missing":      "/api/v1/geocode",
		"too short":    "/api/v1/geocode?q=ab",
		"short padded": "/api/v1/geocode?q=%20%20ab%20%20",
		"too long":     "/api/v1/geocode?q=" + strings.Repeat("a", 121),
		"only control": "/api/v1/geocode?q=%00%01%02%03",
		"bad lang":     "/api/v1/geocode?q=Vitosha&lang=fr",
		"bad utf8":     "/api/v1/geocode?q=%ff%fe%fd%fc",
	} {
		n++
		ip := "198.51.100." + strconv.Itoa(n)
		t.Run(name, func(t *testing.T) {
			rec := serve(t, d, get(target, ip))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
	if n := up.calls.Load(); n != 0 {
		t.Errorf("upstream calls = %d, want 0 for invalid queries", n)
	}
}

func TestGeocodeDefaultsToEnglish(t *testing.T) {
	var lang atomic.Value
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang.Store(r.URL.Query().Get("accept-language"))
		_, _ = w.Write([]byte(`[]`))
	}))
	defer up.Close()
	d := geoDeps(t, &geoUp{Server: up}, nil)
	serve(t, d, geoGet("Vitosha", "", "203.0.113.13"))
	if lang.Load() != "en" {
		t.Errorf("accept-language = %v, want en", lang.Load())
	}
}

func TestGeocodeCacheHitAvoidsTheSecondUpstreamCall(t *testing.T) {
	up := newGeoUp(t, 200, upstreamRows, 0)
	h := router(t, geoDeps(t, up, nil))
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, geoGet("Витоша 1", "bg", "203.0.113.14"))
		if rec.Code != 200 {
			t.Fatalf("status = %d", rec.Code)
		}
	}
	if n := up.calls.Load(); n != 1 {
		t.Errorf("upstream calls = %d, want 1", n)
	}
}

func TestGeocodeAnswers503BusyWhenTheUpstreamBudgetIsSpent(t *testing.T) {
	up := newGeoUp(t, 200, upstreamRows, 0)
	h := router(t, geoDeps(t, up, func(c *config.Geocoder) { c.UpstreamPerSecond = 0.001 }))

	first := httptest.NewRecorder()
	h.ServeHTTP(first, geoGet("first query", "en", "203.0.113.15"))
	if first.Code != 200 {
		t.Fatalf("first status = %d", first.Code)
	}
	second := httptest.NewRecorder()
	h.ServeHTTP(second, geoGet("second query", "en", "203.0.113.15"))
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("second status = %d, want 503", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Error("503 carries no Retry-After")
	}
	var body struct{ Error string }
	_ = json.Unmarshal(second.Body.Bytes(), &body)
	if body.Error != "busy" {
		t.Errorf("error code = %q, want busy (the page keys its message on it)", body.Error)
	}
	if n := up.calls.Load(); n != 1 {
		t.Errorf("upstream calls = %d, want 1", n)
	}
}

func TestGeocodeIsRateLimitedPerClient(t *testing.T) {
	up := newGeoUp(t, 200, upstreamRows, 0)
	d := geoDeps(t, up, nil)
	h := router(t, d)
	burst := int(d.Config.RateLimit.Geocode.Burst)

	codes := func(ip string, n int) (last int) {
		for i := 0; i < n; i++ {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, geoGet("Витоша 1", "bg", ip))
			last = rec.Code
		}
		return last
	}
	if got := codes("203.0.113.20", burst); got != 200 {
		t.Fatalf("within the burst: status %d, want 200", got)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, geoGet("Витоша 1", "bg", "203.0.113.20"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("past the burst: status %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 carries no Retry-After")
	}
	if got := codes("203.0.113.21", 1); got != 200 {
		t.Errorf("another client: status %d, want 200 (buckets are per client)", got)
	}
}

func TestGeocodeUpstreamTimeoutIs504(t *testing.T) {
	up := newGeoUp(t, 200, upstreamRows, 2*time.Second)
	d := geoDeps(t, up, func(c *config.Geocoder) { c.RequestTimeout = 100 * time.Millisecond })
	rec := serve(t, d, geoGet("slow query", "en", "203.0.113.30"))
	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("status = %d, want 504", rec.Code)
	}
}

func TestGeocodeUpstreamFailureIs502(t *testing.T) {
	up := newGeoUp(t, 500, "oops", 0)
	rec := serve(t, geoDeps(t, up, nil), geoGet("failing query", "en", "203.0.113.31"))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "oops") {
		t.Error("upstream body leaked into the response")
	}
}

func TestGeocodeUnconfiguredFailsClosed(t *testing.T) {
	rec := serve(t, deps(t, fixture(t)), geoGet("Vitosha", "en", "203.0.113.32"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 with no geocoder wired", rec.Code)
	}
}

// TestGeocodeNeverLogsTheQuery: a typed address is personal data, so no code
// path may write it to the log, successful or not.
func TestGeocodeNeverLogsTheQuery(t *testing.T) {
	const distinctive = "Zzqxj-Unlikely-Street-77"

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ok := newGeoUp(t, 200, upstreamRows, 0)
	bad := newGeoUp(t, 500, "oops", 0)
	slow := newGeoUp(t, 200, upstreamRows, 2*time.Second)
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()

	cases := map[string]api.Deps{
		"success":     geoDeps(t, ok, nil),
		"upstream500": geoDeps(t, bad, nil),
		"timeout":     geoDeps(t, slow, func(c *config.Geocoder) { c.RequestTimeout = 50 * time.Millisecond }),
		"unreachable": geoDeps(t, ok, func(c *config.Geocoder) { c.URL = deadURL }),
		"busy":        geoDeps(t, ok, func(c *config.Geocoder) { c.UpstreamPerSecond = 0.001 }),
	}
	for name, d := range cases {
		h := router(t, d)
		// Two requests so the "busy" case actually reaches an empty bucket.
		for i := 0; i < 2; i++ {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, geoGet(distinctive+" "+name+string(rune('a'+i)), "en", "203.0.113.40"))
		}
	}
	if strings.Contains(logs.String(), "Zzqxj") || strings.Contains(strings.ToLower(logs.String()), "unlikely-street") {
		t.Errorf("the query text reached the log:\n%s", logs.String())
	}
}
