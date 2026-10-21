//go:build e2e

package e2e

import (
	"testing"
	"time"
)

func TestRemarkDatesShareOneSeason(t *testing.T) {
	for _, now := range []time.Time{
		time.Date(2027, time.January, 2, 12, 0, 0, 0, time.UTC),
		time.Date(2027, time.January, 15, 12, 0, 0, 0, time.UTC),
		time.Date(2027, time.July, 1, 12, 0, 0, 0, time.UTC),
	} {
		d := remarkDates(now)
		for i, day := range d {
			if day.Year() != d[0].Year() {
				t.Errorf("%s: date %d is %s, outside the season of %s", now.Format("2006-01-02"), i, day.Format("2006-01-02"), d[0].Format("2006-01-02"))
			}
			if i > 0 && !day.Before(d[i-1]) {
				t.Errorf("%s: date %d is not older than date %d", now.Format("2006-01-02"), i, i-1)
			}
		}
		if d[0].After(now) {
			t.Errorf("%s: newest remark %s is in the future", now.Format("2006-01-02"), d[0].Format("2006-01-02"))
		}
	}
}
