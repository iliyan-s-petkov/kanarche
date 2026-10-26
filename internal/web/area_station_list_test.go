package web_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"kanarche.eu/internal/snapshot"
)

var staticListRe = regexp.MustCompile(`(?s)<details class="area-sensors area-sensors--static".*?</details>`)

// stationListFixture gives sofia a columnar sensor body: devices 101 and 105
// share station 101 (PM box plus climate box), 102 has no PM10, 103 is silent.
func stationListFixture(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	snap := sensorsFixture(t)
	snap.AreaSensors = map[string]snapshot.Body{"sofia": {JSON: []byte(`{"generated_at":"2026-09-27T12:00:00Z","sensors":{
"id":[101,102,103,105],"station":[101,102,103,101],
"P2":[7.26,11.0,null,null],"P1":[12.5,null,null,null],
"last_seen":["2026-09-27T11:50:00Z","2026-09-27T11:40:00Z","2026-09-26T08:00:00Z","2026-09-27T11:55:00Z"]}}`)}}
	return snap
}

func stationList(t *testing.T, path string) string {
	t.Helper()
	body := fetch(t, renderer(t, stationListFixture(t)), path).Body.String()
	m := staticListRe.FindString(body)
	if m == "" {
		t.Fatalf("%s carries no server-rendered station list", path)
	}
	return m
}

// Crawlers that do not run JS read the stations and their latest PM from the raw HTML.
func TestAreaStationListInRawHTML(t *testing.T) {
	list := stationList(t, "/en/area/sofia")
	for _, want := range []string{
		"All sensors in this area (3)",
		`<th scope="row">101</th>`, `<th scope="row">102</th>`, `<th scope="row">103</th>`,
		`<td class="num">7.3</td>`, `<td class="num">12.5</td>`, `<td class="num">11.0</td>`,
		`<time datetime="2026-09-27T11:55:00Z">`,
	} {
		if !strings.Contains(list, want) {
			t.Errorf("station list lacks %q:\n%s", want, list)
		}
	}
	if strings.Contains(list, `<th scope="row">105</th>`) {
		t.Error("the climate box is listed apart from its station")
	}
}

// The list is the sensors section's H3: a heading inside the summary, after the H2.
func TestAreaStationListSummaryIsAnH3UnderTheSensorsHeading(t *testing.T) {
	list := stationList(t, "/en/area/sofia")
	if !strings.Contains(list, `<summary><h3 class="sensors-h3">All sensors in this area (3)</h3></summary>`) {
		t.Errorf("the summary is not an H3:\n%s", list)
	}
	body := fetch(t, renderer(t, stationListFixture(t)), "/en/area/sofia").Body.String()
	if h2, h3 := strings.Index(body, `id="sensors-h"`), strings.Index(body, `area-sensors--static`); h2 < 0 || h3 < h2 {
		t.Errorf("the list is not under the sensors heading: h2=%d list=%d", h2, h3)
	}
}

func TestAreaStationListBulgarianUsesComma(t *testing.T) {
	if list := stationList(t, "/area/sofia"); !strings.Contains(list, `<td class="num">11,0</td>`) {
		t.Errorf("BG list does not use the decimal comma:\n%s", list)
	}
}

// The island replaces the static list, so the host must not stay hidden without JS.
func TestAreaStationListHostIsVisibleWithoutJS(t *testing.T) {
	body := fetch(t, renderer(t, stationListFixture(t)), "/en/area/sofia").Body.String()
	if host := sensorsHost(t, body); strings.Contains(host, " hidden") {
		t.Errorf("the host hides the static list: %s", host)
	}
}

func TestAreaStationListIsCapped(t *testing.T) {
	snap := stationListFixture(t)
	var ids, p2, seen []string
	for i := 1; i <= 80; i++ {
		ids = append(ids, fmt.Sprint(i))
		p2 = append(p2, "5.0")
		seen = append(seen, `"2026-09-27T11:00:00Z"`)
	}
	snap.AreaSensors["sofia"] = snapshot.Body{JSON: []byte(`{"sensors":{"id":[` + strings.Join(ids, ",") +
		`],"station":[` + strings.Join(ids, ",") + `],"P2":[` + strings.Join(p2, ",") +
		`],"P1":[],"last_seen":[` + strings.Join(seen, ",") + `]}}`)}
	body := fetch(t, renderer(t, snap), "/en/area/sofia").Body.String()
	list := staticListRe.FindString(body)
	if n := strings.Count(list, `<th scope="row">`); n != 50 {
		t.Errorf("rows = %d, want 50", n)
	}
	if !strings.Contains(list, "Showing 1 to 50 of 80 sensors") {
		t.Errorf("the cap is not stated:\n%s", list)
	}
}

func TestAreaStationListAbsentWithoutSensors(t *testing.T) {
	body := fetch(t, renderer(t, sensorsFixture(t)), "/en/area/sofia").Body.String()
	if strings.Contains(body, "area-sensors--static") {
		t.Error("an empty static list was rendered")
	}
}
