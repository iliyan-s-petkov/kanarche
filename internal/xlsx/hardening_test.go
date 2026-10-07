package xlsx

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"airbg.org/internal/xlsx/xlsxtest"
)

const relsHead = `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`

// paddedRels is a valid rels document of exactly n bytes.
func paddedRels(n int) []byte {
	head, tail := relsHead+"<!--", "--></Relationships>"
	return []byte(head + letters(n-len(head)-len(tail)) + tail)
}

func relsWith(rel string) string {
	return strings.Replace(xlsxtest.WorkbookRels, "</Relationships>", rel+"</Relationships>", 1)
}

func withWorkbookRels(rels string) []xlsxtest.Part {
	return xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.WorkbookRelsName, Data: []byte(rels)})
}

// Finding 1: Open work is bounded and cancellable.

func TestOpenSkipsUnlistedSheetRels(t *testing.T) {
	parts := minimalParts()
	for i := range 50 {
		parts = append(parts, xlsxtest.Part{Name: fmt.Sprintf("xl/worksheets/_rels/s%d.xml.rels", i), Stream: xlsxtest.Zeros(1 << 20)})
	}
	b, err := open(t, parts)
	if err != nil {
		t.Fatal(err)
	}
	if b.total > 64<<10 {
		t.Fatalf("Open decompressed %d bytes", b.total)
	}
}

func TestRelsPartCap(t *testing.T) {
	if MaxRelsBytes != 64<<10 {
		t.Fatalf("MaxRelsBytes = %d", MaxRelsBytes)
	}
	sheetRels := "xl/worksheets/_rels/sheet1.xml.rels"
	for _, name := range []string{xlsxtest.WorkbookRelsName, sheetRels} {
		t.Run(name, func(t *testing.T) {
			body := func(n int) []byte {
				if name == sheetRels {
					return paddedRels(n)
				}
				// Workbook rels: the real relationships plus padding.
				rels := strings.TrimSuffix(xlsxtest.WorkbookRels, "</Relationships>")
				pad := n - len(rels) - len("<!---->") - len("</Relationships>")
				return []byte(rels + "<!--" + letters(pad) + "--></Relationships>")
			}
			parts := xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: name, Data: body(MaxRelsBytes)})
			if _, err := open(t, parts); err != nil {
				t.Fatalf("at cap: %v", err)
			}
			parts = xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: name, Data: body(MaxRelsBytes + 1)})
			_, err := open(t, parts)
			wantErr(t, err, ErrPartTooLarge)
		})
	}
}

func TestRatioFloor(t *testing.T) {
	if ratioFloor != 64<<10 {
		t.Fatalf("ratioFloor = %d", ratioFloor)
	}
	parts := xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.SheetName, Stream: xlsxtest.Zeros(512 << 10)})
	b, err := open(t, parts)
	if err != nil {
		t.Fatal(err)
	}
	_, err = readPart(t, b, xlsxtest.SheetName, MaxSheetBytes)
	wantErr(t, err, ErrRatio)
}

func TestOpenContextCancelled(t *testing.T) {
	data := build(t, minimalParts())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := OpenContext(ctx, bytes.NewReader(data), int64(len(data)))
	wantErr(t, err, context.Canceled)
}

func TestPartReadHonoursContext(t *testing.T) {
	b, err := open(t, xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.SheetName, Data: xlsxtest.Noise(1<<20, 9)}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rc, err := b.part(ctx, xlsxtest.SheetName, MaxSheetBytes)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	cancel()
	buf := make([]byte, 4096)
	for {
		if _, err = rc.Read(buf); err != nil {
			break
		}
	}
	wantErr(t, err, context.Canceled)
}

// Finding 2: cancellation inside a run of large tokens.

func TestDecoderHonoursContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	d := newDecoder(ctx, strings.NewReader("<a><b/><c/></a>"))
	if _, err := d.Token(); err != nil {
		t.Fatal(err)
	}
	cancel()
	_, err := d.Token()
	wantErr(t, err, context.Canceled)
}

func TestRowsCancelInsideLongTokenRun(t *testing.T) {
	var big strings.Builder
	big.WriteString(`<row r="3">`)
	for i := range 40 {
		big.WriteString(letters(maxTokenBytes - 64))
		fmt.Fprintf(&big, `<x n="%d"/>`, i)
	}
	big.WriteString(`</row>`)
	sheet := xlsxtest.SheetXML(header, `<row r="2"><c r="B2"><v>1</v></c></row>`, big.String())
	b := bookOf(t, sheet, shared())
	ctx, cancel := context.WithCancel(context.Background())
	err := b.RowsContext(ctx, xlsxtest.DefaultSheet, req, func(Row) error {
		cancel()
		return nil
	})
	wantErr(t, err, context.Canceled)
	if b.total > int64(len(sheet))/4 {
		t.Fatalf("read %d of %d bytes after cancel", b.total, len(sheet))
	}
}

