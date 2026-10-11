package burgas

import (
	"encoding/binary"
	"math"
	"unicode/utf16"
)

// Synthetic workbook builder. It writes the BIFF8 records and the OLE2
// container the extractor reads, so tests need no real RZI file.

type tcell struct {
	row, col int
	str      string
	num      float64
	isNum    bool
	asRK     bool // write an integer as an RK record
}

func s(row, col int, v string) tcell  { return tcell{row: row, col: col, str: v} }
func n(row, col int, v float64) tcell { return tcell{row: row, col: col, num: v, isNum: true} }
func rk(row, col int, v int) tcell {
	return tcell{row: row, col: col, num: float64(v), isNum: true, asRK: true}
}

func le16(v int) []byte { return binary.LittleEndian.AppendUint16(nil, uint16(v)) }
func le32(v uint32) []byte {
	return binary.LittleEndian.AppendUint32(nil, v)
}

func rec(id int, body []byte) []byte {
	return append(append(le16(id), le16(len(body))...), body...)
}

func bof(kind int) []byte {
	body := append(le16(biff8Version), le16(kind)...)
	return rec(recBOF, append(body, make([]byte, 12)...))
}

func utf16le(str string) []byte {
	var out []byte
	for _, u := range utf16.Encode([]rune(str)) {
		out = append(out, le16(int(u))...)
	}
	return out
}

type bookOpts struct {
	cells      []tcell
	splitStr   int  // with splitChar > 0, split the SST inside string number splitStr
	splitChar  int  // after this many characters
	latin1     bool // store ASCII strings with the compressed flag
	sheets     int  // extra BOUNDSHEET records (empty names)
	date1904   bool
	extraRecs  int      // junk records appended to the sheet
	rawGlobals [][]byte // whole records put in the globals before the SST
	flipTail   bool     // re-encode the part after a forced split as UTF-16
	rawRecs    [][]byte // whole records appended to the sheet as given
}

// buildStream returns a Workbook stream: globals with SST, one sheet.
func buildStream(o bookOpts) []byte {
	var strs []string
	idx := map[string]int{}
	for _, c := range o.cells {
		if !c.isNum {
			if _, ok := idx[c.str]; !ok {
				idx[c.str] = len(strs)
				strs = append(strs, c.str)
			}
		}
	}
	sstRecs := encodeSST(strs, o)
	var sheet []byte
	sheet = append(sheet, bof(bofSheet)...)
	for _, c := range o.cells {
		sheet = append(sheet, cellRecord(c, idx)...)
	}
	for _, r := range o.rawRecs {
		sheet = append(sheet, r...)
	}
	for i := 0; i < o.extraRecs; i++ {
		sheet = append(sheet, rec(0x0001, nil)...)
	}
	sheet = append(sheet, rec(recEOF, nil)...)

	globals := func(off int) []byte {
		g := bof(bofGlobals)
		if o.date1904 {
			g = append(g, rec(recDateMode, le16(1))...)
		}
		for _, r := range o.rawGlobals {
			g = append(g, r...)
		}
		g = append(g, sstRecs...)
		bs := func(off int) []byte {
			body := append(le32(uint32(off)), 0, 0, 1, 0) // visible worksheet, name "x"
			return rec(recBoundSht, append(body, 'x'))
		}
		g = append(g, bs(off)...)
		for i := 0; i < o.sheets; i++ {
			g = append(g, bs(0)...)
		}
		return append(g, rec(recEOF, nil)...)
	}
	size := len(globals(0))
	return append(globals(size), sheet...)
}

func isASCII(str string) bool {
	for i := 0; i < len(str); i++ {
		if str[i] >= 0x80 {
			return false
		}
	}
	return true
}

func cellRecord(c tcell, idx map[string]int) []byte {
	head := append(le16(c.row), le16(c.col)...)
	head = append(head, le16(0)...) // xf
	switch {
	case c.isNum && c.asRK:
		return rec(recRK, append(head, le32(uint32(int32(c.num))<<2|2)...))
	case c.isNum:
		return rec(recNumber, binary.LittleEndian.AppendUint64(head, math.Float64bits(c.num)))
	}
	return rec(recLabelSST, append(head, le32(uint32(idx[c.str]))...))
}

// cfbOpts lets a test damage the container.
type cfbOpts struct {
	fatMut  func(fat []uint32)
	dirSize uint64 // 0 keeps the real stream size
}

