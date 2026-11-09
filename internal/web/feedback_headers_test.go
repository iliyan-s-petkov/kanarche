package web_test

import (
	"strings"
	"testing"
)

// The survey is a plain outbound link. The policy served with the pages must be
// the configured one, byte for byte, whether or not the survey is on, and must
// not grow a frame-src, a connect-src for the survey or a looser form-action.
func TestFeedbackDoesNotChangeTheSecurityHeaders(t *testing.T) {
	cfgCSP := testConfig(t).Listen.CSP
	on, off := rendererWithFeedback(t, surveyBase), rendererWithFeedback(t, "")
	for _, path := range []string{"/", "/en/about", "/privacy"} {
		withLink, without := framed(t, on, path), framed(t, off, path)
		for _, h := range []string{"Content-Security-Policy", "Permissions-Policy", "Referrer-Policy", "X-Content-Type-Options", "Strict-Transport-Security"} {
			if a, b := withLink.Header().Get(h), without.Header().Get(h); a != b {
				t.Errorf("%s %s differs with the survey on:\n on: %s\noff: %s", path, h, a, b)
			}
		}
		csp := withLink.Header().Get("Content-Security-Policy")
		if csp != cfgCSP {
			t.Errorf("%s CSP = %q, want the configured policy", path, csp)
		}
		for _, banned := range []string{"feedback", "frame-src"} {
			if strings.Contains(csp, banned) {
				t.Errorf("%s CSP contains %q", path, banned)
			}
		}
		if !strings.Contains(csp, "form-action 'none'") {
			t.Errorf("%s CSP lost form-action 'none': %s", path, csp)
		}
	}
}
