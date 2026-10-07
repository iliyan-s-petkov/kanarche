// Package xlsx is a minimal, hardened reader for untrusted .xlsx workbooks.
// It reads only the parts needed to stream one worksheet. Every cap is
// enforced on actual decompressed bytes, never on zip header claims.
package xlsx

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/xml"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"path"
	"strings"
	"unicode/utf8"
)

// Container caps. Safety limits, not tuning knobs.
const (
	MaxEntries            = 256
	MaxSheetBytes         = 512 << 20
	MaxSharedStringsBytes = 32 << 20
	MaxSmallPartBytes     = 1 << 20
	MaxTotalBytes         = 640 << 20
	MaxRatio              = 100
	MaxDepth              = 32

	// The ratio check starts after this much output so tiny parts pass.
	ratioFloor = 1 << 20
	// Largest single XML token (text run, tag with attributes).
	maxTokenBytes = 1 << 20
)

var (
	ErrNotZip         = errors.New("xlsx: not a zip archive")
	ErrOLE            = errors.New("xlsx: OLE/CFB file (encrypted or legacy .xls)")
	ErrTooManyEntries = errors.New("xlsx: too many zip entries")
	ErrBadName        = errors.New("xlsx: unsafe entry name")
	ErrDuplicateName  = errors.New("xlsx: duplicate entry name")
	ErrMissingPart    = errors.New("xlsx: missing part")
	ErrMacro          = errors.New("xlsx: macro-enabled content type")
	ErrBinaryPart     = errors.New("xlsx: binary or embedded part")
	ErrExternal       = errors.New("xlsx: external link or relationship")
	ErrNestedZip      = errors.New("xlsx: nested zip in part")
	ErrMethod         = errors.New("xlsx: unsupported compression method")
	ErrPartTooLarge   = errors.New("xlsx: part over size cap")
	ErrTotalTooLarge  = errors.New("xlsx: total decompressed size over cap")
	ErrRatio          = errors.New("xlsx: compression ratio over cap")
	ErrCorrupt        = errors.New("xlsx: part size or checksum mismatch")
	ErrDirective      = errors.New("xlsx: XML directive (DOCTYPE/ENTITY)")
	ErrDepth          = errors.New("xlsx: XML nesting too deep")
	ErrTokenTooLarge  = errors.New("xlsx: XML token too large")
	ErrBadWorkbook    = errors.New("xlsx: malformed workbook")
)

const (
	contentTypesPart = "[Content_Types].xml"
	workbookPart     = "xl/workbook.xml"
	workbookRelsPart = "xl/_rels/workbook.xml.rels"

	relTypeWorksheet     = "/worksheet"
	relTypeSharedStrings = "/sharedStrings"
)

// Relationship id namespaces, transitional and strict OOXML.
var relNS = map[string]bool{
	"http://schemas.openxmlformats.org/officeDocument/2006/relationships": true,
	"http://purl.oclc.org/ooxml/officeDocument/relationships":             true,
}

var (
	zipMagic = []byte("PK\x03\x04")
	oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
)

// Book is an opened workbook. It is not safe for concurrent use.
type Book struct {
	files    map[string]*zip.File // keyed by lower-case name
	total    int64                // decompressed bytes read across parts
	maxTotal int64
	sheets   []sheetRef
	shared   string // shared-strings part, "" when absent

	lim        limits
	strs       []string // loaded on first Rows call
	strsLoaded bool
}

type sheetRef struct{ name, part string }

// Open validates the container and reads the workbook index.
func Open(r io.ReaderAt, size int64) (*Book, error) {
	head := make([]byte, 8)
	n, _ := r.ReadAt(head, 0)
	head = head[:n]
	if bytes.HasPrefix(head, oleMagic) {
		return nil, ErrOLE
	}
	if !bytes.HasPrefix(head, zipMagic) {
		return nil, ErrNotZip
	}
	zr, err := zip.NewReader(r, size)
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, fmt.Errorf("%w: %v", ErrNotZip, err)
	}
	if zr == nil {
		return nil, ErrNotZip
	}
	if len(zr.File) > MaxEntries {
		return nil, fmt.Errorf("%w: %d > %d", ErrTooManyEntries, len(zr.File), MaxEntries)
	}
	b := &Book{files: make(map[string]*zip.File, len(zr.File)), maxTotal: MaxTotalBytes, lim: defaultLimits}
	for _, f := range zr.File {
		if err := checkName(f.Name); err != nil {
			return nil, err
		}
		key := strings.ToLower(f.Name)
		if b.files[key] != nil {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateName, f.Name)
		}
		b.files[key] = f
	}
	for _, name := range []string{contentTypesPart, workbookPart, workbookRelsPart} {
		if b.files[strings.ToLower(name)] == nil {
			return nil, fmt.Errorf("%w: %s", ErrMissingPart, name)
		}
	}
	if err := b.checkContentTypes(); err != nil {
		return nil, err
	}
	rels, err := b.readRels(workbookRelsPart)
	if err != nil {
		return nil, err
	}
	for key, f := range b.files {
		if strings.HasPrefix(key, "xl/worksheets/_rels/") && strings.HasSuffix(key, ".rels") {
			if _, err := b.readRels(f.Name); err != nil {
				return nil, err
			}
		}
	}
	if err := b.readWorkbook(rels); err != nil {
		return nil, err
	}
	return b, nil
}

