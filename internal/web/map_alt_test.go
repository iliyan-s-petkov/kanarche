package web_test

import (
	"strings"
	"testing"
)

// The map's description is a sentence for screen readers. The route to the
// numbers is a separate, always visible "Data table" link beside the map: a
// link buried in a visually-hidden paragraph took focus while clipped away
// (A11Y-07), so it must never live inside one.
func TestTheMapDescriptionIsHiddenProseAndHoldsNoLink(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/en/").Body.String()

	i := strings.Index(body, `id="map-alt"`)
	if i < 0 {
		t.Fatal("the map has no text alternative to describe it")
	}
	start := strings.LastIndex(body[:i], "<p")
	para := body[start : start+strings.Index(body[start:], "</p>")]
	if !strings.Contains(para, "visually-hidden") {
		t.Errorf("the description paints under the map; it belongs to screen readers alone")
	}
	if strings.Contains(para, "<a ") {
		t.Errorf("a focusable link sits inside the visually-hidden description")
	}
}

func TestTheMapHasAVisibleDataTableLink(t *testing.T) {
	body := fetch(t, renderer(t, fixture(t)), "/en/").Body.String()

	i := strings.Index(body, `>Data table</a>`)
	if i < 0 {
		t.Fatal("the home page has no Data table link")
	}
	start := strings.LastIndex(body[:i], "<a ")
	anchor := body[start:i]
	if !strings.Contains(anchor, `href="/en/areas"`) {
		t.Errorf("the Data table link does not lead to /areas: %s", anchor)
	}
	open := strings.LastIndex(body[:start], "<p")
	if strings.Contains(body[open:start], "visually-hidden") {
		t.Errorf("the Data table link is inside a visually-hidden element")
	}
}
