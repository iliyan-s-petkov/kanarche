package i18n_test

import "testing"

// The six survey keys exist, are non-empty and differ between bg and en. Key
// parity in general is TestCataloguesHaveIdenticalKeys; this pins these names.
func TestFeedbackKeysExistInBothLanguages(t *testing.T) {
	c := loaded(t)
	for _, key := range []string{
		"nav.feedback",
		"about.feedback.heading", "about.feedback.body", "about.feedback.link",
		"privacy.feedback.heading", "privacy.feedback.body",
	} {
		bg, en := c.T("bg", key), c.T("en", key)
		if !c.Has("bg", key) || !c.Has("en", key) || bg == "" || en == "" {
			t.Errorf("%s missing or empty (bg=%q en=%q)", key, bg, en)
		}
		if bg == en {
			t.Errorf("%s is identical in bg and en: %q", key, bg)
		}
	}
}
