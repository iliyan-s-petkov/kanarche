package web_test

import (
	"net/http"
	"strings"
	"testing"
)

// The 503 shown before the first snapshot is a restart notice, not an error.
// It asks the client to come back (Retry-After) and reloads the page itself
// (meta refresh), so a visitor who lands during a reboot sees the map later
// without doing anything.
func TestUnavailableIsANoticeThatRetriesItself(t *testing.T) {
	const refresh = `<meta http-equiv="refresh" content="20">`

	for _, tc := range []struct {
		name  string
		path  string
		title string
		body  string
	}{
		{"bg index", "/", "Картата се зарежда", "Сървърът току-що се рестартира. Страницата ще се обнови сама до минута."},
		{"en index", "/en/", "The map is loading", "The server has just restarted. This page will refresh by itself within a minute."},
		{"bg area", "/area/sofia", "Картата се зарежда", "Сървърът току-що се рестартира. Страницата ще се обнови сама до минута."},
		{"en area", "/en/area/sofia", "The map is loading", "The server has just restarted. This page will refresh by itself within a minute."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := fetch(t, renderer(t, nil), tc.path)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "30" {
				t.Errorf("Retry-After = %q, want %q", got, "30")
			}
			body := rec.Body.String()
			if !strings.Contains(body, refresh) {
				t.Errorf("the page is missing %s", refresh)
			}
			if !strings.Contains(body, tc.title) {
				t.Errorf("the page is missing the title %q", tc.title)
			}
			if !strings.Contains(body, tc.body) {
				t.Errorf("the page is missing the body %q", tc.body)
			}
			if !strings.Contains(body, `<meta name="robots" content="noindex">`) {
				t.Error("the notice is indexable; it must stay noindex")
			}
		})
	}
}

// Only the restart notice refreshes and carries Retry-After. A rendered 404
// is final, so it must not send either.
func TestNotFoundHasNoRefreshAndNoRetryAfter(t *testing.T) {
	rec := fetch(t, renderer(t, fixture(t)), "/area/nope")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q on a 404, want none", got)
	}
	if strings.Contains(rec.Body.String(), `http-equiv="refresh"`) {
		t.Error("the 404 page refreshes itself; only the restart notice may")
	}
}
