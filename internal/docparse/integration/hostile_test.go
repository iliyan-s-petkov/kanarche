package integration

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"crypto/rc4"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
)

// Refusal texts the image must produce for the inputs that test a specific
// gate (cmd/rzi-extract and internal/docparse/pdf).
const (
	refEncrypted = "pdf is encrypted"
	refJS        = "pdf contains javascript"
	refPages     = "pdf has too many pages"
	refMagic     = "magic mismatch"
	refSize      = "input over size cap"
	refNoBlocks  = "no point blocks found"
)

// fixture is one generated input. want is the exact refusal text, or ""
// when any refusal (or a killed run) is acceptable.
type fixture struct {
	name string
	ext  string
	data []byte
	want string
}

// hostileCorpus builds the Task 8 corpus in memory. Nothing is committed or
// downloaded; every byte is a function of this file.
func hostileCorpus() []fixture { return corpusOnce() }

// corpusOnce memoises the corpus; the two large inputs take seconds to deflate.
var corpusOnce = sync.OnceValue(buildCorpus)

func buildCorpus() []fixture {
	return []fixture{
		{"xref-loop.pdf", "pdf", xrefLoopPDF(), ""},
		{"deep-nesting.pdf", "pdf", deepNestingPDF(), ""},
		{"encrypted.pdf", "pdf", encryptedPDF(), refEncrypted},
		{"js-action.pdf", "pdf", jsActionPDF(), refJS},
		{"1000-pages.pdf", "pdf", manyPagesPDF(1000), refPages},
		{"huge-stream.pdf", "pdf", hugeStreamPDF(), ""},
		{"not-a-pdf.pdf", "pdf", []byte("<!DOCTYPE html><html><body>not a pdf</body></html>\n"), refMagic},
		{"zip-bomb.xlsx", "xlsx", zipBomb(), ""},
		{"xml-depth.xlsx", "xlsx", xmlDepthXLSX(), ""},
		{"fat-cycle.xls", "xls", fatCycleXLS(), ""},
		{"5mb-plus-one.pdf", "pdf", oversizePDF(), refSize},
	}
}

// controlPDF passes the pdfinfo gate and yields text that is not a Varna
// table, so a working image refuses it with refNoBlocks.
func controlPDF() fixture {
	return fixture{"control.pdf", "pdf", buildPDF(onePage("BT /F1 12 Tf 72 700 Td (control) Tj ET"), ""), refNoBlocks}
}

// onePage is catalog, pages, one page with Helvetica, and its content.
func onePage(content string) []string {
	return []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 /MediaBox [0 0 612 792] >>",
		"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		stream("", []byte(content)),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
}

func stream(dict string, body []byte) string {
	return fmt.Sprintf("<< /Length %d%s >>\nstream\n%s\nendstream", len(body), dict, body)
}

// buildPDF writes objs as objects 1..n with a correct xref table. trailer is
// extra trailer dictionary content; object 1 is the root.
func buildPDF(objs []string, trailer string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offs := writeObjs(&b, objs)
	xref := b.Len()
	writeXref(&b, offs)
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R%s >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, trailer, xref)
	return b.Bytes()
}

func writeObjs(b *bytes.Buffer, objs []string) []int {
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = b.Len()
		fmt.Fprintf(b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	return offs
}

func writeXref(b *bytes.Buffer, offs []int) {
	fmt.Fprintf(b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(b, "%010d 00000 n \n", o)
	}
}

// xrefLoopPDF has an xref section whose /Prev points back at itself.
func xrefLoopPDF() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offs := writeObjs(&b, onePage("BT /F1 12 Tf 72 700 Td (loop) Tj ET"))
	xref := b.Len()
	writeXref(&b, offs)
	fmt.Fprintf(&b, "trailer\n<< /Size 6 /Root 1 0 R /Prev %d >>\nstartxref\n%d\n%%%%EOF\n", xref, xref)
	return b.Bytes()
}

