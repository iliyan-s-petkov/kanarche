package snapshot

import (
	"sort"
	"time"

	"kanarche.eu/internal/store"
)

// profileMonths is how many calendar months the profile covers, ending with the month of now.
const profileMonths = 12

// minBestWorstDays is the fewest days with data a month needs to count as the cleanest or most polluted.
const minBestWorstDays = 10

// MonthMean is the mean of the daily values in one calendar month (UTC).
type MonthMean struct {
	Year  int
	Month time.Month
	Mean  float64
	Days  int
}

// YearProfile is the last 12 calendar months of daily PM2.5 means for one area.
// It is built from the same daily series as WHODays, in the same UTC day buckets.
type YearProfile struct {
	DaysWithData int
	// Mean is the mean of all daily values.
	Mean float64
	// Months holds the months that have data, oldest first.
	Months []MonthMean
	// Best and Worst are the lowest and highest monthly mean among months with at least
	// minBestWorstDays days. Both are nil unless two such months exist.
	Best, Worst *MonthMean
	// Winter is December, January and February. Summer is June, July and August.
	// Each is the mean of the daily values in those months, absent without a day.
	WinterMean, SummerMean float64
	HasWinter, HasSummer   bool
	// DaysOver counts days strictly above WHODayPM25.
	DaysOver  int
	WorstDay  time.Time
	WorstMean float64
}

// fullDayBounds returns the first whole UTC day inside the window and the current UTC day.
// A bucket before the first or from the second is partial.
func fullDayBounds(since, now time.Time) (firstFull, today time.Time) {
	firstFull = since.UTC().Truncate(24 * time.Hour)
	if firstFull.Before(since) {
		firstFull = firstFull.Add(24 * time.Hour)
	}
	return firstFull, now.UTC().Truncate(24 * time.Hour)
}

// summariseYearProfile reduces a daily series to its YearProfile, or nil when no day remains.
// Days outside the 12 calendar months ending with now's month are dropped, and so are partial buckets.
func summariseYearProfile(points []store.Point, since, now time.Time) *YearProfile {
	firstFull, today := fullDayBounds(since, now)
	nowUTC := now.UTC()
	first := time.Date(nowUTC.Year(), nowUTC.Month()-(profileMonths-1), 1, 0, 0, 0, 0, time.UTC)
	if first.After(firstFull) {
		firstFull = first
	}

	type acc struct {
		sum  float64
		days int
	}
	byMonth := map[time.Time]*acc{}
	var order []time.Time
	var winter, summer acc
	var sum float64
	out := &YearProfile{}
	for _, p := range points {
		t := p.Time.UTC()
		if t.Before(firstFull) || !t.Before(today) {
			continue
		}
		key := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		a := byMonth[key]
		if a == nil {
			a = &acc{}
			byMonth[key] = a
			order = append(order, key)
		}
		a.sum += p.Value
		a.days++
		sum += p.Value
		switch t.Month() {
		case time.December, time.January, time.February:
			winter.sum += p.Value
			winter.days++
		case time.June, time.July, time.August:
			summer.sum += p.Value
			summer.days++
		}
		if out.DaysWithData == 0 {
			out.WorstDay, out.WorstMean = p.Time, p.Value
		}
		out.DaysWithData++
		if p.Value > WHODayPM25 {
			out.DaysOver++
		}
		// Strict, so the earliest of equal worst days is kept.
		if p.Value > out.WorstMean {
			out.WorstDay, out.WorstMean = p.Time, p.Value
		}
	}
	if out.DaysWithData == 0 {
		return nil
	}
	out.Mean = sum / float64(out.DaysWithData)
	if winter.days > 0 {
		out.WinterMean, out.HasWinter = winter.sum/float64(winter.days), true
	}
	if summer.days > 0 {
		out.SummerMean, out.HasSummer = summer.sum/float64(summer.days), true
	}

	// Series arrive oldest first, but sort the keys so the table never depends on it.
	sort.Slice(order, func(i, j int) bool { return order[i].Before(order[j]) })
	var qualifying []int
	for _, key := range order {
		a := byMonth[key]
		out.Months = append(out.Months, MonthMean{Year: key.Year(), Month: key.Month(), Mean: a.sum / float64(a.days), Days: a.days})
		if a.days >= minBestWorstDays {
			qualifying = append(qualifying, len(out.Months)-1)
		}
	}
	if len(qualifying) >= 2 {
		best, worst := qualifying[0], qualifying[0]
		for _, i := range qualifying {
			// Strict, so the earlier of equal months wins.
			if out.Months[i].Mean < out.Months[best].Mean {
				best = i
			}
			if out.Months[i].Mean > out.Months[worst].Mean {
				worst = i
			}
		}
		b, w := out.Months[best], out.Months[worst]
		out.Best, out.Worst = &b, &w
	}
	return out
}
