package web_test

import (
	"html"
	"strings"
	"testing"
)

// TestPrivacyPageNamesControllerRightsAndRecipients pins the GDPR art. 13
// content of /privacy: who the controller is, how to reach them, the legal
// basis, the data subject's rights with the КЗЛД complaint route, and who
// receives the IP address.
func TestPrivacyPageNamesControllerRightsAndRecipients(t *testing.T) {
	rr := renderer(t, fixture(t))
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"/en/privacy", []string{
			"Who is responsible",
			"Iliyan Petkov, a private individual in Bulgaria",
			"privacy@kanarche.eu",
			"GDPR art. 6(1)(f)",
			"Your rights",
			"Commission for Personal Data Protection (cpdp.bg)",
			"Who receives your IP address",
			"Hostellation hosts the server",
		}},
		{"/privacy", []string{
			"Кой отговаря",
			"Илиян Петков, частно лице в България",
			"privacy@kanarche.eu",
			"ОРЗД",
			"Вашите права",
			"Комисията за защита на личните данни (cpdp.bg)",
			"Кой получава IP адреса ви",
			"Hostellation хоства сървъра",
		}},
	} {
		text := strings.Join(strings.Fields(html.UnescapeString(visibleText(fetch(t, rr, tc.path).Body.String()))), " ")
		for _, w := range tc.want {
			if !strings.Contains(text, w) {
				t.Errorf("%s lacks %q", tc.path, w)
			}
		}
	}
}

// TestTermsContactNamesOperatorAndMailbox pins the terms contact section: the
// operator is named and the privacy mailbox is the first channel.
func TestTermsContactNamesOperatorAndMailbox(t *testing.T) {
	rr := renderer(t, fixture(t))
	en := strings.Join(strings.Fields(html.UnescapeString(visibleText(fetch(t, rr, "/en/terms").Body.String()))), " ")
	for _, w := range []string{"run by Iliyan Petkov", "privacy@kanarche.eu"} {
		if !strings.Contains(en, w) {
			t.Errorf("/en/terms lacks %q", w)
		}
	}
}
