package burgas

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"unicode/utf16"

	"github.com/richardlehane/mscfb"
)

// Hard caps. The input is hostile, so every one of these refuses the file.
const (
	MaxInputBytes   = 4 << 20
	MaxStreamBytes  = 4 << 20
	MaxRecords      = 200_000
	MaxRecordLen    = 8224 // BIFF8 limit for one record body
	MaxSheets       = 16
	MaxSSTStrings   = 50_000
	MaxStringChars  = 4096
	MaxSSTChars     = 2 << 20
	MaxCells        = 100_000
	maxContinuation = 4096
)

// BIFF8 record ids used by the reader.
const (
	recBOF       = 0x0809
	recEOF       = 0x000A
	recBoundSht  = 0x0085
	recSST       = 0x00FC
	recContinue  = 0x003C
	recLabelSST  = 0x00FD
	recLabel     = 0x0204
	recNumber    = 0x0203
	recRK        = 0x027E
	recMulRK     = 0x00BD
	recDateMode  = 0x0022
	biff8Version = 0x0600
	bofGlobals   = 0x0005
	bofSheet     = 0x0010
)

var (
	ErrBIFF     = errors.New("burgas: bad BIFF8 stream")
	ErrTooLarge = errors.New("burgas: input over size cap")
)

// workbookStream opens the OLE2 container and returns the Workbook stream.
func workbookStream(data []byte) (out []byte, err error) {
	defer func() {
		if recover() != nil {
			out, err = nil, fmt.Errorf("%w: reader failed", ErrContainer)
		}
	}()
	if len(data) > MaxInputBytes {
		return nil, ErrTooLarge
	}
	if err := checkChains(data); err != nil {
		return nil, err
	}
	r, err := mscfb.New(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrContainer, err)
	}
	for i := 0; i < maxDirEntry; i++ {
		f, err := r.Next()
		if err != nil {
			break
		}
		if f.Name != "Workbook" && f.Name != "Book" {
			continue
		}
		if f.Size == 0 || f.Size > MaxStreamBytes {
			return nil, fmt.Errorf("%w: stream size", ErrBIFF)
		}
		buf := make([]byte, f.Size)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, fmt.Errorf("%w: short stream", ErrBIFF)
		}
		return buf, nil
	}
	return nil, fmt.Errorf("%w: no workbook stream", ErrBIFF)
}

type record struct {
	id   uint16
	data []byte
}

// recordAt reads one record header and body at pos and returns the position
// after it. A length field is never trusted past the end of the stream.
func recordAt(b []byte, pos int) (record, int, error) {
	if pos < 0 || pos+4 > len(b) {
		return record{}, 0, fmt.Errorf("%w: truncated header", ErrBIFF)
	}
	id := binary.LittleEndian.Uint16(b[pos:])
	n := int(binary.LittleEndian.Uint16(b[pos+2:]))
	if n > MaxRecordLen {
		return record{}, 0, fmt.Errorf("%w: record too long", ErrBIFF)
	}
	end := pos + 4 + n
	if end > len(b) {
		return record{}, 0, fmt.Errorf("%w: truncated record", ErrBIFF)
	}
	return record{id, b[pos+4 : end]}, end, nil
}

type cellPos struct{ row, col int }

type cell struct {
	isNum bool
	num   float64
	str   string
}

// sheet is a sparse grid; row and column are the 16-bit BIFF8 indices.
type sheet struct {
	cells    map[uint32]cell
	maxRow   int
	date1904 bool
}

func key(row, col int) uint32 { return uint32(row)<<16 | uint32(col) }

func (s *sheet) at(row, col int) (cell, bool) {
	c, ok := s.cells[key(row, col)]
	return c, ok
}

func (s *sheet) put(p cellPos, c cell) error {
	row, col := p.row, p.col
	if len(s.cells) >= MaxCells {
		return fmt.Errorf("%w: too many cells", ErrBIFF)
	}
	s.cells[key(row, col)] = c
	if row > s.maxRow {
		s.maxRow = row
	}
	return nil
}

type boundSheet struct {
	offset uint32
	kind   byte
}

