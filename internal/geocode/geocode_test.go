package geocode_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/geocode"
)

const nominatimBody = `[
 {"display_name":"бул. Витоша 1, Център, София, България","lat":"42.6931","lon":"23.3201",
  "boundingbox":["42.6930","42.6932","23.3200","23.3202"]},
 {"display_name":"Витоша, София","lat":"42.5","lon":"23.3","boundingbox":["42.4","42.6","23.2","23.4"]}
]`

// stub is an upstream that records what it was asked and answers body.
type stub struct {
	*httptest.Server
	calls atomic.Int32
	last  atomic.Pointer[http.Request]
}

func newStub(t *testing.T, status int, body string) *stub {
	t.Helper()
	s := &stub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.calls.Add(1)
		s.last.Store(r)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

func cfgFor(url string) config.Geocoder {
	return config.Geocoder{
		URL:               url,
		UserAgent:         "kanarche.eu collector (+https://kanarche.eu)",
		RequestTimeout:    time.Second,
		CacheTTL:          24 * time.Hour,
		CacheMaxEntries:   100,
		UpstreamPerSecond: 1000,
	}
}

func TestSearchSendsTheNominatimPolicyParameters(t *testing.T) {
	up := newStub(t, 200, nominatimBody)
	svc := geocode.New(cfgFor(up.URL))

	if _, err := svc.Search(t.Context(), "Витоша 1", "bg"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	r := up.last.Load()
	q := r.URL.Query()
	for k, want := range map[string]string{
		"q": "Витоша 1", "countrycodes": "bg", "limit": "8", "format": "jsonv2", "accept-language": "bg",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("upstream %s = %q, want %q", k, got, want)
		}
	}
	if r.URL.Path != "/search" {
		t.Errorf("upstream path = %q, want /search", r.URL.Path)
	}
	if got := r.Header.Get("User-Agent"); got != "kanarche.eu collector (+https://kanarche.eu)" {
		t.Errorf("User-Agent = %q, want the configured identifying one", got)
	}
}

func TestSearchPassesTheLanguageThrough(t *testing.T) {
	up := newStub(t, 200, nominatimBody)
	svc := geocode.New(cfgFor(up.URL))
	if _, err := svc.Search(t.Context(), "Vitosha", "en"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got := up.last.Load().URL.Query().Get("accept-language"); got != "en" {
		t.Errorf("accept-language = %q, want en", got)
	}
}

func TestSearchReducesTheResponse(t *testing.T) {
	up := newStub(t, 200, nominatimBody)
	res, err := geocode.New(cfgFor(up.URL)).Search(t.Context(), "Витоша", "bg")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("got %d results, want 2", len(res))
	}
	r := res[0]
	if r.Label != "бул. Витоша 1, Център, София, България" || r.Lat != 42.6931 || r.Lon != 23.3201 {
		t.Errorf("result = %+v", r)
	}
	// bbox is [west, south, east, north]; Nominatim sends [south, north, west, east].
	if want := [4]float64{23.32, 42.693, 23.3202, 42.6932}; r.BBox != want {
		t.Errorf("bbox = %v, want %v", r.BBox, want)
	}
}

func TestSearchDropsUnusableRows(t *testing.T) {
	body := `[{"display_name":"","lat":"1","lon":"2","boundingbox":["1","1","2","2"]},
	 {"display_name":"bad lat","lat":"abc","lon":"2","boundingbox":["1","1","2","2"]},
	 {"display_name":"out of range","lat":"99","lon":"2","boundingbox":["1","1","2","2"]},
	 {"display_name":"no bbox","lat":"42.1","lon":"23.1"},
	 {"display_name":"ok","lat":"42.1","lon":"23.1","boundingbox":["42","42.2","23","23.2"]}]`
	up := newStub(t, 200, body)
	res, err := geocode.New(cfgFor(up.URL)).Search(t.Context(), "anything", "en")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// A row without a usable bbox still has a point, so only the three
	// unusable ones go.
	if len(res) != 2 || res[0].Label != "no bbox" || res[1].Label != "ok" {
		t.Errorf("results = %+v, want [no bbox, ok]", res)
	}
	if res[0].BBox != [4]float64{} {
		t.Errorf("no-bbox row carries bbox %v, want zero", res[0].BBox)
	}
}

func TestSearchCapsResultsAtFive(t *testing.T) {
	var rows []string
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		rows = append(rows, `{"display_name":"`+n+`","lat":"42","lon":"23","boundingbox":["42","42","23","23"]}`)
	}
	up := newStub(t, 200, "["+strings.Join(rows, ",")+"]")
	res, err := geocode.New(cfgFor(up.URL)).Search(t.Context(), "many", "en")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 5 {
		t.Errorf("got %d results, want 5", len(res))
	}
}