// buildCFB wraps a stream in a minimal v3 OLE2 container with 512 byte
// sectors and no mini stream. Short streams are padded to 4096 bytes.
func buildCFB(stream []byte, o cfbOpts) []byte {
	if len(stream) < miniCutoff {
		stream = append(stream, make([]byte, miniCutoff-len(stream))...)
	}
	nStream := (len(stream) + 511) / 512
	nFat := 1
	for nFat*128 < nFat+1+nStream {
		nFat++
	}
	fat := make([]uint32, nFat*128)
	for i := range fat {
		fat[i] = 0xFFFFFFFF
	}
	for i := 0; i < nFat; i++ {
		fat[i] = 0xFFFFFFFD
	}
	dirSec := nFat
	fat[dirSec] = sectEnd
	first := nFat + 1
	for i := 0; i < nStream; i++ {
		if i == nStream-1 {
			fat[first+i] = sectEnd
		} else {
			fat[first+i] = uint32(first + i + 1)
		}
	}
	if o.fatMut != nil {
		o.fatMut(fat)
	}
	size := uint64(len(stream))
	if o.dirSize != 0 {
		size = o.dirSize
	}

	hdr := make([]byte, 512)
	copy(hdr, cfbMagic)
	binary.LittleEndian.PutUint16(hdr[24:], 0x3E)
	binary.LittleEndian.PutUint16(hdr[26:], 3)
	binary.LittleEndian.PutUint16(hdr[28:], 0xFFFE)
	binary.LittleEndian.PutUint16(hdr[30:], 9)
	binary.LittleEndian.PutUint16(hdr[32:], 6)
	binary.LittleEndian.PutUint32(hdr[44:], uint32(nFat))
	binary.LittleEndian.PutUint32(hdr[48:], uint32(dirSec))
	binary.LittleEndian.PutUint32(hdr[56:], miniCutoff)
	binary.LittleEndian.PutUint32(hdr[60:], sectEnd)
	binary.LittleEndian.PutUint32(hdr[68:], sectEnd)
	for i := 0; i < 109; i++ {
		v := uint32(0xFFFFFFFF)
		if i < nFat {
			v = uint32(i)
		}
		binary.LittleEndian.PutUint32(hdr[76+4*i:], v)
	}

	dir := make([]byte, 512)
	entry := func(i int, name string, typ byte, child, start uint32, size uint64) {
		e := dir[i*128 : (i+1)*128]
		nm := utf16le(name)
		copy(e, nm)
		binary.LittleEndian.PutUint16(e[64:], uint16(len(nm)+2))
		e[66], e[67] = typ, 1
		binary.LittleEndian.PutUint32(e[68:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(e[72:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(e[76:], child)
		binary.LittleEndian.PutUint32(e[116:], start)
		binary.LittleEndian.PutUint64(e[120:], size)
	}
	entry(0, "Root Entry", 5, 1, sectEnd, 0)
	entry(1, "Workbook", 2, 0xFFFFFFFF, uint32(first), size)
	for i := 2; i < 4; i++ {
		binary.LittleEndian.PutUint32(dir[i*128+68:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(dir[i*128+72:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(dir[i*128+76:], 0xFFFFFFFF)
	}

	out := hdr
	for _, v := range fat {
		out = binary.LittleEndian.AppendUint32(out, v)
	}
	out = append(out, dir...)
	out = append(out, stream...)
	if r := len(stream) % 512; r != 0 {
		out = append(out, make([]byte, 512-r)...)
	}
	return out
}

// blockCells lays out one block the way the RZI workbook does, starting at
// row top. rows are {planned, date, ie, ec, info}; a nil entry is skipped.
type trow struct {
	planned, date, ie, ec tcell
	info                  string
}

func blockCells(top int, point string, lat, lon string, rows []trow) []tcell {
	cs := []tcell{
		s(top, 0, "ПУНКТ ЗА ВЗЕМАНЕ НА ПРОБИ №"), s(top, 2, point),
		s(top, 3, "наименование:"), s(top, 5, "Тестов плаж"),
		s(top+1, 0, "географски координати:"), s(top+1, 2, "ширина"), s(top+1, 3, lat),
		s(top+1, 4, "дължина"), s(top+1, 5, lon),
		s(top+2, 0, "времетраене на сезона за къпане "),
		s(top+5, 0, "Планирана дата за пробонабиране, съгласно графика за мониторинг"),
		s(top+5, 1, "Дата на пробонабиране "),
		s(top+5, 2, " Резултат от анализ за чревни ентерококи\nMPN/100 cm3"),
		s(top+5, 3, "Резултат от анализ за Ешерихия коли\nMPN/100 cm3"),
		s(top+5, 4, "Допълнителна\nинформация"),
	}
	for i, r := range rows {
		row := top + 6 + i
		for _, c := range []tcell{r.planned, r.date, r.ie, r.ec} {
			if c.isNum || c.str != "" {
				c.row = row
				cs = append(cs, c)
			}
		}
		if r.info != "" {
			cs = append(cs, s(row, 4, r.info))
		}
	}
	return cs
}

// col returns c moved to column col.
