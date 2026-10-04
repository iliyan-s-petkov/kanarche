package api_test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// A statement timeout (SQLSTATE 57014) or a context deadline is load, not a
// server fault: the client gets a retryable 503, not a 500 with no hint.
func TestSeriesQueryTimeoutIs503WithRetryAfter(t *testing.T) {
	timeouts := map[string]error{
		"statement timeout": fmt.Errorf("store: sensor series: %w", &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout"}),
		"context deadline":  fmt.Errorf("store: sensor series: %w", context.DeadlineExceeded),
	}
	for name, cause := range timeouts {
		t.Run(name, func(t *testing.T) {
			d := deps(t, fixture(t))
			d.Store = &stubSource{slug: "sofia", err: cause}
			rec := serve(t, d, get("/api/v1/sensor/42/series?metric=P2&period=30d", "203.0.113.50"))
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
			want := strconv.Itoa(int(d.Config.RateLimit.Series.RetryAfter.Seconds()))
			if got := rec.Header().Get("Retry-After"); got != want {
				t.Errorf("Retry-After = %q, want %q", got, want)
			}
			if strings.Contains(rec.Body.String(), "statement") {
				t.Errorf("body leaks the database error: %s", rec.Body.String())
			}
		})
	}
}

// Only timeouts are retryable; a missing relation is a real fault.
func TestSeriesOtherStoreErrorStays500(t *testing.T) {
	d := deps(t, fixture(t))
	d.Store = &stubSource{slug: "sofia", err: &pgconn.PgError{Code: "42P01"}}
	rec := serve(t, d, get("/api/v1/sensor/42/series?metric=P2&period=30d", "203.0.113.51"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "" {
		t.Errorf("Retry-After set on a 500")
	}
}

// The area series routes share the mapping.
func TestAreaSeriesQueryTimeoutIs503(t *testing.T) {
	d := deps(t, fixture(t))
	d.Store = &stubSource{slug: "sofia", err: &pgconn.PgError{Code: "57014"}}
	rec := serve(t, d, get("/api/v1/area/sofia/series?metric=P2&period=30d", "203.0.113.52"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}