// parseWorkbook reads the globals substream and then the first worksheet.
func parseWorkbook(b []byte) (*sheet, error) {
	count := 0
	next := func(pos int) (record, int, error) {
		count++
		if count > MaxRecords {
			return record{}, 0, fmt.Errorf("%w: too many records", ErrBIFF)
		}
		return recordAt(b, pos)
	}
	rec, pos, err := next(0)
	if err != nil {
		return nil, err
	}
	if err := checkBOF(rec, bofGlobals); err != nil {
		return nil, err
	}
	sh := &sheet{cells: map[uint32]cell{}}
	var sheets []boundSheet
	var sst []string
	for {
		rec, pos, err = next(pos)
		if err != nil {
			return nil, err
		}
		if rec.id == recEOF {
			break
		}
		switch rec.id {
		case recBoundSht:
			if len(rec.data) < 6 {
				return nil, fmt.Errorf("%w: short boundsheet", ErrBIFF)
			}
			if len(sheets) >= MaxSheets {
				return nil, fmt.Errorf("%w: too many sheets", ErrBIFF)
			}
			sheets = append(sheets, boundSheet{binary.LittleEndian.Uint32(rec.data), rec.data[5]})
		case recDateMode:
			sh.date1904 = len(rec.data) >= 2 && binary.LittleEndian.Uint16(rec.data) == 1
		case recSST:
			chunks := [][]byte{rec.data}
			for len(chunks) < maxContinuation {
				nrec, npos, err := recordAt(b, pos)
				if err != nil || nrec.id != recContinue {
					break
				}
				count++
				if count > MaxRecords {
					return nil, fmt.Errorf("%w: too many records", ErrBIFF)
				}
				chunks = append(chunks, nrec.data)
				pos = npos
			}
			if sst, err = parseSST(chunks); err != nil {
				return nil, err
			}
		}
	}
	for _, bs := range sheets {
		if bs.kind != 0 { // worksheet only
			continue
		}
		return sh, parseSheet(b, int(bs.offset), sst, sh, next)
	}
	return nil, fmt.Errorf("%w: no worksheet", ErrBIFF)
}

func checkBOF(rec record, kind uint16) error {
	if rec.id != recBOF || len(rec.data) < 4 ||
		binary.LittleEndian.Uint16(rec.data) != biff8Version ||
		binary.LittleEndian.Uint16(rec.data[2:]) != kind {
		return fmt.Errorf("%w: not a BIFF8 BOF", ErrBIFF)
	}
	return nil
}

func parseSheet(b []byte, pos int, sst []string, sh *sheet, next func(int) (record, int, error)) error {
	rec, pos, err := next(pos)
	if err != nil {
		return err
	}
	if err := checkBOF(rec, bofSheet); err != nil {
		return err
	}
	for {
		rec, pos, err = next(pos)
		if err != nil {
			return err
		}
		d := rec.data
		switch rec.id {
		case recEOF:
			return nil
		case recLabelSST:
			if len(d) < 10 {
				return fmt.Errorf("%w: short LABELSST", ErrBIFF)
			}
			i := int(binary.LittleEndian.Uint32(d[6:]))
			if i < 0 || i >= len(sst) {
				return fmt.Errorf("%w: SST index", ErrBIFF)
			}
			err = sh.put(rc(d), cell{str: sst[i]})
		case recLabel:
			if len(d) < 8 {
				return fmt.Errorf("%w: short LABEL", ErrBIFF)
			}
			r := &strReader{chunks: [][]byte{d[6:]}}
			var s string
			if s, err = r.unicode(false, new(int)); err == nil {
				err = sh.put(rc(d), cell{str: s})
			}
		case recNumber:
			if len(d) < 14 {
				return fmt.Errorf("%w: short NUMBER", ErrBIFF)
			}
			f := math.Float64frombits(binary.LittleEndian.Uint64(d[6:]))
			err = sh.put(rc(d), cell{isNum: true, num: f})
		case recRK:
			if len(d) < 10 {
				return fmt.Errorf("%w: short RK", ErrBIFF)
			}
			err = sh.put(rc(d), cell{isNum: true, num: rkValue(binary.LittleEndian.Uint32(d[6:]))})
		case recMulRK:
			// row, first col, (xf, rk)*, last col
			if len(d) < 4+6+2 || (len(d)-6)%6 != 0 {
				return fmt.Errorf("%w: bad MULRK", ErrBIFF)
			}
			row := int(binary.LittleEndian.Uint16(d))
			first := int(binary.LittleEndian.Uint16(d[2:]))
			n := (len(d) - 6) / 6
			for i := 0; i < n && err == nil; i++ {
				v := rkValue(binary.LittleEndian.Uint32(d[6+6*i:]))
				if first+i > 0xFFFF {
					return fmt.Errorf("%w: MULRK column", ErrBIFF)
				}
				err = sh.put(cellPos{row, first + i}, cell{isNum: true, num: v})
			}
		}
		if err != nil {
			return err
		}
	}
}

