package web_test

import (
	"html"
	"regexp"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/snapshot"
)

var (
	officialTableRE = regexp.MustCompile(`(?s)<table class="table official-stations__table">.*?</table>`)
	officialRowRE   = regexp.MustCompile(`(?s)<tr>(.*?)</tr>`)
	officialCellRE  = regexp.MustCompile(`(?s)<t[hd][^>]*>(.*?)</t[hd]>`)
)

// officialSnapshot is Shumen with two official stations inside: BG0038A reports PM2.5, PM10 and NO2 but no ozone,
// BG0050A has no fresh reading. BG0052A reports too but stands outside the area.
func officialSnapshot() *snapshot.Snapshot {
	snap := factsSnapshot()
	m := snap.KnownSlugs["shumen"]
	m.Stations = []string{"BG0038A", "BG0050A"}
	snap.KnownSlugs["shumen"] = m
	snap.StationReadings = map[string]snapshot.StationReading{
		"BG0038A": {Values: map[string]float64{"P2": 14.2, "P1": 20, "NO2": 18}, LastSeen: time.Date(2026, 8, 9, 11, 50, 0, 0, time.UTC)},
		"BG0050A": {},
		"BG0052A": {Values: map[string]float64{"P2": 99}, LastSeen: time.Date(2026, 8, 9, 11, 50, 0, 0, time.UTC)},
	}
	return snap
}

// officialRows is the table's rows as cell texts, header row first. It fails when the page has no table.
func officialRows(t *testing.T, body string) [][]string {
	t.Helper()
	tbl := officialTableRE.FindString(body)
	if tbl == "" {
		t.Fatalf("no official stations table in page")
	}
	var rows [][]string
	for _, r := range officialRowRE.FindAllStringSubmatch(tbl, -1) {
		var cells []string
		for _, c := range officialCellRE.FindAllStringSubmatch(r[1], -1) {
			cells = append(cells, strings.TrimSpace(html.UnescapeString(tagRE.ReplaceAllString(c[1], ""))))
		}
		rows = append(rows, cells)
	}
	return rows
}

func TestAreaOfficialTableListsStationsInsideTheArea(t *testing.T) {
	body := fetch(t, renderer(t, officialSnapshot()), "/area/shumen").Body.String()
	rows := officialRows(t, body)
	if len(rows) != 3 {
		t.Fatalf("rows = %v, want a header and two stations", rows)
	}
	if strings.Contains(body, "BG0052A") || strings.Contains(strings.Join(rows[1], "|"), "99") {
		t.Errorf("a station outside the area is listed: %v", rows)
	}
	if got := rows[1][:5]; got[0] != "АИС Шумен" || got[1] != "14,2" || got[2] != "20,0" || got[3] != "18,0" || got[4] != "няма измерване" {
		t.Errorf("fresh station row = %v, want АИС Шумен, 14,2, 20,0, 18,0, no reading (ozone)", got)
	}
}

// A station with no fresh reading keeps its row, with no values and an empty updated cell.
func TestAreaOfficialTableKeepsAStaleStation(t *testing.T) {
	body := fetch(t, renderer(t, officialSnapshot()), "/area/shumen").Body.String()
	rows := officialRows(t, body)
	stale := rows[2]
	if !strings.Contains(stale[0], "Хиподрума") {
		t.Fatalf("second row = %v, want the stale station", stale)
	}
	for i := 1; i <= 4; i++ {
		if stale[i] != "няма измерване" {
			t.Errorf("stale cell %d = %q, want no reading", i, stale[i])
		}
	}
	if len(stale) != 6 || stale[5] != "" {
		t.Errorf("stale updated cell = %q (row %v), want an empty cell", stale[len(stale)-1], stale)
	}
}

func TestAreaOfficialTableHeadingCountsStations(t *testing.T) {
	for path, want := range map[string]string{
		"/area/shumen":    "Официални станции (2)",
		"/en/area/shumen": "Official stations (2)",
	} {
		body := fetch(t, renderer(t, officialSnapshot()), path).Body.String()
		if !strings.Contains(body, ">"+want+"</h3>") {
			t.Errorf("%s: no h3 %q", path, want)
		}
		// The official table comes before the in-view list.
		if i, j := strings.Index(body, want), strings.Index(body, `class="sensors-h3"`); j >= 0 && i > j {
			t.Errorf("%s: official heading is below the sensor list", path)
		}
	}
}

func TestAreaWithoutOfficialStationsHasNoOfficialTable(t *testing.T) {
	snap := officialSnapshot()
	m := snap.KnownSlugs["shumen"]
	m.Stations = nil
	snap.KnownSlugs["shumen"] = m
	for _, path := range []string{"/area/shumen", "/en/area/shumen", "/area/pleven"} {
		body := fetch(t, renderer(t, snap), path).Body.String()
		if strings.Contains(body, "official-stations__") || strings.Contains(body, "Официални станции (") || strings.Contains(body, "Official stations (") {
			t.Errorf("%s renders an official stations block for an area without stations", path)
		}
	}
}
