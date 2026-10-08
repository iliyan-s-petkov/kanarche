package web

import (
	"encoding/json"
	"strings"
	"sync"

	"kanarche.eu/internal/i18n"
)

// storageKeysFile is where the published allow-list lives — the same file
// web/src/__tests__/storage-allowlist.test.js and web/e2e/storage-allowlist.spec.js
// check code and a real browser against (OpenProject #584). Reading it from
// the embedded static tree, rather than copying the keys into Go, is what
// OpenProject #585 asks for: the privacy section cannot list a key the
// allow-list does not have, or omit one it does.
const storageKeysFile = "static/storage-keys.json"

// storageAllowList is the shape of storage-keys.json.
type storageAllowList struct {
	LocalStorage   []string `json:"localStorage"`
	Legacy         []string `json:"legacy"`
	SessionStorage []string `json:"sessionStorage"`
}

// StorageKeyInfo is one row of the privacy section's list: a storage key and
// the plain-language purpose the catalogue gives it.
type StorageKeyInfo struct {
	Key     string
	Purpose string
}

// loadStorageKeys parses the embedded allow-list once. A parse failure yields
// an empty list rather than a panic: the rest of the site must still render,
// and TestPrivacySectionListsEveryStorageKey catches the missing section.
var loadStorageKeys = sync.OnceValue(func() []string {
	raw, err := staticFS.ReadFile(storageKeysFile)
	if err != nil {
		return nil
	}
	var list storageAllowList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	// localStorage first, then any session-only key not already listed —
	// today's file has none, but the section must not silently drop one if a
	// future file does.
	seen := make(map[string]bool, len(list.LocalStorage)+len(list.SessionStorage))
	keys := make([]string, 0, len(list.LocalStorage)+len(list.SessionStorage))
	for _, k := range append(append([]string{}, list.LocalStorage...), list.SessionStorage...) {
		if seen[k] {
			continue
		}
		seen[k] = true
		keys = append(keys, k)
	}
	return keys
})

// storagePurposeKey is the catalogue key holding a storage key's one-line
// purpose. A colon is a legal JSON-object key and Catalogue is a plain map, so
// "kanarche:theme" needs no escaping to become "privacy.storage.kanarche:theme".
func storagePurposeKey(storageKey string) string {
	return "privacy.storage." + storageKey
}

// storageKeyInfos builds the privacy section's rows for one language, in the
// allow-list's file order (not sorted — the file already orders the keys by
// what they control, and re-sorting here would just make the two disagree).
func storageKeyInfos(cat *i18n.Catalogue, lang string) []StorageKeyInfo {
	keys := loadStorageKeys()
	infos := make([]StorageKeyInfo, 0, len(keys))
	for _, k := range keys {
		infos = append(infos, StorageKeyInfo{Key: k, Purpose: cat.T(lang, storagePurposeKey(k))})
	}
	return infos
}

// loadLegacyStorageKeys are the pre-rename keys: read as a fallback and cleared
// with the rest, never written and not listed in the privacy section.
var loadLegacyStorageKeys = sync.OnceValue(func() []string {
	raw, err := staticFS.ReadFile(storageKeysFile)
	if err != nil {
		return nil
	}
	var list storageAllowList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	return list.Legacy
})

// storageKeysCSV is the "clear my settings" list: published keys, then legacy.
func storageKeysCSV() string {
	return strings.Join(append(append([]string{}, loadStorageKeys()...), loadLegacyStorageKeys()...), ",")
}