// Sheets lists the worksheet names in workbook order.
func (b *Book) Sheets() []string {
	out := make([]string, len(b.sheets))
	for i, s := range b.sheets {
		out[i] = s.name
	}
	return out
}

func checkName(name string) error {
	bad := name == "" || !utf8.ValidString(name) ||
		strings.ContainsAny(name, "\\\x00") || strings.HasPrefix(name, "/")
	if !bad {
		// A trailing slash marks a directory entry.
		for _, seg := range strings.Split(strings.TrimSuffix(name, "/"), "/") {
			if seg == "" || seg == "." || seg == ".." {
				bad = true
				break
			}
		}
	}
	if bad {
		return fmt.Errorf("%w: %q", ErrBadName, name)
	}
	key := strings.ToLower(name)
	switch {
	case strings.HasSuffix(key, ".bin"),
		strings.HasPrefix(key, "xl/embeddings/"),
		strings.HasPrefix(key, "xl/activex/"):
		return fmt.Errorf("%w: %s", ErrBinaryPart, name)
	case strings.HasPrefix(key, "xl/externallinks/"):
		return fmt.Errorf("%w: %s", ErrExternal, name)
	}
	return nil
}

func (b *Book) readSmall(name string) ([]byte, error) {
	rc, err := b.part(name, MaxSmallPartBytes)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (b *Book) checkContentTypes() error {
	data, err := b.readSmall(contentTypesPart)
	if err != nil {
		return err
	}
	lower := bytes.ToLower(data)
	if bytes.Contains(lower, []byte("macroenabled")) || bytes.Contains(lower, []byte("vbaproject")) {
		return ErrMacro
	}
	return nil
}

type rel struct{ typ, target string }

// readRels parses a relationships part and rejects external targets.
func (b *Book) readRels(name string) (map[string]rel, error) {
	data, err := b.readSmall(name)
	if err != nil {
		return nil, err
	}
	out := map[string]rel{}
	d := newDecoder(bytes.NewReader(data))
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Relationship" {
			continue
		}
		var id string
		var r rel
		for _, a := range se.Attr {
			switch a.Name.Local {
			case "Id":
				id = a.Value
			case "Type":
				r.typ = a.Value
			case "Target":
				r.target = a.Value
			case "TargetMode":
				if strings.EqualFold(a.Value, "External") {
					return nil, fmt.Errorf("%w: %s in %s", ErrExternal, r.target, name)
				}
			}
		}
		out[id] = r
	}
}

// readWorkbook maps sheet names to worksheet parts via the workbook rels.
func (b *Book) readWorkbook(rels map[string]rel) error {
	for _, r := range rels {
		if strings.HasSuffix(r.typ, relTypeSharedStrings) {
			p, err := b.resolve(r.target)
			if err != nil {
				return err
			}
			b.shared = p
		}
	}
	data, err := b.readSmall(workbookPart)
	if err != nil {
		return err
	}
	d := newDecoder(bytes.NewReader(data))
	seen := map[string]bool{}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", workbookPart, err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "sheet" {
			continue
		}
		var name, rid string
		for _, a := range se.Attr {
			switch {
			case a.Name.Local == "name" && a.Name.Space == "":
				name = a.Value
			case a.Name.Local == "id" && relNS[a.Name.Space]:
				rid = a.Value
			}
		}
		r, ok := rels[rid]
		if name == "" || !ok || seen[name] {
			return fmt.Errorf("%w: sheet %q rel %q", ErrBadWorkbook, name, rid)
		}
		seen[name] = true
		if !strings.HasSuffix(r.typ, relTypeWorksheet) {
			// Chartsheets and dialog sheets are listed but never read.
			continue
		}
		p, err := b.resolve(r.target)
		if err != nil {
			return err
		}
		b.sheets = append(b.sheets, sheetRef{name: name, part: p})
	}
}

// resolve turns a workbook-relative target into an existing part name.
func (b *Book) resolve(target string) (string, error) {
	p := strings.TrimPrefix(target, "/")
	if p == target {
		p = path.Join("xl", target)
	}
	if err := checkName(p); err != nil || p != path.Clean(p) {
		return "", fmt.Errorf("%w: target %q", ErrBadName, target)
	}
	if b.files[strings.ToLower(p)] == nil {
		return "", fmt.Errorf("%w: %s", ErrMissingPart, p)
	}
	return p, nil
}

