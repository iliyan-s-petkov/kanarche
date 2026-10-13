package dobrich

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/upstream/rzi"
	"kanarche.eu/internal/xlsx/xlsxtest"
)

var now = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

func hasWarning(f rzi.File, sub string) bool {
	for _, w := range f.Warnings {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

// roundTrip proves the record decodes under the contract and validates.
func roundTrip(t *testing.T, f rzi.File) (rzi.Verdict, rzi.File) {
	t.Helper()
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	got, err := rzi.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return rzi.ValidateDetailed(got, sites, rzi.LastGood{}, now), got
}

func TestExtractSyntheticBlock(t *testing.T) {
	f := Extract(book(t, pointSheet("08001",
		okRow(8, d1, lt(15), lt(15)),
		data(9, nu(colPlan, d2), nu(colDate, d2+1), at(colIE, eq(15)), at(colEC, eq(192)),
			tx(colInfo, "info"), tx(colMeasures, "none"), tx(colVisual, "visual")),
	)))
	if f.Error != "" || len(f.Warnings) != 0 {
		t.Fatalf("error %q warnings %v", f.Error, f.Warnings)
	}
	if f.Schema != rzi.SchemaVersion || f.Source != rzi.SourceDobrich || f.ExtractorVersion != ExtractorVersion {
		t.Fatalf("envelope: %+v", f)
	}
	if len(f.Blocks) != 1 {
		t.Fatalf("blocks = %d", len(f.Blocks))
	}
	b := f.Blocks[0]
	if b.Point != "08001" || b.Name != "Тестов плаж" {
		t.Fatalf("point %q name %q", b.Point, b.Name)
	}
	if d := b.Lat - (43 + 43.0/60 + 36.0/3600); d > 1e-9 || d < -1e-9 {
		t.Fatalf("lat %v", b.Lat)
	}
	if d := b.Lon - (28 + 34.0/60 + 25.0/3600); d > 1e-9 || d < -1e-9 {
		t.Fatalf("lon %v", b.Lon)
	}
	want := []rzi.Row{
		{Planned: "2026-06-08", Date: "2026-06-08", IE: rzi.Count{Op: "<", Value: 15}, EC: rzi.Count{Op: "<", Value: 15}, Unit: "MPN/100ml"},
		{Planned: "2026-06-22", Date: "2026-06-23", IE: rzi.Count{Op: "=", Value: 15}, EC: rzi.Count{Op: "=", Value: 192}, Unit: "MPN/100ml", Remarks: "info; none; visual"},
	}
	if len(b.Rows) != len(want) {
		t.Fatalf("rows = %+v", b.Rows)
	}
	for i := range want {
		if b.Rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, b.Rows[i], want[i])
		}
	}
}

func TestCountForms(t *testing.T) {
	cases := []struct {
		name string
		cell cellv
		want rzi.Count
		ok   bool
	}{
		{"below detection text", tx(0, "<15"), rzi.Count{Op: "<", Value: 15}, true},
		{"below with space", tx(0, "< 15"), rzi.Count{Op: "<", Value: 15}, true},
		{"numeric exact", nu(0, 192), rzi.Count{Op: "=", Value: 192}, true},
		{"text exact", tx(0, "15"), rzi.Count{Op: "=", Value: 15}, true},
		{"zero", nu(0, 0), rzi.Count{Op: "=", Value: 0}, true},
		{"fraction", nu(0, 15.5), rzi.Count{}, false},
		{"negative", nu(0, -3), rzi.Count{}, false},
		{"over the cap", nu(0, rzi.MaxCount+1), rzi.Count{}, false},
		{"below zero", tx(0, "<0"), rzi.Count{}, false},
		{"greater than", tx(0, ">15"), rzi.Count{}, false},
		{"words", tx(0, "n/a"), rzi.Count{}, false},
		{"huge text", tx(0, "99999999999999999999"), rzi.Count{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := Extract(book(t, pointSheet("08001", okRow(8, d1, c.cell, lt(15)))))
			if c.ok {
				if f.Error != "" || len(f.Blocks) != 1 || len(f.Blocks[0].Rows) != 1 || f.Blocks[0].Rows[0].IE != c.want {
					t.Fatalf("got %+v", f)
				}
				return
			}
			// A bad value drops the row, with a warning, and the block
			// then has no rows.
			if !hasWarning(f, "unreadable value") {
				t.Fatalf("warnings %v", f.Warnings)
			}
			if len(f.Blocks) == 1 && len(f.Blocks[0].Rows) != 0 {
				t.Fatalf("row kept: %+v", f.Blocks[0].Rows)
			}
		})
	}
}

