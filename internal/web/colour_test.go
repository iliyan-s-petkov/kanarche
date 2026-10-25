package web

import (
	"testing"

	"kanarche.eu/internal/upstream"
)

// bandColour is a second copy of a rule web/src/lib/colour.js already applies to
// the map's dots, so these cases are that file's cases: the boundary is
// INCLUSIVE and belongs to the lower band, the top band is open, and anything
// the scale does not claim gets no colour rather than the nearest one.
func TestBandColourPicksTheBandTheValueFallsIn(t *testing.T) {
	cases := []struct {
		name   string
		metric string
		value  float64
		want   string
	}{
		{"the first band", "P2", 1, "#50f0e6"},
		// 5 is EAQI PM2.5's first upper bound. On the boundary is IN the lower
		// band, which is the one rule the two copies could most easily disagree
		// about without anyone noticing.
		{"exactly on a boundary stays below it", "P2", 5, "#50f0e6"},
		{"just past a boundary moves up", "P2", 5.1, "#50ccaa"},
		{"the open top band", "P2", 10000, "#7d2181"},
		// PM10's table is not PM2.5's: 25 is "Fair" for PM10 and "Moderate" for
		// PM2.5. A lookup that ignored the metric would paint both the same.
		{"the metric picks the table", "P1", 25, "#50ccaa"},
		{"and the same value bands differently for PM2.5", "P2", 25, "#f0e641"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := bandColour(c.metric, c.value); got != c.want {
				t.Errorf("bandColour(%q, %v) = %q, want %q", c.metric, c.value, got, c.want)
			}
		})
	}
}

// A metric no table claims gets no colour rather than the nearest one: a
// colour here would be a class the scale never published, and the caller draws
// no swatch instead.
func TestBandColourRefusesAMetricWithNoScale(t *testing.T) {
	for _, metric := range []string{"", "durP1", "signal"} {
		if got := bandColour(metric, 20); got != "" {
			t.Errorf("bandColour(%q, 20) = %q, want no colour", metric, got)
		}
	}
}

// Every metric the store keeps now has a table (api.Scales), so every row this
// package renders can carry a swatch. Before the official layer, five of the
// seven community metrics drew none.
func TestBandColourCoversEveryCanonicalMetric(t *testing.T) {
	for _, metric := range upstream.CanonicalMetrics() {
		if got := bandColour(metric, 20); got == "" {
			t.Errorf("bandColour(%q, 20) is empty; the metric has no band table", metric)
		}
	}
}

// The first table published for a metric, matched on the metric and not on
// position — the same choice bandsFor makes in web/src/islands/map.js.
func TestBandColourUsesTheSameTableTheMapDoes(t *testing.T) {
	if got, want := bandColour("P2", 20), "#f0e641"; got != want {
		t.Errorf("bandColour(P2, 20) = %q, want the EAQI band %q", got, want)
	}
}

// A health band exists only where an index publishes one. PM and the three EAQI
// gases have it; benzene and CO are axis-only tables, weather and noise have
// their own word lists. A bar and a band word on those would claim a health class.
func TestHealthBandOnlyForEAQIMetrics(t *testing.T) {
	cases := []struct {
		name       string
		metric     string
		value      float64
		wantColour string
		wantKey    string
	}{
		{"pm25 moderate", "P2", 19.6, "#f0e641", "moderate"},
		{"pm25 boundary stays low", "P2", 5, "#50f0e6", "good"},
		{"pm10 fair", "P1", 25, "#50ccaa", "fair"},
		{"no2 good", "NO2", 5, "#50f0e6", "good"},
		{"benzene has no health band", "C6H6", 1.4, "", ""},
		{"carbon monoxide has no health band", "CO", 2190, "", ""},
		{"temperature has no health band", "temperature", 11.6, "", ""},
		{"noise has its own words, not health ones", "noise_LA_max", 60, "", ""},
		{"unknown metric", "not_a_metric", 1, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			colour, key := healthBand(c.metric, c.value)
			if colour != c.wantColour || key != c.wantKey {
				t.Errorf("healthBand(%q, %v) = %q, %q; want %q, %q", c.metric, c.value, colour, key, c.wantColour, c.wantKey)
			}
		})
	}
}
