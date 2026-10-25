package snapshot

import (
	"sort"

	"kanarche.eu/internal/store"
)

// OfficialStation is where one ИАОС station stands, keyed by its EoI code. The name and classification live
// in the committed table in the eea package; the position is what the snapshot adds.
type OfficialStation struct {
	Code     string
	Lon, Lat float64
}

// officialStationsFrom lists every EEA station among the sensors by its EoI code.
func officialStationsFrom(sensors []store.SensorReading) map[string]OfficialStation {
	out := map[string]OfficialStation{}
	for _, sr := range sensors {
		if sr.Source != "eea" || sr.StationCode == "" {
			continue
		}
		out[sr.StationCode] = OfficialStation{Code: sr.StationCode, Lon: sr.Lon, Lat: sr.Lat}
	}
	return out
}

// stationCodesByArea lists the EEA station codes inside each area, sorted. The area link comes from the
// database's point-in-polygon test, so a station is inside every area that contains it.
func stationCodesByArea(sensors []store.SensorReading) map[string][]string {
	seen := map[string]map[string]bool{}
	for _, sr := range sensors {
		if sr.Source != "eea" || sr.StationCode == "" {
			continue
		}
		for _, slug := range sr.AreaSlugs {
			if seen[slug] == nil {
				seen[slug] = map[string]bool{}
			}
			seen[slug][sr.StationCode] = true
		}
	}
	out := make(map[string][]string, len(seen))
	for slug, codes := range seen {
		list := make([]string, 0, len(codes))
		for c := range codes {
			list = append(list, c)
		}
		sort.Strings(list)
		out[slug] = list
	}
	return out
}
