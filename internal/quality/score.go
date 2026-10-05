package quality

import (
	"math"
	"slices"

	"airbg.org/internal/config"
	"airbg.org/internal/upstream"
)

// NeighbourRadiusMetres (the search radius for the spatial check, spec §6.3)
// and the Earth radius used for the haversine distance now live in
// config.Quality (airbg.yaml quality.neighbour_radius_metres /
// earth_radius_metres).

type Scored struct {
	Reading upstream.Reading
	Flag    Flag
}

// Scorer holds the quality thresholds. They were package constants; they are
// now configuration, so the checks that use them need an owner rather than
// reading a global.
type Scorer struct {
	cfg config.Quality
}

func NewScorer(cfg config.Quality) *Scorer { return &Scorer{cfg: cfg} }

// Score evaluates a whole poll batch. Neighbour comparison runs in memory over
// the batch rather than against the database: one poll returns every Bulgarian
// sensor at once, so the neighbourhood is already in hand.
//
// Checks run in order and the first failure wins: clamp, range, stuck, then
// spatial. Every input reading appears in the output — readings are flagged,
// never dropped.
func (s *Scorer) Score(readings []upstream.Reading, hist *History) []Scored {
	// Group by metric so a sensor is only ever compared against the same
	// quantity, and so one bad metric cannot influence another.
	byMetric := make(map[string][]upstream.Reading)
	for _, r := range readings {
		byMetric[r.Metric] = append(byMetric[r.Metric], r)
	}

	// The reference population excludes out-of-range values: a sensor reporting
	// -999 must not drag the neighbourhood median it is being compared against.
	// Clamp sentinels are excluded for the same reason; the range check does
	// not catch them. See README.md.
	reference := make(map[string][]upstream.Reading, len(byMetric))
	for metric, group := range byMetric {
		valid := make([]upstream.Reading, 0, len(group))
		for _, r := range group {
			if s.InRange(metric, r.Value) && !s.IsClamped(metric, r.Value) {
				valid = append(valid, r)
			}
		}
		reference[metric] = valid
	}

	// A capped lone reading is out_of_range, so like the others it stays out of
	// the history and the neighbour reference population.
	lone := s.loneCapped(readings, reference)
	if len(lone) > 0 {
		for metric, group := range reference {
			reference[metric] = slices.DeleteFunc(group, func(r upstream.Reading) bool {
				return lone[sensorMetric{r.SensorID, r.Metric}]
			})
		}
	}

	// Observe every in-range reading before judging any, so a rule that needs a
	// sibling metric (humidity vs temperature) sees this poll's value of it.
	// Clamped and out-of-range readings never enter the history.
	for _, r := range readings {
		if !s.IsClamped(r.Metric, r.Value) && s.InRange(r.Metric, r.Value) && !lone[sensorMetric{r.SensorID, r.Metric}] {
			hist.Observe(r.SensorID, r.Metric, r.Value)
		}
	}

	out := make([]Scored, 0, len(readings))
	for _, r := range readings {
		if lone[sensorMetric{r.SensorID, r.Metric}] {
			out = append(out, Scored{Reading: r, Flag: FlagOutOfRange})
			continue
		}
		out = append(out, Scored{Reading: r, Flag: s.scoreOne(r, reference[r.Metric], hist)})
	}
	return out
}

func (s *Scorer) scoreOne(r upstream.Reading, population []upstream.Reading, hist *History) Flag {
	// Order is load-bearing; see README.md.
	if s.IsClamped(r.Metric, r.Value) {
		return FlagClamped
	}
	if !s.InRange(r.Metric, r.Value) {
		return FlagOutOfRange
	}

	if hist.IsStuck(r.SensorID, r.Metric) || s.failureSignature(r, hist) {
		return FlagStuck
	}

	return s.SpatialCheck(r.Metric, r.Value, s.neighboursOf(r, population))
}

func (s *Scorer) neighboursOf(r upstream.Reading, population []upstream.Reading) []Neighbour {
	neighbours := make([]Neighbour, 0, 8)
	for _, other := range population {
		if other.SensorID == r.SensorID {
			continue
		}
		if s.haversineMetres(r.Lon, r.Lat, other.Lon, other.Lat) > s.cfg.NeighbourRadiusMetres {
			continue
		}
		neighbours = append(neighbours, Neighbour{Lon: other.Lon, Lat: other.Lat, Value: other.Value})
	}
	return neighbours
}

type sensorMetric struct {
	sensor int64
	metric string
}

// loneCapped finds in-range readings above their metric's lone cap that have
// fewer than min_neighbours neighbours; the spatial check cannot judge them.
// Strictly greater than the cap: a reading equal to it stays no_neighbours.
func (s *Scorer) loneCapped(readings []upstream.Reading, reference map[string][]upstream.Reading) map[sensorMetric]bool {
	var lone map[sensorMetric]bool
	for _, r := range readings {
		limit, ok := s.cfg.LoneCaps[r.Metric]
		if !ok || r.Value <= limit || s.IsClamped(r.Metric, r.Value) || !s.InRange(r.Metric, r.Value) {
			continue
		}
		if len(s.neighboursOf(r, reference[r.Metric])) < s.cfg.MinNeighbours {
			if lone == nil {
				lone = make(map[sensorMetric]bool)
			}
			lone[sensorMetric{r.SensorID, r.Metric}] = true
		}
	}
	return lone
}

// haversineMetres returns great-circle distance. Accurate enough at the 15 km
// scale this is used for, and avoids a database round-trip per reading.
func (s *Scorer) haversineMetres(lon1, lat1, lon2, lat2 float64) float64 {
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	return 2 * s.cfg.EarthRadiusMetres * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// failureSignature matches BME280/DHT22 failure shapes over a full history
// window: temperature frozen within tolerance; humidity exactly 0; humidity
// exactly 100 while temperature is also frozen (fog alone moves temperature).
func (s *Scorer) failureSignature(r upstream.Reading, hist *History) bool {
	tol := s.cfg.TemperatureFrozenTolerance
	switch r.Metric {
	case "temperature":
		return hist.Frozen(r.SensorID, "temperature", tol)
	case "humidity":
		v, ok := hist.Constant(r.SensorID, "humidity")
		switch {
		case !ok:
			return false
		case v == 0:
			return true
		case v == 100:
			return hist.Frozen(r.SensorID, "temperature", tol)
		}
	}
	return false
}
