package httpx

import (
	"context"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Verdict lifetimes, size bound and lookup limits.
const (
	crawlerPositiveTTL  = 24 * time.Hour
	crawlerNegativeTTL  = time.Hour
	crawlerErrorTTL     = time.Minute
	crawlerLookupBudget = 2 * time.Second
	crawlerCacheMax     = 4096
	crawlerMaxLookups   = 16
)

// DNSResolver is the slice of net.Resolver the verifier needs.
type DNSResolver interface {
	LookupAddr(ctx context.Context, addr string) ([]string, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// Documented reverse DNS suffixes per crawler family.
var crawlerDomains = map[string][]string{
	"googlebot": {".googlebot.com", ".google.com", ".googleusercontent.com"},
	"bingbot":   {".search.msn.com"},
}

// claimedCrawler returns the verifiable family the User-Agent claims, or "".
func claimedCrawler(userAgent string) string {
	ua := strings.ToLower(userAgent)
	for family := range crawlerDomains {
		if strings.Contains(ua, family) {
			return family
		}
	}
	return ""
}

type crawlerKey struct {
	family string
	ip     netip.Addr
}

type crawlerEntry struct {
	verified bool
	expires  time.Time
}

type crawlerCall struct {
	done     chan struct{}
	verified bool
}

// CrawlerVerifier confirms a claimed Googlebot or Bingbot by reverse DNS with
// forward confirmation. The User-Agent alone never verifies anything.
type CrawlerVerifier struct {
	resolver DNSResolver
	now      func() time.Time

	mu       sync.Mutex
	cache    map[crawlerKey]crawlerEntry
	inflight map[crawlerKey]*crawlerCall
	sem      chan struct{}
}

func NewCrawlerVerifier(r DNSResolver) *CrawlerVerifier {
	return &CrawlerVerifier{
		resolver: r,
		now:      time.Now,
		cache:    map[crawlerKey]crawlerEntry{},
		inflight: map[crawlerKey]*crawlerCall{},
		sem:      make(chan struct{}, crawlerMaxLookups),
	}
}

// Verified reports whether ip is a genuine crawler of the family the
// User-Agent claims. Any failure, timeout or doubt returns false.
func (v *CrawlerVerifier) Verified(ctx context.Context, userAgent string, ip netip.Addr) bool {
	family := claimedCrawler(userAgent)
	if v == nil || family == "" || !ip.IsValid() {
		return false
	}
	key := crawlerKey{family, ip}

	v.mu.Lock()
	if e, ok := v.cache[key]; ok && v.now().Before(e.expires) {
		v.mu.Unlock()
		return e.verified
	}
	call, running := v.inflight[key]
	if !running {
		call = &crawlerCall{done: make(chan struct{})}
		v.inflight[key] = call
	}
	v.mu.Unlock()

	if !running {
		go v.lookup(key, call)
	}
	select {
	case <-call.done:
		if call.verified {
			slog.Debug("crawler verified", "crawler", family)
		}
		return call.verified
	case <-ctx.Done():
		return false
	}
}

// lookup runs detached from any request so every waiter shares one result.
func (v *CrawlerVerifier) lookup(key crawlerKey, call *crawlerCall) {
	verified, definitive := false, false
	select {
	case v.sem <- struct{}{}:
		ctx, cancel := context.WithTimeout(context.Background(), crawlerLookupBudget)
		verified, definitive = v.check(ctx, key)
		cancel()
		<-v.sem
	default:
		// Too many lookups in flight: not verified, retried after the error TTL.
	}

	ttl := crawlerErrorTTL
	if definitive {
		ttl = crawlerNegativeTTL
		if verified {
			ttl = crawlerPositiveTTL
		}
	}

	v.mu.Lock()
	v.store(key, crawlerEntry{verified, v.now().Add(ttl)})
	delete(v.inflight, key)
	call.verified = verified
	v.mu.Unlock()
	close(call.done)
}

// store inserts under v.mu, dropping expired then arbitrary entries at the bound.
func (v *CrawlerVerifier) store(key crawlerKey, e crawlerEntry) {
	if _, exists := v.cache[key]; !exists && len(v.cache) >= crawlerCacheMax {
		now := v.now()
		for k, old := range v.cache {
			if !now.Before(old.expires) {
				delete(v.cache, k)
			}
		}
		for k := range v.cache {
			if len(v.cache) < crawlerCacheMax {
				break
			}
			delete(v.cache, k)
		}
	}
	v.cache[key] = e
}

// check returns the verdict and whether it is definitive (no lookup error).
func (v *CrawlerVerifier) check(ctx context.Context, key crawlerKey) (verified, definitive bool) {
	names, err := v.resolver.LookupAddr(ctx, key.ip.String())
	if err != nil && len(names) == 0 {
		return false, false
	}
	definitive = true
	for _, name := range names {
		name = strings.ToLower(strings.TrimSuffix(name, "."))
		if !hasCrawlerSuffix(name, crawlerDomains[key.family]) {
			continue
		}
		// Forward confirmation: the claimed hostname must resolve back to this IP.
		addrs, err := v.resolver.LookupHost(ctx, name)
		if err != nil && len(addrs) == 0 {
			definitive = false
			continue
		}
		for _, a := range addrs {
			if got, perr := netip.ParseAddr(a); perr == nil && got.Unmap() == key.ip {
				return true, true
			}
		}
	}
	return false, definitive
}

func hasCrawlerSuffix(name string, suffixes []string) bool {
	for _, s := range suffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}
