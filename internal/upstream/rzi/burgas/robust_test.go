package burgas

import (
	"errors"
	"strings"
	"testing"

	"kanarche.eu/internal/upstream/rzi"
)

// These tests call the readers directly, without Extract's recover, so a
// missing bounds check shows up as a panic and not as a refusal.

func goodStream() []byte {
	return buildStream(bookOpts{cells: std([]trow{ok(46160, 46160, 15, 15)})})
}

func TestEveryTruncationIsAnError(t *testing.T) {
	st := goodStream()
	for n := 0; n < len(st); n++ {
		if _, err := parseWorkbook(st[:n:n]); err == nil {
			// Cutting inside the trailing sheet records still leaves no EOF.
			t.Fatalf("stream cut to %d bytes parsed", n)
		}
	}
}

func TestLengthFieldsAreNotTrusted(t *testing.T) {
	st := goodStream()
	// Every record's length field set to the maximum, one at a time.
	for pos := 0; pos+4 <= len(st); {
		mut := append([]byte(nil), st...)
		mut[pos+2], mut[pos+3] = 0xFF, 0xFF
		if _, err := parseWorkbook(mut); err == nil {
			t.Fatalf("record at %d with length 65535 parsed", pos)
		}
		pos += 4 + int(st[pos+2]) | int(st[pos+3])<<8
	}
}

func TestOversizeRecordRefused(t *testing.T) {
	big := rec(0x0001, make([]byte, MaxRecordLen+1))
	_, err := parseWorkbook(buildStream(bookOpts{cells: std(nil), rawRecs: [][]byte{big}}))
	if !errors.Is(err, ErrBIFF) {
		t.Fatalf("err = %v", err)
	}
	fine := rec(0x0001, make([]byte, MaxRecordLen))
	if _, err := parseWorkbook(buildStream(bookOpts{cells: std(nil), rawRecs: [][]byte{fine}})); err != nil {
		t.Fatalf("record at the limit: %v", err)
	}
}

func TestSSTIndexOutOfRange(t *testing.T) {
	d := append(append(le16(3), le16(0)...), le16(0)...)
	d = append(d, le32(100000)...)
	_, err := parseWorkbook(buildStream(bookOpts{cells: std(nil), rawRecs: [][]byte{rec(recLabelSST, d)}}))
	if !errors.Is(err, ErrBIFF) {
		t.Fatalf("err = %v", err)
	}
}

func TestShortCellRecords(t *testing.T) {
	for _, id := range []int{recLabelSST, recLabel, recNumber, recRK, recMulRK} {
		for n := 0; n < 14; n++ {
			st := buildStream(bookOpts{cells: std(nil), rawRecs: [][]byte{rec(id, make([]byte, n))}})
			// Must not panic; a short body is an error or a harmless cell.
			parseWorkbook(st)
		}
	}
}

func TestMulRK(t *testing.T) {
	// Row 8, columns 2..4: three integers 10, 20, 30.
	d := append(le16(8), le16(2)...)
	for _, v := range []int{10, 20, 30} {
		d = append(d, le16(0)...)
		d = append(d, le32(uint32(v)<<2|2)...)
	}
	d = append(d, le16(4)...)
	sh, err := parseWorkbook(buildStream(bookOpts{cells: std(nil), rawRecs: [][]byte{rec(recMulRK, d)}}))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{10, 20, 30} {
		if c, _ := sh.at(8, 2+i); c.num != want {
			t.Errorf("col %d = %v", 2+i, c.num)
		}
	}
	// A body that is not a whole number of entries is refused.
	bad := append(d, 0)
	if _, err := parseWorkbook(buildStream(bookOpts{cells: std(nil), rawRecs: [][]byte{rec(recMulRK, bad)}})); !errors.Is(err, ErrBIFF) {
		t.Errorf("odd MULRK: %v", err)
	}
}

func TestBadValuesAreRowWarnings(t *testing.T) {
	cases := map[string]trow{
		"over cap":   {n(0, 0, 46160), n(0, 1, 46160), n(0, 2, rzi.MaxCount+1), n(0, 3, 15), ""},
		"negative":   {n(0, 0, 46160), n(0, 1, 46160), n(0, 2, -5), n(0, 3, 15), ""},
		"fraction":   {n(0, 0, 46160), n(0, 1, 46160), n(0, 2, 15.5), n(0, 3, 15), ""},
		"text":       {n(0, 0, 46160), n(0, 1, 46160), s(0, 2, "много"), n(0, 3, 15), ""},
		"serial low": {n(0, 0, 46160), n(0, 1, 5), n(0, 2, 15), n(0, 3, 15), ""},
		"serial big": {n(0, 0, 46160), n(0, 1, 1e12), n(0, 2, 15), n(0, 3, 15), ""},
		"missing ec": {n(0, 0, 46160), n(0, 1, 46160), n(0, 2, 15), tcell{}, ""},
	}
	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			f := extract(t, bookOpts{cells: std([]trow{ok(46174, 46174, 15, 15), row})})
			if f.Error != "" || len(f.Blocks[0].Rows) != 1 || len(f.Warnings) != 1 {
				t.Fatalf("error %q rows %d warnings %v", f.Error, len(f.Blocks[0].Rows), f.Warnings)
			}
		})
	}
}

func TestLongRemarksAreClipped(t *testing.T) {
	row := ok(46160, 46160, 15, 15)
	row.info = strings.Repeat("ж", 600)
	f := extract(t, bookOpts{cells: std([]trow{row})})
	if r := []rune(f.Blocks[0].Rows[0].Remarks); len(r) != rzi.MaxRemarksRunes {
		t.Fatalf("remarks %d runes", len(r))
	}
	if _, status := roundTrip(t, f); status != rzi.StatusOK {
		t.Errorf("status %s", status)
	}
}

// I10: the record the extractor writes stays under the contract's cap.
func TestOutputSizeCap(t *testing.T) {
	var cells []tcell
	for b := 0; b < rzi.MaxBlocks; b++ {
		rows := make([]trow, rzi.MaxRowsPerBlock)
		for i := range rows {
			rows[i] = ok(46160, 46160, 15, 15)
			rows[i].info = strings.Repeat("ж", rzi.MaxRemarksRunes)
		}
		cells = append(cells, blockCells(b*70, "02001", `N 42° 49' 14"`, `E 27° 53' 07"`, rows)...)
	}
	f := Extract(buildCFB(buildStream(bookOpts{cells: cells}), cfbOpts{}))
	if f.Error != "output over size cap" {
		t.Fatalf("error %q blocks %d", f.Error, len(f.Blocks))
	}
}