// deepNestingPDF nests arrays 50000 deep in the catalog and the page.
func deepNestingPDF() []byte {
	deep := strings.Repeat("[", 50000) + strings.Repeat("]", 50000)
	objs := onePage("BT /F1 12 Tf 72 700 Td (deep) Tj ET")
	objs[0] = "<< /Type /Catalog /Pages 2 0 R /Deep " + deep + " >>"
	objs[2] = "<< /Type /Page /Parent 2 0 R /Deep " + deep + " /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>"
	return buildPDF(objs, "")
}

// pdfPad is the standard security handler password padding.
var pdfPad = []byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

func rc4Bytes(key, in []byte) []byte {
	c, err := rc4.NewCipher(key)
	if err != nil {
		panic(err)
	}
	out := make([]byte, len(in))
	c.XORKeyStream(out, in)
	return out
}

// encryptedPDF uses the RC4 40-bit handler (R2) with an empty user
// password, so poppler opens it and pdfinfo reports "Encrypted: yes".
func encryptedPDF() []byte {
	id := []byte("kanarche-rzi-t8!")
	const p = int32(-44)
	ownerKey := md5.Sum(append([]byte("owner"), pdfPad[:32-len("owner")]...))
	o := rc4Bytes(ownerKey[:5], pdfPad)
	h := md5.New()
	h.Write(pdfPad)
	h.Write(o)
	binary.Write(h, binary.LittleEndian, p)
	h.Write(id)
	key := h.Sum(nil)[:5]
	u := rc4Bytes(key, pdfPad)
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 /MediaBox [0 0 612 792] >>",
		"<< /Type /Page /Parent 2 0 R >>",
		fmt.Sprintf("<< /Filter /Standard /V 1 /R 2 /Length 40 /O <%x> /U <%x> /P %d >>", o, u, p),
	}
	return buildPDF(objs, fmt.Sprintf(" /Encrypt 4 0 R /ID [<%x> <%x>]", id, id))
}

// jsActionPDF runs JavaScript on open and names it in the document tree.
func jsActionPDF() []byte {
	objs := onePage("BT /F1 12 Tf 72 700 Td (js) Tj ET")
	objs[0] = "<< /Type /Catalog /Pages 2 0 R /OpenAction 6 0 R /Names << /JavaScript << /Names [(a) 6 0 R] >> >> >>"
	objs = append(objs, "<< /Type /Action /S /JavaScript /JS (app.alert\\(1\\)) >>")
	return buildPDF(objs, "")
}

