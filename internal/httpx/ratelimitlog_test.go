package httpx_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/httpx"
	"kanarche.eu/internal/ratelimit"
)

const secretUA = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html) SecretTail"

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func limitedHandler(t *testing.T) http.Handler {
	t.Helper()
	l := ratelimit.New(testBucket(1, 1, time.Hour), testShardCount)
	pages := ratelimit.New(testBucket(1, 1, time.Hour), testShardCount)
	chain := httpx.Chain{Resolver: resolver(t), Limiter: l, PageLimiter: pages, MaxBodyBytes: 1024}
	return chain.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
}

func hit(h http.Handler, path, ua string) {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.RemoteAddr = "203.0.113.77:5555"
	r.Header.Set("X-Forwarded-For", "198.51.100.9")
	r.Header.Set("Cookie", "sid=cookieval")
	r.Header.Set("User-Agent", ua)
	h.ServeHTTP(httptest.NewRecorder(), r)
}

func TestRateLimitedLogHasGroupAndCrawlerAndNoClientData(t *testing.T) {
	buf := captureLog(t)
	httpx.SetRateLimitLogClockForTesting(time.Now)
	h := limitedHandler(t)

	path := "/api/v1/areas/secretslug?q=secretquery"
	hit(h, path, secretUA)
	hit(h, path, secretUA)

	out := buf.String()
	for _, want := range []string{"rate limited", "group=api", "crawler=googlebot", "retry_after="} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q missing %q", out, want)
		}
	}
	for _, bad := range []string{"203.0.113.77", "198.51.100.9", "SecretTail", "Mozilla", "secretslug", "secretquery", "/api/", "cookieval"} {
		if strings.Contains(out, bad) {
			t.Errorf("log %q leaks %q", out, bad)
		}
	}
}

func TestRateLimitedLogGroupPagesAndNonCrawler(t *testing.T) {
	buf := captureLog(t)
	httpx.SetRateLimitLogClockForTesting(time.Now)
	h := limitedHandler(t)
	hit(h, "/", "curl/8")
	hit(h, "/", "curl/8")
	out := buf.String()
	if !strings.Contains(out, "group=pages") || !strings.Contains(out, "crawler=none") {
		t.Errorf("log %q want group=pages crawler=none", out)
	}
}

func TestRateLimitedLogIsSampledPerGroupAndCrawler(t *testing.T) {
	buf := captureLog(t)
	now := time.Unix(1000, 0)
	httpx.SetRateLimitLogClockForTesting(func() time.Time { return now })
	h := limitedHandler(t)

	for i := 0; i < 6; i++ { // 1 allowed, 5 refused
		hit(h, "/api/x", secretUA)
	}
	if n := strings.Count(buf.String(), "rate limited"); n != 1 {
		t.Fatalf("burst logged %d lines, want 1", n)
	}
	now = now.Add(11 * time.Second)
	hit(h, "/api/x", secretUA)
	out := buf.String()
	if n := strings.Count(out, "rate limited"); n != 2 || !strings.Contains(out, "suppressed=4") {
		t.Errorf("after window: %q want 2 lines and suppressed=4", out)
	}
}