// rc returns the row and column of a cell record body of at least 4 bytes.
func rc(d []byte) cellPos {
	return cellPos{int(binary.LittleEndian.Uint16(d)), int(binary.LittleEndian.Uint16(d[2:]))}
}

func rkValue(rk uint32) float64 {
	var v float64
	if rk&2 != 0 {
		v = float64(int32(rk) >> 2)
	} else {
		v = math.Float64frombits(uint64(rk&0xFFFFFFFC) << 32)
	}
	if rk&1 != 0 {
		v /= 100
	}
	return v
}

// strReader reads BIFF8 strings from record chunks. A string's character
// data may continue in the next chunk, which then starts with a new flag byte.
type strReader struct {
	chunks [][]byte
	ci     int
	pos    int
}

var errEnd = fmt.Errorf("%w: string data ends early", ErrBIFF)

// left is the number of bytes left in the current chunk.
func (r *strReader) left() int { return len(r.chunks[r.ci]) - r.pos }

// advance moves to the next chunk when the current one is used up.
func (r *strReader) advance() bool {
	for r.left() == 0 {
		if r.ci+1 >= len(r.chunks) {
			return false
		}
		r.ci++
		r.pos = 0
	}
	return true
}

func (r *strReader) u8() (byte, error) {
	if !r.advance() {
		return 0, errEnd
	}
	v := r.chunks[r.ci][r.pos]
	r.pos++
	return v, nil
}

func (r *strReader) u16() (int, error) {
	lo, err := r.u8()
	if err != nil {
		return 0, err
	}
	hi, err := r.u8()
	if err != nil {
		return 0, err
	}
	return int(lo) | int(hi)<<8, nil
}

func (r *strReader) skip(n int) error {
	for n > 0 {
		if !r.advance() {
			return errEnd
		}
		k := min(n, r.left())
		r.pos += k
		n -= k
	}
	return nil
}

// unicode reads one string: u16 length, flags, optional run and extension
// counts, characters, then the runs and extension data, which are skipped.
// total counts characters over the whole table.
func (r *strReader) unicode(rich bool, total *int) (string, error) {
	n, err := r.u16()
	if err != nil {
		return "", err
	}
	flags, err := r.u8()
	if err != nil {
		return "", err
	}
	if n > MaxStringChars {
		return "", fmt.Errorf("%w: string too long", ErrBIFF)
	}
	*total += n
	if *total > MaxSSTChars {
		return "", fmt.Errorf("%w: string table too large", ErrBIFF)
	}
	runs, ext := 0, 0
	if rich && flags&0x08 != 0 {
		if runs, err = r.u16(); err != nil {
			return "", err
		}
	}
	if rich && flags&0x04 != 0 {
		lo, err := r.u16()
		if err != nil {
			return "", err
		}
		hi, err := r.u16()
		if err != nil {
			return "", err
		}
		ext = lo | hi<<16
	}
	wide := flags&0x01 != 0
	u16s := make([]uint16, 0, n)
	for len(u16s) < n {
		if r.left() == 0 {
			if !r.advance() {
				return "", errEnd
			}
			// A continued string repeats the flag byte.
			f, err := r.u8()
			if err != nil {
				return "", err
			}
			wide = f&0x01 != 0
		}
		unit := 1
		if wide {
			unit = 2
		}
		k := min(n-len(u16s), r.left()/unit)
		if k == 0 {
			return "", errEnd // half a UTF-16 unit at a chunk end
		}
		cur := r.chunks[r.ci][r.pos:]
		for i := 0; i < k; i++ {
			if wide {
				u16s = append(u16s, binary.LittleEndian.Uint16(cur[2*i:]))
			} else {
				u16s = append(u16s, uint16(cur[i]))
			}
		}
		r.pos += k * unit
	}
	if err := r.skip(4 * runs); err != nil {
		return "", err
	}
	if err := r.skip(ext); err != nil {
		return "", err
	}
	return string(utf16.Decode(u16s)), nil
}

// parseSST reads the shared string table from the SST record body and its
// CONTINUE records.
func parseSST(chunks [][]byte) ([]string, error) {
	if len(chunks[0]) < 8 {
		return nil, fmt.Errorf("%w: short SST", ErrBIFF)
	}
	unique := int(binary.LittleEndian.Uint32(chunks[0][4:]))
	if unique < 0 || unique > MaxSSTStrings {
		return nil, fmt.Errorf("%w: too many strings", ErrBIFF)
	}
	r := &strReader{chunks: chunks, pos: 8}
	out := make([]string, 0, min(unique, 1024))
	total := 0
	for i := 0; i < unique; i++ {
		s, err := r.unicode(true, &total)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}
