package config

import (
	"strings"
	"testing"
)

// The shipped file leaves the survey unset: the site shows no survey link.
func TestFeedbackIsEmptyInTheCommittedConfig(t *testing.T) {
	clearAmbientEnv(t)
	if got := good(t).Feedback.URL; got != "" {
		t.Errorf("committed feedback.url = %q, want empty", got)
	}
}

func TestFeedbackURLComesFromTheEnvironment(t *testing.T) {
	t.Setenv("KANARCHE_FEEDBACK_URL", "https://feedback.kanarche.eu/")
	if got := good(t).Feedback.URL; got != "https://feedback.kanarche.eu/" {
		t.Errorf("Feedback.URL = %q, want the environment value", got)
	}
}

// The value lands in an href, so anything but a plain https URL is refused at startup.
func TestFeedbackURLMustBeAPlainHTTPSURL(t *testing.T) {
	for _, bad := range []string{
		"javascript:alert(1)",
		"http://feedback.kanarche.eu/",
		"feedback.kanarche.eu",
		"https://",
		"https://user:pw@feedback.kanarche.eu/",
		"https://feedback.kanarche.eu/?lang=en",
		"https://feedback.kanarche.eu/#top",
		"https://feedback.kanarche.eu/ ",
		" https://feedback.kanarche.eu/",
	} {
		cfg := good(t)
		cfg.Feedback.URL = bad
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "feedback.url") {
			t.Errorf("Validate(%q) = %v, want a feedback.url problem", bad, err)
		}
	}
}

func TestFeedbackURLAcceptsEmptyAndHTTPS(t *testing.T) {
	for _, ok := range []string{"", "https://feedback.kanarche.eu", "https://feedback.kanarche.eu/", "https://feedback.kanarche.eu/s"} {
		cfg := good(t)
		cfg.Feedback.URL = ok
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", ok, err)
		}
	}
}
