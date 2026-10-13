package dobrich

import (
	"fmt"
	"strings"
	"testing"

	"kanarche.eu/internal/store"
	"kanarche.eu/internal/xlsx/xlsxtest"
)

// Synthetic workbook builder. It writes the multi-sheet layout the extractor
// reads (one sheet per point, label and coordinates above a header row), so
// tests need no real RZI file.

type cellv struct {
	col   int
	text  string
	num   float64
	isNum bool
}

func tx(col int, s string) cellv  { return cellv{col: col, text: s} }
func nu(col int, v float64) cellv { return cellv{col: col, num: v, isNum: true} }

type srow struct {
	r     int
	cells []cellv
}

type sheetSpec struct {
	name string
	rows []srow
}

// Column positions of the standard layout.
const (
	colPlan = iota
	colDate
	colIE
	colEC
	colInfo
	colMeasures
	colVisual
)

// Date serials used by the tests.
const (
	d1 = 46181 // 2026-06-08
	d2 = 46195 // 2026-06-22
)

// std point: 08001, N 43 43 36, E 28 34 25.
var sites = []store.BathingSite{
	{ID: "BG3300000000008001", Lat: 43.72667, Lon: 28.57361},
	{ID: "BG3300000000008002", Lat: 43.69639, Lon: 28.56611},
}

func labelRow(point string) srow {
	return srow{3, []cellv{tx(0, `Пункт за вземане на проби №`+point+`     Наименование "Тестов плаж"`)}}
}

func coordRow(lat, lon string) srow {
	return srow{4, []cellv{tx(0, `Географски координати: ширина N `+lat+` дължина E `+lon)}}
}

func headerRow(unit string) srow {
	return srow{7, []cellv{
		tx(colPlan, "Планирана дата за пробонабиране, съгласно графика за мониторинг"),
		tx(colDate, "Дата на пробона-биране (във формат година-месец-ден)"),
		tx(colIE, " Резултат от анализ за чревни ентерококи "+unit),
		tx(colEC, "Резултат от анализ за Ешерихия коли "+unit),
		tx(colInfo, "Допълнителна информация*"),
		tx(colMeasures, "Предприети мерки*"),
		tx(colVisual, "Данни от визуалното наблюдение за замърсяване"),
	}}
}

// data builds the data row at sheet row r.
func data(r int, cells ...cellv) srow { return srow{r, cells} }

// okRow is a plain row: planned and actual date the same, two counts.
func okRow(r int, date float64, ie, ec cellv) srow {
	return data(r, nu(colPlan, date), nu(colDate, date), at(colIE, ie), at(colEC, ec))
}

// at moves a cell to column col.
func at(col int, c cellv) cellv { c.col = col; return c }

// lt is the text "<n", eq a numeric count.
func lt(n int) cellv { return tx(0, fmt.Sprintf("<%d", n)) }
func eq(n int) cellv { return nu(0, float64(n)) }

// pointSheet is a standard sheet for point named in both the sheet name
// and the label.
func pointSheet(point string, rows ...srow) sheetSpec {
	s := sheetSpec{name: "КР №" + point}
	s.rows = append(s.rows,
		labelRow(point),
		coordRow(`43º 43' 36"`, `28º 34' 25"`),
		headerRow("MPN/100cm3"),
	)
	s.rows = append(s.rows, rows...)
	return s
}

// without drops the rows numbered r from a sheet.
func (s sheetSpec) without(r ...int) sheetSpec {
	var out []srow
	for _, x := range s.rows {
		drop := false
		for _, n := range r {
			drop = drop || x.r == n
		}
		if !drop {
			out = append(out, x)
		}
	}
	s.rows = out
	return s
}

func (s sheetSpec) with(rows ...srow) sheetSpec {
	s.rows = append(append([]srow(nil), s.rows...), rows...)
	return s
}

// book assembles the workbook parts and zips them.
func book(t testing.TB, sheets ...sheetSpec) []byte {
	t.Helper()
	b, err := xlsxtest.Build(xlsxtest.Options{Parts: bookParts(sheets)})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func bookParts(sheets []sheetSpec) []xlsxtest.Part {
	index := map[string]int{}
	var shared []string
	sst := func(s string) int {
		i, ok := index[s]
		if !ok {
			i = len(shared)
			index[s] = i
			shared = append(shared, `<si><t xml:space="preserve">`+esc(s)+`</t></si>`)
		}
		return i
	}
	var ct, wb, rels strings.Builder
	ct.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	wb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>`)
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	var parts []xlsxtest.Part
	for i, sh := range sheets {
		n := i + 1
		fmt.Fprintf(&ct, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, n)
		fmt.Fprintf(&wb, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, esc(sh.name), n, n)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, n, n)
		var rows []string
		for _, r := range sh.rows {
			var c strings.Builder
			fmt.Fprintf(&c, `<row r="%d">`, r.r)
			for _, cell := range r.cells {
				ref := fmt.Sprintf("%s%d", xlsxtest.ColName(cell.col), r.r)
				if cell.isNum {
					fmt.Fprintf(&c, `<c r="%s"><v>%v</v></c>`, ref, cell.num)
				} else {
					fmt.Fprintf(&c, `<c r="%s" t="s"><v>%d</v></c>`, ref, sst(cell.text))
				}
			}
			c.WriteString(`</row>`)
			rows = append(rows, c.String())
		}
		parts = append(parts, xlsxtest.Part{Name: fmt.Sprintf("xl/worksheets/sheet%d.xml", n), Data: []byte(xlsxtest.SheetXML(rows...))})
	}
	sstRel := len(sheets) + 1
	fmt.Fprintf(&ct, `<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/></Types>`)
	wb.WriteString(`</sheets></workbook>`)
	fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/></Relationships>`, sstRel)
	head := []xlsxtest.Part{
		{Name: xlsxtest.ContentTypesName, Data: []byte(ct.String())},
		{Name: xlsxtest.WorkbookName, Data: []byte(wb.String())},
		{Name: xlsxtest.WorkbookRelsName, Data: []byte(rels.String())},
	}
	parts = append(head, parts...)
	return append(parts, xlsxtest.Part{Name: xlsxtest.SharedName, Data: []byte(xlsxtest.SharedXML(shared...))})
}

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
