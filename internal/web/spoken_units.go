package web

import (
	"encoding/json"
	"html/template"
	"strings"
)

// spokenUnitKeys maps a printed unit to the catalogue key holding the words a
// screen reader should say for it. Keyed by the symbol, not the metric, so a
// new metric that reuses µg/m³ is covered without an entry.
var spokenUnitKeys = map[string]string{
	"µg/m³": "unit.spoken.ugm3",
	"°C":    "unit.spoken.celsius",
	"%":     "unit.spoken.percent",
	"hPa":   "unit.spoken.hpa",
	"dB(A)": "unit.spoken.dba",
}

// spokenWords is the spoken form of a printed unit, or "" when none is defined.
func (p PageData) spokenWords(visible string) string {
	if key, ok := spokenUnitKeys[visible]; ok {
		return p.T(key)
	}
	return ""
}

// SpokenUnit is the words for the metric's unit, falling back to the printed
// symbol so a caller never ends up with nothing to say.
func (p PageData) SpokenUnit(metric string) string {
	visible := p.T("unit." + metric)
	if words := p.spokenWords(visible); words != "" {
		return words
	}
	return visible
}

// unitMarkup prints the symbol for the eye and hides it from the speech layer,
// with the words beside it for a screen reader. The leading space keeps
// textContent readable when the visible span is stripped.
func (p PageData) unitMarkup(metric string) string {
	visible := p.T("unit." + metric)
	words := p.spokenWords(visible)
	if words == "" {
		return htmlEscape(visible)
	}
	return `<span aria-hidden="true">` + htmlEscape(visible) + `</span><span class="visually-hidden"> ` + htmlEscape(words) + `</span>`
}

// UnitHTML is unitMarkup for templates.
func (p PageData) UnitHTML(metric string) template.HTML { return template.HTML(p.unitMarkup(metric)) }

// readoutUnitSpoken is the words for a readout cell's unit, "" when they would
// only repeat the symbol.
func (p PageData) readoutUnitSpoken(metric string) string {
	return p.spokenWords(p.T("unit." + metric))
}

// SpokenUnitsAttr is the symbol to words table the browser reads from the
// document element, so aria-labels it builds speak the same words.
func (p PageData) SpokenUnitsAttr() string {
	out := make(map[string]string, len(spokenUnitKeys))
	for symbol, key := range spokenUnitKeys {
		out[symbol] = p.T(key)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// homeDirKeys are the compass words the near-you sentence is built from in the
// browser, in the order nearbylist.js indexes them.
var homeDirKeys = []string{"n", "ne", "e", "se", "s", "sw", "w", "nw"}

// HomeNowHTML is the home page's text summary: the country's air-now sentence
// and the highest province. It is for the speech layer, the readouts above it
// carry the same figures on screen. The second sentence sits in its own live
// node and the browser swaps it for the near-you sentence after a Find me fix,
// using the copy carried on the paragraph.
func (p PageData) HomeNowHTML() template.HTML {
	if len(p.Areas) == 0 {
		return ""
	}
	values := make([]float64, 0, len(p.Areas))
	top, topValue, sensors := "", 0.0, 0
	for _, a := range p.Areas {
		if a.Covered {
			sensors += a.SensorCount
		}
		if !a.HasValue {
			continue
		}
		if len(values) == 0 || a.Value > topValue {
			topValue, top = a.Value, a.Name
		}
		values = append(values, a.Value)
	}

	row := AreaRow{Kind: "country", Covered: len(values) > 0, SensorCount: sensors}
	if len(values) > 0 {
		row.HasValue, row.Value = true, medianOf(values)
	}

	second := ""
	if len(values) > 0 && !p.stale() {
		second = p.T("home.now_worst")
		second = strings.NewReplacer(
			"{area}", htmlEscape(top),
			"{value}", htmlEscape(formatValue(topValue, p.Lang)),
			"{unit}", p.unitMarkup(p.DefaultMetric),
			"{band}", htmlEscape(p.T("band."+bandLabel(p.DefaultMetric, topValue))),
		).Replace(second)
	}

	var attrs strings.Builder
	attr := func(name, value string) {
		attrs.WriteString(` ` + name + `="` + htmlEscape(value) + `"`)
	}
	attr("data-t-near-km", p.T("home.near.km"))
	attr("data-t-near-m", p.T("home.near.m"))
	for _, d := range homeDirKeys {
		attr("data-t-dir-"+d, p.T("nearby.dir."+d))
	}

	return template.HTML(`<p class="visually-hidden" id="home-summary" data-home-summary` + attrs.String() + `>` +
		p.areaNowText(row) + ` <span data-home-second aria-live="polite">` + second + `</span></p>`)
}