// part opens one entry with a decompressed cap of limit bytes. The reader
// also enforces the book-wide total, the ratio cap, the nested-zip check
// and the CRC.
func (b *Book) part(name string, limit int64) (io.ReadCloser, error) {
	f := b.files[strings.ToLower(name)]
	if f == nil {
		return nil, fmt.Errorf("%w: %s", ErrMissingPart, name)
	}
	raw, err := f.OpenRaw()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	in := &countReader{r: raw}
	pr := &partReader{name: name, f: f, book: b, in: in, limit: limit, crc: crc32.NewIEEE()}
	switch f.Method {
	case zip.Store:
		pr.dec = in
	case zip.Deflate:
		fr := flate.NewReader(in)
		pr.dec, pr.closer = fr, fr
	default:
		return nil, fmt.Errorf("%w: %d in %s", ErrMethod, f.Method, name)
	}
	head := make([]byte, len(zipMagic))
	n, err := io.ReadFull(pr, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		pr.Close()
		return nil, err
	}
	if bytes.Equal(head[:n], zipMagic) {
		pr.Close()
		return nil, fmt.Errorf("%w: %s", ErrNestedZip, name)
	}
	return &prefixed{r: io.MultiReader(bytes.NewReader(head[:n]), pr), c: pr}, nil
}

type countReader struct {
	r io.Reader
	n int64
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

type partReader struct {
	name   string
	f      *zip.File
	book   *Book
	in     *countReader // compressed bytes consumed
	dec    io.Reader
	closer io.Closer
	limit  int64
	out    int64
	crc    hash.Hash32
	err    error
}

func (r *partReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	// Ask for at most one byte past either cap so overshoot is detected
	// without decompressing a whole buffer beyond it.
	room := min(r.limit-r.out, r.book.maxTotal-r.book.total) + 1
	if int64(len(p)) > room {
		p = p[:room]
	}
	n, err := r.dec.Read(p)
	r.out += int64(n)
	r.book.total += int64(n)
	switch {
	case r.out > r.limit:
		r.err = fmt.Errorf("%w: %s over %d bytes", ErrPartTooLarge, r.name, r.limit)
	case r.book.total > r.book.maxTotal:
		r.err = fmt.Errorf("%w: %d bytes", ErrTotalTooLarge, r.book.maxTotal)
	case r.out > ratioFloor && r.out > MaxRatio*r.in.n:
		r.err = fmt.Errorf("%w: %s %d from %d", ErrRatio, r.name, r.out, r.in.n)
	}
	if r.err != nil {
		return 0, r.err
	}
	r.crc.Write(p[:n])
	if err == io.EOF {
		if uint64(r.out) != r.f.UncompressedSize64 || r.crc.Sum32() != r.f.CRC32 {
			r.err = fmt.Errorf("%w: %s", ErrCorrupt, r.name)
			return 0, r.err
		}
	}
	if err != nil {
		r.err = err
		if err == io.ErrUnexpectedEOF {
			r.err = fmt.Errorf("%w: %s truncated", ErrCorrupt, r.name)
		}
	}
	return n, r.err
}

func (r *partReader) Close() error {
	if r.closer != nil {
		return r.closer.Close()
	}
	return nil
}

type prefixed struct {
	r io.Reader
	c io.Closer
}

func (p *prefixed) Read(b []byte) (int, error) { return p.r.Read(b) }
func (p *prefixed) Close() error               { return p.c.Close() }

// decoder is a strict xml.Decoder that refuses directives, bounds element
// depth and bounds the bytes behind any single token.
type decoder struct {
	d     *xml.Decoder
	in    *tokenGuard
	depth int
}

func newDecoder(r io.Reader) *decoder {
	g := &tokenGuard{br: bufio.NewReaderSize(r, 64<<10), max: maxTokenBytes}
	d := xml.NewDecoder(g) // uses g.ReadByte directly, no extra buffer
	d.Strict = true
	return &decoder{d: d, in: g}
}

func (x *decoder) Token() (xml.Token, error) {
	tok, err := x.d.Token()
	if err != nil {
		return nil, err
	}
	x.in.mark = x.in.n
	switch tok.(type) {
	case xml.Directive:
		return nil, ErrDirective
	case xml.StartElement:
		x.depth++
		if x.depth > MaxDepth {
			return nil, fmt.Errorf("%w: > %d", ErrDepth, MaxDepth)
		}
	case xml.EndElement:
		x.depth--
	}
	return tok, nil
}

// tokenGuard fails once more than max bytes are read since the last token.
type tokenGuard struct {
	br      *bufio.Reader
	n, mark int64
	max     int64
}

func (g *tokenGuard) ReadByte() (byte, error) {
	if g.n-g.mark >= g.max {
		return 0, ErrTokenTooLarge
	}
	c, err := g.br.ReadByte()
	if err == nil {
		g.n++
	}
	return c, err
}

func (g *tokenGuard) Read(p []byte) (int, error) {
	room := g.max - (g.n - g.mark)
	if room <= 0 {
		return 0, ErrTokenTooLarge
	}
	if int64(len(p)) > room {
		p = p[:room]
	}
	n, err := g.br.Read(p)
	g.n += int64(n)
	return n, err
}
