package httpx_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"kanarche.eu/internal/httpx"
	"kanarche.eu/internal/ratelimit"
)

const (
	googleUA = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	bingUA   = "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)"
	crawlIP  = "203.0.113.77"
)

// fakeDNS is a resolver with fixed answers and call counters. No real DNS.
type fakeDNS struct {
	mu      sync.Mutex
	ptr     map[string][]string
	fwd     map[string][]string
	err     error
	block   chan struct{}
	addrN   int
	hostN   int
	started chan struct{}
}

func (f *fakeDNS) LookupAddr(ctx context.Context, addr string) ([]string, error) {
	f.mu.Lock()
	f.addrN++
	started := f.started
	f.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.ptr[addr], nil
}

func (f *fakeDNS) LookupHost(_ context.Context, host string) ([]string, error) {
	f.mu.Lock()
	f.hostN++
	f.mu.Unlock()
	return f.fwd[host], nil
}

func (f *fakeDNS) calls() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.addrN, f.hostN
}

// crawlerChain returns a handler whose limiter allows one request per client.
func crawlerChain(t *testing.T, dns httpx.DNSResolver) http.Handler {
	t.Helper()
	chain := httpx.Chain{
		Resolver:     resolver(t),
		Limiter:      ratelimit.New(testBucket(0, 1, time.Hour), testShardCount),
		MaxBodyBytes: 4096,
		Crawlers:     httpx.NewCrawlerVerifier(dns),
	}
	return chain.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

// statuses sends n requests and returns the last status.
func lastStatus(h http.Handler, ua, ip string, n int) int {
	code := 0
	for i := 0; i < n; i++ {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
		r.RemoteAddr = ip + ":40000"
		r.Header.Set("User-Agent", ua)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		code = rec.Code
	}
	return code
}

func TestSpoofedGooglebotWithForeignPTRStaysLimited(t *testing.T) {
	dns := &fakeDNS{
		ptr: map[string][]string{crawlIP: {"evil.example.net."}},
		fwd: map[string][]string{"evil.example.net": {crawlIP}},
	}
	if got := lastStatus(crawlerChain(t, dns), googleUA, crawlIP, 5); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", got)
	}
}

func TestSuffixLookalikePTRStaysLimited(t *testing.T) {
	dns := &fakeDNS{
		ptr: map[string][]string{crawlIP: {"crawl.notgooglebot.com."}},
		fwd: map[string][]string{"crawl.notgooglebot.com": {crawlIP}},
	}
	if got := lastStatus(crawlerChain(t, dns), googleUA, crawlIP, 5); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", got)
	}
}

// Any Google Cloud VM reverse-resolves under googleusercontent.com, so that suffix proves nothing.
func TestCloudVMPTRStaysLimited(t *testing.T) {
	dns := &fakeDNS{
		ptr: map[string][]string{crawlIP: {"77.113.0.203.bc.googleusercontent.com."}},
		fwd: map[string][]string{"77.113.0.203.bc.googleusercontent.com": {crawlIP}},
	}
	if got := lastStatus(crawlerChain(t, dns), googleUA, crawlIP, 5); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", got)
	}
}

func TestMatchingPTRWithoutForwardConfirmationStaysLimited(t *testing.T) {
	dns := &fakeDNS{
		ptr: map[string][]string{crawlIP: {"crawl-1.googlebot.com."}},
		fwd: map[string][]string{"crawl-1.googlebot.com": {"198.51.100.9"}},
	}
	if got := lastStatus(crawlerChain(t, dns), googleUA, crawlIP, 5); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", got)
	}
}

func TestVerifiedGooglebotIsExempt(t *testing.T) {
	dns := &fakeDNS{
		ptr: map[string][]string{crawlIP: {"crawl-203-0-113-77.googlebot.com."}},
		fwd: map[string][]string{"crawl-203-0-113-77.googlebot.com": {crawlIP}},
	}
	if got := lastStatus(crawlerChain(t, dns), googleUA, crawlIP, 5); got != http.StatusOK {
		t.Fatalf("status = %d, want 200", got)
	}
}