func TestCacheHitAvoidsASecondUpstreamCall(t *testing.T) {
	up := newStub(t, 200, nominatimBody)
	svc := geocode.New(cfgFor(up.URL))
	for _, q := range []string{"Витоша", "Витоша", "  витоша  "} {
		if _, err := svc.Search(t.Context(), q, "bg"); err != nil {
			t.Fatalf("Search(%q): %v", q, err)
		}
	}
	if n := up.calls.Load(); n != 1 {
		t.Errorf("upstream calls = %d, want 1 (identical and normalised queries share an entry)", n)
	}
}

func TestCacheIsKeyedByLanguage(t *testing.T) {
	up := newStub(t, 200, nominatimBody)
	svc := geocode.New(cfgFor(up.URL))
	_, _ = svc.Search(t.Context(), "Vitosha", "bg")
	_, _ = svc.Search(t.Context(), "Vitosha", "en")
	if n := up.calls.Load(); n != 2 {
		t.Errorf("upstream calls = %d, want 2 (labels differ per language)", n)
	}
}

func TestCacheEntriesExpireAfterTheTTL(t *testing.T) {
	up := newStub(t, 200, nominatimBody)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	svc := geocode.New(cfgFor(up.URL))
	svc.SetClockForTesting(func() time.Time { return now })

	_, _ = svc.Search(t.Context(), "Витоша", "bg")
	now = now.Add(23 * time.Hour)
	_, _ = svc.Search(t.Context(), "Витоша", "bg")
	if n := up.calls.Load(); n != 1 {
		t.Fatalf("calls after 23h = %d, want 1", n)
	}
	now = now.Add(2 * time.Hour)
	_, _ = svc.Search(t.Context(), "Витоша", "bg")
	if n := up.calls.Load(); n != 2 {
		t.Errorf("calls after 25h = %d, want 2", n)
	}
}

func TestCacheEvictsTheLeastRecentlyUsedAtCapacity(t *testing.T) {
	up := newStub(t, 200, nominatimBody)
	cfg := cfgFor(up.URL)
	cfg.CacheMaxEntries = 2
	svc := geocode.New(cfg)

	for _, q := range []string{"aaa", "bbb", "aaa", "ccc"} { // bbb is now the oldest-used
		if _, err := svc.Search(t.Context(), q, "en"); err != nil {
			t.Fatalf("Search(%q): %v", q, err)
		}
	}
	before := up.calls.Load() // aaa, bbb, ccc = 3 upstream calls
	_, _ = svc.Search(t.Context(), "aaa", "en")
	_, _ = svc.Search(t.Context(), "ccc", "en")
	if up.calls.Load() != before {
		t.Errorf("aaa and ccc should still be cached")
	}
	_, _ = svc.Search(t.Context(), "bbb", "en")
	if up.calls.Load() != before+1 {
		t.Errorf("bbb should have been evicted and fetched again")
	}
}