func TestDateCells(t *testing.T) {
	f := Extract(book(t, pointSheet("08001",
		// Real date cell, a moved sample, and an ISO text date.
		data(8, nu(colPlan, d1), nu(colDate, d1+2), at(colIE, lt(15)), at(colEC, lt(15))),
		data(9, nu(colPlan, d2), tx(colDate, "2026-06-26"), at(colIE, lt(15)), at(colEC, lt(15))),
		data(10, nu(colPlan, d2), tx(colDate, "5.7.2026 г."), at(colIE, lt(15)), at(colEC, lt(15))),
		// A date-time serial keeps its day.
		data(11, nu(colPlan, d2), nu(colDate, d2+0.75), at(colIE, lt(15)), at(colEC, lt(15))),
	)))
	if f.Error != "" || len(f.Warnings) != 0 {
		t.Fatalf("%+v", f)
	}
	r := f.Blocks[0].Rows
	if r[0].Planned != "2026-06-08" || r[0].Date != "2026-06-10" || r[1].Date != "2026-06-26" || r[2].Date != "2026-07-05" || r[3].Date != "2026-06-22" {
		t.Fatalf("rows %+v", r)
	}
}

func TestBadDateTextDropsRowWithWarning(t *testing.T) {
	f := Extract(book(t, pointSheet("08001",
		okRow(8, d1, lt(15), lt(15)),
		data(9, nu(colPlan, d2), tx(colDate, "2026-06=02"), at(colIE, lt(15)), at(colEC, lt(15))),
		okRow(10, d2+14, lt(15), lt(15)),
	)))
	if f.Error != "" || len(f.Blocks) != 1 {
		t.Fatalf("%+v", f)
	}
	if n := len(f.Blocks[0].Rows); n != 2 {
		t.Fatalf("rows = %d", n)
	}
	if !hasWarning(f, "point 08001 row 9: unreadable sample date") {
		t.Fatalf("warnings %v", f.Warnings)
	}
	// The warning carries no document text.
	for _, w := range f.Warnings {
		if strings.Contains(w, "=") {
			t.Fatalf("warning leaks cell text: %q", w)
		}
	}
}

func TestOtherDateFailures(t *testing.T) {
	cases := map[string]struct {
		row  srow
		warn string
	}{
		"calendar-invalid text": {data(8, nu(colPlan, d1), tx(colDate, "2026-02-30"), at(colIE, lt(15)), at(colEC, lt(15))), "unreadable sample date"},
		"dotted impossible day": {data(8, nu(colPlan, d1), tx(colDate, "31.02.2026"), at(colIE, lt(15)), at(colEC, lt(15))), "unreadable sample date"},
		"dotted month 13":       {data(8, nu(colPlan, d1), tx(colDate, "01.13.2026"), at(colIE, lt(15)), at(colEC, lt(15))), "unreadable sample date"},
		"iso year too old":      {data(8, nu(colPlan, d1), tx(colDate, "1899-06-01"), at(colIE, lt(15)), at(colEC, lt(15))), "unreadable sample date"},
		"dotted year too old":   {data(8, nu(colPlan, d1), tx(colDate, "01.06.1899"), at(colIE, lt(15)), at(colEC, lt(15))), "unreadable sample date"},
		"serial too small":      {data(8, nu(colPlan, d1), nu(colDate, 12), at(colIE, lt(15)), at(colEC, lt(15))), "unreadable sample date"},
		"serial too large":      {data(8, nu(colPlan, d1), nu(colDate, 9999999), at(colIE, lt(15)), at(colEC, lt(15))), "unreadable sample date"},
		"serial below floor":    {data(8, nu(colPlan, d1), nu(colDate, 60.9), at(colIE, lt(15)), at(colEC, lt(15))), "unreadable sample date"},
		"planned unreadable":    {data(8, tx(colPlan, "soon"), nu(colDate, d1), at(colIE, lt(15)), at(colEC, lt(15))), "unreadable planned date"},
		"date missing":          {data(8, nu(colPlan, d1), at(colIE, lt(15)), at(colEC, lt(15))), "sample date missing"},
		"value missing":         {data(8, nu(colPlan, d1), nu(colDate, d1), at(colIE, lt(15))), "value missing"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := Extract(book(t, pointSheet("08001", c.row, okRow(9, d2, lt(15), lt(15)))))
			if f.Error != "" || len(f.Blocks) != 1 || len(f.Blocks[0].Rows) != 1 {
				t.Fatalf("%+v", f)
			}
			if !hasWarning(f, c.warn) {
				t.Fatalf("warnings %v", f.Warnings)
			}
		})
	}
}

