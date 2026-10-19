package web_test

import (
	"regexp"
	"strings"
	"testing"
)

var freshnessHostRe = regexp.MustCompile(`(?s)<div data-island="freshness"[^>]*>`)

// 3.4: the refresh status names what changed, so the pill carries the sentence and its subject.
func TestAreaFreshnessCarriesTheChangeSentence(t *testing.T) {
	body := fetch(t, renderer(t, sensorsFixture(t)), "/en/area/sofia-grad-oblast").Body.String()
	host := freshnessHostRe.FindString(body)
	if host == "" {
		t.Fatal("no freshness island on the area page")
	}
	if v, _ := attr(host, "data-t-changed"); v != "{subject}: air quality is now {band}, was {was}." {
		t.Errorf("data-t-changed = %q", v)
	}
	if v, _ := attr(host, "data-t-sensor"); v != "Sensor" {
		t.Errorf("data-t-sensor = %q", v)
	}
	subject, ok := attr(host, "data-subject")
	if !ok || subject == "" {
		t.Fatal("the area page names no subject for the change sentence")
	}
	if !strings.Contains(body, subject+" right now") {
		t.Errorf("subject %q is not the one the area summary uses", subject)
	}
}

func TestHomeFreshnessHasNoAreaSubject(t *testing.T) {
	host := freshnessHostRe.FindString(fetch(t, renderer(t, sensorsFixture(t)), "/en/").Body.String())
	if host == "" {
		t.Fatal("no freshness island on the home page")
	}
	if _, ok := attr(host, "data-subject"); ok {
		t.Error("the home page names an area subject")
	}
	if v, _ := attr(host, "data-t-changed"); !strings.Contains(v, "{subject}") {
		t.Errorf("home data-t-changed = %q", v)
	}
}

func TestBulgarianChangeSentence(t *testing.T) {
	host := freshnessHostRe.FindString(fetch(t, renderer(t, sensorsFixture(t)), "/area/sofia-grad-oblast").Body.String())
	if v, _ := attr(host, "data-t-changed"); v != "{subject}: качеството на въздуха вече е {band}, преди беше {was}." {
		t.Errorf("bg data-t-changed = %q", v)
	}
}
