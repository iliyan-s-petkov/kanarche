package xlsx

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Sheet and shared-string caps.
const (
	MaxSharedStrings      = 1_000_000
	MaxStringBytes        = 4096
	MaxSharedStringsTotal = 32 << 20
	MaxRows               = 2_000_000
	MaxCellsPerRow        = 64
	MaxColumn             = 16384 // XFD
	maxRowNumber          = 1 << 20
	maxValueBytes         = 256 // text of one <v>
)

var (
	ErrNoSheet       = errors.New("xlsx: no such worksheet")
	ErrSharedStrings = errors.New("xlsx: shared strings over cap")
	ErrSharedIndex   = errors.New("xlsx: shared string index invalid")
	ErrFormula       = errors.New("xlsx: formula present")
	ErrCellType      = errors.New("xlsx: unsupported cell type")
	ErrCellValue     = errors.New("xlsx: invalid cell value")
	ErrCellRef       = errors.New("xlsx: invalid cell or row reference")
	ErrRowCap        = errors.New("xlsx: too many rows")
	ErrCellsPerRow   = errors.New("xlsx: too many cells in a row")
	ErrHeader        = errors.New("xlsx: bad header row")
	ErrBadSheet      = errors.New("xlsx: malformed worksheet")
)

// limits are per-Book so tests can lower the ones too slow to reach.
type limits struct {
	rows     int
	strCount int
	strTotal int64
}

var defaultLimits = limits{rows: MaxRows, strCount: MaxSharedStrings, strTotal: MaxSharedStringsTotal}

// Row maps header names to cell text. Empty cells and columns outside the
// header are absent.
type Row map[string]string

// Rows is RowsContext with a background context.
func (b *Book) Rows(sheetName string, required []string, fn func(Row) error) error {
	return b.RowsContext(context.Background(), sheetName, required, fn)
}

// RowsContext streams the named sheet. The first row is the header and must
// hold each required name exactly once. fn is called for every later row
// that has at least one cell under a header column. fn's error is returned
// unwrapped.
func (b *Book) RowsContext(ctx context.Context, sheetName string, required []string, fn func(Row) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var part string
	for _, s := range b.sheets {
		if s.name == sheetName {
			part = s.part
		}
	}
	if part == "" {
		return fmt.Errorf("%w: %q", ErrNoSheet, sheetName)
	}
	if err := b.loadShared(ctx); err != nil {
		return err
	}
	rc, err := b.part(ctx, part, MaxSheetBytes)
	if err != nil {
		return err
	}
	defer rc.Close()
	s := &sheetReader{book: b, d: newDecoder(ctx, rc), required: required, fn: fn}
	if err := s.run(); err != nil {
		if s.rowNum > 0 && !errors.Is(err, errCallback) {
			return fmt.Errorf("%s row %d: %w", sheetName, s.rowNum, err)
		}
		return unwrapCallback(err)
	}
	return ctx.Err()
}

// loadShared reads the shared-string table once per Book.
func (b *Book) loadShared(ctx context.Context) error {
	if b.strsLoaded {
		return nil
	}
	if b.shared == "" {
		b.strsLoaded = true
		return nil
	}
	rc, err := b.part(ctx, b.shared, MaxSharedStringsBytes)
	if err != nil {
		return err
	}
	defer rc.Close()
	d := newDecoder(ctx, rc)
	var (
		strs     []string
		cur      strings.Builder
		inSI     bool
		inT      bool
		phonetic int // depth inside <rPh>, whose text is not part of the value
		total    int64
	)
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%s: %w", b.shared, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "si":
				if inSI {
					return fmt.Errorf("%w: nested si", ErrBadSheet)
				}
				if len(strs) >= b.lim.strCount {
					return fmt.Errorf("%w: more than %d strings", ErrSharedStrings, b.lim.strCount)
				}
				inSI = true
				cur.Reset()
			case "rPh":
				phonetic++
			case "t":
				inT = inSI && phonetic == 0
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "si":
				strs = append(strs, cur.String())
				inSI = false
			case "rPh":
				phonetic--
			case "t":
				inT = false
			}
		case xml.CharData:
			if !inT {
				continue
			}
			if cur.Len()+len(t) > MaxStringBytes {
				return fmt.Errorf("%w: string %d over %d bytes", ErrSharedStrings, len(strs), MaxStringBytes)
			}
			total += int64(len(t))
			if total > b.lim.strTotal {
				return fmt.Errorf("%w: total over %d bytes", ErrSharedStrings, b.lim.strTotal)
			}
			cur.Write(t)
		}
	}
	b.strs, b.strsLoaded = strs, true
	return nil
}

