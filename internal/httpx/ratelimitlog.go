package httpx

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Known crawlers by lowercase User-Agent substring. Only the short name is
// ever logged, never the User-Agent itself.
var crawlerNames = []string{
	"googlebot", "bingbot", "yandex", "duckduckbot", "applebot", "baiduspider",
	"gptbot", "claudebot", "ccbot", "petalbot", "semrushbot", "ahrefsbot",
}

func crawlerName(userAgent string) string {
	ua := strings.ToLower(userAgent)
	for _, n := range crawlerNames {
		if strings.Contains(ua, n) {
			return n
		}
	}
	return "none"
}

const rateLimitLogEvery = 10 * time.Second

// Sampling: at most one line per group+crawler per 10s, carrying the count of
// refusals suppressed since the previous line. Keys are bounded by the fixed
// group and crawler lists.
type rateLimitLogGate struct {
	mu   sync.Mutex
	now  func() time.Time
	last map[string]time.Time
	skip map[string]int
}

var rateLimitGate = newRateLimitLogGate(time.Now)

func newRateLimitLogGate(now func() time.Time) *rateLimitLogGate {
	return &rateLimitLogGate{now: now, last: map[string]time.Time{}, skip: map[string]int{}}
}

func (g *rateLimitLogGate) admit(key string) (bool, int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	t := g.now()
	if prev, ok := g.last[key]; ok && t.Sub(prev) < rateLimitLogEvery {
		g.skip[key]++
		return false, 0
	}
	g.last[key] = t
	n := g.skip[key]
	g.skip[key] = 0
	return true, n
}

// SetRateLimitLogClockForTesting swaps the sampling clock and clears its state.
func SetRateLimitLogClockForTesting(now func() time.Time) {
	rateLimitGate = newRateLimitLogGate(now)
}

// logRateLimited logs only the group, crawler name, Retry-After and the
// suppressed count. No IP, forwarded header, User-Agent, path, query or cookie.
func logRateLimited(r *http.Request, group string, retryAfterSecs int) {
	crawler := crawlerName(r.UserAgent())
	ok, suppressed := rateLimitGate.admit(group + "|" + crawler)
	if !ok {
		return
	}
	slog.Info("rate limited",
		"group", group,
		"crawler", crawler,
		"retry_after", retryAfterSecs,
		"suppressed", suppressed)
}
