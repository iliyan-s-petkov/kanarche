package web

import (
	"strconv"
	"strings"

	"kanarche.eu/internal/api"
)

// HomeAboutPM is the map home's PM explainer. Its PM2.5 thresholds are the EAQI
// upper edges read from api.Scales(), the table the map colours by, so the copy
// cannot drift from the legend; the catalogue carries the five placeholders.
func (p PageData) HomeAboutPM() string {
	e := pm25Edges()
	return strings.NewReplacer(
		"{good}", e[0],
		"{fair}", e[1],
		"{moderate}", e[2],
		"{poor}", e[3],
		"{verypoor}", e[4],
	).Replace(p.T("home.about_pm"))
}

// pm25Edges returns the finite upper edges of the PM2.5 EAQI table (good through
// very poor). The last band is open-ended, so it contributes no edge.
func pm25Edges() [5]string { return eaqiEdges("P2") }

// pm10Edges is the PM10 twin of pm25Edges.
func pm10Edges() [5]string { return eaqiEdges("P1") }

func eaqiEdges(metric string) [5]string {
	var out [5]string
	for _, s := range api.Scales() {
		if s.Metric != metric || s.Name != "eaqi" {
			continue
		}
		for i, b := range s.Bands {
			if i >= len(out) || b.Upper == nil {
				break
			}
			out[i] = strconv.FormatFloat(*b.Upper, 'f', -1, 64)
		}
		break
	}
	return out
}

// bandColour is the table's swatch colour for one reading: the first band whose
// inclusive upper bound is at or above the value, from the first scale table
// published for that metric.
//
// It is the same rule web/src/lib/colour.js applies to the map's dots, and the
// same table selection web/src/islands/map.js's bandsFor makes — deliberately,
// because a province coloured one way as a dot and another way as a row would
// be the site disagreeing with itself on one screen. The rule lives twice
// because the audiences differ: the map colours GeoJSON in the browser from
// /api/v1/scales, while this row is rendered server-side and must be coloured
// with no JavaScript at all. colour_test.go pins the two copies to the same
// boundary cases.
//
// An empty string, not a colour, when there is no band table for the metric or
// no value: a swatch drawn anyway would assert a class the scale does not
// claim. The caller renders the chip without a swatch instead.
// bandLabelKeys maps api.Scales' English band labels to the catalogue key
// suffix under "band." — the words themselves live in bg.json/en.json (§4 of
// the SEO6 plan), not in LabelBG, so a third language is a catalogue file and
// not a code change here.
var bandLabelKeys = map[string]string{
	"Good":           "good",
	"Fair":           "fair",
	"Moderate":       "moderate",
	"Poor":           "poor",
	"Very poor":      "very_poor",
	"Extremely poor": "extremely_poor",
}

// bandLabel is bandColour's counterpart for the area-page sentence: the same
// band lookup, returning the catalogue key suffix instead of a colour. Empty
// when the metric has no scale, exactly like bandColour.
func bandLabel(metric string, value float64) string {
	for _, scale := range api.Scales() {
		if scale.Metric != metric {
			continue
		}
		for _, band := range scale.Bands {
			if band.Upper == nil || value <= *band.Upper {
				return bandLabelKeys[band.Label]
			}
		}
		return ""
	}
	return ""
}

// bandClass is the index of the band value falls in, best first, or -1 when the
// metric has no scale. The table chip's outline weight is keyed on it (A11Y-05).
func bandClass(metric string, value float64) int {
	for _, scale := range api.Scales() {
		if scale.Metric != metric {
			continue
		}
		for i, band := range scale.Bands {
			if band.Upper == nil || value <= *band.Upper {
				return i
			}
		}
		return -1
	}
	return -1
}

func bandColour(metric string, value float64) string {
	for _, scale := range api.Scales() {
		if scale.Metric != metric {
			continue
		}
		for _, band := range scale.Bands {
			// Upper is INCLUSIVE, so a value exactly on a boundary belongs to
			// the lower band; a nil Upper is the open-ended top.
			if band.Upper == nil || value <= *band.Upper {
				return band.Colour
			}
		}
		// Only reachable from a scale with no open top band, which would be a
		// server bug. No colour rather than the last band's, for the reason
		// above: better to show nothing than to claim a class.
		return ""
	}
	return ""
}

// healthBand is the colour and the catalogue key suffix of the health class a
// reading falls in, for the bar and the word under a readout card. Only the
// EAQI tables are health classes: benzene and CO carry an axis, weather and
// noise carry their own word lists, and a bar there would claim a class no
// index publishes. Both results are empty for those, and for an unknown metric.
func healthBand(metric string, value float64) (colour, key string) {
	for _, scale := range api.Scales() {
		if scale.Metric != metric {
			continue
		}
		if scale.Name != "eaqi" {
			return "", ""
		}
		for _, band := range scale.Bands {
			if band.Upper == nil || value <= *band.Upper {
				return band.Colour, bandLabelKeys[band.Label]
			}
		}
		return "", ""
	}
	return "", ""
}
