package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/geocode"
	"kanarche.eu/internal/httpx"
	"kanarche.eu/internal/ratelimit"
)

// NewGeocodeLimiter builds the per-client bucket for /api/v1/geocode. Each miss
// spends the shared upstream budget, so this bucket is tighter than the global
// one. The caller owns its evictor, as with NewSeriesLimiter.
func NewGeocodeLimiter(cfg config.Config) *ratelimit.Limiter {
	return ratelimit.New(cfg.RateLimit.Geocode, cfg.RateLimit.ShardCount)
}

// busyRetryAfter is the Retry-After on a spent upstream budget: the budget
// refills at one request per second, so a couple of seconds is enough.
const busyRetryAfter = "2"

// handleGeocode proxies one address search.
//
// The query is personal data. This handler and everything it calls must never
// log it, and must never put it in an error string; failures are logged by
// kind only.
func (d Deps) handleGeocode(w http.ResponseWriter, r *http.Request) {
	// Fail closed: without both the service and its per-client bucket the route
	// is not offered at all, rather than offered unlimited.
	if d.Geocoder == nil || d.GeocodeLimiter == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "Address search is not available.")
		return
	}

	if ok, retryAfter := d.GeocodeLimiter.Allow(httpx.BucketKeyFrom(r.Context())); !ok {
		secs := int(retryAfter.Seconds())
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many requests. Please slow down.")
		return
	}

	q, ok := geocode.Clean(r.URL.Query().Get("q"))
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request",
			`The "q" parameter must be 3 to 120 characters.`)
		return
	}
	lang := r.URL.Query().Get("lang")
	switch lang {
	case "":
		lang = "en"
	case "bg", "en":
	default:
		writeError(w, http.StatusBadRequest, "bad_request", `The "lang" parameter must be bg or en.`)
		return
	}

	results, err := d.Geocoder.Search(r.Context(), q, lang)
	switch {
	case err == nil:
	case errors.Is(err, geocode.ErrBusy):
		w.Header().Set("Retry-After", busyRetryAfter)
		writeError(w, http.StatusServiceUnavailable, "busy", "Address search is busy. Please try again in a moment.")
		return
	case errors.Is(err, geocode.ErrTimeout):
		slog.Warn("geocoder failed", "kind", "timeout")
		writeError(w, http.StatusGatewayTimeout, "timeout", "Address search did not answer in time.")
		return
	default:
		slog.Warn("geocoder failed", "kind", "upstream")
		writeError(w, http.StatusBadGateway, "bad_gateway", "Address search failed.")
		return
	}

	if results == nil {
		results = []geocode.Result{}
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	// private: a shared cache must not serve one visitor's address to another,
	// and the request must still reach the per-client bucket.
	setCacheControl(h, cachePrivate, 600)
	_ = json.NewEncoder(w).Encode(results)
}
