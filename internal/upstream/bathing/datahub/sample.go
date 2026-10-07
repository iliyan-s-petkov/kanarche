package datahub

import (
	"io"
	"sort"
	"strconv"

	"airbg.org/internal/xlsx"
	"airbg.org/internal/xlsx/xlsxtest"
)

// SampleOptions bounds the cut: up to MaxPerSeason rows of the country for
// each season from MinSeason on, plus the first MaxOther rows of any other
// country.
type SampleOptions struct {
	MinSeason    int
	MaxPerSeason int
	MaxOther     int
}

// EmitSample writes a small workbook holding the first rows of book, in file
// order and with every column unchanged. It is used to cut the committed test
// fixture from the real file, so the fixture needs no hand edits.
func EmitSample(book *xlsx.Book, country string, opt SampleOptions, w io.Writer) error {
	sheets := book.Sheets()
	if len(sheets) == 0 {
		return ErrNoSheet
	}
	var cols []string
	var kept []xlsx.Row
	perSeason := map[int]int{}
	var nOther int
	err := book.Rows(sheets[0], required, func(r xlsx.Row) error {
		if cols == nil {
			for k := range r {
				cols = append(cols, k)
			}
			sort.Strings(cols)
		}
		if r["countryCode"] == country {
			season, err := strconv.Atoi(r["season"])
			if err != nil || season < opt.MinSeason || perSeason[season] >= opt.MaxPerSeason {
				return nil
			}
			perSeason[season]++
		} else {
			if nOther >= opt.MaxOther {
				return nil
			}
			nOther++
		}
		kept = append(kept, r)
		return nil
	})
	if err != nil {
		return err
	}
	rows := [][]string{cols}
	for _, r := range kept {
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = r[c]
		}
		rows = append(rows, row)
	}
	data, err := xlsxtest.TableBook(rows)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
