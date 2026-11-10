package snapshot

import (
	"math"
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

// hourlyPoints returns n hourly points ending at the current hour, newest last, all with value v.
func hourlyPoints(now time.Time, n int, v float64) []store.Point {
	top := now.Truncate(time.Hour)
	pts := make([]store.Point, 0, n)
	for i := n - 1; i >= 0; i-- {
		pts = append(pts, store.Point{Time: top.Add(-time.Duration(i) * time.Hour), Value: v})
	}
	return pts
}

func TestMeanLast24Hours(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 30, 0, 0, time.UTC)

	full := hourlyPoints(now, 24, 10)
	full[0].Value = 34 // mean is (34 + 23*10) / 24 = 11
	edge := append([]store.Point{{Time: now.Truncate(time.Hour).Add(-24 * time.Hour), Value: 1000}}, hourlyPoints(now, 24, 10)...)
	cases := []struct {
		name   string
		points []store.Point
		want   float64
		ok     bool
	}{
		{"full 24", full, 11, true},
		{"18 present", hourlyPoints(now, 18, 20), 20, true},
		{"17 present", hourlyPoints(now, 17, 20), 0, false},
		{"empty", nil, 0, false},
		// The 25th bucket is the partial hour at the window edge, outside the last 24 hours.
		{"edge bucket ignored", edge, 10, true},
	}
	for _, c := range cases {
		got, ok := meanLast24Hours(c.points, now)
		if ok != c.ok || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: got (%v, %v), want (%v, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}
