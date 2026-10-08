package quality

import (
	"testing"

	"kanarche.eu/internal/upstream"
)

// feed scores the same sensor n times so its history fills, returning the last flag.
func feed(t *testing.T, s *Scorer, h *History, n int, batch func(i int) []upstream.Reading, id int64, metric string) Flag {
	t.Helper()
	var last Flag
	for i := 0; i < n; i++ {
		last = flagOfMetric(t, s.Score(batch(i), h), id, metric)
	}
	return last
}

func flagOfMetric(t *testing.T, scored []Scored, id int64, metric string) Flag {
	t.Helper()
	for _, s := range scored {
		if s.Reading.SensorID == id && s.Reading.Metric == metric {
			return s.Flag
		}
	}
	t.Fatalf("no scored reading for sensor %d %s", id, metric)
	return ""
}

// -150 C is the owner's failed station; the range check already owns it.
func TestMinus150TemperatureIsOutOfRange(t *testing.T) {
	scored := testScorer().Score([]upstream.Reading{at(1, "temperature", -150, 0)}, NewHistory(12))
	if got := flagOf(t, scored, 1); got != FlagOutOfRange {
		t.Errorf("flag = %v, want %v", got, FlagOutOfRange)
	}
}

func TestTemperatureFrozenWithinToleranceIsStuck(t *testing.T) {
	s, h := testScorer(), NewHistory(12)
	// Alternates by 0.01: not an exact repeat, but inside the 0.01 tolerance.
	batch := func(i int) []upstream.Reading {
		return []upstream.Reading{at(1, "temperature", 20.00+0.01*float64(i%2), 0)}
	}
	if got := feed(t, s, h, 11, batch, 1, "temperature"); got == FlagStuck {
		t.Fatalf("stuck after 11 readings, want a full window of 12")
	}
	if got := flagOfMetric(t, s.Score(batch(11), h), 1, "temperature"); got != FlagStuck {
		t.Errorf("flag after 12 readings within 0.01 = %v, want stuck", got)
	}
}

func TestTemperatureMovingMoreThanToleranceIsNotStuck(t *testing.T) {
	s, h := testScorer(), NewHistory(12)
	batch := func(i int) []upstream.Reading {
		return []upstream.Reading{at(1, "temperature", 20.00+0.02*float64(i%2), 0)}
	}
	if got := feed(t, s, h, 24, batch, 1, "temperature"); got == FlagStuck {
		t.Errorf("flag = stuck for a 0.02 swing, tolerance is 0.01")
	}
}

func TestHumidityZeroFlatIsStuckWithoutTemperature(t *testing.T) {
	s, h := testScorer(), NewHistory(12)
	batch := func(int) []upstream.Reading { return []upstream.Reading{at(1, "humidity", 0, 0)} }
	if got := feed(t, s, h, 12, batch, 1, "humidity"); got != FlagStuck {
		t.Errorf("humidity at exactly 0 for the window = %v, want stuck", got)
	}
}

func TestHumidityHundredWithMovingTemperatureIsFog(t *testing.T) {
	s, h := testScorer(), NewHistory(12)
	batch := func(i int) []upstream.Reading {
		return []upstream.Reading{
			at(1, "humidity", 100, 0),
			at(1, "temperature", 5+0.3*float64(i), 0),
		}
	}
	if got := feed(t, s, h, 24, batch, 1, "humidity"); got == FlagStuck {
		t.Errorf("humidity 100 with a moving temperature = stuck, want it left alone (fog)")
	}
}

func TestHumidityHundredWithFrozenTemperatureIsStuck(t *testing.T) {
	s, h := testScorer(), NewHistory(12)
	batch := func(int) []upstream.Reading {
		// Humidity listed first: the temperature window must already be full
		// when humidity is judged in the same batch.
		return []upstream.Reading{at(1, "humidity", 100, 0), at(1, "temperature", 21.5, 0)}
	}
	if got := feed(t, s, h, 12, batch, 1, "humidity"); got != FlagStuck {
		t.Errorf("humidity 100 + frozen temperature = %v, want stuck on the 12th reading", got)
	}
}

func TestHumidityNinetyNineFlatIsNotASignature(t *testing.T) {
	// Only the exact end values are failure shapes; 99.0 is fog and 99.0 repeated
	// is still handled by the generic repeat rule only.
	s, h := testScorer(), NewHistory(12)
	batch := func(i int) []upstream.Reading {
		return []upstream.Reading{at(1, "humidity", 99+0.1*float64(i%3), 0), at(1, "temperature", 21.5, 0)}
	}
	if got := feed(t, s, h, 24, batch, 1, "humidity"); got == FlagStuck {
		t.Errorf("varying 99.x humidity = stuck, want ok")
	}
}

func TestHistorySeedRestoresRun(t *testing.T) {
	s, h := testScorer(), NewHistory(12)
	for i := 0; i < 11; i++ {
		h.Observe(1, "P1", 42)
	}
	// 11 seeded readings: the first live poll completes the window.
	if got := flagOfMetric(t, s.Score([]upstream.Reading{at(1, "P1", 42, 0)}, h), 1, "P1"); got != FlagStuck {
		t.Errorf("flag = %v, want stuck on the first poll after seeding 11 equal readings", got)
	}
}
