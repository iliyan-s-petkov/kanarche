package store_test

import (
	"testing"
	"time"

	"kanarche.eu/internal/quality"
)

func seedRows(id int64, metric string, value float64, flag quality.Flag, n int, newest time.Time, step time.Duration) []quality.Scored {
	out := make([]quality.Scored, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, sample(id, metric, value, flag, newest.Add(-time.Duration(i)*step)))
	}
	return out
}

func TestSeedHistoryRestoresFrozenSeries(t *testing.T) {
	ctx, _, st := newStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	var rows []quality.Scored
	// Sensor 1: 12 identical P1 readings inside the window -> stuck once seeded.
	rows = append(rows, seedRows(1, "P1", 42, quality.FlagOK, 12, now.Add(-time.Minute), 5*time.Minute)...)
	// Sensor 2: moving temperature -> not stuck.
	for i := 0; i < 12; i++ {
		rows = append(rows, sample(2, "temperature", 20+float64(i), quality.FlagOK, now.Add(-time.Duration(i+1)*5*time.Minute)))
	}
	// Sensor 3: 12 identical readings but all older than the window -> ignored.
	rows = append(rows, seedRows(3, "P1", 42, quality.FlagOK, 12, now.Add(-4*time.Hour), 5*time.Minute)...)
	// Sensor 4: out_of_range and clamped rows never entered the live history.
	rows = append(rows, seedRows(4, "P1", 42, quality.FlagOutOfRange, 6, now.Add(-time.Minute), 5*time.Minute)...)
	rows = append(rows, seedRows(4, "P1", 42, quality.FlagClamped, 6, now.Add(-40*time.Minute), 5*time.Minute)...)
	if _, err := st.WriteReadings(ctx, rows); err != nil {
		t.Fatalf("WriteReadings: %v", err)
	}

	h := quality.NewHistory(12)
	n, err := st.SeedHistory(ctx, h, 3*time.Hour, 12)
	if err != nil {
		t.Fatalf("SeedHistory: %v", err)
	}
	if n != 24 {
		t.Errorf("seeded %d rows, want 24 (sensor 1 and 2 only)", n)
	}
	if !h.IsStuck(1, "P1") {
		t.Error("sensor 1 not stuck after seeding 12 identical readings")
	}
	if h.IsStuck(2, "temperature") {
		t.Error("sensor 2 stuck, its temperature moves")
	}
	if h.IsStuck(3, "P1") {
		t.Error("sensor 3 stuck from rows outside the seed window")
	}
	if h.IsStuck(4, "P1") {
		t.Error("sensor 4 stuck from out_of_range/clamped rows")
	}
}

func TestSeedHistoryKeepsOnlyTheLastNInOrder(t *testing.T) {
	ctx, _, st := newStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	// 20 older readings at 7.0, then the 3 newest at 9.0. depth=4 keeps the
	// 3 newest plus one 7.0, so the series is not constant.
	rows := seedRows(1, "P1", 7, quality.FlagOK, 20, now.Add(-10*time.Minute), time.Minute)
	rows = append(rows, seedRows(1, "P1", 9, quality.FlagOK, 3, now, 3*time.Second)...)
	if _, err := st.WriteReadings(ctx, rows); err != nil {
		t.Fatalf("WriteReadings: %v", err)
	}

	h := quality.NewHistory(4)
	n, err := st.SeedHistory(ctx, h, 3*time.Hour, 4)
	if err != nil {
		t.Fatalf("SeedHistory: %v", err)
	}
	if n != 4 {
		t.Errorf("seeded %d rows, want 4", n)
	}
	if h.IsStuck(1, "P1") {
		t.Error("stuck, want the window to hold three 9s and one 7")
	}
	if v, ok := h.Constant(1, "P1"); ok {
		t.Errorf("Constant = %v, want not constant", v)
	}
}

func TestSeedHistoryEmptyTable(t *testing.T) {
	ctx, _, st := newStore(t)
	n, err := st.SeedHistory(ctx, quality.NewHistory(12), 3*time.Hour, 12)
	if err != nil || n != 0 {
		t.Errorf("SeedHistory on empty table = (%d, %v), want (0, nil)", n, err)
	}
}
