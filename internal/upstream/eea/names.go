package eea

import (
	"embed"
	"encoding/json"
	"sort"
)

//go:embed bg_station_names.json
var bgStationNamesFile embed.FS

// bgStationName is one row of the ExEA-to-Bulgarian-name join. See
// README.md for how the table was built and why it is a committed file
// rather than a runtime fetch.
type bgStationName struct {
	EoI     string `json:"eoi"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Type    string `json:"type"`
	Area    string `json:"area"`
}

var bgStationNames = loadBGStationNames()

func loadBGStationNames() map[string]bgStationName {
	raw, err := bgStationNamesFile.ReadFile("bg_station_names.json")
	if err != nil {
		panic("eea: bg_station_names.json: " + err.Error())
	}
	var rows []bgStationName
	if err := json.Unmarshal(raw, &rows); err != nil {
		panic("eea: bg_station_names.json: " + err.Error())
	}
	out := make(map[string]bgStationName, len(rows))
	for _, r := range rows {
		out[r.EoI] = r
	}
	return out
}

// applyBGStationName overwrites the metadata file's national code with the
// Bulgarian name from the committed join table. A station absent from the
// table (outside the 32 covered on 2026-09-09) keeps the national code it
// already has, so a future station is visible rather than blank.
func applyBGStationName(st Station) Station {
	n, ok := bgStationNames[st.Code]
	if !ok {
		return st
	}
	st.Name = n.Name
	return st
}

// StationInfo is the committed name and classification of one ИАОС station.
type StationInfo struct {
	Code string
	Name string
	Type string
	Area string
}

// LookupStationInfo returns the committed row for an EoI code such as BG0038A.
func LookupStationInfo(code string) (StationInfo, bool) {
	n, ok := bgStationNames[code]
	if !ok {
		return StationInfo{}, false
	}
	return StationInfo{Code: n.EoI, Name: n.Name, Type: n.Type, Area: n.Area}, true
}

// StationCodes lists every committed EoI code, sorted.
func StationCodes() []string {
	out := make([]string, 0, len(bgStationNames))
	for code := range bgStationNames {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}