func TestSkippedRowsAreQuiet(t *testing.T) {
	// Footnote text under the table and a planned-only row are not data.
	f := Extract(book(t, pointSheet("08001",
		okRow(8, d1, lt(15), lt(15)),
		data(9, nu(colPlan, d2)),
		srow{12, nil},
		data(22, tx(colPlan, "* бележка под таблицата")),
		data(23, tx(colPlan, "** още една")),
	)))
	if f.Error != "" || len(f.Warnings) != 0 || len(f.Blocks[0].Rows) != 1 {
		t.Fatalf("%+v", f)
	}
}

func TestSheetNameMustMatchPoint(t *testing.T) {
	wrong := pointSheet("08002", okRow(8, d1, lt(15), lt(15)))
	wrong.name = "КР №08001"
	f := Extract(book(t, wrong, pointSheet("08002", okRow(8, d1, lt(15), lt(15)))))
	if f.Error != "" {
		t.Fatal(f.Error)
	}
	// The sheet named 08001 whose label says 08002 is dropped whole.
	if len(f.Blocks) != 1 || f.Blocks[0].Point != "08002" {
		t.Fatalf("blocks %+v", f.Blocks)
	}
	if !hasWarning(f, "differs from the sheet name") {
		t.Fatalf("warnings %v", f.Warnings)
	}
}

func TestUnparsableSheetNameSkipped(t *testing.T) {
	other := pointSheet("08001", okRow(8, d1, lt(15), lt(15)))
	other.name = "Лист1"
	f := Extract(book(t, other, pointSheet("08002", okRow(8, d1, lt(15), lt(15)))))
	if len(f.Blocks) != 1 || !hasWarning(f, "sheet 1: not a point sheet") {
		t.Fatalf("%+v", f)
	}
}

func TestMissingLabel(t *testing.T) {
	s := pointSheet("08001", okRow(8, d1, lt(15), lt(15))).without(3)
	f := Extract(book(t, s, pointSheet("08002", okRow(8, d1, lt(15), lt(15)))))
	if len(f.Blocks) != 1 || !hasWarning(f, "point label missing") {
		t.Fatalf("%+v", f)
	}
}

