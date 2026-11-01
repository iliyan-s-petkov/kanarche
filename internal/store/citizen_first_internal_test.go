package store

import (
	"strings"
	"testing"
)

func mustPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: want a panic, got none", name)
		}
	}()
	fn()
}

func TestCitizenFirstRefusesAmbiguousQueries(t *testing.T) {
	ok := "SELECT 1 FROM " + citizenMark + "(SELECT sensor_id FROM x) per_sensor"
	if got := citizenFirst(ok, "slug", 1); !strings.Contains(got, "has_official") || strings.Contains(got, citizenMark) {
		t.Fatalf("a well-formed query was not rewritten: %s", got)
	}
	// An extra plain (SELECT, as notFaulty adds, is fine: only the mark must be unique.
	_ = citizenFirst(ok+" WHERE NOT EXISTS (SELECT 1)", "slug", 1)

	mustPanic(t, "no mark", func() { citizenFirst("SELECT 1 FROM (SELECT sensor_id FROM x) per_sensor", "slug", 1) })
	mustPanic(t, "two marks", func() { citizenFirst(ok+" UNION "+ok, "slug", 1) })
	mustPanic(t, "no alias", func() { citizenFirst("SELECT 1 FROM "+citizenMark+"(SELECT sensor_id FROM x) q", "slug", 1) })
}
