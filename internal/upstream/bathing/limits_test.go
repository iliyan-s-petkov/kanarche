package bathing_test

import (
	"testing"

	"kanarche.eu/internal/upstream/bathing"
)

// Annex I of 2006/7/EC; the panel marks samples against these.
func TestLimitsMatchTheDirective(t *testing.T) {
	want := map[string]bathing.Limits{
		"coastal": {EColi: [2]int{250, 500}, Enterococci: [2]int{100, 200}},
		"lake":    {EColi: [2]int{500, 1000}, Enterococci: [2]int{200, 400}},
	}
	for zone, w := range want {
		if got := bathing.LimitsByZone[zone]; got != w {
			t.Errorf("%s limits = %+v, want %+v", zone, got, w)
		}
	}
	if len(bathing.LimitsByZone) != len(want) {
		t.Errorf("LimitsByZone has %d zones, want %d", len(bathing.LimitsByZone), len(want))
	}
}