func TestCoordinateForms(t *testing.T) {
	cases := map[string]struct {
		lat, lon string
		ok       bool
	}{
		"degree sign ordinal":  {`43º 43' 36"`, `28º 34' 25"`, true},
		"degree sign proper":   {`43° 43′ 36″`, `28° 34′ 25″`, true},
		"extra spaces":         {`  43º 43' 36" `, ` 28º 34' 25"`, true},
		"no seconds":           {`43º 43'`, `28º 34'`, true},
		"decimal seconds":      {`43º 43' 36,5"`, `28º 34' 25.5"`, true},
		"latitude over 90":     {`91º 43' 36"`, `28º 34' 25"`, false},
		"minutes over 59":      {`43º 61' 36"`, `28º 34' 25"`, false},
		"seconds over 59":      {`43º 43' 75"`, `28º 34' 25"`, false},
		"no numbers":           {`—`, `—`, false},
		"four numbers":         {`43 43 36 1`, `28º 34' 25"`, false},
		"longitude over 180":   {`43º 43' 36"`, `181º 34' 25"`, false},
		"letters without data": {``, ``, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s := pointSheet("08001", okRow(8, d1, lt(15), lt(15)))
			s.rows[1] = coordRow(c.lat, c.lon)
			f := Extract(book(t, s, pointSheet("08002", okRow(8, d1, lt(15), lt(15)))))
			if f.Error != "" {
				t.Fatal(f.Error)
			}
			if c.ok && len(f.Blocks) != 2 || !c.ok && (len(f.Blocks) != 1 || !hasWarning(f, "point 08001: coordinates missing or unreadable")) {
				t.Fatalf("blocks %d warnings %v", len(f.Blocks), f.Warnings)
			}
		})
	}
}

func TestCoordinatesMissingRow(t *testing.T) {
	s := pointSheet("08001", okRow(8, d1, lt(15), lt(15))).without(4)
	f := Extract(book(t, s, pointSheet("08002", okRow(8, d1, lt(15), lt(15)))))
	if len(f.Blocks) != 1 || !hasWarning(f, "coordinates missing") {
		t.Fatalf("%+v", f)
	}
}

func TestHeaderProblems(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		s := pointSheet("08001", okRow(8, d1, lt(15), lt(15))).without(7)
		f := Extract(book(t, s, pointSheet("08002", okRow(8, d1, lt(15), lt(15)))))
		if len(f.Blocks) != 1 || !hasWarning(f, "point 08001: table header missing") {
			t.Fatalf("%+v", f)
		}
	})
	t.Run("unit differs between indicators", func(t *testing.T) {
		s := pointSheet("08001", okRow(8, d1, lt(15), lt(15)))
		s.rows[2].cells[3] = tx(colEC, "Резултат от анализ за Ешерихия коли cfu/100ml")
		f := Extract(book(t, s, pointSheet("08002", okRow(8, d1, lt(15), lt(15)))))
		if len(f.Blocks) != 1 || !hasWarning(f, "table header not recognised") {
			t.Fatalf("%+v", f)
		}
	})
	t.Run("no unit", func(t *testing.T) {
		s := pointSheet("08001", okRow(8, d1, lt(15), lt(15)))
		s.rows[2] = headerRow("")
		f := Extract(book(t, s))
		if f.Error == "" || len(f.Blocks) != 0 {
			t.Fatalf("%+v", f)
		}
	})
	t.Run("no sample date column", func(t *testing.T) {
		s := pointSheet("08001", okRow(8, d1, lt(15), lt(15)))
		s.rows[2].cells[1] = tx(colDate, "Нещо друго")
		f := Extract(book(t, s))
		if f.Error == "" {
			t.Fatalf("%+v", f)
		}
	})
	t.Run("cfu unit", func(t *testing.T) {
		s := pointSheet("08001", okRow(8, d1, lt(15), lt(15)))
		s.rows[2] = headerRow("CFU/100 ml")
		f := Extract(book(t, s))
		if f.Error != "" || f.Blocks[0].Rows[0].Unit != "cfu/100ml" {
			t.Fatalf("%+v", f)
		}
	})
	t.Run("header moved down", func(t *testing.T) {
		s := pointSheet("08001").without(7).with(headerRow("MPN/100cm3"))
		s.rows[len(s.rows)-1].r = 10
		s = s.with(okRow(11, d1, lt(15), lt(15)))
		f := Extract(book(t, s))
		if f.Error != "" || len(f.Blocks[0].Rows) != 1 {
			t.Fatalf("%+v", f)
		}
	})
}

