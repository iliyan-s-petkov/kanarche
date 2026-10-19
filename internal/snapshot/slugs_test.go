package snapshot

import "testing"

func TestCurrentSlug(t *testing.T) {
	for in, want := range map[string]string{
		"sofiya":             "sofia",
		"sofiya-grad-oblast": "sofia-grad-oblast",
		"sofia":              "sofia",
		"varna":              "varna",
		"":                   "",
	} {
		if got := CurrentSlug(in); got != want {
			t.Errorf("CurrentSlug(%q) = %q, want %q", in, got, want)
		}
	}
}
