package web_test

import (
	"strings"
	"testing"
)

// The map section's heading is visible and the metric picker, the status
// filter and the count sit in one toolbar row directly above the map shell.
func TestAreaMapHeadingIsVisible(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	body := fetch(t, rr, "/en/area/high").Body.String()

	at := strings.Index(body, `id="area-map-h"`)
	if at < 0 {
		t.Fatal("the map section lost its heading")
	}
	tag := body[strings.LastIndex(body[:at], "<h2"):at+strings.Index(body[at:], ">")]
	if strings.Contains(tag, "visually-hidden") {
		t.Errorf("the map heading is still hidden: %s", tag)
	}
}

func TestAreaToolbarHoldsPickerFilterAndCountInOneRow(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	body := fetch(t, rr, "/en/area/high").Body.String()

	start := strings.Index(body, `class="toolbar toolbar--map"`)
	shell := strings.Index(body, `class="map-shell"`)
	if start < 0 || shell < 0 || start > shell {
		t.Fatalf("no map toolbar above the map shell: toolbar=%d shell=%d", start, shell)
	}
	row := body[start:shell]
	if strings.Count(row, `class="toolbar`) != 1 {
		t.Error("the controls are split over more than one toolbar")
	}
	picker := strings.Index(row, `data-island="switcher"`)
	filter := strings.Index(row, `data-island="sensorbar"`)
	if picker < 0 || filter < 0 || picker > filter {
		t.Errorf("want picker then filter in the same row, got picker=%d filter=%d", picker, filter)
	}
	if strings.Contains(row[:filter], "\n</div>") {
		t.Error("the toolbar closes before the filter, so the filter is not in the same row")
	}
	if strings.Contains(body[:start], `data-island="sensorbar"`) {
		t.Error("the filter is outside the toolbar")
	}
}
