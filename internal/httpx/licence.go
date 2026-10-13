package httpx

import (
	"net/http"
	"strings"
)

// licenceLink points API responses at the page that lists each source's licence.
const licenceLink = `<https://kanarche.eu/licences>; rel="license"`

// LicenceLink sets the licence Link header on every /api/v1/ response, errors
// and mux 404 and 405 answers included, because it wraps the whole mux.
func LicenceLink(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			w.Header().Set("Link", licenceLink)
		}
		next.ServeHTTP(w, r)
	})
}
