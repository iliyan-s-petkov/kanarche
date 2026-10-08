package snapshot_test

import (
	"math"
	"testing"

	"kanarche.eu/internal/snapshot"
)

func nearestWindCell(cells []snapshot.WindCell, lon, lat float64) float64 {
	best := math.Inf(1)
	for _, c := range cells {
		best = math.Min(best, math.Max(math.Abs(c.Lon-lon), math.Abs(c.Lat-lat)))
	}
	return best
}

func TestWindLatticeCoversNEBulgariaAndTheDanube(t *testing.T) {
	cells := snapshot.WindLattice()
	for name, p := range map[string][2]float64{
		"Shumen": {26.9, 43.3},
		"Ruse":   {25.95, 43.85},
		"Varna":  {27.9, 43.2},
		"Sofia":  {23.3, 42.7},
	} {
		if d := nearestWindCell(cells, p[0], p[1]); d > 0.2 {
			t.Errorf("%s is %.3f deg from the nearest lattice point, want <= 0.2", name, d)
		}
	}
}

func TestWindLatticeIsAlignedToTheModelGrid(t *testing.T) {
	for _, c := range snapshot.WindLattice() {
		for _, v := range []float64{c.Lon, c.Lat} {
			if r := math.Abs(v/0.25 - math.Round(v/0.25)); r > 1e-9 {
				t.Fatalf("(%v, %v) is not a multiple of 0.25", c.Lon, c.Lat)
			}
		}
		if lon, lat := float64(c.Q)*0.25, float64(c.R)*0.25; lon != c.Lon || lat != c.Lat {
			t.Fatalf("cell (%d,%d) centre %v,%v disagrees with its index", c.Q, c.R, c.Lon, c.Lat)
		}
	}
}

// 31 longitudes (21.75-29.25) by 17 latitudes (40.75-44.75).
func TestWindLatticeCountAndUniqueness(t *testing.T) {
	cells := snapshot.WindLattice()
	if len(cells) != 31*17 {
		t.Fatalf("lattice has %d points, want %d", len(cells), 31*17)
	}
	seen := map[[2]int]bool{}
	for _, c := range cells {
		k := [2]int{c.Q, c.R}
		if seen[k] {
			t.Fatalf("duplicate lattice point %v", k)
		}
		seen[k] = true
	}
}

// The lattice takes no sensors, so an empty database still gets the full field.
func TestWindLatticeIsIndependentOfSensors(t *testing.T) {
	a, b := snapshot.WindLattice(), snapshot.WindLattice()
	if len(a) == 0 || len(a) != len(b) || a[0] != b[0] || a[len(a)-1] != b[len(b)-1] {
		t.Fatal("lattice is not a stable constant set")
	}
}
