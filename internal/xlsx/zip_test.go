package xlsx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"kanarche.eu/internal/xlsx/xlsxtest"
)

func minimalParts() []xlsxtest.Part {
	sheet, shared := xlsxtest.Table([][]string{{"countryCode", "season"}, {"BG", "2025"}})
	return xlsxtest.Minimal(sheet, shared)
}

func build(t *testing.T, parts []xlsxtest.Part) []byte {
	t.Helper()
	b, err := xlsxtest.Build(xlsxtest.Options{Parts: parts})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func open(t *testing.T, parts []xlsxtest.Part) (*Book, error) {
	t.Helper()
	b := build(t, parts)
	return Open(bytes.NewReader(b), int64(len(b)))
}

func wantErr(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("err = %v, want %v", err, target)
	}
}

func TestOpenAcceptsMinimalWorkbook(t *testing.T) {
	b, err := open(t, minimalParts())
	if err != nil {
		t.Fatal(err)
	}
	if got := b.Sheets(); len(got) != 1 || got[0] != xlsxtest.DefaultSheet {
		t.Fatalf("Sheets() = %v", got)
	}
}

func TestOpenRejectsTooManyEntries(t *testing.T) {
	parts := minimalParts()
	for i := len(parts); i < MaxEntries+1; i++ {
		parts = append(parts, xlsxtest.Part{Name: fmt.Sprintf("customXml/item%d.xml", i), Data: []byte("<a/>")})
	}
	_, err := open(t, parts)
	wantErr(t, err, ErrTooManyEntries)

	// The cap itself is allowed.
	_, err = open(t, parts[:MaxEntries])
	if err != nil {
		t.Fatalf("at cap: %v", err)
	}
}

func TestOpenRejectsTraversalName(t *testing.T) {
	for _, name := range []string{
		"../evil.xml",
		"xl/../../evil.xml",
		"/etc/passwd",
		`xl\worksheets\sheet2.xml`,
		"xl/a\x00b.xml",
		"xl/\xff.xml",
		"xl//a.xml",
		"xl/./a.xml",
		"",
	} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			parts := append(minimalParts(), xlsxtest.Part{Name: name, Data: []byte("x")})
			_, err := open(t, parts)
			wantErr(t, err, ErrBadName)
		})
	}
}

func TestOpenRejectsDuplicateName(t *testing.T) {
	for _, name := range []string{xlsxtest.SheetName, "XL/Worksheets/Sheet1.xml"} {
		parts := append(minimalParts(), xlsxtest.Part{Name: name, Data: []byte("<worksheet/>")})
		_, err := open(t, parts)
		wantErr(t, err, ErrDuplicateName)
	}
}

func TestOpenRejectsMissingRequiredPart(t *testing.T) {
	for _, name := range []string{xlsxtest.ContentTypesName, xlsxtest.WorkbookName, xlsxtest.WorkbookRelsName} {
		var parts []xlsxtest.Part
		for _, p := range minimalParts() {
			if p.Name != name {
				parts = append(parts, p)
			}
		}
		_, err := open(t, parts)
		wantErr(t, err, ErrMissingPart)
	}
}

func TestOpenRejectsMacroContentType(t *testing.T) {
	for _, ct := range []string{
		`application/vnd.ms-excel.sheet.macroEnabled.main+xml`,
		`application/vnd.ms-office.vbaProject`,
	} {
		data := bytes.Replace([]byte(xlsxtest.ContentTypes), []byte("application/xml"), []byte(ct), 1)
		parts := xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.ContentTypesName, Data: data})
		_, err := open(t, parts)
		wantErr(t, err, ErrMacro)
	}
}

func TestOpenRejectsVBAProject(t *testing.T) {
	for _, name := range []string{
		"xl/vbaProject.bin",
		"xl/VBAProject.BIN",
		"xl/embeddings/oleObject1.xml",
		"xl/activeX/activeX1.xml",
		"xl/printerSettings/printerSettings01.bin",
	} {
		parts := append(minimalParts(), xlsxtest.Part{Name: name, Data: []byte("x")})
		_, err := open(t, parts)
		wantErr(t, err, ErrBinaryPart)
	}
}

