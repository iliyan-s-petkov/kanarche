// Package xlsxtest builds xlsx fixtures in test code, including malformed and
// hostile ones, so that no opaque binary fixture is needed for the reader.
package xlsxtest

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/xml"
	"fmt"
	"hash/crc32"
	"io"
	"strconv"
	"strings"
)

// Part is one zip entry. Exactly one of Data or Stream supplies the content.
type Part struct {
	Name   string
	Data   []byte
	Stream func(w io.Writer) error
	// Store writes the entry uncompressed instead of deflated.
	Store bool
	// DeclaredSize, when non-zero, replaces the uncompressed size in the
	// headers. The CRC stays correct, so only the size lies.
	DeclaredSize uint64
	// BadCRC writes a CRC that does not match the content.
	BadCRC bool
	// Raw writes Data as-is under Method, with no compression.
	Raw    bool
	Method uint16
}

// Options lists the entries in write order. Duplicate names are kept.
type Options struct {
	Parts []Part
}

// Build returns the zip archive as bytes.
func Build(opts Options) ([]byte, error) {
	var buf bytes.Buffer
	if err := BuildTo(&buf, opts); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BuildTo streams the zip archive to w.
func BuildTo(w io.Writer, opts Options) error {
	zw := zip.NewWriter(w)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestSpeed)
	})
	for _, p := range opts.Parts {
		if err := writePart(zw, p); err != nil {
			return fmt.Errorf("part %q: %w", p.Name, err)
		}
	}
	return zw.Close()
}

func writePart(zw *zip.Writer, p Part) error {
	if p.Raw {
		return writeRaw(zw, p)
	}
	if p.DeclaredSize != 0 || p.BadCRC {
		return writeLying(zw, p)
	}
	method := zip.Deflate
	if p.Store {
		method = zip.Store
	}
	w, err := zw.CreateHeader(&zip.FileHeader{Name: p.Name, Method: method})
	if err != nil {
		return err
	}
	if p.Stream != nil {
		return p.Stream(w)
	}
	_, err = w.Write(p.Data)
	return err
}

// writeLying deflates the content itself and writes it raw under a header
// whose uncompressed size is p.DeclaredSize.
func writeLying(zw *zip.Writer, p Part) error {
	var comp bytes.Buffer
	crc := crc32.NewIEEE()
	fw, err := flate.NewWriter(&comp, flate.BestSpeed)
	if err != nil {
		return err
	}
	dst := io.MultiWriter(fw, crc)
	if p.Stream != nil {
		err = p.Stream(dst)
	} else {
		_, err = dst.Write(p.Data)
	}
	if err != nil {
		return err
	}
	if err := fw.Close(); err != nil {
		return err
	}
	size := p.DeclaredSize
	if size == 0 {
		size = uint64(len(p.Data))
	}
	sum := crc.Sum32()
	if p.BadCRC {
		sum ^= 1
	}
	return createRaw(zw, p.Name, zip.Deflate, sum, size, comp.Bytes())
}

// writeRaw stores Data unmodified under p.Method.
func writeRaw(zw *zip.Writer, p Part) error {
	size := p.DeclaredSize
	if size == 0 {
		size = uint64(len(p.Data))
	}
	return createRaw(zw, p.Name, p.Method, crc32.ChecksumIEEE(p.Data), size, p.Data)
}

func createRaw(zw *zip.Writer, name string, method uint16, sum uint32, size uint64, data []byte) error {
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name:               name,
		Method:             method,
		CRC32:              sum,
		CompressedSize64:   uint64(len(data)),
		UncompressedSize64: size,
	})
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// Zeros returns a Stream that writes n zero bytes without holding them.
func Zeros(n int64) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := io.CopyN(w, zeroReader{}, n)
		return err
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// Noise returns n bytes that deflate barely compresses. The first byte is
// never 'P', so the content is never mistaken for a nested zip.
func Noise(n int, seed uint64) []byte {
	out := make([]byte, n)
	x := seed | 1
	for i := range out {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		out[i] = byte(x)
	}
	if n > 0 && out[0] == 'P' {
		out[0] = 'Q'
	}
	return out
}

const (
	ContentTypesName = "[Content_Types].xml"
	WorkbookName     = "xl/workbook.xml"
	WorkbookRelsName = "xl/_rels/workbook.xml.rels"
	SheetName        = "xl/worksheets/sheet1.xml"
	SharedName       = "xl/sharedStrings.xml"
	// DefaultSheet is the sheet name Minimal registers in workbook.xml.
	DefaultSheet = "Sheet1"
)

const ContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/></Types>`

const Workbook = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`

const WorkbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/></Relationships>`

// Minimal returns the parts of a valid one-sheet workbook with the given
// worksheet and shared-strings XML.
func Minimal(sheetXML, sharedXML string) []Part {
	return []Part{
		{Name: ContentTypesName, Data: []byte(ContentTypes)},
		{Name: WorkbookName, Data: []byte(Workbook)},
		{Name: WorkbookRelsName, Data: []byte(WorkbookRels)},
		{Name: SheetName, Data: []byte(sheetXML)},
		{Name: SharedName, Data: []byte(sharedXML)},
	}
}

// Replace returns parts with every entry named p.Name swapped for p, or p
// appended when no entry has that name.
func Replace(parts []Part, p Part) []Part {
	out := make([]Part, 0, len(parts)+1)
	found := false
	for _, q := range parts {
		if q.Name == p.Name {
			out = append(out, p)
			found = true
			continue
		}
		out = append(out, q)
	}
	if !found {
		out = append(out, p)
	}
	return out
}

// SheetXML wraps row elements in a worksheet document.
func SheetXML(rows ...string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
		strings.Join(rows, "") + `</sheetData></worksheet>`
}

// SharedXML wraps si elements in a shared-strings document.
func SharedXML(items ...string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		strings.Join(items, "") + `</sst>`
}

// Table encodes rows as a worksheet plus shared strings. Values that parse as
// numbers become numeric cells; the rest become shared strings.
func Table(rows [][]string) (sheetXML, sharedXML string) {
	index := map[string]int{}
	var shared []string
	var sheet strings.Builder
	for ri, row := range rows {
		fmt.Fprintf(&sheet, `<row r="%d">`, ri+1)
		for ci, v := range row {
			ref := ColName(ci) + strconv.Itoa(ri+1)
			if _, err := strconv.ParseFloat(v, 64); err == nil {
				fmt.Fprintf(&sheet, `<c r="%s"><v>%s</v></c>`, ref, v)
				continue
			}
			i, ok := index[v]
			if !ok {
				i = len(shared)
				index[v] = i
				shared = append(shared, `<si><t>`+escape(v)+`</t></si>`)
			}
			fmt.Fprintf(&sheet, `<c r="%s" t="s"><v>%d</v></c>`, ref, i)
		}
		sheet.WriteString(`</row>`)
	}
	return SheetXML(sheet.String()), SharedXML(shared...)
}

// TableBook returns a complete valid workbook holding rows.
func TableBook(rows [][]string) ([]byte, error) {
	sheet, shared := Table(rows)
	return Build(Options{Parts: Minimal(sheet, shared)})
}

// ColName converts a zero-based column index to letters (0 is A).
func ColName(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
