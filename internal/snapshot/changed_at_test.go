package snapshot

import (
	"testing"
	"time"
)

func TestCarryChangedAt(t *testing.T) {
	t0 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	meta := AreaMeta{Slug: "a", SensorCount: 3, Values: map[string]float64{"pm25": 10}}
	prevMeta := meta
	prevMeta.ChangedAt = t0
	prev := &Snapshot{KnownSlugs: map[string]AreaMeta{"a": prevMeta}}

	if got := carryChangedAt(prev, meta, t1); !got.Equal(t0) {
		t.Errorf("unchanged data: got %v, want carried %v", got, t0)
	}

	changed := meta
	changed.Values = map[string]float64{"pm25": 11}
	if got := carryChangedAt(prev, changed, t1); !got.Equal(t1) {
		t.Errorf("changed value: got %v, want now %v", got, t1)
	}

	moreSensors := meta
	moreSensors.SensorCount = 4
	if got := carryChangedAt(prev, moreSensors, t1); !got.Equal(t1) {
		t.Errorf("changed sensor count: got %v, want now %v", got, t1)
	}

	if got := carryChangedAt(nil, meta, t1); !got.IsZero() {
		t.Errorf("no previous snapshot: got %v, want zero (unknown)", got)
	}
	if got := carryChangedAt(&Snapshot{}, meta, t1); !got.IsZero() {
		t.Errorf("area absent from previous: got %v, want zero", got)
	}
}