func TestOpenRejectsXLSB(t *testing.T) {
	parts := append(minimalParts(), xlsxtest.Part{Name: "xl/workbook.bin", Data: []byte("x")})
	_, err := open(t, parts)
	wantErr(t, err, ErrBinaryPart)
}

func TestOpenRejectsOLEHeader(t *testing.T) {
	body := append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 504)...)
	_, err := Open(bytes.NewReader(body), int64(len(body)))
	wantErr(t, err, ErrOLE)

	html := []byte("<html><body>403 Forbidden</body></html>")
	_, err = Open(bytes.NewReader(html), int64(len(html)))
	wantErr(t, err, ErrNotZip)
}

func TestOpenRejectsExternalRelationship(t *testing.T) {
	ext := `<Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://evil.example/" TargetMode="External"/></Relationships>`
	rels := bytes.Replace([]byte(xlsxtest.WorkbookRels), []byte("</Relationships>"), []byte(ext), 1)

	t.Run("workbook", func(t *testing.T) {
		parts := xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.WorkbookRelsName, Data: rels})
		_, err := open(t, parts)
		wantErr(t, err, ErrExternal)
	})
	t.Run("worksheet", func(t *testing.T) {
		sheetRels := `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + ext
		parts := append(minimalParts(), xlsxtest.Part{Name: "xl/worksheets/_rels/sheet1.xml.rels", Data: []byte(sheetRels)})
		_, err := open(t, parts)
		wantErr(t, err, ErrExternal)
	})
}

func TestOpenRejectsExternalLinks(t *testing.T) {
	parts := append(minimalParts(), xlsxtest.Part{Name: "xl/externalLinks/externalLink1.xml", Data: []byte("<externalLink/>")})
	_, err := open(t, parts)
	wantErr(t, err, ErrExternal)
}

func TestPartRejectsNestedZip(t *testing.T) {
	inner := build(t, minimalParts())
	parts := xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.SheetName, Data: inner, Store: true})
	b, err := open(t, parts)
	if err != nil {
		t.Fatal(err)
	}
	rc, err := b.part(context.Background(), xlsxtest.SheetName, MaxSheetBytes)
	if err == nil {
		_, err = io.Copy(io.Discard, rc)
		rc.Close()
	}
	wantErr(t, err, ErrNestedZip)

	// Opened parts during Open are checked too.
	parts = xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.WorkbookName, Data: inner, Store: true})
	_, err = open(t, parts)
	wantErr(t, err, ErrNestedZip)
}

func readPart(t *testing.T, b *Book, name string, limit int64) (int64, error) {
	t.Helper()
	rc, err := b.part(context.Background(), name, limit)
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	return io.Copy(io.Discard, rc)
}

func TestPartStopsZipBombAtRatio(t *testing.T) {
	if testing.Short() {
		t.Skip("streams a 600 MiB bomb")
	}
	const size = 600 << 20
	parts := xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.SheetName, Stream: xlsxtest.Zeros(size)})
	b, err := open(t, parts)
	if err != nil {
		t.Fatal(err)
	}
	compressed := int64(b.files[xlsxtest.SheetName].CompressedSize64)
	n, err := readPart(t, b, xlsxtest.SheetName, MaxSheetBytes)
	wantErr(t, err, ErrRatio)
	t.Logf("compressed %d, read %d before stop", compressed, n)
	if limit := MaxRatio*compressed + 64<<10; n > limit {
		t.Fatalf("read %d bytes before stopping, want <= %d (compressed %d)", n, limit, compressed)
	}
}

func TestPartRejectsOversizedPart(t *testing.T) {
	const limit = 1 << 20
	parts := xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.SheetName, Data: xlsxtest.Noise(limit+1, 1)})
	b, err := open(t, parts)
	if err != nil {
		t.Fatal(err)
	}
	n, err := readPart(t, b, xlsxtest.SheetName, limit)
	wantErr(t, err, ErrPartTooLarge)
	if n > limit {
		t.Fatalf("read %d bytes, cap %d", n, limit)
	}
	// Exactly at the cap is accepted.
	parts = xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.SheetName, Data: xlsxtest.Noise(limit, 1)})
	b, err = open(t, parts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readPart(t, b, xlsxtest.SheetName, limit); err != nil {
		t.Fatalf("at cap: %v", err)
	}
}

func TestPartStopsLyingHeaderAtCap(t *testing.T) {
	const limit = 1 << 20
	cases := []struct {
		name     string
		declared uint64
		data     []byte
	}{
		{"declares 1 KiB, holds 50 MiB", 1 << 10, nil},
		{"declares 4 GiB, holds 2 MiB", 4 << 30, xlsxtest.Noise(2<<20, 2)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := xlsxtest.Part{Name: xlsxtest.SheetName, DeclaredSize: c.declared, Data: c.data}
			if c.data == nil {
				p.Data = xlsxtest.Noise(50<<20, 3)
			}
			b, err := open(t, xlsxtest.Replace(minimalParts(), p))
			if err != nil {
				t.Fatal(err)
			}
			n, err := readPart(t, b, xlsxtest.SheetName, limit)
			if err == nil {
				t.Fatal("lying header accepted")
			}
			if n > limit {
				t.Fatalf("read %d bytes, cap %d", n, limit)
			}
		})
	}
	// Only our own cap can stop a header that declares more than it holds.
	b, err := open(t, xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.SheetName, DeclaredSize: 4 << 30, Data: xlsxtest.Noise(2<<20, 2)}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = readPart(t, b, xlsxtest.SheetName, limit)
	wantErr(t, err, ErrPartTooLarge)
}

func TestPartRejectsCorruptSize(t *testing.T) {
	// Declared size above the actual content and under every cap.
	p := xlsxtest.Part{Name: xlsxtest.SheetName, DeclaredSize: 1 << 20, Data: []byte("<worksheet/>")}
	b, err := open(t, xlsxtest.Replace(minimalParts(), p))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readPart(t, b, xlsxtest.SheetName, MaxSheetBytes); err == nil {
		t.Fatal("size mismatch accepted")
	}
}

func TestTotalDecompressedCap(t *testing.T) {
	const each = 2 << 20
	parts := xlsxtest.Replace(minimalParts(), xlsxtest.Part{Name: xlsxtest.SheetName, Data: xlsxtest.Noise(each, 4)})
	parts = xlsxtest.Replace(parts, xlsxtest.Part{Name: xlsxtest.SharedName, Data: xlsxtest.Noise(each, 5)})
	b, err := open(t, parts)
	if err != nil {
		t.Fatal(err)
	}
	b.maxTotal = b.total + each + each/2
	if _, err := readPart(t, b, xlsxtest.SheetName, MaxSheetBytes); err != nil {
		t.Fatalf("first part: %v", err)
	}
	_, err = readPart(t, b, xlsxtest.SharedName, MaxSharedStringsBytes)
	wantErr(t, err, ErrTotalTooLarge)
	if b.total > b.maxTotal+1 {
		t.Fatalf("total %d over cap %d", b.total, b.maxTotal)
	}
	if MaxTotalBytes != 640<<20 {
		t.Fatalf("MaxTotalBytes = %d", MaxTotalBytes)
	}
}

func TestPartRejectsUnknownMethod(t *testing.T) {
	b, err := open(t, minimalParts())
	if err != nil {
		t.Fatal(err)
	}
	b.files[xlsxtest.SheetName].Method = 14 // LZMA
	_, err = readPart(t, b, xlsxtest.SheetName, MaxSheetBytes)
	wantErr(t, err, ErrMethod)
}

func TestCapsMatchPlan(t *testing.T) {
	if MaxEntries != 256 || MaxSheetBytes != 512<<20 || MaxSharedStringsBytes != 32<<20 ||
		MaxSmallPartBytes != 1<<20 || MaxRatio != 100 {
		t.Fatal("caps drifted from the plan")
	}
}