// manyPagesPDF has n real, empty pages.
func manyPagesPDF(n int) []byte {
	kids := make([]string, n)
	objs := []string{"<< /Type /Catalog /Pages 2 0 R >>", ""}
	for i := range n {
		kids[i] = fmt.Sprintf("%d 0 R", i+3)
		objs = append(objs, "<< /Type /Page /Parent 2 0 R >>")
	}
	objs[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d /MediaBox [0 0 612 792] >>", strings.Join(kids, " "), n)
	return buildPDF(objs, "")
}

// hugeStreamPDF has one page whose Flate content inflates to 128 MiB of
// text operators.
func hugeStreamPDF() []byte {
	line := []byte("BT /F1 9 Tf 20 700 Td (" + strings.Repeat("A", 100) + ") Tj ET\n")
	var z bytes.Buffer
	w, _ := zlib.NewWriterLevel(&z, zlib.BestCompression)
	for written := 0; written < 128<<20; written += len(line) {
		w.Write(line)
	}
	w.Close()
	objs := onePage("")
	objs[3] = stream(" /Filter /FlateDecode", z.Bytes())
	return buildPDF(objs, "")
}

// oversizePDF is one byte over the 5 MiB input cap.
func oversizePDF() []byte {
	b := bytes.Repeat([]byte{'\n'}, 5<<20+1)
	copy(b, "%PDF-1.4\n")
	return b
}

// zipBomb holds one sheet that inflates to 256 MiB.
func zipBomb() []byte {
	return buildZip(map[string]func(w zipWriter){
		"[Content_Types].xml": text(`<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`),
		"xl/worksheets/sheet1.xml": func(w zipWriter) {
			chunk := bytes.Repeat([]byte("<c/>"), 1<<14)
			for i := 0; i < (256<<20)/len(chunk); i++ {
				w.Write(chunk)
			}
		},
	})
}

// xmlDepthXLSX nests 100000 elements in xl/workbook.xml.
func xmlDepthXLSX() []byte {
	const n = 100000
	return buildZip(map[string]func(w zipWriter){
		"[Content_Types].xml": text(`<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`),
		"xl/workbook.xml":     text(strings.Repeat("<a>", n) + strings.Repeat("</a>", n)),
	})
}

type zipWriter interface{ Write([]byte) (int, error) }

func text(s string) func(w zipWriter) { return func(w zipWriter) { w.Write([]byte(s)) } }

// buildZip writes entries in a fixed order with a fixed timestamp.
func buildZip(entries map[string]func(w zipWriter)) []byte {
	names := []string{"[Content_Types].xml", "xl/workbook.xml", "xl/worksheets/sheet1.xml"}
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, n := range names {
		fill, ok := entries[n]
		if !ok {
			continue
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: n, Method: zip.Deflate})
		if err != nil {
			panic(err)
		}
		fill(w)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// CFB sector markers.
const (
	freeSect   = 0xFFFFFFFF
	endOfChain = 0xFFFFFFFE
	fatSect    = 0xFFFFFFFD
	noStream   = 0xFFFFFFFF
)

// fatCycleXLS is a CFB v3 file whose Workbook stream starts at sector 2 and
// the FAT links sector 2 to itself.
func fatCycleXLS() []byte {
	const sec = 512
	b := make([]byte, sec*4)
	put16 := func(off int, v uint16) { binary.LittleEndian.PutUint16(b[off:], v) }
	put32 := func(off int, v uint32) { binary.LittleEndian.PutUint32(b[off:], v) }

	copy(b, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	put16(0x18, 0x003E)
	put16(0x1A, 3)
	put16(0x1C, 0xFFFE)
	put16(0x1E, 9)
	put16(0x20, 6)
	put32(0x2C, 1)          // FAT sectors
	put32(0x30, 1)          // first directory sector
	put32(0x38, 4096)       // mini stream cutoff
	put32(0x3C, endOfChain) // first mini FAT sector
	put32(0x44, endOfChain) // first DIFAT sector
	for i := range 109 {
		put32(0x4C+4*i, freeSect)
	}
	put32(0x4C, 0) // DIFAT[0]: FAT is sector 0

	fat := sec // sector 0
	for i := range sec / 4 {
		put32(fat+4*i, freeSect)
	}
	put32(fat+0, fatSect)
	put32(fat+4, endOfChain)
	put32(fat+8, 2) // the cycle

	dir := 2 * sec // sector 1
	entry := func(i int, name string, typ byte, child, start, size uint32) {
		off := dir + 128*i
		u := []rune(name)
		for j, r := range u {
			put16(off+2*j, uint16(r))
		}
		put16(off+64, uint16(2*(len(u)+1)))
		b[off+66] = typ
		b[off+67] = 1
		put32(off+68, noStream)
		put32(off+72, noStream)
		put32(off+76, child)
		put32(off+116, start)
		put32(off+120, size)
	}
	entry(0, "Root Entry", 5, 1, endOfChain, 0)
	entry(1, "Workbook", 2, noStream, 2, 8192)
	for i := 2; i < 4; i++ {
		put32(dir+128*i+68, noStream)
		put32(dir+128*i+72, noStream)
		put32(dir+128*i+76, noStream)
	}
	return b
}