func TestTokenBucketAnswersBusyWithoutCallingUpstream(t *testing.T) {
	up := newStub(t, 200, nominatimBody)
	cfg := cfgFor(up.URL)
	cfg.UpstreamPerSecond = 1
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	svc := geocode.New(cfg)
	svc.SetClockForTesting(func() time.Time { return now })

	if _, err := svc.Search(t.Context(), "first one", "en"); err != nil {
		t.Fatalf("first Search: %v", err)
	}
	_, err := svc.Search(t.Context(), "second one", "en")
	if !errors.Is(err, geocode.ErrBusy) {
		t.Fatalf("second Search err = %v, want ErrBusy", err)
	}
	if n := up.calls.Load(); n != 1 {
		t.Errorf("upstream calls = %d, want 1: the refused query must not reach it", n)
	}
	// A cached answer costs no token, so it is served even while the bucket is empty.
	if _, err := svc.Search(t.Context(), "first one", "en"); err != nil {
		t.Errorf("cached Search while bucket empty: %v", err)
	}
	now = now.Add(1100 * time.Millisecond)
	if _, err := svc.Search(t.Context(), "second one", "en"); err != nil {
		t.Errorf("Search after refill: %v", err)
	}
}

func TestUpstreamTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer slow.Close()
	cfg := cfgFor(slow.URL)
	cfg.RequestTimeout = 100 * time.Millisecond

	start := time.Now()
	_, err := geocode.New(cfg).Search(t.Context(), "timeout-q", "en")
	if !errors.Is(err, geocode.ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if time.Since(start) > time.Second {
		t.Errorf("took %v; the configured timeout was not applied", time.Since(start))
	}
}

func TestUpstreamFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"server error", 500, "oops", geocode.ErrUpstream},
		{"upstream throttling is busy", 429, "slow down", geocode.ErrBusy},
		{"not json", 200, "<html>", geocode.ErrUpstream},
	} {
		t.Run(tc.name, func(t *testing.T) {
			up := newStub(t, tc.status, tc.body)
			_, err := geocode.New(cfgFor(up.URL)).Search(t.Context(), "failing-q", "en")
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// Go's own network errors embed the full request URL, which carries the query.
func TestErrorsDoNotCarryTheQuery(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()

	_, err := geocode.New(cfgFor(url)).Search(t.Context(), "distinctive-home-address", "en")
	if err == nil {
		t.Fatal("want an error from a closed upstream")
	}
	if strings.Contains(err.Error(), "distinctive-home-address") {
		t.Errorf("error text carries the query: %q", err.Error())
	}
}

func TestErrorsAreNotCached(t *testing.T) {
	up := newStub(t, 500, "oops")
	svc := geocode.New(cfgFor(up.URL))
	_, _ = svc.Search(t.Context(), "failing-q", "en")
	_, _ = svc.Search(t.Context(), "failing-q", "en")
	if n := up.calls.Load(); n != 2 {
		t.Errorf("calls = %d, want 2: a failure must not be remembered for 24h", n)
	}
}

func TestClean(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"  Витоша 1  ", "Витоша 1", true},
		{"ab", "", false},
		{"   ab   ", "", false},
		{"abc", "abc", true},
		{"ab\x07", "", false},
		{"Sofia\x00 Vitosha\n1", "Sofia Vitosha 1", true},
		{strings.Repeat("я", 120), strings.Repeat("я", 120), true},
		{strings.Repeat("я", 121), "", false},
		{"", "", false},
		{"\xff\xfe\xfd\xfc", "", false},
	} {
		got, ok := geocode.Clean(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("Clean(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

// A redirect would be a second, unreviewed destination: it is not followed.
func TestUpstreamRedirectIsNotFollowed(t *testing.T) {
	target := newStub(t, 200, nominatimBody)
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/search", http.StatusFound)
	}))
	defer redir.Close()

	_, err := geocode.New(cfgFor(redir.URL)).Search(t.Context(), "redirect-q", "en")
	if !errors.Is(err, geocode.ErrUpstream) {
		t.Errorf("err = %v, want ErrUpstream", err)
	}
	if n := target.calls.Load(); n != 0 {
		t.Errorf("redirect target was called %d times, want 0", n)
	}
}
