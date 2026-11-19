package snapshot

import (
	"sort"
	"time"

	"kanarche.eu/internal/store"
)

// StationReading is the latest PM2.5, PM10, NO2 and O3 of one official station and when it last reported.
// A station with no fresh reading has no Values and keeps the time the registry last saw it.
type StationReading struct {
	Values   map[string]float64
	LastSeen time.Time
}

// stationMetrics are the metrics an area page tabulates for an official station.
var stationMetrics = []string{"P2", "P1", "NO2", "O3"}

// stationReadingsFrom keys every registry station by its EoI code, then fills it from the fresh readings.
// A station can have several devices: a metric comes from the first device that has it, the time is the newest.
func stationReadingsFrom(registry, fresh []store.SensorReading) map[string]StationReading {
	out := map[string]StationReading{}
	add := func(list []store.SensorReading) {
		for _, sr := range list {
			if sr.Source != "eea" || sr.StationCode == "" {
				continue
			}
			r := out[sr.StationCode]
			if r.Values == nil {
				r.Values = map[string]float64{}
			}
			for _, m := range stationMetrics {
				if v, ok := sr.Values[m]; ok {
					if _, has := r.Values[m]; !has {
						r.Values[m] = v
					}
				}
			}
			if sr.LastSeen.After(r.LastSeen) {
				r.LastSeen = sr.LastSeen
			}
			out[sr.StationCode] = r
		}
	}
	add(registry)
	add(fresh)
	return out
}

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
