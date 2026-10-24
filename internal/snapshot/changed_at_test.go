package snapshot

import (
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

func persisted(stamps ...store.AreaChange) map[string]store.AreaChange {
	m := map[string]store.AreaChange{}
	for _, s := range stamps {
		m[s.Slug] = s
	}
	return m
}

// A fresh process has only what the database kept, so passing the stored
// stamps back in is a restart.
func TestChangeStampSurvivesRestart(t *testing.T) {
	t0 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(48 * time.Hour)
	meta := AreaMeta{Slug: "a", Covered: true, SensorCount: 3, Values: map[string]float64{"pm25": 10.2}}

	first := changeStamp(persisted(), meta, t0)
	if !first.ChangedAt.Equal(t0) {
		t.Fatalf("new area: got %v, want now %v", first.ChangedAt, t0)
	}
	stored := persisted(first)

	same := meta
	same.Values = map[string]float64{"pm25": 10.4}
	same.SensorCount = 4
	if got := changeStamp(stored, same, t1); !got.ChangedAt.Equal(t0) {
		t.Errorf("sub-display drift after restart: got %v, want kept %v", got.ChangedAt, t0)
	}

	for name, mut := range map[string]func(*AreaMeta){
		"value":   func(m *AreaMeta) { m.Values = map[string]float64{"pm25": 12.0} },
		"metric":  func(m *AreaMeta) { m.Values = map[string]float64{"pm25": 10.2, "pm10": 20} },
		"name_bg": func(m *AreaMeta) { m.NameBG = "x" },
		"name_en": func(m *AreaMeta) { m.NameEN = "x" },
		"parent":  func(m *AreaMeta) { m.ParentSlug = "p" },
		"kind":    func(m *AreaMeta) { m.Kind = "city" },
		"covered": func(m *AreaMeta) { m.Covered = false },
		"day":     func(m *AreaMeta) { m.Day = &DayRange{Min: 1, Max: 2, Buckets: 80} },
	} {
		m := meta
		mut(&m)
		if got := changeStamp(stored, m, t1); !got.ChangedAt.Equal(t1) {
			t.Errorf("changed %s: got %v, want now %v", name, got.ChangedAt, t1)
		}
	}
}