type cell struct {
	col int
	val string
}

type sheetReader struct {
	book     *Book
	d        *decoder
	required []string
	fn       func(Row) error

	sheetDataDepth int // 0 until <sheetData> opens
	rows           int
	rowNum         int
	inRow, inCell  bool
	inV            bool
	cells          []cell
	lastCol        int
	cellCol        int
	cellType       string
	val            []byte
	hasV           bool

	header map[int]string // column -> name, nil until row 1 is read
}

var errCallback = errors.New("callback")

type callbackErr struct{ err error }

func (c callbackErr) Error() string        { return c.err.Error() }
func (c callbackErr) Is(target error) bool { return target == errCallback }
func (c callbackErr) Unwrap() error        { return c.err }

func unwrapCallback(err error) error {
	var c callbackErr
	if errors.As(err, &c) {
		return c.err
	}
	return err
}

func (s *sheetReader) run() error {
	for {
		tok, err := s.d.Token()
		if err == io.EOF {
			return fmt.Errorf("%w: no sheetData", ErrBadSheet)
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := s.start(t); err != nil {
				return err
			}
		case xml.EndElement:
			done, err := s.end(t)
			if err != nil || done {
				return err
			}
		case xml.CharData:
			if s.inV {
				if len(s.val)+len(t) > maxValueBytes {
					return fmt.Errorf("%w: value over %d bytes", ErrCellValue, maxValueBytes)
				}
				s.val = append(s.val, t...)
			}
		}
	}
}

func (s *sheetReader) start(t xml.StartElement) error {
	switch t.Name.Local {
	case "f":
		return ErrFormula
	case "is":
		return fmt.Errorf("%w: inline string", ErrCellType)
	}
	depth := s.d.depth
	if s.sheetDataDepth == 0 {
		if depth == 1 && t.Name.Local != "worksheet" {
			return fmt.Errorf("%w: root %q", ErrBadSheet, t.Name.Local)
		}
		if depth == 2 && t.Name.Local == "sheetData" {
			s.sheetDataDepth = depth
		}
		return nil
	}
	switch depth - s.sheetDataDepth {
	case 1:
		if t.Name.Local != "row" {
			return fmt.Errorf("%w: %q in sheetData", ErrBadSheet, t.Name.Local)
		}
		return s.startRow(t)
	case 2:
		if t.Name.Local == "c" {
			return s.startCell(t)
		}
	case 3:
		if s.inCell && t.Name.Local == "v" {
			s.inV, s.hasV = true, true
		}
	}
	return nil
}

func (s *sheetReader) startRow(t xml.StartElement) error {
	if s.rows++; s.rows > s.book.lim.rows {
		return fmt.Errorf("%w: over %d", ErrRowCap, s.book.lim.rows)
	}
	next := s.rowNum + 1
	if v, ok := attr(t, "r"); ok {
		n, ok := parseUint(v, maxRowNumber)
		if !ok || n <= s.rowNum {
			return fmt.Errorf("%w: row r=%q after %d", ErrCellRef, v, s.rowNum)
		}
		next = n
	}
	s.rowNum, s.inRow, s.lastCol, s.cells = next, true, 0, s.cells[:0]
	return nil
}

func (s *sheetReader) startCell(t xml.StartElement) error {
	if len(s.cells) >= MaxCellsPerRow {
		return fmt.Errorf("%w: over %d", ErrCellsPerRow, MaxCellsPerRow)
	}
	col := s.lastCol + 1
	if v, ok := attr(t, "r"); ok {
		c, r, ok := parseRef(v)
		if !ok || r != s.rowNum || c <= s.lastCol {
			return fmt.Errorf("%w: cell %q in row %d", ErrCellRef, v, s.rowNum)
		}
		col = c
	}
	typ, _ := attr(t, "t")
	switch typ {
	case "", "n", "s":
	default:
		return fmt.Errorf("%w: t=%q", ErrCellType, typ)
	}
	s.inCell, s.cellCol, s.cellType, s.val, s.hasV = true, col, typ, s.val[:0], false
	s.lastCol = col
	// Counted on entry so the cap holds for valueless cells too.
	s.cells = append(s.cells, cell{col: col})
	return nil
}

