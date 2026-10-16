package web_test

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"testing"

	"kanarche.eu/internal/snapshot"
)

// sensorsFixture places three sensors: two in Sofia districts, one in the city with no district.
func sensorsFixture(t *testing.T) *snapshot.Snapshot {
	t.Helper()
	snap := areaPageFixture(t)
	snap.SensorLocations = map[int64]snapshot.SensorLocation{
		101: {SensorID: 101, Slug: "mladost"},
		102: {SensorID: 102, Slug: "lozenets"},
		103: {SensorID: 103, Slug: "sofiya"},
		201: {SensorID: 201, Slug: "plovdiv"},
	}
	return snap
}

var sensorsHostRe = regexp.MustCompile(`(?s)<div class="area-sensors-host"[^>]*>`)

func sensorsHost(t *testing.T, body string) string {
	t.Helper()
	return sensorsHostRe.FindString(body)
}

func attr(tag, name string) (string, bool) {
	m := regexp.MustCompile(` ` + regexp.QuoteMeta(name) + `="([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		return "", false
	}
	return html.UnescapeString(m[1]), true
}

// A11Y-12: the area map's description names content that is really on the page.
func TestAreaMapTextAlternativeNamesTheSensorTable(t *testing.T) {
	body := fetch(t, renderer(t, sensorsFixture(t)), "/en/area/sofiya-grad-oblast").Body.String()
	i := strings.Index(body, `id="area-map-alt"`)
	if i < 0 {
		t.Fatal("the area map lost its description")
	}
	para := body[i : i+200]
	if strings.Contains(para, "provinces") {
		t.Errorf("the description still points at a province list the page does not have: %q", para)
	}
	if !strings.Contains(para, "The table of sensors below the map carries the same readings as text.") {
		t.Errorf("the description does not name the sensor table: %q", para)
	}
	if sensorsHost(t, body) == "" {
		t.Error("the description names a sensor table the page does not render")
	}
	if strings.Index(body, `class="area-sensors-host"`) < strings.Index(body, `id="area-map"`) {
		t.Error("the sensor table is not below the map")
	}
}

// D3: the shell carries every label the table needs, in the page language.
func TestAreaSensorTableShellCarriesItsCopy(t *testing.T) {
	host := sensorsHost(t, fetch(t, renderer(t, sensorsFixture(t)), "/en/area/sofiya-grad-oblast").Body.String())
	if host == "" {
		t.Fatal("no sensor table host on a covered area page")
	}
	want := map[string]string{
		"data-t-summary":     "All sensors in this area ({n})",
		"data-t-summary-one": "The sensor in this area",
		"data-t-caption":     "Sensors in {area} and their latest readings",
		"data-area":          "Sofia-City",
		"data-t-col-sensor":  "Sensor",
		"data-t-col-value":   "{metric}, {unit}",
		"data-t-col-band":    "Air quality",
		"data-t-col-updated": "Updated",
		"data-t-open":        "Show sensor {id} on the map",
		"data-t-filter":      "Filter sensors",
		"data-t-status":      "Showing {from} to {to} of {total} sensors",
		"data-t-empty":       "No sensor matches this filter.",
		"data-t-none":        "No live sensors in this area.",
		"data-t-range-one":   "Showing 1 sensor",
		"data-t-page-next":   "Next page",
		"data-t-per-page":    "Rows per page",
		"data-t-p2":          "PM2.5",
		"data-t-unit-p1":     "µg/m³",
	}
	for name, v := range want {
		got, ok := attr(host, name)
		if !ok || got != v {
			t.Errorf("%s = %q (present %v), want %q", name, got, ok, v)
		}
	}
	if !strings.Contains(host, " hidden") {
		t.Error("the host shows before the script fills it")
	}
}

// District names are server data: the sensors body carries no area column.
func TestAreaSensorTableNamesSofiaDistricts(t *testing.T) {
	rr := renderer(t, sensorsFixture(t))
	host := sensorsHost(t, fetch(t, rr, "/en/area/sofiya-grad-oblast").Body.String())
	raw, ok := attr(host, "data-districts")
	if !ok {
		t.Fatal("no district map on a Sofia page")
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("data-districts is not JSON: %v (%q)", err, raw)
	}
	if got["101"] != "Mladost" || got["102"] != "Lozenets" || len(got) != 2 {
		t.Errorf("districts = %v, want 101 Mladost and 102 Lozenets only", got)
	}
	if v, _ := attr(host, "data-t-col-district"); v != "District" {
		t.Errorf("district header = %q", v)
	}

	for _, path := range []string{"/en/area/plovdiv-oblast", "/en/area/mladost"} {
		h := sensorsHost(t, fetch(t, rr, path).Body.String())
		if h == "" {
			t.Fatalf("%s: no sensor table host", path)
		}
		if _, ok := attr(h, "data-districts"); ok {
			t.Errorf("%s: a district column where it says nothing", path)
		}
	}
}

func TestAreaSensorTableBulgarianCopy(t *testing.T) {
	host := sensorsHost(t, fetch(t, renderer(t, sensorsFixture(t)), "/area/sofiya-grad-oblast").Body.String())
	if v, _ := attr(host, "data-t-summary"); v != "Всички сензори в района ({n})" {
		t.Errorf("bg summary = %q", v)
	}
	raw, _ := attr(host, "data-districts")
	if !strings.Contains(raw, "Младост") {
		t.Errorf("bg districts = %q", raw)
	}
}

func TestUncoveredAreaHasNoSensorTable(t *testing.T) {
	body := fetch(t, renderer(t, sensorsFixture(t)), "/en/area/silistra-oblast").Body.String()
	if sensorsHost(t, body) != "" {
		t.Error("an area with no sensors renders a sensor table")
	}
}
