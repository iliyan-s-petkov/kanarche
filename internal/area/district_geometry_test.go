package area_test

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

type ring [][2]float64

type districtFile struct {
	Features []struct {
		Properties struct {
			Slug   string `json:"slug"`
			NameEN string `json:"name_en"`
			Source string `json:"source"`
		} `json:"properties"`
		Geometry struct {
			Type        string           `json:"type"`
			Coordinates [][][][2]float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

func readDistricts(t *testing.T, path string) districtFile {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var fc districtFile
	if err := json.Unmarshal(raw, &fc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return fc
}

// pointInRing is the even-odd ray cast.
func pointInRing(p [2]float64, r ring) bool {
	in := false
	for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
		if (r[i][1] > p[1]) != (r[j][1] > p[1]) &&
			p[0] < (r[j][0]-r[i][0])*(p[1]-r[i][1])/(r[j][1]-r[i][1])+r[i][0] {
			in = !in
		}
	}
	return in
}

func segDist(p, a, b [2]float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	l := dx*dx + dy*dy
	u := 0.0
	if l > 0 {
		u = math.Max(0, math.Min(1, ((p[0]-a[0])*dx+(p[1]-a[1])*dy)/l))
	}
	return math.Hypot(p[0]-(a[0]+u*dx), p[1]-(a[1]+u*dy))
}

func cross(o, a, b [2]float64) float64 {
	return (a[0]-o[0])*(b[1]-o[1]) - (a[1]-o[1])*(b[0]-o[0])
}

// segmentsCross is a proper crossing; touching at shared vertices is allowed.
func segmentsCross(a, b, c, d [2]float64) bool {
	d1, d2, d3, d4 := cross(a, b, c), cross(a, b, d), cross(c, d, a), cross(c, d, b)
	return d1*d2 < 0 && d3*d4 < 0
}

func selfIntersects(r ring) bool {
	n := len(r) - 1
	for i := 0; i < n; i++ {
		for j := i + 2; j < n; j++ {
			if i == 0 && j == n-1 {
				continue
			}
			if segmentsCross(r[i], r[i+1], r[j], r[j+1]) {
				return true
			}
		}
	}
	return false
}

// nearRing reports whether p is inside the ring or within tol degrees of its edge.
func nearRing(p [2]float64, r ring, tol float64) bool {
	if pointInRing(p, r) {
		return true
	}
	for i := 0; i+1 < len(r); i++ {
		if segDist(p, r[i], r[i+1]) <= tol {
			return true
		}
	}
	return false
}

// TestPlovdivAndVarnaDistrictGeometry checks every district ring is closed,
// does not cross itself and lies inside its parent city polygon, within the
// simplification tolerance (0.002 deg) plus a margin.
func TestPlovdivAndVarnaDistrictGeometry(t *testing.T) {
	cities := readDistricts(t, "../../data/boundaries/cities.geojson")
	cityRings := map[string][]ring{}
	for _, f := range cities.Features {
		if f.Properties.Slug != "plovdiv" && f.Properties.Slug != "varna" {
			continue
		}
		for _, poly := range f.Geometry.Coordinates {
			cityRings[f.Properties.Slug] = append(cityRings[f.Properties.Slug], ring(poly[0]))
		}
	}
	const tol = 0.004
	for _, c := range []struct {
		file, city string
		want       int
	}{
		{"plovdiv-districts.geojson", "plovdiv", 6},
		{"varna-districts.geojson", "varna", 5},
	} {
		fc := readDistricts(t, "../../data/boundaries/"+c.file)
		if len(fc.Features) != c.want {
			t.Fatalf("%s has %d features, want %d", c.file, len(fc.Features), c.want)
		}
		for _, f := range fc.Features {
			slug := f.Properties.Slug
			if f.Geometry.Type != "MultiPolygon" || len(f.Geometry.Coordinates) == 0 {
				t.Errorf("%s: geometry %q with %d polygons, want a non-empty MultiPolygon", slug, f.Geometry.Type, len(f.Geometry.Coordinates))
			}
			if f.Properties.Source != "OpenStreetMap contributors, ODbL 1.0" || f.Properties.NameEN == "" {
				t.Errorf("%s: source %q name_en %q", slug, f.Properties.Source, f.Properties.NameEN)
			}
			for _, poly := range f.Geometry.Coordinates {
				for ri, rg := range poly {
					r := ring(rg)
					if len(r) < 4 || r[0] != r[len(r)-1] {
						t.Errorf("%s: ring %d is not closed", slug, ri)
						continue
					}
					if selfIntersects(r) {
						t.Errorf("%s: ring %d crosses itself", slug, ri)
					}
					if ri > 0 {
						continue
					}
					for _, p := range r {
						ok := false
						for _, cr := range cityRings[c.city] {
							if nearRing(p, cr, tol) {
								ok = true
								break
							}
						}
						if !ok {
							t.Errorf("%s: vertex %v is outside the %s polygon", slug, p, c.city)
							break
						}
					}
				}
			}
		}
	}
}
