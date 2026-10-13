package dobrich

import (
	"encoding/json"
	"testing"
	"time"

	"kanarche.eu/internal/upstream/rzi"
	"kanarche.eu/internal/xlsx/xlsxtest"
)

// FuzzDobrich feeds arbitrary bytes as a whole file, as the row XML of a
// worksheet inside a valid container, and as cell text to the text parsers,
// so mutations reach the sheet walker and not just the zip layer.
func FuzzDobrich(f *testing.F) {
	good := book(f, pointSheet("08001",
		okRow(8, d1, lt(15), lt(15)),
		okRow(9, d2, eq(15), eq(192)),
		data(10, nu(colPlan, d2), tx(colDate, "2026-06=02")),
	))
	f.Add(good)
	f.Add(good[:len(good)/2])
	f.Add([]byte(`<row r="8"><c r="A8"><v>46181</v></c><c r="B8"><v>46181</v></c><c r="C8" t="s"><v>0</v></c></row>`))
	f.Add([]byte(`<row r="3"><c r="A3" t="s"><v>1</v></c></row>`))
	f.Add([]byte(`<row r="7"><c r="C7" t="s"><v>2</v></c></row><row r="2"/>`))
	f.Add([]byte(`Географски координати: ширина N 43º 43' 36" дължина E 28º 34' 25"`))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, in []byte) {
		start := time.Now()
		defer func() {
			if d := time.Since(start); d > 2*time.Second {
				t.Errorf("input took %v", d)
			}
		}()
		check(t, Extract(in))
		check(t, Extract(wrapRows(t, string(in))))
		// Direct calls run without Extract's recover, so a panic fails here.
		parseCoords(string(in))
		parseLabel(string(in))
	})
}

// wrapRows puts rows inside a valid one-sheet workbook whose shared strings
// hold the label, the coordinates and the header text.
func wrapRows(t *testing.T, rows string) []byte {
	t.Helper()
	parts := bookParts([]sheetSpec{pointSheet("08001")})
	sheet := xlsxtest.SheetXML() // keeps the document frame
	frame := sheet[:len(sheet)-len("</sheetData></worksheet>")]
	parts = xlsxtest.Replace(parts, xlsxtest.Part{Name: "xl/worksheets/sheet1.xml", Data: []byte(frame + rows + "</sheetData></worksheet>")})
	b, err := xlsxtest.Build(xlsxtest.Options{Parts: parts})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// check holds the invariants every result must keep.
func check(t *testing.T, f rzi.File) {
	t.Helper()
	if f.Schema != rzi.SchemaVersion || f.Source != rzi.SourceDobrich || f.Blocks == nil || f.Warnings == nil {
		t.Fatalf("bad envelope: %+v", f)
	}
	if f.Error != "" && len(f.Blocks) != 0 {
		t.Fatal("error record carries blocks")
	}
	if len(f.Blocks) > rzi.MaxBlocks {
		t.Fatalf("%d blocks", len(f.Blocks))
	}
	raw, err := json.Marshal(f)
	if err != nil || len(raw) > rzi.MaxRecordBytes {
		t.Fatalf("marshal: %v, %d bytes", err, len(raw))
	}
	// Validate must not panic whatever the extractor produced.
	rzi.Validate(f, sites, rzi.LastGood{}, now)
}
