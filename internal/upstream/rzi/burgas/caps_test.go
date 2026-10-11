package burgas

import (
	"encoding/binary"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// Each cap is checked by its own reason, called directly so another check
// cannot stand in for it.

func wantErr(t *testing.T, err error, kind error, text string) {
	t.Helper()
	if !errors.Is(err, kind) || !strings.Contains(err.Error(), text) {
		t.Fatalf("err = %v, want %v containing %q", err, kind, text)
	}
}

func TestCapReasons(t *testing.T) {
	base := std([]trow{ok(46160, 46160, 15, 15)})
	t.Run("string too long", func(t *testing.T) {
		cs := append([]tcell{s(40, 0, strings.Repeat("x", MaxStringChars+1))}, base...)
		_, err := parseWorkbook(buildStream(bookOpts{cells: cs}))
		wantErr(t, err, ErrBIFF, "string too long")
	})
	t.Run("string at the cap", func(t *testing.T) {
		cs := append([]tcell{s(40, 0, strings.Repeat("x", MaxStringChars))}, base...)
		if _, err := parseWorkbook(buildStream(bookOpts{cells: cs})); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("string table too large", func(t *testing.T) {
		var cs []tcell
		for i := 0; i < 600; i++ {
			cs = append(cs, s(i, 0, strings.Repeat(string(rune('a'+i%26)), MaxStringChars-10)+itoa(i)))
		}
		_, err := parseWorkbook(buildStream(bookOpts{cells: cs, latin1: true}))
		wantErr(t, err, ErrBIFF, "string table too large")
	})
	t.Run("too many strings", func(t *testing.T) {
		var cs []tcell
		for i := 0; i <= MaxSSTStrings; i++ {
			cs = append(cs, s(i%50000, i/50000, itoa(i)))
		}
		_, err := parseWorkbook(buildStream(bookOpts{cells: cs}))
		wantErr(t, err, ErrBIFF, "too many strings")
	})
	t.Run("declared string count", func(t *testing.T) {
		st := buildStream(bookOpts{cells: base})
		i := slices.Index(st, 0xFC) // SST record id
		binary.LittleEndian.PutUint32(st[i+8:], 0x7FFFFFFF)
		_, err := parseWorkbook(st)
		wantErr(t, err, ErrBIFF, "too many strings")
	})
	t.Run("too many sheets", func(t *testing.T) {
		_, err := parseWorkbook(buildStream(bookOpts{cells: base, sheets: MaxSheets}))
		wantErr(t, err, ErrBIFF, "too many sheets")
	})
	t.Run("too many records", func(t *testing.T) {
		_, err := parseWorkbook(buildStream(bookOpts{cells: base, extraRecs: MaxRecords}))
		wantErr(t, err, ErrBIFF, "too many records")
	})
	t.Run("too many cells", func(t *testing.T) {
		var cs []tcell
		for i := 0; i <= MaxCells/200; i++ {
			for c := 0; c < 200; c++ {
				cs = append(cs, n(i, c, 1))
			}
		}
		_, err := parseWorkbook(buildStream(bookOpts{cells: cs}))
		wantErr(t, err, ErrBIFF, "too many cells")
	})
	t.Run("declared stream size", func(t *testing.T) {
		for _, size := range []uint64{MaxStreamBytes + 1, 1 << 40, 1<<64 - 1} {
			_, err := workbookStream(buildCFB(goodStream(), cfbOpts{dirSize: size}))
			wantErr(t, err, ErrBIFF, "stream size")
		}
	})
}

func itoa(i int) string {
	const d = "0123456789"
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{d[i%10]}, b...)
	}
	return string(b)
}

func bigContainer() []byte {
	// Over 128 sectors, so the FAT spans several sectors.
	return buildCFB(buildStream(bookOpts{cells: std(nil), extraRecs: 30000}), cfbOpts{})
}

func TestChainChecksCalledDirectly(t *testing.T) {
	good := bigContainer()
	if err := checkChains(good); err != nil {
		t.Fatalf("good container: %v", err)
	}
	t.Run("declared FAT shorter than the file", func(t *testing.T) {
		d := slices.Clone(good)
		binary.LittleEndian.PutUint32(d[44:], 1)
		if err := checkChains(d); !errors.Is(err, ErrChain) {
			t.Fatalf("err = %v", err)
		}
	})
	for _, nFat := range []uint32{0, 110, 1 << 30} {
		d := slices.Clone(good)
		binary.LittleEndian.PutUint32(d[44:], nFat)
		if err := checkChains(d); !errors.Is(err, ErrContainer) {
			t.Errorf("nFat %d: %v", nFat, err)
		}
	}
	t.Run("sector inside the FAT but past the file", func(t *testing.T) {
		d := buildCFB(goodStream(), cfbOpts{fatMut: func(fat []uint32) { fat[3], fat[50] = 50, sectEnd }})
		if err := checkChains(d); !errors.Is(err, ErrChain) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("extended DIFAT", func(t *testing.T) {
		d := slices.Clone(good)
		binary.LittleEndian.PutUint32(d[68:], 5)
		if err := checkChains(d); !errors.Is(err, ErrContainer) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("sector size", func(t *testing.T) {
		d := slices.Clone(good)
		d[30] = 31
		if err := checkChains(d); !errors.Is(err, ErrContainer) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestSSTShapes(t *testing.T) {
	t.Run("short SST body", func(t *testing.T) {
		_, err := parseWorkbook(buildStream(bookOpts{cells: std(nil), rawGlobals: [][]byte{rec(recSST, []byte{1, 2, 3})}}))
		wantErr(t, err, ErrBIFF, "short SST")
	})
	t.Run("MULRK column past the grid", func(t *testing.T) {
		d := append(le16(1), le16(0xFFFF)...)
		for i := 0; i < 3; i++ {
			d = append(d, le16(0)...)
			d = append(d, le32(2)...)
		}
		d = append(d, le16(0xFFFF)...)
		_, err := parseWorkbook(buildStream(bookOpts{cells: std(nil), rawRecs: [][]byte{rec(recMulRK, d)}}))
		wantErr(t, err, ErrBIFF, "MULRK column")
	})
	t.Run("ASCII string continues as UTF-16", func(t *testing.T) {
		f := extract(t, bookOpts{cells: std([]trow{ok(46160, 46160, 15, 15)}), latin1: true, flipTail: true, splitStr: 1, splitChar: 2})
		if f.Error != "" || len(f.Warnings) != 0 || len(f.Blocks) != 1 {
			t.Fatalf("error %q warnings %v", f.Error, f.Warnings)
		}
	})
	t.Run("half a UTF-16 unit at a chunk end ends", func(t *testing.T) {
		// Two wide characters announced, one byte in the first chunk and
		// three in the continuation.
		sst := append(le32(1), le32(1)...)
		sst = append(sst, le16(2)...)
		sst = append(sst, 1, 'a')
		st := buildStream(bookOpts{cells: std(nil), rawGlobals: [][]byte{rec(recSST, sst), rec(recContinue, []byte{1, 0, 'b'})}})
		done := make(chan error, 1)
		go func() { _, err := parseWorkbook(st); done <- err }()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("accepted a split UTF-16 unit")
			}
		case <-time.After(time.Second):
			t.Fatal("reader did not return")
		}
	})
}