func TestNoPointSheets(t *testing.T) {
	f := Extract(book(t, sheetSpec{name: "Лист1", rows: []srow{{1, []cellv{tx(0, "x")}}}}))
	if f.Error != "no sampling point blocks found" || len(f.Blocks) != 0 {
		t.Fatalf("%+v", f)
	}
}

func TestSheetCap(t *testing.T) {
	mk := func(n int) []byte {
		var sheets []sheetSpec
		for i := 1; i <= n; i++ {
			sheets = append(sheets, pointSheet(fmt.Sprintf("08%03d", i), okRow(8, d1, lt(15), lt(15))))
		}
		return book(t, sheets...)
	}
	f := Extract(mk(rzi.MaxBlocks))
	if f.Error != "" || len(f.Blocks) != rzi.MaxBlocks {
		t.Fatalf("at the cap: error %q, %d blocks", f.Error, len(f.Blocks))
	}
	f = Extract(mk(rzi.MaxBlocks + 1))
	if f.Error != "document over a structural cap" || len(f.Blocks) != 0 {
		t.Fatalf("over the cap: error %q, %d blocks", f.Error, len(f.Blocks))
	}
}

func TestRowCap(t *testing.T) {
	mk := func(n int) []byte {
		var rows []srow
		for i := 0; i < n; i++ {
			rows = append(rows, okRow(8+i, d1+float64(i), lt(15), lt(15)))
		}
		return book(t, pointSheet("08001", rows...))
	}
	f := Extract(mk(rzi.MaxRowsPerBlock))
	if f.Error != "" || len(f.Blocks[0].Rows) != rzi.MaxRowsPerBlock {
		t.Fatalf("at the cap: %q", f.Error)
	}
	// Over the cap drops that block only; other points are still read.
	var rows []srow
	for i := 0; i <= rzi.MaxRowsPerBlock; i++ {
		rows = append(rows, okRow(8+i, d1+float64(i), lt(15), lt(15)))
	}
	f = Extract(book(t, pointSheet("08001", rows...), pointSheet("08002", okRow(8, d1, lt(15), lt(15)))))
	if f.Error != "" || len(f.Blocks) != 1 || f.Blocks[0].Point != "08002" {
		t.Fatalf("over the cap: %q, blocks %+v", f.Error, f.Blocks)
	}
	if !hasWarning(f, "point 08001: more than 60 rows") {
		t.Fatalf("warnings %v", f.Warnings)
	}
	// With nothing else to extract the record is a plain refusal.
	f = Extract(mk(rzi.MaxRowsPerBlock + 1))
	if f.Error != "no sampling point blocks found" {
		t.Fatalf("only capped block: %q", f.Error)
	}
}