func TestVerifiedBingbotIsExempt(t *testing.T) {
	dns := &fakeDNS{
		ptr: map[string][]string{crawlIP: {"msnbot-203-0-113-77.search.msn.com."}},
		fwd: map[string][]string{"msnbot-203-0-113-77.search.msn.com": {crawlIP}},
	}
	if got := lastStatus(crawlerChain(t, dns), bingUA, crawlIP, 5); got != http.StatusOK {
		t.Fatalf("status = %d, want 200", got)
	}
}

func TestGoogleHostnameDoesNotVerifyBingbotClaim(t *testing.T) {
	dns := &fakeDNS{
		ptr: map[string][]string{crawlIP: {"crawl-1.googlebot.com."}},
		fwd: map[string][]string{"crawl-1.googlebot.com": {crawlIP}},
	}
	if got := lastStatus(crawlerChain(t, dns), bingUA, crawlIP, 5); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", got)
	}
}

func TestVerifiedCrawlerIPWithoutCrawlerUAStaysLimited(t *testing.T) {
	dns := &fakeDNS{
		ptr: map[string][]string{crawlIP: {"crawl-1.googlebot.com."}},
		fwd: map[string][]string{"crawl-1.googlebot.com": {crawlIP}},
	}
	if got := lastStatus(crawlerChain(t, dns), "curl/8", crawlIP, 5); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", got)
	}
	if a, _ := dns.calls(); a != 0 {
		t.Errorf("reverse lookups = %d, want 0 without a crawler claim", a)
	}
}

func TestResolverTimeoutMeansLimited(t *testing.T) {
	dns := &fakeDNS{block: make(chan struct{})}
	defer close(dns.block)
	h := crawlerChain(t, dns)

	// The request's own deadline stands in for the lookup budget.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	lastStatus(h, googleUA, crawlIP, 1)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil).WithContext(ctx)
	r.RemoteAddr = crawlIP + ":40000"
	r.Header.Set("User-Agent", googleUA)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
}

func TestResolverErrorMeansLimited(t *testing.T) {
	dns := &fakeDNS{err: errors.New("servfail")}
	if got := lastStatus(crawlerChain(t, dns), googleUA, crawlIP, 5); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", got)
	}
}

func TestVerdictIsCachedAcrossRequests(t *testing.T) {
	dns := &fakeDNS{
		ptr: map[string][]string{crawlIP: {"crawl-1.googlebot.com."}},
		fwd: map[string][]string{"crawl-1.googlebot.com": {crawlIP}},
	}
	h := crawlerChain(t, dns)
	lastStatus(h, googleUA, crawlIP, 6)
	if a, f := dns.calls(); a != 1 || f != 1 {
		t.Fatalf("lookups = %d reverse, %d forward, want 1 and 1", a, f)
	}
}

func TestNegativeVerdictIsCached(t *testing.T) {
	dns := &fakeDNS{ptr: map[string][]string{crawlIP: {"evil.example.net."}}}
	h := crawlerChain(t, dns)
	lastStatus(h, googleUA, crawlIP, 6)
	if a, _ := dns.calls(); a != 1 {
		t.Fatalf("reverse lookups = %d, want 1", a)
	}
}

func TestConcurrentRequestsShareOneLookup(t *testing.T) {
	dns := &fakeDNS{
		ptr:     map[string][]string{crawlIP: {"crawl-1.googlebot.com."}},
		fwd:     map[string][]string{"crawl-1.googlebot.com": {crawlIP}},
		block:   make(chan struct{}),
		started: make(chan struct{}, 1),
	}
	h := crawlerChain(t, dns)
	lastStatus(h, googleUA, crawlIP, 1) // spend the burst token

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lastStatus(h, googleUA, crawlIP, 1)
		}()
	}
	<-dns.started
	time.Sleep(50 * time.Millisecond)
	close(dns.block)
	wg.Wait()
	if a, _ := dns.calls(); a != 1 {
		t.Fatalf("reverse lookups = %d, want 1", a)
	}
}

func TestNilCrawlerVerifierKeepsLimiting(t *testing.T) {
	chain := httpx.Chain{
		Resolver:     resolver(t),
		Limiter:      ratelimit.New(testBucket(0, 1, time.Hour), testShardCount),
		MaxBodyBytes: 4096,
	}
	h := chain.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	if got := lastStatus(h, googleUA, crawlIP, 5); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", got)
	}
}
