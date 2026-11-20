package web

import (
	"html/template"
	"strconv"
	"strings"

	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/upstream/eea"
)

// OfficialRow is one official station in the area page's table. The cells are already formatted.
type OfficialRow struct {
	Name            string
	P2, P1, NO2, O3 string
	Updated         template.HTML
}

// OfficialList is the table of the official stations inside an area.
type OfficialList struct {
	Rows []OfficialRow
}

// Heading is "Официални станции (n)" with the number of rows.
func (l OfficialList) Heading(p PageData) string {
	return strings.ReplaceAll(p.T("area.official.heading"), "{n}", strconv.Itoa(len(l.Rows)))
}

// officialList lists every registry station inside the area, in the registry's order. A station with no fresh
// reading keeps its row: its cells say there is no measurement and its updated cell stays empty.
// It is nil when the area has no official stations.
func (p PageData) officialList(meta snapshot.AreaMeta, snap *snapshot.Snapshot) *OfficialList {
	noValue := p.T("panel.no_value")
	var rows []OfficialRow
	for _, code := range meta.Stations {
		info, ok := eea.LookupStationInfo(code)
		if !ok {
			continue
		}
		reading := snap.StationReadings[code]
		val := func(metric string) string {
			if v, has := reading.Values[metric]; has {
				return formatValue(v, p.Lang)
			}
			return noValue
		}
		row := OfficialRow{Name: stationName(info, p.Lang), P2: val("P2"), P1: val("P1"), NO2: val("NO2"), O3: val("O3")}
		if !reading.LastSeen.IsZero() {
			row.Updated = template.HTML(timeTag(reading.LastSeen, p.sofiaLayout(reading.LastSeen)))
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	return &OfficialList{Rows: rows}
}
