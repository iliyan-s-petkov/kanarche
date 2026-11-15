package web

import (
	"html/template"
	"strconv"
	"strings"
	"time"

	"kanarche.eu/internal/snapshot"
)

// minYearProfileDays is the fewest days with data before the 12 month profile is shown.
// Below it the section is only the coverage line.
const minYearProfileDays = 180

// longDate formats a UTC day the way the area pages print dates, for example "14 януари 2026 г.".
func (p PageData) longDate(t time.Time) string {
	t = t.UTC()
	return strings.NewReplacer(
		"{day}", strconv.Itoa(t.Day()),
		"{month}", p.monthName(t.Month()),
		"{year}", strconv.Itoa(t.Year()),
	).Replace(p.T("area.over_who.date"))
}

// monthName is the month on its own: nominative lowercase in Bulgarian, capitalised in English.
func (p PageData) monthName(m time.Month) string {
	return p.T("news.month." + strconv.Itoa(int(m)))
}

// areaYearProfileHTML renders the 12 month profile section. The bool is true when the full profile
// (not just the coverage line) was rendered. It returns "" when the area has no profile.
func (p PageData) areaYearProfileHTML(row AreaRow) (template.HTML, bool) {
	y := row.YearProfile
	if y == nil {
		return "", false
	}
	if y.DaysWithData < minYearProfileDays {
		line := strings.NewReplacer("{area}", p.areaPlain(row), "{days}", strconv.Itoa(y.DaysWithData)).Replace(p.T("area.year.short"))
		return template.HTML(`<section id="area-year" class="area-year"><p>` + htmlEscape(line) + `</p></section>`), false
	}

	// Each sentence group is its own key so one without data can be left out whole.
	fill := func(key string, pairs ...string) string {
		return strings.NewReplacer(pairs...).Replace(p.T(key))
	}
	var b strings.Builder
	b.WriteString(`<section id="area-year" class="area-year" aria-labelledby="area-year-h">`)
	b.WriteString(`<h2 id="area-year-h">` + htmlEscape(fill("area.year.heading", "{area_in}", p.areaIn(), "{area}", p.areaPlain(row))) + `</h2>`)

	para1 := []string{fill("area.year.lead", "{area_in}", p.areaIn(), "{area}", p.areaPlain(row), "{avg}", formatValue(y.Mean, p.Lang))}
	if y.Best != nil && y.Worst != nil {
		para1 = append(para1, fill("area.year.months",
			"{best_month}", p.monthName(y.Best.Month), "{best_avg}", formatValue(y.Best.Mean, p.Lang),
			"{worst_month}", p.monthName(y.Worst.Month), "{worst_avg}", formatValue(y.Worst.Mean, p.Lang)))
	}
	if y.HasWinter && y.HasSummer {
		para1 = append(para1, fill("area.year.seasons",
			"{winter_avg}", formatValue(y.WinterMean, p.Lang), "{summer_avg}", formatValue(y.SummerMean, p.Lang)))
	}
	b.WriteString(`<p>` + htmlEscape(strings.Join(para1, " ")) + `</p>`)

	whoKey := "area.year.who"
	if y.DaysOver == 1 {
		whoKey = "area.year.who_one"
	}
	b.WriteString(`<p>` + htmlEscape(fill(whoKey,
		"{days}", strconv.Itoa(y.DaysWithData), "{n}", strconv.Itoa(y.DaysOver), "{who}", strconv.Itoa(whoDay25),
		"{worst_date}", p.longDate(y.WorstDay), "{max}", formatValue(y.WorstMean, p.Lang))) + `</p>`)

	b.WriteString(p.yearTableHTML(y))
	b.WriteString(`</section>`)
	return template.HTML(b.String()), true
}

// yearTableHTML is the month table. Rows carry the year when the months span two calendar years.
func (p PageData) yearTableHTML(y *snapshot.YearProfile) string {
	withYear := len(y.Months) > 0 && y.Months[0].Year != y.Months[len(y.Months)-1].Year
	var b strings.Builder
	b.WriteString(`<table class="table area-year__table"><thead><tr>`)
	for _, key := range []string{"area.year.col.month", "area.year.col.avg", "area.year.col.days"} {
		b.WriteString(`<th scope="col">` + htmlEscape(p.T(key)) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, m := range y.Months {
		label := p.monthName(m.Month)
		if withYear {
			label += " " + strconv.Itoa(m.Year)
		}
		b.WriteString(`<tr><th scope="row">` + htmlEscape(label) + `</th><td>` + formatValue(m.Mean, p.Lang) + `</td><td>` + strconv.Itoa(m.Days) + `</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}