// end returns done once </sheetData> closes.
func (s *sheetReader) end(t xml.EndElement) (bool, error) {
	if s.sheetDataDepth == 0 {
		return false, nil
	}
	switch s.d.depth - s.sheetDataDepth {
	case -1:
		if t.Name.Local == "sheetData" {
			if s.header == nil {
				return true, fmt.Errorf("%w: no header row", ErrHeader)
			}
			return true, nil
		}
	case 0:
		if t.Name.Local == "row" {
			s.inRow = false
			return false, s.endRow()
		}
	case 1:
		if t.Name.Local == "c" {
			s.inCell = false
			return false, s.endCell()
		}
	case 2:
		if t.Name.Local == "v" {
			s.inV = false
		}
	}
	return false, nil
}

func (s *sheetReader) endCell() error {
	last := &s.cells[len(s.cells)-1]
	v := string(s.val)
	switch {
	case s.cellType == "s":
		i, ok := parseUint(v, len(s.book.strs)-1)
		if !s.hasV || !ok {
			return fmt.Errorf("%w: %q of %d", ErrSharedIndex, v, len(s.book.strs))
		}
		last.val = s.book.strs[i]
	case !s.hasV:
		// Styled blank cell.
	case !isDecimal(v):
		return fmt.Errorf("%w: number %q", ErrCellValue, v)
	default:
		last.val = v
	}
	return nil
}

func (s *sheetReader) endRow() error {
	if s.header == nil {
		return s.readHeader()
	}
	var row Row
	for _, c := range s.cells {
		name, ok := s.header[c.col]
		if !ok || c.val == "" {
			continue
		}
		if row == nil {
			row = make(Row, len(s.header))
		}
		row[name] = c.val
	}
	if row == nil {
		return nil
	}
	if err := s.fn(row); err != nil {
		return callbackErr{err}
	}
	return nil
}

func (s *sheetReader) readHeader() error {
	s.header = make(map[int]string, len(s.cells))
	seen := map[string]bool{}
	for _, c := range s.cells {
		if c.val == "" {
			continue
		}
		if seen[c.val] {
			return fmt.Errorf("%w: duplicate column %q", ErrHeader, c.val)
		}
		seen[c.val] = true
		s.header[c.col] = c.val
	}
	for _, name := range s.required {
		if !seen[name] {
			return fmt.Errorf("%w: missing column %q", ErrHeader, name)
		}
	}
	return nil
}

func attr(t xml.StartElement, local string) (string, bool) {
	for _, a := range t.Attr {
		if a.Name.Local == local && a.Name.Space == "" {
			return a.Value, true
		}
	}
	return "", false
}

// parseUint accepts 0 or a digit string without a leading zero, up to max.
// The bound is checked per digit in uint64 so no input can wrap.
func parseUint(s string, max int) (int, bool) {
	if s == "" || max < 0 || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		if n > (math.MaxUint64-9)/10 {
			return 0, false
		}
		n = n*10 + uint64(s[i]-'0')
		if n > uint64(max) {
			return 0, false
		}
	}
	return int(n), true
}

// parseRef parses an A1 reference: 1 to 3 upper-case letters up to XFD,
// then a row number from 1.
func parseRef(s string) (col, row int, ok bool) {
	i := 0
	for i < len(s) && s[i] >= 'A' && s[i] <= 'Z' {
		col = col*26 + int(s[i]-'A'+1)
		i++
	}
	if i == 0 || i > 3 || col > MaxColumn {
		return 0, 0, false
	}
	row, ok = parseUint(s[i:], maxRowNumber)
	if !ok || row == 0 {
		return 0, 0, false
	}
	return col, row, true
}

// isDecimal accepts -?digits(.digits)?([eE][+-]?digits)?, the form Excel
// writes. It rejects NaN, Inf, hex and underscores, which ParseFloat allows.
func isDecimal(s string) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	digits := func() int {
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		return i - start
	}
	if digits() == 0 {
		return false
	}
	if i < len(s) && s[i] == '.' {
		i++
		if digits() == 0 {
			return false
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if digits() == 0 {
			return false
		}
	}
	if i != len(s) {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
