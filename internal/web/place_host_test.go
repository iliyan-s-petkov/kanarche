package web_test

import (
	"strings"
	"testing"
)

// The sensor card is the kit's .place-host slot: a card under the map, in the
// flow. It used to be styled as an overlay positioned "within #area-map", but
// its container is a SIBLING of the map shell, so with no positioned ancestor
// the card resolved against the initial containing block and opened in the
// viewport's top-right corner, over the header. The class is what earns it the
// slot's spacing, so it is worth pinning.
func TestSensorPanelCarriesThePlaceHostClass(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	body := fetch(t, rr, "/en/area/high").Body.String()

	if !strings.Contains(body, `<div class="place-host" data-island="panel"`) {
		t.Error("the sensor panel's container is not the kit's place-host slot")
	}
}

// Order is the fix, not decoration: a card that describes the marker the reader
// just clicked has to come after the map, and after the freshness line, which
// is now an overlay inside the map shell and says how old the map itself is.
func TestSensorPanelComesAfterTheMapAndItsFreshnessOverlay(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	body := fetch(t, rr, "/en/area/high").Body.String()

	mapAt := strings.Index(body, `id="area-map"`)
	panelAt := strings.Index(body, `class="place-host"`)
	freshAt := strings.Index(body, `data-island="freshness"`)

	if mapAt < 0 || panelAt < 0 || freshAt < 0 {
		t.Fatalf("the area page is missing one of its parts: map=%d panel=%d freshness=%d", mapAt, panelAt, freshAt)
	}
	if !(mapAt < freshAt && freshAt < panelAt) {
		t.Errorf("the freshness overlay is not on the map above the panel: map=%d freshness=%d panel=%d", mapAt, freshAt, panelAt)
	}
}

// Desktop reads top-down: the readings, then where (map), then which sensor (the
// card), then how much over time (the chart), then the sensors. The card sits
// right above the chart: with a sensor open the chart renders nothing and the
// card is the view, so the two stay adjacent.
func TestReadoutsComeBeforeTheMapAndTheChartComesLast(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	body := fetch(t, rr, "/en/area/high").Body.String()

	readoutAt := strings.Index(body, `data-island="readouts"`)
	mapAt := strings.Index(body, `id="area-map"`)
	panelAt := strings.Index(body, `class="place-host"`)
	chartAt := strings.Index(body, `id="chart"`)

	if mapAt < 0 || panelAt < 0 || chartAt < 0 || readoutAt < 0 {
		t.Fatalf("the area page is missing one of its parts: readouts=%d map=%d panel=%d chart=%d", readoutAt, mapAt, panelAt, chartAt)
	}
	if !(readoutAt < mapAt && mapAt < panelAt && panelAt < chartAt) {
		t.Errorf("the area page order is wrong: readouts=%d map=%d panel=%d chart=%d", readoutAt, mapAt, panelAt, chartAt)
	}
}

// The section order of the redesign: header, Now, health, map, history, sensors,
// then the rest. Each is found by its heading id so a reordered template fails here.
func TestAreaSectionsFollowTheDesignOrder(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	body := fetch(t, rr, "/en/area/high").Body.String()

	ids := []string{`id="area-now-h"`, `id="area-health-h"`, `id="area-map-h"`, `id="chart"`, `id="sensors-h"`}
	prev := -1
	for _, id := range ids {
		at := strings.Index(body, id)
		if at < 0 {
			t.Fatalf("the area page lost %s", id)
		}
		if at < prev {
			t.Errorf("%s is out of order (at %d, previous section at %d)", id, at, prev)
		}
		prev = at
	}
	if at := strings.Index(body, `class="area-summary"`); at < 0 || at > strings.Index(body, `id="area-now-h"`) {
		t.Error("the lead summary does not come before the Now section")
	}
}

// Every section on the page is a labelled section with an H2, and the sensors
// section holds the two lists as H3 summaries.
func TestAreaSectionsAreLabelledWithHeadings(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	body := fetch(t, rr, "/en/area/high").Body.String()

	for _, want := range []string{
		`<section class="area-now-section" aria-labelledby="area-now-h">`,
		`<h2 id="area-now-h"`,
		`<section class="area-chart" aria-labelledby="chart-heading">`,
		`<section class="area-sensors-sec" aria-labelledby="sensors-h">`,
		`<h2 id="sensors-h">Sensors</h2>`,
		`<h2 id="area-map-h" class="visually-hidden">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the area page lacks %q", want)
		}
	}
	sec := body[strings.Index(body, `id="sensors-h"`):]
	if !strings.Contains(sec, `data-nearby`) || !strings.Contains(sec, `data-area-sensors`) {
		t.Error("the sensors section does not hold both lists")
	}
}

// PR #21: an official station can show a real value while its flag is
// source_invalid. The panel needs the label to render it, not blank text.
func TestSensorPanelCarriesTheSourceInvalidFlagLabel(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	body := fetch(t, rr, "/en/area/high").Body.String()

	if !strings.Contains(body, `data-t-flag-source-invalid="The newest reading was rejected by its source. This is the last accepted one."`) {
		t.Error("the area page is missing the source_invalid flag label")
	}
}

// The no-coverage notice does NOT travel with the chart. It says why the page
// has no numbers, so it belongs beside the readouts it explains — printed at
// the foot it would arrive after the reader has already given up.
func TestNoCoverageNoticeStaysAboveTheMap(t *testing.T) {
	rr := renderer(t, fixture(t))
	body := fetch(t, rr, "/en/area/vidin").Body.String()

	noticeAt := strings.Index(body, `class="notice"`)
	mapAt := strings.Index(body, `id="area-map"`)
	if noticeAt < 0 {
		t.Fatal("the uncovered area lost its no-coverage notice")
	}
	if mapAt >= 0 && noticeAt > mapAt {
		t.Errorf("the no-coverage notice sank below the map: notice=%d map=%d", noticeAt, mapAt)
	}
}

// The readings strip comes before the map shell on desktop: the reader sees
// how much first, then where. Phones lift the map above it with CSS order.
func TestReadoutsStripComesBeforeTheMap(t *testing.T) {
	rr := renderer(t, rankingSnapshot())
	body := fetch(t, rr, "/en/area/high").Body.String()

	mapAt := strings.Index(body, `id="area-map"`)
	readoutAt := strings.Index(body, `data-island="readouts"`)

	if mapAt < 0 {
		t.Fatal("the area page lost its map")
	}
	if readoutAt < 0 {
		t.Fatal("the area page lost its readouts strip")
	}
	if readoutAt > mapAt {
		t.Errorf("the readouts strip comes after the map: readouts=%d map=%d", readoutAt, mapAt)
	}
}
