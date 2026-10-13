package xlsx

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"kanarche.eu/internal/xlsx/xlsxtest"
)

// gridOf collects every GridRow of the default sheet.
func gridOf(b *Book) ([]GridRow, error) {
	var rows []GridRow
	err := b.Cells(xlsxtest.DefaultSheet, func(r GridRow) error {
		cp := GridRow{Num: r.Num, Cells: append([]GridCell(nil), r.Cells...)}
		rows = append(rows, cp)
		return nil
	})
	return rows, err
}

func TestCellsNoHeaderRowAndTypes(t *testing.T) {
	// Row 1 is absent, text sits in row 3, numbers in row 8; column D is blank.
	sheet := xlsxtest.SheetXML(
		`<row r="3"><c r="A3" t="s"><v>0</v></c></row>`,
		`<row r="8"><c r="A8"><v>46181</v></c><c r="C8" t="s"><v>1</v></c><c r="D8" s="5"/><c r="F8"><v>15</v></c></row>`,
	)
	b := bookOf(t, sheet, xlsxtest.SharedXML(`<si><t>title</t></si>`, `<si><t>15</t></si>`))
	rows, err := gridOf(b)
	if err != nil {
		t.Fatal(err)
	}
	want := []GridRow{
		{Num: 3, Cells: []GridCell{{Col: 0, Val: "title"}}},
		{Num: 8, Cells: []GridCell{
			{Col: 0, Val: "46181", Num: true},
			{Col: 2, Val: "15"},
			{Col: 5, Val: "15", Num: true},
		}},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestCellsUnknownSheet(t *testing.T) {
	b := bookOf(t, xlsxtest.SheetXML(), shared())
	err := b.Cells("nope", func(GridRow) error { return nil })
	wantErr(t, err, ErrNoSheet)
}

func TestCellsFormulaRefused(t *testing.T) {
	b := bookOf(t, xlsxtest.SheetXML(`<row r="1"><c r="A1"><f>1+1</f><v>2</v></c></row>`), shared())
	_, err := gridOf(b)
	wantErr(t, err, ErrFormula)
}

func TestCellsCallbackErrorUnwrapped(t *testing.T) {
	boom := errors.New("boom")
	b := bookOf(t, xlsxtest.SheetXML(`<row r="1"><c r="A1"><v>1</v></c></row>`), shared())
	err := b.Cells(xlsxtest.DefaultSheet, func(GridRow) error { return boom })
	if err != boom {
		t.Fatalf("err = %v", err)
	}
}

func TestCellsRowCap(t *testing.T) {
	var rows []string
	for i := 1; i <= 6; i++ {
		rows = append(rows, fmt.Sprintf(`<row r="%d"><c r="A%d"><v>1</v></c></row>`, i, i))
	}
	b := bookOf(t, xlsxtest.SheetXML(rows...), shared())
	b.lim.rows = 6
	if _, err := gridOf(b); err != nil {
		t.Fatalf("at cap: %v", err)
	}
	b = bookOf(t, xlsxtest.SheetXML(append(rows, `<row r="7"/>`)...), shared())
	b.lim.rows = 6
	_, err := gridOf(b)
	wantErr(t, err, ErrRowCap)
}

func TestGridCellsPerRowCap(t *testing.T) {
	var s strings.Builder
	s.WriteString(`<row r="1">`)
	for i := 0; i <= MaxCellsPerRow; i++ {
		fmt.Fprintf(&s, `<c r="%s1"><v>1</v></c>`, xlsxtest.ColName(i))
	}
	s.WriteString(`</row>`)
	b := bookOf(t, xlsxtest.SheetXML(s.String()), shared())
	_, err := gridOf(b)
	wantErr(t, err, ErrCellsPerRow)
}

func TestCellsBadSharedIndex(t *testing.T) {
	b := bookOf(t, xlsxtest.SheetXML(`<row r="1"><c r="A1" t="s"><v>9</v></c></row>`), shared())
	_, err := gridOf(b)
	wantErr(t, err, ErrSharedIndex)
}

func TestCellsBadCellType(t *testing.T) {
	b := bookOf(t, xlsxtest.SheetXML(`<row r="1"><c r="A1" t="str"><v>x</v></c></row>`), shared())
	_, err := gridOf(b)
	wantErr(t, err, ErrCellType)
}
