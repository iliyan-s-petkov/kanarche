package datahub

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"airbg.org/internal/metrics"
	"airbg.org/internal/upstream/bathing"
)

var newEditionFound = metrics.Gauge("airbg_sea_datahub_newer_edition", "Set to 1 when a newer EEA Datahub edition is detected, 0 otherwise.")

// init registers the Watch function with the bathing package to make it available
// to the bathing.Collector.Loop without creating a circular import at the collector level.
func init() {
	bathing.DatahubWatcher = Watch
}

// Watch checks if a newer edition of the Datahub is available by doing a HEAD
// request to the next edition URL. It returns true if 200 OK is received,
// false if 404 or any other non-200 status, and an error on network/validation issues.
// No body is read from the response. The allowedHost is used to validate
// the derived URL still points to the same origin.
func Watch(ctx context.Context, currentURL string, allowedHost string, timeout time.Duration, httpClient *http.Client) (bool, error) {
	nextURL := deriveNextEditionURL(currentURL)

	// Validate the next URL uses the same allowed host
	u, err := url.Parse(nextURL)
	if err != nil {
		return false, fmt.Errorf("datahub watch: invalid next url: %w", err)
	}
	if u.Hostname() != allowedHost {
		return false, fmt.Errorf("datahub watch: derived url uses different host: %s", u.Hostname())
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, nextURL, nil)
	if err != nil {
		return false, fmt.Errorf("datahub watch: create request: %w", err)
	}

	client := http.Client{Timeout: timeout}
	if httpClient != nil {
		client = *httpClient
		client.Timeout = timeout
	}
	client.CheckRedirect = sameOriginOnly

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("datahub watch: head request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		slog.Info("sea datahub: newer edition published", "url", nextURL)
		newEditionFound.Set(1)
		return true, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	return false, fmt.Errorf("datahub watch: unexpected status %d", resp.StatusCode)
}

// deriveNextEditionURL bumps the year in the Datahub URL to the next edition.
// It extracts the current year from the URL and replaces it with year+1,
// and changes _vXX_r00 to _vXX_r01 (revision bumped for the new year).
// Examples:
//   "...bathing-water-status_p_1990-2025_v01_r00/bw_assessment_eea_datahub_1990_2025.xlsx"
//   becomes
//   "...bathing-water-status_p_1990-2026_v01_r01/bw_assessment_eea_datahub_1990_2026.xlsx"
func deriveNextEditionURL(currentURL string) string {
	// Find all 4-digit years in the 2000-2099 range
	yearPattern := regexp.MustCompile(`(20\d{2})`)

	allMatches := yearPattern.FindAllString(currentURL, -1)
	if len(allMatches) == 0 {
		return currentURL // No year found, return unchanged
	}

	// Get the last occurrence of a year (most likely the data year)
	currentYearStr := allMatches[len(allMatches)-1]
	currentYear, err := strconv.Atoi(currentYearStr)
	if err != nil {
		return currentURL
	}

	nextYear := currentYear + 1
	nextYearStr := strconv.Itoa(nextYear)

	// Replace the last occurrence of the current year with the next year
	// We need to replace from the end of the string to avoid replacing the "1990" part
	result := currentURL
	lastIdx := strings.LastIndex(result, currentYearStr)
	if lastIdx != -1 {
		result = result[:lastIdx] + nextYearStr + result[lastIdx+len(currentYearStr):]
	}

	// Also replace all other occurrences for consistency (e.g., "1990-2025" -> "1990-2026")
	result = strings.ReplaceAll(result, currentYearStr, nextYearStr)

	// Bump the revision from r00 to r01
	revisionPattern := regexp.MustCompile(`(_v\d+_)r00`)
	result = revisionPattern.ReplaceAllString(result, `${1}r01`)

	return result
}
