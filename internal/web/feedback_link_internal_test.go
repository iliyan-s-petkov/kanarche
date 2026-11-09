package web

import "testing"

func TestFeedbackLink(t *testing.T) {
	for _, tt := range []struct {
		name, base, lang, want string
	}{
		{"unset", "", "bg", ""},
		{"default language is bg", "https://feedback.kanarche.eu/", "bg", "https://feedback.kanarche.eu/?lang=bg"},
		{"english", "https://feedback.kanarche.eu/", "en", "https://feedback.kanarche.eu/?lang=en"},
		// The survey offers BG and EN only, so any other page language gets EN.
		{"third language falls back to en", "https://feedback.kanarche.eu/", "de", "https://feedback.kanarche.eu/?lang=en"},
		{"no trailing slash", "https://feedback.kanarche.eu", "en", "https://feedback.kanarche.eu?lang=en"},
		{"path kept", "https://feedback.kanarche.eu/s", "bg", "https://feedback.kanarche.eu/s?lang=bg"},
		// Validation refuses a query, but the method must still never emit two.
		{"existing query replaced", "https://feedback.kanarche.eu/?x=1", "bg", "https://feedback.kanarche.eu/?lang=bg"},
		{"unparseable", "https://%zz", "bg", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := PageData{Lang: tt.lang, feedbackURL: tt.base}
			if got := p.FeedbackLink(); got != tt.want {
				t.Errorf("FeedbackLink() = %q, want %q", got, tt.want)
			}
		})
	}
}