func TestRowsCancelOnLastRow(t *testing.T) {
	b := bookOf(t, xlsxtest.SheetXML(header, `<row r="2"><c r="B2"><v>1</v></c></row>`), shared())
	ctx, cancel := context.WithCancel(context.Background())
	err := b.RowsContext(ctx, xlsxtest.DefaultSheet, req, func(Row) error {
		cancel()
		return nil
	})
	wantErr(t, err, context.Canceled)
}

// Finding 3: content types are decoded, not byte-matched.

func TestContentTypesDecoded(t *testing.T) {
	for _, ct := range []string{
		`application/vnd.ms-excel.sheet.macro&#69;nabled.main+xml`,
		`application/vnd.ms-office.vba&#x50;roject`,
	} {
		data := strings.Replace(xlsxtest.ContentTypes, `ContentType="application/xml"`, `ContentType="`+ct+`"`, 1)
		_, err := open(t, xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.ContentTypesName, Data: []byte(data)}))
		wantErr(t, err, ErrMacro)
	}
	dt := strings.Replace(xlsxtest.ContentTypes, "?>", `?><!DOCTYPE Types [<!ENTITY m "macroEnabled">]>`, 1)
	_, err := open(t, xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.ContentTypesName, Data: []byte(dt)}))
	wantErr(t, err, ErrDirective)
}

// Finding 4: TargetMode is trimmed.

func TestTargetModeTrimmed(t *testing.T) {
	for _, mode := range []string{" External", "External\n", "\texternal "} {
		rel := `<Relationship Id="rId9" Type="x/hyperlink" Target="https://evil.example/" TargetMode="` + mode + `"/>`
		_, err := open(t, withWorkbookRels(relsWith(rel)))
		wantErr(t, err, ErrExternal)
	}
}

// Finding 5: relationship ids.

func TestRelationshipIDs(t *testing.T) {
	ws := "http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet"
	ss := "http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings"
	t.Run("empty id", func(t *testing.T) {
		_, err := open(t, withWorkbookRels(relsWith(`<Relationship Id="" Type="`+ws+`" Target="worksheets/sheet1.xml"/>`)))
		wantErr(t, err, ErrBadWorkbook)
	})
	t.Run("missing id", func(t *testing.T) {
		_, err := open(t, withWorkbookRels(relsWith(`<Relationship Type="`+ws+`" Target="worksheets/sheet1.xml"/>`)))
		wantErr(t, err, ErrBadWorkbook)
	})
	t.Run("duplicate id", func(t *testing.T) {
		_, err := open(t, withWorkbookRels(relsWith(`<Relationship Id="rId1" Type="`+ws+`" Target="worksheets/sheet1.xml"/>`)))
		wantErr(t, err, ErrBadWorkbook)
	})
	t.Run("two sharedStrings", func(t *testing.T) {
		_, err := open(t, withWorkbookRels(relsWith(`<Relationship Id="rId3" Type="`+ss+`" Target="sharedStrings.xml"/>`)))
		wantErr(t, err, ErrBadWorkbook)
	})
	t.Run("empty id matches sheet without r:id", func(t *testing.T) {
		rels := strings.Replace(xlsxtest.WorkbookRels, `Id="rId1"`, `Id=""`, 1)
		wb := strings.Replace(xlsxtest.Workbook, ` r:id="rId1"`, "", 1)
		parts := xlsxtest.Replace(withWorkbookRels(rels), xlsxtest.Part{Name: xlsxtest.WorkbookName, Data: []byte(wb)})
		_, err := open(t, parts)
		wantErr(t, err, ErrBadWorkbook)
	})
	t.Run("sheet without r:id", func(t *testing.T) {
		wb := strings.Replace(xlsxtest.Workbook, ` r:id="rId1"`, "", 1)
		_, err := open(t, xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.WorkbookName, Data: []byte(wb)}))
		wantErr(t, err, ErrBadWorkbook)
	})
}

// Finding 6: parseUint cannot wrap.

func TestParseUintOverflow(t *testing.T) {
	for _, c := range []struct {
		s   string
		max int
		ok  bool
	}{
		{"0", 10, true},
		{"10", 10, true},
		{"11", 10, false},
		{"18446744073709551617", 10, false}, // 2^64+1 wraps to 1 in uint64
		{"4294967297", 10, false},           // 2^32+1 wraps to 1 in uint32
		{"36893488147419103233", 1 << 20, false},
		{"1", -1, false},
		{"18446744073709551620", math.MaxInt, false}, // prefix fits, next digit wraps to 4
		{strconv.Itoa(math.MaxInt), math.MaxInt, true},
	} {
		n, ok := parseUint(c.s, c.max)
		if ok != c.ok || n < 0 || (ok && n > c.max) {
			t.Errorf("parseUint(%q, %d) = %d, %v", c.s, c.max, n, ok)
		}
	}
}

// Finding 7: ASCII-only entry names.

func TestOpenRejectsNonASCIIName(t *testing.T) {
	kelvin := "xl/wor\u212Abook.xml" // Kelvin sign folds to k
	var parts []xlsxtest.Part
	for _, p := range minimalParts() {
		if p.Name == xlsxtest.WorkbookName {
			p.Name = kelvin
		}
		parts = append(parts, p)
	}
	_, err := open(t, parts)
	wantErr(t, err, ErrBadName)

	_, err = open(t, append(minimalParts(), xlsxtest.Part{Name: "customXml/\u00e9.xml", Data: []byte("<a/>")}))
	wantErr(t, err, ErrBadName)
}

