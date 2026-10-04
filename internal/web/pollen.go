package web

import (
	"strings"
	"time"

	"airbg.org/internal/snapshot"
)

// PollenBlock is the area page's pollen table, localised for the template.
type PollenBlock struct {
	Days []PollenDayHead
	Rows []PollenRow
	// Chip is the toolbar summary; empty when today has no value.
	Chip string
}

type PollenDayHead struct {
	Date  string
	Label string
}

type PollenRow struct {
	Name  string
	Cells []PollenCellView
}

// PollenCellView is one table cell; Level is the CSS key, LevelText the word.
type PollenCellView struct {
	Has       bool
	Level     string
	LevelText string
	Mean      string
}

var pollenWeekdays = [...]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// pollenBlock localises v; nil when there is no table.
func (rr *Renderer) pollenBlock(v *snapshot.PollenView, lang string) *PollenBlock {
	if v == nil {
		return nil
	}
	t := func(k string) string { return rr.cat.T(lang, k) }
	b := &PollenBlock{}
	for i, d := range v.Days {
		b.Days = append(b.Days, PollenDayHead{Date: d, Label: pollenDayLabel(t, i, d)})
	}
	for _, sv := range v.Species {
		row := PollenRow{Name: t("pollen.species." + sv.Name)}
		for _, d := range sv.Days {
			c := PollenCellView{Has: d.Has}
			if d.Has {
				c.Level, c.LevelText, c.Mean = d.Level, t("pollen.level."+d.Level), formatValue(d.Mean, lang)
			}
			row.Cells = append(row.Cells, c)
		}
		b.Rows = append(b.Rows, row)
	}
	if s := v.Summary; s != nil {
		if s.Species == "" {
			b.Chip = t("pollen.chip.quiet")
		} else {
			b.Chip = strings.NewReplacer(
				"{level}", strings.ToLower(t("pollen.level."+s.Level)),
				"{species}", t("pollen.species."+s.Species),
			).Replace(t("pollen.chip"))
		}
	}
	return b
}

// pollenDayLabel: today, tomorrow, then weekday and day.month.
func pollenDayLabel(t func(string) string, i int, date string) string {
	switch i {
	case 0:
		return t("pollen.day.today")
	case 1:
		return t("pollen.day.tomorrow")
	}
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t("pollen.weekday."+pollenWeekdays[d.Weekday()]) + " " + d.Format("2.01")
}
