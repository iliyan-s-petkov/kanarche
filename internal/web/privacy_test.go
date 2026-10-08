package web_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kanarche.eu/internal/i18n"
)

// storageAllowListKeys reads the published allow-list directly, the same file
// OpenProject #584's vitest/e2e guards check code and a browser against, so
// this test fails the moment the about page's privacy section and the
// allow-list disagree about what "every key" means.
func storageAllowListKeys(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("static", "storage-keys.json"))
	if err != nil {
		t.Fatalf("reading storage-keys.json: %v", err)
	}
	var list struct {
		LocalStorage   []string `json:"localStorage"`
		SessionStorage []string `json:"sessionStorage"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("parsing storage-keys.json: %v", err)
	}
	return append(list.LocalStorage, list.SessionStorage...)
}

// TestPrivacySectionListsEveryStorageKey is OpenProject #585's core guard: the
// about page's privacy section must name every key storage-keys.json allows,
// with its purpose, in both languages — proving the list is rendered from the
// file (storagekeys.go) rather than copied into the template by hand, which
// could drift the moment either side changes.
func TestPrivacySectionListsEveryStorageKey(t *testing.T) {
	keys := storageAllowListKeys(t)
	if len(keys) == 0 {
		t.Fatal("storage-keys.json has no keys to assert against")
	}

	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}

	rr := renderer(t, fixture(t))

	for _, tc := range []struct {
		path string
		lang string
	}{
		{"/about-the-data", "bg"},
		{"/en/about-the-data", "en"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			body := fetch(t, rr, tc.path).Body.String()
			for _, key := range keys {
				if !strings.Contains(body, key) {
					t.Errorf("body does not list storage key %q", key)
				}
				purpose := cat.T(tc.lang, "privacy.storage."+key)
				if purpose == "" || strings.HasPrefix(purpose, "!") {
					t.Fatalf("no catalogue purpose for %q in %q", key, tc.lang)
				}
				if !strings.Contains(body, purpose) {
					t.Errorf("body does not contain the %q purpose for %q: %q", tc.lang, key, purpose)
				}
			}
		})
	}
}

// TestPrivacySectionSaysNoCookies pins the literal claim (a) asks for: the
// about page must say the site sets no cookies, in both languages.
func TestPrivacySectionSaysNoCookies(t *testing.T) {
	rr := renderer(t, fixture(t))

	for _, tc := range []struct {
		path string
		want string
	}{
		{"/about-the-data", "не задава бисквитки"},
		{"/en/about-the-data", "sets no cookies"},
	} {
		body := fetch(t, rr, tc.path).Body.String()
		if !strings.Contains(body, tc.want) {
			t.Errorf("%s body does not contain %q", tc.path, tc.want)
		}
	}
}

// TestPrivacySectionHasClearSettingsIsland pins the (c) requirement's markup
// contract: a data-island="clearsettings" element carrying the exact
// allow-listed keys (so the island cannot silently clear a different set than
// what is published) plus an aria-live status region for the confirmation.
func TestPrivacySectionHasClearSettingsIsland(t *testing.T) {
	keys := storageAllowListKeys(t)
	rr := renderer(t, fixture(t))

	for _, path := range []string{"/about-the-data", "/en/about-the-data"} {
		body := fetch(t, rr, path).Body.String()
		if !strings.Contains(body, `data-island="clearsettings"`) {
			t.Errorf("%s: no clearsettings island", path)
		}
		for _, key := range keys {
			if !strings.Contains(body, key) {
				t.Errorf("%s: data-keys is missing %q", path, key)
			}
		}
		if !strings.Contains(body, `aria-live="polite"`) {
			t.Errorf("%s: no aria-live status region", path)
		}
	}
}

// TestAboutPageRendersWithoutASnapshotIncludingPrivacy re-asserts the no-snapshot
// guarantee (see TestAboutPageRendersWithoutASnapshot) specifically for the new
// section: a nil snapshot must not blank out the privacy list, since
// storageKeyInfos does not depend on the snapshot at all.
func TestAboutPageRendersWithoutASnapshotIncludingPrivacy(t *testing.T) {
	rec := fetch(t, renderer(t, nil), "/about-the-data")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	for _, key := range storageAllowListKeys(t) {
		if !strings.Contains(rec.Body.String(), key) {
			t.Errorf("body without a snapshot is missing storage key %q", key)
		}
	}
}
