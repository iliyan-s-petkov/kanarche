package web

import (
	"encoding/json"
	"html/template"
	"sort"
	"strconv"
	"strings"
	"time"

	"kanarche.eu/internal/snapshot"
)

// maxStaticStations caps the server-rendered station list. Larger areas
// (a whole oblast) would otherwise add hundreds of rows to the HTML.
const maxStaticStations = 50

// StationRow is one station in the server-rendered list.
type StationRow struct {
	ID      int64
	P2, P1  string
	Updated template.HTML
}

// StationList is the no-JS copy of the area's sensor table.
type StationList struct {
	Rows  []StationRow
	Total int
	Shown int
}

// sensorColumnsIn reads only the columns the list needs from the snapshot's
// columnar sensor body.
type sensorColumnsIn struct {
	Sensors struct {
		ID       []int64      `json:"id"`
		Station  []int64      `json:"station"`
		P2       []*float64   `json:"P2"`
		P1       []*float64   `json:"P1"`
		LastSeen []*time.Time `json:"last_seen"`
	} `json:"sensors"`
}

// stationList groups devices by station like web/src/lib/stations.js does: the
// first device with a reading supplies the value, the newest last_seen the time.
// Rows are ordered by station id, as in the island's table.
func (p PageData) stationList(snap *snapshot.Snapshot, slug string) *StationList {
	body, ok := snap.AreaSensors[slug]
	if !ok || len(body.JSON) == 0 {
		return nil
	}
	var in sensorColumnsIn
	if err := json.Unmarshal(body.JSON, &in); err != nil {
		return nil
	}
	c := in.Sensors
	type agg struct {
		p2, p1 *float64
		seen   time.Time
	}
	by := map[int64]*agg{}
	for i, id := range c.ID {
		st := id
		if i < len(c.Station) {
			st = c.Station[i]
		}
		a := by[st]
		if a == nil {
			a = &agg{}
			by[st] = a
		}
		if a.p2 == nil && i < len(c.P2) {
			a.p2 = c.P2[i]
		}
		if a.p1 == nil && i < len(c.P1) {
			a.p1 = c.P1[i]
		}
		if i < len(c.LastSeen) && c.LastSeen[i] != nil && c.LastSeen[i].After(a.seen) {
			a.seen = *c.LastSeen[i]
		}
	}
	if len(by) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(by))
	for id := range by {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	out := &StationList{Total: len(ids)}
	if len(ids) > maxStaticStations {
		ids = ids[:maxStaticStations]
	}
	out.Shown = len(ids)
	noValue := p.T("panel.no_value")
	val := func(v *float64) string {
		if v == nil {
			return noValue
		}
		return formatValue(*v, p.Lang)
	}
	for _, id := range ids {
		a := by[id]
		row := StationRow{ID: id, P2: val(a.p2), P1: val(a.p1)}
		if !a.seen.IsZero() {
			row.Updated = template.HTML(timeTag(a.seen, p.sofiaLayout(a.seen)))
		}
		out.Rows = append(out.Rows, row)
	}
	return out
}

// Summary is the details heading, reusing the island's own strings.
func (l StationList) Summary(p PageData) string {
	if l.Total == 1 {
		return p.T("area.sensors.summary_one")
	}
	return strings.ReplaceAll(p.T("area.sensors.summary"), "{n}", strconv.Itoa(l.Total))
}

// Caption names the area in the table caption.
func (l StationList) Caption(p PageData) string {
	return strings.ReplaceAll(p.T("area.sensors.caption"), "{area}", p.Area.Name)
}

// Capped says how many of the stations are listed, "" when all of them are.
func (l StationList) Capped(p PageData) string {
	if l.Shown >= l.Total {
		return ""
	}
	r := strings.NewReplacer("{from}", "1", "{to}", strconv.Itoa(l.Shown), "{total}", strconv.Itoa(l.Total))
	return r.Replace(p.T("area.sensors.status"))
}
