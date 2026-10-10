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
// gaugeUnit is the only unit an arc is drawn for. A particulate reading starts
// at zero and gets worse, so the share of the scale it has used up is a fact
// about the air. Pressure, temperature and noise do not work that way — 1013
// hPa is an ordinary day, not 97% of a danger — and an arc there would invent
// an alarm out of a scale that only ever meant to bound an axis.
const gaugeUnit = "µg/m³"

// gaugePercent places a reading on its metric's published scale, 0-100, for the
// arc a readout card draws. The ceiling is the scale's DRAWN ceiling — the same
// top the map ramp and the legend stop at — so a full ring means the top of the
// key, not merely past the last guideline band. Scaled to the top band bound
// instead, every Bulgarian winter inversion drew an identical full ring: 51 and
// 500 looked the same. Falls back to the highest finite band for a scale that
// states no ceiling.
//
// ok is false for a metric counted in anything but gaugeUnit, and for one with
// no bands at all, so the card renders a plain figure instead.
func gaugePercent(metric string, value float64) (int, bool) {
	ceiling, found := 0.0, false
	for _, scale := range api.Scales() {
		if scale.Metric != metric {
			continue
		}
		if scale.Unit != gaugeUnit {
			return 0, false
		}
		if scale.Ceiling != nil && *scale.Ceiling > 0 {
			ceiling, found = *scale.Ceiling, true
		} else {
			ceiling, found = finiteCeiling(scale.Bands)
		}
		break
	}
	if !found || ceiling <= 0 {
		return 0, false
	}
	pct := int(value / ceiling * 100)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct, true
}

// finiteCeiling is the highest bound in the table, not the last one: nothing
// requires a published scale to arrive in ascending order, and taking the last
// would put the arc against whichever bound happened to be written at the end.
func finiteCeiling(bands []api.Band) (float64, bool) {
	top, found := 0.0, false
	for _, band := range bands {
		if band.Upper != nil && *band.Upper > top {
			top, found = *band.Upper, true
		}
	}
	return top, found
}

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
