package web_test

import (
	"strings"
	"testing"
	"time"
)

// pmAnchor is the markup of a contextual link to a PM explainer page.
func pmAnchor(href, label string) string {
	return `<a class="link" href="` + href + `">` + label + `</a>`
}

// The station table headers on an area page link the PM labels to the explainer pages.
func TestAreaStationTableHeadersLinkToPMPages(t *testing.T) {
	for path, want := range map[string][]string{
		"/area/sofia":    {pmAnchor("/pm25", "ФПЧ2.5"), pmAnchor("/pm10", "ФПЧ10")},
		"/en/area/sofia": {pmAnchor("/en/pm25", "PM2.5"), pmAnchor("/en/pm10", "PM10")},
	} {
		list := stationList(t, path)
		for _, w := range want {
			if !strings.Contains(list, w) {
				t.Errorf("%s: station table lacks %q", path, w)
			}
		}
	}
}

// The 24 hour comparison on a city page links each PM label once, in its first sentence.
func TestAreaCompareLinksPMLabels(t *testing.T) {
	rr := renderer(t, compareFixture(t, time.Now()))
	for path, want := range map[string][]string{
		"/area/plovdiv":    {pmAnchor("/pm25", "ФПЧ2.5"), pmAnchor("/pm10", "ФПЧ10")},
		"/en/area/plovdiv": {pmAnchor("/en/pm25", "PM2.5"), pmAnchor("/en/pm10", "PM10")},
	} {
		body := fetch(t, rr, path).Body.String()
		i := strings.Index(body, `<div class="area-compare">`)
		if i < 0 {
			t.Fatalf("%s: no comparison block", path)
		}
		block := body[i : i+strings.Index(body[i:], "</div>")]
		for _, w := range want {
			if got := strings.Count(block, w); got != 1 {
				t.Errorf("%s: %q appears %d times in the comparison, want 1", path, w, got)
			}
		}
	}
}

// About FAQ answers that name PM2.5 or PM10 link them to the explainer pages. The FAQPage text stays plain.
func TestAboutFAQLinksPMPages(t *testing.T) {
	rr := renderer(t, fixture(t))
	for path, want := range map[string][]string{
		"/about-the-data":    {`href="/pm25"`, `href="/pm10"`},
		"/en/about-the-data": {`href="/en/pm25"`, `href="/en/pm10"`},
	} {
		sec := aboutFAQSectionRE.FindString(fetch(t, rr, path).Body.String())
		for _, w := range want {
			if !strings.Contains(sec, w) {
				t.Errorf("%s: FAQ lacks %s", path, w)
			}
		}
	}
}
