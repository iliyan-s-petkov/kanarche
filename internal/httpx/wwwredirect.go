package httpx

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// WWWToApex 301s requests whose Host is exactly www.<host of baseURL> to the
// same path and query on baseURL. Every other Host (apex, localhost, the origin
// IP, health checks) passes through untouched. A baseURL that is unparseable or
// already starts with "www." disables the redirect.
func WWWToApex(next http.Handler, baseURL string) http.Handler {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || strings.HasPrefix(strings.ToLower(u.Hostname()), "www.") {
		return next
	}
	wwwHost := "www." + strings.ToLower(u.Hostname())
	origin := u.Scheme + "://" + u.Host
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if strings.ToLower(host) == wwwHost {
			http.Redirect(w, r, origin+r.URL.RequestURI(), http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(w, r)
	})
}