// date1904Book is the standard workbook with the 1904 date system flag.
func date1904Book(t *testing.T, flag string) []byte {
	t.Helper()
	parts := bookParts([]sheetSpec{pointSheet("08001", okRow(8, d1, lt(15), lt(15)))})
	for i, p := range parts {
		if p.Name == xlsxtest.WorkbookName {
			parts[i].Data = []byte(strings.Replace(string(p.Data), "<sheets>", `<workbookPr date1904="`+flag+`"/><sheets>`, 1))
		}
	}
	b, err := xlsxtest.Build(xlsxtest.Options{Parts: parts})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDate1904Refused(t *testing.T) {
	for _, flag := range []string{"1", "true"} {
		f := Extract(date1904Book(t, flag))
		if f.Error != "workbook uses the 1904 date system" || len(f.Blocks) != 0 {
			t.Fatalf("flag %s: error %q blocks %d", flag, f.Error, len(f.Blocks))
		}
	}
	for _, flag := range []string{"0", "false"} {
		if f := Extract(date1904Book(t, flag)); f.Error != "" || len(f.Blocks) != 1 {
			t.Fatalf("flag %s: error %q", flag, f.Error)
		}
	}
}

func TestWarningsCapped(t *testing.T) {
	var rows []srow
	for i := 0; i < 50; i++ {
		rows = append(rows, data(8+i, nu(colPlan, d1), tx(colDate, "bad")))
	}
	var sheets []sheetSpec
	for i := 1; i <= 10; i++ {
		sheets = append(sheets, pointSheet(fmt.Sprintf("08%03d", i), rows...))
	}
	f := Extract(book(t, sheets...))
	if len(f.Warnings) > maxWarnings+1 || !hasWarning(f, "more warnings suppressed") {
		t.Fatalf("%d warnings", len(f.Warnings))
	}
}

func TestRefusals(t *testing.T) {
	good := book(t, pointSheet("08001", okRow(8, d1, lt(15), lt(15))))
	cases := map[string]struct {
		data []byte
		want string
	}{
		"not a zip":   {[]byte("PK\x03\x04 but not a zip"), "unreadable document"},
		"legacy xls":  {[]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0, 0}, "unreadable document"},
		"empty":       {nil, "unreadable document"},
		"truncated":   {good[:len(good)/2], "unreadable document"},
		"formula":     {formulaBook(t), "unsupported workbook content"},
		"zip bomb":    {bombBook(t), "document over a structural cap"},
		"inline text": {inlineBook(t), "unsupported workbook content"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := Extract(c.data)
			if f.Error != c.want || len(f.Blocks) != 0 || f.Blocks == nil || f.Warnings == nil {
				t.Fatalf("error %q blocks %v", f.Error, f.Blocks)
			}
			if f.SHA256 == "" || f.Source != rzi.SourceDobrich {
				t.Fatalf("envelope %+v", f)
			}
		})
	}
}

