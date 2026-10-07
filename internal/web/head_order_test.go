package web_test

import (
	"strings"
	"testing"
)

// The theme script is fetched and run before the stylesheets in the head.
// A classic script waits for every stylesheet above it, so placed after them
// it could not run until app.css had arrived.
func TestThemeInitPrecedesStylesheets(t *testing.T) {
	for _, path := range []string{"/", "/embed"} {
		assertThemeInitFirst(t, framed(t, renderer(t, fixture(t)), path).Body.String(), path)
	}
}

func assertThemeInitFirst(t *testing.T, body, path string) {
	t.Helper()
	script := strings.Index(body, `<script src="/static/theme-init.js`)
	sheet := strings.Index(body, `<link rel="stylesheet"`)
	if script < 0 || sheet < 0 {
		t.Fatalf("%s: theme-init script (%d) or stylesheet link (%d) missing", path, script, sheet)
	}
	if script > sheet {
		t.Errorf("%s: theme-init.js (offset %d) comes after the first stylesheet (offset %d)", path, script, sheet)
	}
	if head := body[:strings.Index(body, "</head>")]; !strings.Contains(head, "theme-init.js") {
		t.Errorf("%s: theme-init.js is not in the head", path)
	}
}
