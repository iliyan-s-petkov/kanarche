package web

import (
	"strings"
	"testing"

	"kanarche.eu/internal/i18n"
)

// TestStorageKeysHaveACataloguePurpose is OpenProject #585's guard: a key
// added to storage-keys.json with no matching privacy.storage.<key> entry in
// both catalogues must fail this test, not ship as a blank row on the privacy
// section.
func TestStorageKeysHaveACataloguePurpose(t *testing.T) {
	keys := loadStorageKeys()
	if len(keys) == 0 {
		t.Fatal("loadStorageKeys returned no keys — storage-keys.json failed to load or parse")
	}

	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}

	for _, k := range keys {
		wantKey := storagePurposeKey(k)
		for _, lang := range cat.Languages() {
			if !cat.Has(lang, wantKey) {
				t.Errorf("storage key %q has no %q entry in the %q catalogue", k, wantKey, lang)
			}
			if got := cat.T(lang, wantKey); got == "" {
				t.Errorf("storage key %q purpose is empty in %q", k, lang)
			}
		}
	}
}

// The legacy keys are what "clear my settings" must also remove, and each is
// the airbg: predecessor of a published kanarche: key; the privacy list shows
// only the published ones.
func TestLegacyStorageKeysMirrorThePublishedKeys(t *testing.T) {
	keys, legacy := loadStorageKeys(), loadLegacyStorageKeys()
	if len(legacy) != len(keys) {
		t.Fatalf("got %d legacy keys for %d published keys", len(legacy), len(keys))
	}
	for i, k := range keys {
		if want := "airbg:" + strings.TrimPrefix(k, "kanarche:"); legacy[i] != want {
			t.Errorf("legacy[%d] = %q, want %q", i, legacy[i], want)
		}
	}
	for _, info := range storageKeyInfos(mustCatalogue(t), "en") {
		if strings.HasPrefix(info.Key, "airbg:") {
			t.Errorf("privacy list shows legacy key %q", info.Key)
		}
	}
}

func mustCatalogue(t *testing.T) *i18n.Catalogue {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	return cat
}

// TestStorageKeysCSVMatchesKeys pins the island's data-keys contract: the CSV
// the template writes to the "clear my settings" button is the allow-list
// followed by the legacy keys, comma-joined, in file order.
func TestStorageKeysCSVMatchesKeys(t *testing.T) {
	keys := append(loadStorageKeys(), loadLegacyStorageKeys()...)
	got := storageKeysCSV()
	want := ""
	for i, k := range keys {
		if i > 0 {
			want += ","
		}
		want += k
	}
	if got != want {
		t.Errorf("storageKeysCSV() = %q, want %q", got, want)
	}
}
