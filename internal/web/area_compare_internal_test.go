package web

import (
	"testing"
	"time"
)

// The kind filter lives in areaCompareHTML itself, not only in the handler.
func TestAreaCompareHTMLKindFilter(t *testing.T) {
	rr := acRenderer(t)
	t0 := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	p := acPageData(rr, "en", t0, t0)
	row := AreaRow{Covered: true, HasValue: true, Value: 12, Day24Mean: map[string]float64{"P2": 18, "P1": 31}}
	for kind, want := range map[string]bool{"city": true, "neighbourhood": false, "oblast": false} {
		row.Kind = kind
		if got := p.areaCompareHTML(row) != ""; got != want {
			t.Errorf("kind %q: rendered = %v, want %v", kind, got, want)
		}
	}
}
