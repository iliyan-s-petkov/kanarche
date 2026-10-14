package config

import (
	"strings"
	"testing"
)

// The shipped file leaves both social networks unset: the footer shows GitHub only.
func TestSocialIsEmptyInTheCommittedConfig(t *testing.T) {
	cfg := good(t)
	if cfg.Social.FacebookURL != "" || cfg.Social.LinkedInURL != "" {
		t.Errorf("committed social = %+v, want both empty", cfg.Social)
	}
}

func TestSocialURLsComeFromTheEnvironment(t *testing.T) {
	t.Setenv("KANARCHE_SOCIAL_FACEBOOK_URL", "https://www.facebook.com/airbg")
	t.Setenv("KANARCHE_SOCIAL_LINKEDIN_URL", "https://www.linkedin.com/company/airbg")
	cfg := good(t)
	if cfg.Social.FacebookURL != "https://www.facebook.com/airbg" || cfg.Social.LinkedInURL != "https://www.linkedin.com/company/airbg" {
		t.Errorf("social = %+v, want the environment values", cfg.Social)
	}
}

// The AIRBG_ spelling is no longer read: only KANARCHE_ reaches the config.
func TestLegacyAirbgPrefixIsIgnored(t *testing.T) {
	t.Setenv("AIRBG_SOCIAL_FACEBOOK_URL", "https://www.facebook.com/old")
	if got := good(t).Social.FacebookURL; got != "" {
		t.Errorf("FacebookURL = %q with only AIRBG_ set, want empty", got)
	}
	t.Setenv("KANARCHE_SOCIAL_FACEBOOK_URL", "https://www.facebook.com/new")
	if got := good(t).Social.FacebookURL; got != "https://www.facebook.com/new" {
		t.Errorf("FacebookURL = %q with KANARCHE_ set, want the KANARCHE_ value", got)
	}
}

func TestSocialURLsMustBeHTTPS(t *testing.T) {
	for _, bad := range []string{"javascript:alert(1)", "http://www.facebook.com/airbg", "facebook.com/airbg", "https://", "https://user:pw@www.facebook.com/airbg"} {
		cfg := good(t)
		cfg.Social.FacebookURL = bad
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "social.facebook_url") {
			t.Errorf("Validate(%q) = %v, want a social.facebook_url problem", bad, err)
		}
	}
}

func TestSocialProblemsAreReportedInFixedOrder(t *testing.T) {
	cfg := good(t)
	cfg.Social.FacebookURL = "http://a"
	cfg.Social.LinkedInURL = "http://b"
	for i := 0; i < 20; i++ {
		msg := cfg.Validate().Error()
		if strings.Index(msg, "social.facebook_url") > strings.Index(msg, "social.linkedin_url") {
			t.Fatalf("social problems out of order: %s", msg)
		}
	}
}
