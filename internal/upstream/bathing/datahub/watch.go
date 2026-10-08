package datahub

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"kanarche.eu/internal/metrics"
)

var newEditionFound = metrics.Gauge("airbg_sea_datahub_newer_edition", "1 while the EEA Datahub has an edition newer than the pinned one, else 0.")

var (
	yearPattern     = regexp.MustCompile(`20\d{2}`)
	revisionPattern = regexp.MustCompile(`(_v\d+_)r00`)
)

// Watch sends one HEAD to the edition after the pinned cfg.URL and reports
// whether it exists. No body is read. 200 sets the gauge to 1 and 404 sets it
// to 0, so the gauge clears once the pin is bumped. Any other status or a
// network error returns an error and leaves the gauge alone. The derived URL
// must be https on cfg.AllowedHosts, and redirects use the Fetch guard.
func Watch(ctx context.Context, cfg FetchConfig) (bool, error) {
	next := deriveNextEditionURL(cfg.URL)
	if next == cfg.URL {
		return false, nil // no year to bump, probing would only find the pinned file
	}
	u, err := url.Parse(next)
	if err != nil {
		return false, fmt.Errorf("datahub watch: url: %w", err)
	}
	if u.Scheme != "https" {
		return false, ErrNotHTTPS
	}
	if !slices.Contains(cfg.AllowedHosts, u.Hostname()) {
		return false, fmt.Errorf("%w: %q", ErrHostNotAllowed, u.Hostname())
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, next, nil)
	if err != nil {
		return false, fmt.Errorf("datahub watch: request: %w", err)
	}
	resp, err := client(cfg).Do(req)
	if err != nil {
		return false, fmt.Errorf("datahub watch: %w", err)
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		slog.Info("sea datahub: newer edition published", "url", next)
		newEditionFound.Set(1)
		return true, nil
	case http.StatusNotFound:
		newEditionFound.Set(0)
		return false, nil
	}
	return false, fmt.Errorf("datahub watch: status %d", resp.StatusCode)
}

// deriveNextEditionURL bumps every occurrence of the last year in the URL and
// the r00 revision: ..._1990-2025_v01_r00/..._1990_2025.xlsx becomes
// ..._1990-2026_v01_r01/..._1990_2026.xlsx. A URL with no year is returned unchanged.
func deriveNextEditionURL(current string) string {
	u, err := url.Parse(current)
	if err != nil {
		return current
	}
	// Only the path carries the edition, so host and port digits are never touched.
	years := yearPattern.FindAllString(u.Path, -1)
	if len(years) == 0 {
		return current
	}
	last := years[len(years)-1]
	n, _ := strconv.Atoi(last) // the pattern guarantees digits
	// Replacing every occurrence also moves the range end in 1990-2025.
	path := strings.ReplaceAll(u.Path, last, strconv.Itoa(n+1))
	u.Path = revisionPattern.ReplaceAllString(path, "${1}r01")
	return u.String()
}