func formulaBook(t *testing.T) []byte {
	parts := bookParts([]sheetSpec{pointSheet("08001")})
	parts = xlsxtest.Replace(parts, xlsxtest.Part{Name: "xl/worksheets/sheet1.xml", Data: []byte(
		xlsxtest.SheetXML(`<row r="8"><c r="A8"><f>1+1</f><v>2</v></c></row>`))})
	b, err := xlsxtest.Build(xlsxtest.Options{Parts: parts})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func inlineBook(t *testing.T) []byte {
	parts := bookParts([]sheetSpec{pointSheet("08001")})
	parts = xlsxtest.Replace(parts, xlsxtest.Part{Name: "xl/worksheets/sheet1.xml", Data: []byte(
		xlsxtest.SheetXML(`<row r="8"><c r="A8" t="inlineStr"><is><t>x</t></is></c></row>`))})
	b, err := xlsxtest.Build(xlsxtest.Options{Parts: parts})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// bombBook holds a sheet that inflates far past the ratio cap.
func bombBook(t *testing.T) []byte {
	parts := bookParts([]sheetSpec{pointSheet("08001")})
	parts = xlsxtest.Replace(parts, xlsxtest.Part{Name: "xl/worksheets/sheet1.xml", Stream: func(w io.Writer) error {
		if _, err := io.WriteString(w, xlsxtest.SheetXML()[:strings.Index(xlsxtest.SheetXML(), "</sheetData>")]); err != nil {
			return err
		}
		pad := bytes.Repeat([]byte(" "), 1<<20)
		for range 64 {
			if _, err := w.Write(pad); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "</sheetData></worksheet>")
		return err
	}})
	b, err := xlsxtest.Build(xlsxtest.Options{Parts: parts})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRecordValidatesOnIngest(t *testing.T) {
	// A format mismatch with the app side would show up here as a status
	// other than ok.
	f := Extract(book(t,
		pointSheet("08001",
			okRow(8, d1, lt(15), lt(15)),
			okRow(9, d2, eq(15), eq(192)),
		),
		withCoords(pointSheet("08002", okRow(8, d1, eq(30), lt(15))), `43º 41' 47"`, `28º 33' 58"`),
	))
	if f.Error != "" || len(f.Warnings) != 0 {
		t.Fatalf("%q %v", f.Error, f.Warnings)
	}
	v, got := roundTrip(t, f)
	if v.Status != rzi.StatusOK || len(v.Rejects) != 0 {
		t.Fatalf("status %s rejects %+v", v.Status, v.Rejects)
	}
	if len(v.Accepted) != 3 || len(got.Blocks) != 2 {
		t.Fatalf("accepted %d", len(v.Accepted))
	}
	byKey := map[string]rzi.Sample{}
	for _, s := range v.Accepted {
		byKey[s.SiteID+s.Date.Format(time.DateOnly)] = s
	}
	s := byKey["BG33000000000080012026-06-22"]
	if s.EC != 192 || s.ECBelow || s.IE != 15 || s.IEBelow || s.Unit != "MPN/100ml" {
		t.Fatalf("sample %+v", s)
	}
	s = byKey["BG33000000000080012026-06-08"]
	if s.EC != 15 || !s.ECBelow || !s.IEBelow || s.Season != 2026 {
		t.Fatalf("sample %+v", s)
	}
}

func TestWarningsNeedReview(t *testing.T) {
	// A dropped row keeps the file out of ok: an operator decides.
	f := Extract(book(t, pointSheet("08001",
		okRow(8, d1, lt(15), lt(15)),
		data(9, nu(colPlan, d2), tx(colDate, "2026-06=02"), at(colIE, lt(15)), at(colEC, lt(15))),
	)))
	v, _ := roundTrip(t, f)
	if v.Status != rzi.StatusNeedsReview || len(v.Accepted) != 1 {
		t.Fatalf("status %s accepted %d", v.Status, len(v.Accepted))
	}
}

func TestNameIsClippedAndRemarksBounded(t *testing.T) {
	s := pointSheet("08001", data(8, nu(colPlan, d1), nu(colDate, d1), at(colIE, lt(15)), at(colEC, lt(15)),
		tx(colInfo, strings.Repeat("я", rzi.MaxRemarksRunes*2))))
	s.rows[0] = srow{3, []cellv{tx(0, "Пункт за вземане на проби №08001 Наименование "+strings.Repeat("я", rzi.MaxNameRunes*2))}}
	f := Extract(book(t, s))
	if f.Error != "" {
		t.Fatal(f.Error)
	}
	b := f.Blocks[0]
	if n := len([]rune(b.Name)); n != rzi.MaxNameRunes {
		t.Fatalf("name runes %d", n)
	}
	if n := len([]rune(b.Rows[0].Remarks)); n != rzi.MaxRemarksRunes {
		t.Fatalf("remarks runes %d", n)
	}
	if v, _ := roundTrip(t, f); v.Status != rzi.StatusOK {
		t.Fatalf("status %s %+v", v.Status, v.Rejects)
	}
}

func TestOutputOverSizeCap(t *testing.T) {
	// Many long remarks across blocks push the record past the contract cap.
	long := strings.Repeat("я", rzi.MaxRemarksRunes)
	var sheets []sheetSpec
	for i := 1; i <= rzi.MaxBlocks; i++ {
		var rows []srow
		for j := 0; j < rzi.MaxRowsPerBlock; j++ {
			rows = append(rows, data(8+j, nu(colPlan, d1), nu(colDate, d1+float64(j)), at(colIE, lt(15)), at(colEC, lt(15)), tx(colInfo, long)))
		}
		sheets = append(sheets, pointSheet(fmt.Sprintf("08%03d", i), rows...))
	}
	f := Extract(book(t, sheets...))
	if f.Error != "output over size cap" || len(f.Blocks) != 0 {
		t.Fatalf("error %q blocks %d", f.Error, len(f.Blocks))
	}
}

func TestRemarksTextBothColumnsOptional(t *testing.T) {
	s := pointSheet("08001", okRow(8, d1, lt(15), lt(15)))
	s.rows[2] = srow{7, s.rows[2].cells[:4]} // header without the remark columns
	f := Extract(book(t, s))
	if f.Error != "" || f.Blocks[0].Rows[0].Remarks != "" {
		t.Fatalf("%+v", f)
	}
}

func withCoords(s sheetSpec, lat, lon string) sheetSpec {
	s.rows = append([]srow(nil), s.rows...)
	s.rows[1] = coordRow(lat, lon)
	return s
}
