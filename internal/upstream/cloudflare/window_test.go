package cloudflare_test

import (
	"testing"
	"time"

	"kanarche.eu/internal/upstream/cloudflare"
)

// Today's row accumulates all day, so every run must ask for it again. The
// requested range ends on the current UTC day however current the table is.
func TestWindowAlwaysIncludesTodayForRepull(t *testing.T) {
	now := time.Date(2026, 9, 28, 14, 20, 0, 0, time.UTC)
	today := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	cases := map[string]struct {
		maxDay   time.Time
		has      bool
		wantFrom time.Time
	}{
		"table current":    {today, true, today.AddDate(0, 0, -2)},
		"yesterday newest": {today.AddDate(0, 0, -1), true, today.AddDate(0, 0, -3)},
		"empty table":      {time.Time{}, false, today.AddDate(0, 0, -29)},
	}
	for name, c := range cases {
		since, until := cloudflare.Window(now, c.maxDay, c.has)
		if !until.Equal(today) {
			t.Errorf("%s: until = %s, want today %s", name, until, today)
		}
		if !since.Equal(c.wantFrom) {
			t.Errorf("%s: since = %s, want %s", name, since, c.wantFrom)
		}
	}
}

// A clock in another zone that is already tomorrow in UTC terms still maps to
// the UTC day, so an hourly run near midnight asks for the right row.
func TestWindowUsesTheUTCDayForNonUTCClocks(t *testing.T) {
	sofia := time.FixedZone("EEST", 3*3600)
	now := time.Date(2026, 9, 29, 1, 30, 0, 0, sofia)
	_, until := cloudflare.Window(now, time.Time{}, false)
	if want := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC); !until.Equal(want) {
		t.Errorf("until = %s, want %s", until, want)
	}
}
