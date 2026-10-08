package xlsx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"airbg.org/internal/xlsx/xlsxtest"
)

var req = []string{"countryCode", "season"}

func bookOf(t *testing.T, sheet, shared string) *Book {
	t.Helper()
	b, err := open(t, xlsxtest.Minimal(sheet, shared))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func collect(b *Book, required []string) ([]Row, error) {
	var rows []Row
	err := b.Rows(xlsxtest.DefaultSheet, required, func(r Row) error {
		rows = append(rows, r)
		return nil
	})
	return rows, err
}

func rowsOf(t *testing.T, sheet, shared string) ([]Row, error) {
	t.Helper()
	return collect(bookOf(t, sheet, shared), req)
}

// header is row 1 with the required columns as shared strings 0 and 1.
const header = `<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>`

var headerShared = []string{`<si><t>countryCode</t></si>`, `<si><t>season</t></si>`}

func shared(extra ...string) string {
	return xlsxtest.SharedXML(append(append([]string{}, headerShared...), extra...)...)
}

func TestRowsMinimal(t *testing.T) {
	rows, err := rowsOf(t, xlsxtest.SheetXML(header,
		`<row r="2" spans="1:2"><c r="A2" t="s" s="3"><v>2</v></c><c r="B2"><v>2025</v></c></row>`),
		shared(`<si><t>BG</t></si>`))
	if err != nil {
		t.Fatal(err)
	}
	want := []Row{{"countryCode": "BG", "season": "2025"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %v", rows)
	}
}

func TestRowsByHeaderName(t *testing.T) {
	sheet, sst := xlsxtest.Table([][]string{
		{"lat", "season", "extra", "countryCode"},
		{"42.5", "2025", "x", "BG"},
		{"55.1", "2018", "y", "DK"},
	})
	rows, err := rowsOf(t, sheet, sst)
	if err != nil {
		t.Fatal(err)
	}
	want := []Row{
		{"lat": "42.5", "season": "2025", "extra": "x", "countryCode": "BG"},
		{"lat": "55.1", "season": "2018", "extra": "y", "countryCode": "DK"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %v", rows)
	}
}

func TestRowsIgnoresColumnsBeyondHeader(t *testing.T) {
	rows, err := rowsOf(t, xlsxtest.SheetXML(header,
		`<row r="2"><c r="A2"><v>1</v></c><c r="B2"><v>2</v></c><c r="Z2"><v>9</v></c></row>`), shared())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(rows[0]) != 2 {
		t.Fatalf("rows = %v", rows)
	}
}

func TestMissingRequiredColumn(t *testing.T) {
	sheet, sst := xlsxtest.Table([][]string{{"countryCode", "quality"}, {"BG", "1 - Excellent"}})
	_, err := rowsOf(t, sheet, sst)
	wantErr(t, err, ErrHeader)

	_, err = rowsOf(t, xlsxtest.SheetXML(), xlsxtest.SharedXML())
	wantErr(t, err, ErrHeader)
}

func TestDuplicateRequiredColumn(t *testing.T) {
	sheet, sst := xlsxtest.Table([][]string{{"countryCode", "season", "season"}, {"BG", "2025", "2024"}})
	_, err := rowsOf(t, sheet, sst)
	wantErr(t, err, ErrHeader)
}

func TestRichTextSharedString(t *testing.T) {
	rows, err := rowsOf(t, xlsxtest.SheetXML(header, `<row r="2"><c r="A2" t="s"><v>2</v></c></row>`),
		shared(`<si><r><rPr><b/></rPr><t>Bul</t></r><r><t xml:space="preserve">ga ria</t></r><rPh sb="0" eb="1"><t>X</t></rPh></si>`))
	if err != nil {
		t.Fatal(err)
	}
	if got := rows[0]["countryCode"]; got != "Bulga ria" {
		t.Fatalf("got %q", got)
	}
}

func TestSharedStringsCountCap(t *testing.T) {
	if MaxSharedStrings != 1_000_000 {
		t.Fatalf("MaxSharedStrings = %d", MaxSharedStrings)
	}
	gen := func(n int) string {
		var s strings.Builder
		s.WriteString(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
		for _, h := range headerShared {
			s.WriteString(h)
		}
		for i := 2; i < n; i++ {
			fmt.Fprintf(&s, `<si><t>%x</t></si>`, uint32(i)*2654435761)
		}
		s.WriteString(`</sst>`)
		return s.String()
	}
	if _, err := rowsOf(t, xlsxtest.SheetXML(header), gen(MaxSharedStrings)); err != nil {
		t.Fatalf("at cap: %v", err)
	}
	_, err := rowsOf(t, xlsxtest.SheetXML(header), gen(MaxSharedStrings+1))
	wantErr(t, err, ErrSharedStrings)
}

func TestSharedStringLengthCap(t *testing.T) {
	at := strings.Repeat("é", MaxStringBytes/2) // two bytes each
	if _, err := rowsOf(t, xlsxtest.SheetXML(header), shared(`<si><t>`+at+`</t></si>`)); err != nil {
		t.Fatalf("at cap: %v", err)
	}
	// Split over two runs so only the joined length is over.
	over := `<si><r><t>` + at + `</t></r><r><t>x</t></r></si>`
	_, err := rowsOf(t, xlsxtest.SheetXML(header), shared(over))
	wantErr(t, err, ErrSharedStrings)
}

func TestSharedStringsTotalCap(t *testing.T) {
	if MaxSharedStringsTotal != 32<<20 {
		t.Fatalf("MaxSharedStringsTotal = %d", MaxSharedStringsTotal)
	}
	b := bookOf(t, xlsxtest.SheetXML(header), shared(`<si><t>0123456789</t></si>`, `<si><t>0123456789</t></si>`))
	b.lim.strTotal = int64(len("countryCode") + len("season") + 15)
	_, err := collect(b, req)
	wantErr(t, err, ErrSharedStrings)
}

func TestSharedStringIndexOutOfRange(t *testing.T) {
	for _, v := range []string{"3", "-1", "1e1", "", "99999999999999999999"} {
		t.Run(v, func(t *testing.T) {
			_, err := rowsOf(t, xlsxtest.SheetXML(header, `<row r="2"><c r="A2" t="s"><v>`+v+`</v></c></row>`),
				shared(`<si><t>BG</t></si>`))
			wantErr(t, err, ErrSharedIndex)
		})
	}
}

func TestRejectsDoctype(t *testing.T) {
	dt := `<!DOCTYPE worksheet [<!ENTITY x "BG">]>`
	sheet := strings.Replace(xlsxtest.SheetXML(header), "?>", "?>"+dt, 1)
	_, err := rowsOf(t, sheet, shared())
	wantErr(t, err, ErrDirective)

	sst := strings.Replace(shared(), "?>", "?>"+dt, 1)
	_, err = rowsOf(t, xlsxtest.SheetXML(header), sst)
	wantErr(t, err, ErrDirective)

	// Workbook-level parts go through the same decoder.
	wb := strings.Replace(xlsxtest.Workbook, "?>", "?>"+dt, 1)
	_, err = open(t, xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.WorkbookName, Data: []byte(wb)}))
	wantErr(t, err, ErrDirective)
}

func TestRejectsUndeclaredEntity(t *testing.T) {
	// In a shared string no later check would catch a leaked entity.
	_, err := rowsOf(t, xlsxtest.SheetXML(header), shared(`<si><t>a&x;</t></si>`))
	if err == nil || !strings.Contains(err.Error(), "entity") {
		t.Fatalf("err = %v, want undeclared entity error", err)
	}
}

func TestRejectsFormula(t *testing.T) {
	for _, row := range []string{
		`<row r="2"><c r="A2"><f>1+1</f><v>2</v></c></row>`,
		`<row r="2"><c r="A2"><f t="shared" si="0"/><v>2</v></c></row>`,
	} {
		_, err := rowsOf(t, xlsxtest.SheetXML(header, row), shared())
		wantErr(t, err, ErrFormula)
	}
}

func TestRejectsInlineStr(t *testing.T) {
	for _, row := range []string{
		`<row r="2"><c r="A2" t="inlineStr"><is><t>BG</t></is></c></row>`,
		`<row r="2"><c r="A2"><is><t>BG</t></is></c></row>`,
	} {
		_, err := rowsOf(t, xlsxtest.SheetXML(header, row), shared())
		wantErr(t, err, ErrCellType)
	}
}

func TestRejectsErrorAndBoolCells(t *testing.T) {
	for _, typ := range []string{"e", "b", "str", "d", "x"} {
		t.Run(typ, func(t *testing.T) {
			row := `<row r="2"><c r="A2" t="` + typ + `"><v>1</v></c></row>`
			_, err := rowsOf(t, xlsxtest.SheetXML(header, row), shared())
			wantErr(t, err, ErrCellType)
		})
	}
}

func TestRejectsBadNumber(t *testing.T) {
	for _, v := range []string{"abc", "NaN", "Inf", "0x1p3", "1_0", "", "1.", ".5", "--1"} {
		t.Run(v, func(t *testing.T) {
			row := `<row r="2"><c r="B2" t="n"><v>` + v + `</v></c></row>`
			_, err := rowsOf(t, xlsxtest.SheetXML(header, row), shared())
			wantErr(t, err, ErrCellValue)
		})
	}
	for _, v := range []string{"2025", "-0.5", "1.25E-3", "42.123456789012344"} {
		row := `<row r="2"><c r="B2" t="n"><v>` + v + `</v></c></row>`
		if _, err := rowsOf(t, xlsxtest.SheetXML(header, row), shared()); err != nil {
			t.Fatalf("%s: %v", v, err)
		}
	}
}

func TestCellValueCap(t *testing.T) {
	row := `<row r="2"><c r="B2"><v>` + strings.Repeat("1", maxValueBytes+1) + `</v></c></row>`
	_, err := rowsOf(t, xlsxtest.SheetXML(header, row), shared())
	wantErr(t, err, ErrCellValue)
}

// letters is text that deflate cannot squeeze past the ratio cap.
func letters(n int) string {
	b := xlsxtest.Noise(n, 7)
	for i := range b {
		b[i] = 'a' + b[i]%26
	}
	return string(b)
}

func TestTokenSizeCap(t *testing.T) {
	// One text token larger than maxTokenBytes, outside any cell.
	sheet := strings.Replace(xlsxtest.SheetXML(header), "<sheetData>",
		"<sheetPr>"+letters(maxTokenBytes+1)+"</sheetPr><sheetData>", 1)
	_, err := rowsOf(t, sheet, shared())
	wantErr(t, err, ErrTokenTooLarge)
}

func TestRowCap(t *testing.T) {
	if MaxRows != 2_000_000 {
		t.Fatalf("MaxRows = %d", MaxRows)
	}
	var rows []string
	rows = append(rows, header)
	for i := 2; i <= 6; i++ {
		rows = append(rows, fmt.Sprintf(`<row r="%d"><c r="B%d"><v>%d</v></c></row>`, i, i, i))
	}
	b := bookOf(t, xlsxtest.SheetXML(rows...), shared())
	b.lim.rows = 6
	if _, err := collect(b, req); err != nil {
		t.Fatalf("at cap: %v", err)
	}
	b = bookOf(t, xlsxtest.SheetXML(append(rows, `<row r="7"/>`)...), shared())
	b.lim.rows = 6
	_, err := collect(b, req)
	wantErr(t, err, ErrRowCap)
}

func TestCellsPerRowCap(t *testing.T) {
	cells := func(n int) string {
		var s strings.Builder
		s.WriteString(`<row r="2">`)
		for i := range n {
			fmt.Fprintf(&s, `<c r="%s2"><v>1</v></c>`, xlsxtest.ColName(i))
		}
		s.WriteString(`</row>`)
		return s.String()
	}
	if _, err := rowsOf(t, xlsxtest.SheetXML(header, cells(MaxCellsPerRow)), shared()); err != nil {
		t.Fatalf("at cap: %v", err)
	}
	_, err := rowsOf(t, xlsxtest.SheetXML(header, cells(MaxCellsPerRow+1)), shared())
	wantErr(t, err, ErrCellsPerRow)
}

func TestDepthCap(t *testing.T) {
	nest := func(n int) string {
		return strings.Repeat("<x>", n) + strings.Repeat("</x>", n)
	}
	// worksheet is depth 1, so n nested elements reach depth n+1.
	ok := strings.Replace(xlsxtest.SheetXML(header), "<sheetData>", "<extLst>"+nest(MaxDepth-2)+"</extLst><sheetData>", 1)
	if _, err := rowsOf(t, ok, shared()); err != nil {
		t.Fatalf("at cap: %v", err)
	}
	deep := strings.Replace(xlsxtest.SheetXML(header), "<sheetData>", "<extLst>"+nest(MaxDepth-1)+"</extLst><sheetData>", 1)
	_, err := rowsOf(t, deep, shared())
	wantErr(t, err, ErrDepth)
}

func TestBadCellRef(t *testing.T) {
	for _, row := range []string{
		`<row r="2"><c r="1A"><v>1</v></c></row>`,
		`<row r="2"><c r="XFE2"><v>1</v></c></row>`,
		`<row r="2"><c r="AAAA2"><v>1</v></c></row>`,
		`<row r="2"><c r="A0"><v>1</v></c></row>`,
		`<row r="2"><c r="A02"><v>1</v></c></row>`,
		`<row r="2"><c r="a2"><v>1</v></c></row>`,
		`<row r="2"><c r="A"><v>1</v></c></row>`,
		`<row r="2"><c r=""><v>1</v></c></row>`,
		`<row r="2"><c r="A3"><v>1</v></c></row>`,                       // row mismatch
		`<row r="2"><c r="B2"><v>1</v></c><c r="A2"><v>1</v></c></row>`, // order
		`<row r="2"><c r="A2"><v>1</v></c><c r="A2"><v>2</v></c></row>`, // repeat
		`<row r="x"><c r="A2"><v>1</v></c></row>`,
		`<row r="1"><c r="A1"><v>1</v></c></row>`, // row not increasing
	} {
		t.Run(row, func(t *testing.T) {
			_, err := rowsOf(t, xlsxtest.SheetXML(header, row), shared())
			wantErr(t, err, ErrCellRef)
		})
	}
	// XFD is the last valid column.
	if _, err := rowsOf(t, xlsxtest.SheetXML(header, `<row r="2"><c r="XFD2"><v>1</v></c></row>`), shared()); err != nil {
		t.Fatalf("XFD: %v", err)
	}
}

func TestRowsUnknownSheet(t *testing.T) {
	b := bookOf(t, xlsxtest.SheetXML(header), shared())
	err := b.Rows("nope", req, func(Row) error { return nil })
	wantErr(t, err, ErrNoSheet)
}

func TestRowsCallbackError(t *testing.T) {
	stop := errors.New("stop")
	sheet, sst := xlsxtest.Table([][]string{{"countryCode", "season"}, {"BG", "1"}, {"BG", "2"}})
	calls := 0
	err := bookOf(t, sheet, sst).Rows(xlsxtest.DefaultSheet, req, func(Row) error {
		calls++
		return stop
	})
	if !errors.Is(err, stop) || calls != 1 {
		t.Fatalf("err = %v, calls = %d", err, calls)
	}
}

func TestRowsContextCancel(t *testing.T) {
	table := [][]string{{"countryCode", "season"}}
	for i := range 20000 {
		table = append(table, []string{"BG", fmt.Sprint(i)})
	}
	sheet, sst := xlsxtest.Table(table)
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := bookOf(t, sheet, sst).RowsContext(ctx, xlsxtest.DefaultSheet, req, func(Row) error {
		calls++
		if calls == 1 {
			cancel()
		}
		return nil
	})
	wantErr(t, err, context.Canceled)
	if calls >= 20000 {
		t.Fatalf("read all %d rows after cancel", calls)
	}

	// Already cancelled: nothing is read.
	err = bookOf(t, sheet, sst).RowsContext(ctx, xlsxtest.DefaultSheet, req, func(Row) error {
		t.Fatal("callback after cancel")
		return nil
	})
	wantErr(t, err, context.Canceled)
}

func TestRowsUsesSheetRels(t *testing.T) {
	// Sheet name resolves through workbook.xml and its rels, not a fixed path.
	wb := strings.Replace(xlsxtest.Workbook, `name="Sheet1"`, `name="data"`, 1)
	rels := strings.Replace(xlsxtest.WorkbookRels, "worksheets/sheet1.xml", "/xl/worksheets/other.xml", 1)
	sheet, sst := xlsxtest.Table([][]string{{"countryCode", "season"}, {"BG", "2025"}})
	parts := xlsxtest.Minimal(xlsxtest.SheetXML(), sst)
	parts = xlsxtest.Replace(parts, xlsxtest.Part{Name: xlsxtest.WorkbookName, Data: []byte(wb)})
	parts = xlsxtest.Replace(parts, xlsxtest.Part{Name: xlsxtest.WorkbookRelsName, Data: []byte(rels)})
	parts = append(parts, xlsxtest.Part{Name: "xl/worksheets/other.xml", Data: []byte(sheet)})
	b, err := open(t, parts)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	if err := b.Rows("data", req, func(Row) error { n++; return nil }); err != nil || n != 1 {
		t.Fatalf("err = %v, n = %d", err, n)
	}
}

func TestCapsMatchPlanSheet(t *testing.T) {
	if MaxStringBytes != 4096 || MaxCellsPerRow != 64 || MaxColumn != 16384 {
		t.Fatal("caps drifted from the plan")
	}
}

// BenchmarkRowsLarge streams a synthetic sheet shaped like the EEA file:
// 700k rows, 13 columns, about 90k distinct shared strings.
func BenchmarkRowsLarge(b *testing.B) {
	const nRows, nStrings = 700_000, 90_000
	cols := []string{"countryCode", "bathingWaterIdentifier", "groupIdentifier", "bathingWaterName",
		"bathingWaterType", "geographicalConstraint", "lon", "lat", "bwProfile", "season", "quality",
		"monitoringCalendar", "management"}
	path := filepath.Join(b.TempDir(), "large.xlsx")
	f, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	sheet := func(w io.Writer) error {
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1" spans="1:13">`)
		for i := range cols {
			fmt.Fprintf(w, `<c r="%s1" t="s"><v>%d</v></c>`, xlsxtest.ColName(i), i)
		}
		fmt.Fprint(w, `</row>`)
		for r := 2; r <= nRows+1; r++ {
			fmt.Fprintf(w, `<row r="%d" spans="1:13">`, r)
			for c := range cols {
				ref := xlsxtest.ColName(c) + fmt.Sprint(r)
				switch c {
				case 6, 7:
					fmt.Fprintf(w, `<c r="%s" s="1"><v>%d.%06d</v></c>`, ref, r%90, r)
				case 9:
					fmt.Fprintf(w, `<c r="%s"><v>%d</v></c>`, ref, 1990+r%36)
				default:
					fmt.Fprintf(w, `<c r="%s" t="s"><v>%d</v></c>`, ref, len(cols)+(r*13+c)%(nStrings-len(cols)))
				}
			}
			fmt.Fprint(w, `</row>`)
		}
		_, err := fmt.Fprint(w, `</sheetData></worksheet>`)
		return err
	}
	sst := func(w io.Writer) error {
		fmt.Fprint(w, `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
		for _, c := range cols {
			fmt.Fprintf(w, `<si><t>%s</t></si>`, c)
		}
		for i := len(cols); i < nStrings; i++ {
			fmt.Fprintf(w, `<si><t>string value number %d with some length</t></si>`, i)
		}
		_, err := fmt.Fprint(w, `</sst>`)
		return err
	}
	parts := xlsxtest.Replace(xlsxtest.Minimal("", ""), xlsxtest.Part{Name: xlsxtest.SheetName, Stream: sheet})
	parts = xlsxtest.Replace(parts, xlsxtest.Part{Name: xlsxtest.SharedName, Stream: sst})
	if err := xlsxtest.BuildTo(f, xlsxtest.Options{Parts: parts}); err != nil {
		b.Fatal(err)
	}
	st, _ := f.Stat()

	var peak uint64
	sample := func() {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		peak = max(peak, m.HeapInuse)
	}
	b.ResetTimer()
	for range b.N {
		book, err := Open(f, st.Size())
		if err != nil {
			b.Fatal(err)
		}
		n := 0
		err = book.Rows(xlsxtest.DefaultSheet, []string{"countryCode", "season"}, func(Row) error {
			if n++; n%50_000 == 0 {
				sample()
			}
			return nil
		})
		if err != nil || n != nRows {
			b.Fatalf("err = %v, rows = %d", err, n)
		}
	}
	b.StopTimer()
	f.Close()
	b.ReportMetric(float64(peak)/(1<<20), "peak-heap-MiB")
	b.ReportMetric(float64(st.Size())/(1<<20), "zip-MiB")
}