// Finding 8: guards that had no failing test.

func TestPartRejectsCRCMismatch(t *testing.T) {
	b, err := open(t, xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.SheetName, Data: []byte("<worksheet/>"), BadCRC: true}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = readPart(t, b, xlsxtest.SheetName, MaxSheetBytes)
	wantErr(t, err, ErrCorrupt)
}

func TestLoadSharedHonoursContext(t *testing.T) {
	b := bookOf(t, xlsxtest.SheetXML(header), shared())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wantErr(t, b.loadShared(ctx), context.Canceled)
	if b.strsLoaded {
		t.Fatal("marked loaded after cancel")
	}
}

func TestSharedStringsNestedSI(t *testing.T) {
	_, err := rowsOf(t, xlsxtest.SheetXML(header), shared(`<si><si><t>a</t></si></si>`))
	wantErr(t, err, ErrBadSheet)
}

// printerSettings allowlist.

const printerRel = `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/printerSettings" Target="../printerSettings/printerSettings1.bin"/>`

func printerPart(name string) xlsxtest.Part {
	return xlsxtest.Part{
		Name:         name,
		Data:         append([]byte("PK\x03\x04"), xlsxtest.Noise(64, 11)...),
		Raw:          true,
		Method:       99,
		DeclaredSize: 4 << 30,
	}
}

func TestPrinterSettingsAccepted(t *testing.T) {
	sheetRels := xlsxtest.Part{Name: "xl/worksheets/_rels/sheet1.xml.rels", Data: []byte(relsHead + printerRel + "</Relationships>")}
	run := func(parts []xlsxtest.Part) int64 {
		t.Helper()
		b, err := open(t, parts)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := collect(b, req); err != nil {
			t.Fatal(err)
		}
		return b.total
	}
	with := run(append(minimalParts(), sheetRels, printerPart("xl/printerSettings/printerSettings1.bin")))
	without := run(append(minimalParts(), sheetRels))
	if with != without {
		t.Fatalf("total with printerSettings %d, without %d", with, without)
	}
	run(append(minimalParts(), printerPart("xl/printerSettings/printerSettings9999.bin")))
}

func TestPrinterSettingsNeverOpened(t *testing.T) {
	name := "xl/printerSettings/printerSettings1.bin"
	b, err := open(t, append(minimalParts(), printerPart(name)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = readPart(t, b, name, MaxSmallPartBytes)
	wantErr(t, err, ErrMissingPart)
}

func TestPrinterSettingsNamesExact(t *testing.T) {
	for _, name := range []string{
		"xl/printerSettings/printerSettings.bin",
		"xl/printerSettings/printerSettings01.bin",
		"xl/printerSettings/printerSettings1.bin.bin",
		"xl/printersettings/printerSettings1.bin",
		"xl/printerSettings/x/printerSettings1.bin",
		"xl/worksheets/printerSettings1.bin",
		"xl/printerSettings/evil.bin",
		"xl/printerSettings/printerSettings10000.bin",
		"xl/xl/printerSettings/printerSettings1.bin",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := open(t, append(minimalParts(), printerPart(name)))
			wantErr(t, err, ErrBinaryPart)
		})
	}
}

func TestPrinterSettingsNotATarget(t *testing.T) {
	ps := printerPart("xl/printerSettings/printerSettings1.bin")
	ws := strings.Replace(xlsxtest.WorkbookRels, "worksheets/sheet1.xml", "printerSettings/printerSettings1.bin", 1)
	_, err := open(t, append(withWorkbookRels(ws), ps))
	wantErr(t, err, ErrBadName)

	ss := strings.Replace(xlsxtest.WorkbookRels, `Target="sharedStrings.xml"`, `Target="printerSettings/printerSettings1.bin"`, 1)
	_, err = open(t, append(withWorkbookRels(ss), ps))
	wantErr(t, err, ErrBadName)

	// Any non-.xml target is refused, not only .bin.
	txt := strings.Replace(xlsxtest.WorkbookRels, "worksheets/sheet1.xml", "media/data.txt", 1)
	_, err = open(t, append(withWorkbookRels(txt), xlsxtest.Part{Name: "xl/media/data.txt", Data: []byte(xlsxtest.SheetXML())}))
	wantErr(t, err, ErrBadName)
}

func TestPrinterSettingsDuplicateAndCount(t *testing.T) {
	ps := printerPart("xl/printerSettings/printerSettings1.bin")
	_, err := open(t, append(minimalParts(), ps, ps))
	wantErr(t, err, ErrDuplicateName)

	parts := minimalParts()
	for i := 1; len(parts) < MaxEntries+1; i++ {
		parts = append(parts, printerPart(fmt.Sprintf("xl/printerSettings/printerSettings%d.bin", i)))
	}
	_, err = open(t, parts)
	wantErr(t, err, ErrTooManyEntries)
}
