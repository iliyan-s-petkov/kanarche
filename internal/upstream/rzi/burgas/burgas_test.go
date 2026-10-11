package burgas

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/store"
	"kanarche.eu/internal/upstream/rzi"
)

var now = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

// 02001 near Obzor: N 42 49 14, E 27 53 07.
var sites = []store.BathingSite{
	{ID: "BG3410610135002001", Lat: 42.82056, Lon: 27.88528},
	{ID: "BG3410610135002002", Lat: 42.81556, Lon: 27.88667},
}

func std(rows []trow) []tcell {
	return blockCells(0, "02001", `N 42° 49' 14"`, `E 27° 53' 07"`, rows)
}

func ok(plan, date, ie, ec float64) trow {
	return trow{n(0, 0, plan), n(0, 1, date), n(0, 2, ie), n(0, 3, ec), ""}
}

func extract(t *testing.T, o bookOpts) rzi.File {
	t.Helper()
	return Extract(buildCFB(buildStream(o), cfbOpts{}))
}

// roundTrip proves the record decodes under the contract and validates.
func roundTrip(t *testing.T, f rzi.File) ([]rzi.Sample, rzi.Status) {
	t.Helper()
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	got, err := rzi.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	samples, _, status := rzi.Validate(got, sites, rzi.LastGood{}, now)
	return samples, status
}

func TestExtractSyntheticBlock(t *testing.T) {
	rows := []trow{
		ok(46160, 46160, 15, 15),
		// Moved sample: planned date differs from the actual date.
		ok(46174, 46177, 77, 46),
		{}, {}, // empty trailing rows
	}
	f := extract(t, bookOpts{cells: std(rows)})
	if f.Error != "" || len(f.Warnings) != 0 {
		t.Fatalf("error %q warnings %v", f.Error, f.Warnings)
	}
	if len(f.Blocks) != 1 || f.Blocks[0].Point != "02001" || len(f.Blocks[0].Rows) != 2 {
		t.Fatalf("blocks: %+v", f.Blocks)
	}
	b := f.Blocks[0]
	if b.Lat < 42.8205 || b.Lat > 42.8206 || b.Lon < 27.8852 || b.Lon > 27.8854 {
		t.Errorf("coords %v %v", b.Lat, b.Lon)
	}
	want := rzi.Row{Planned: "2026-06-01", Date: "2026-06-04", IE: rzi.Count{Op: "=", Value: 77},
		EC: rzi.Count{Op: "=", Value: 46}, Unit: "MPN/100ml"}
	if b.Rows[1] != want {
		t.Errorf("row 1 = %+v, want %+v", b.Rows[1], want)
	}
	if b.Rows[0].Date != "2026-05-18" {
		t.Errorf("serial 46160 = %s", b.Rows[0].Date)
	}
	samples, status := roundTrip(t, f)
	if status != rzi.StatusOK || len(samples) != 2 {
		t.Fatalf("status %s samples %d", status, len(samples))
	}
}

func TestTextDateAndCensoring(t *testing.T) {
	rows := []trow{
		{s(0, 0, "02.06.2026г."), s(0, 1, "02.06.2026г."), s(0, 2, "<10"), s(0, 3, "< 15"), ""},
		{n(0, 0, 46174), s(0, 1, "14.8.2026 г."), s(0, 2, "30"), n(0, 3, 12), "извън мониторинг"},
	}
	f := extract(t, bookOpts{cells: std(rows)})
	if len(f.Warnings) != 0 || f.Error != "" {
		t.Fatalf("error %q warnings %v", f.Error, f.Warnings)
	}
	r := f.Blocks[0].Rows
	if r[0].IE != (rzi.Count{Op: "<", Value: 10}) || r[0].EC != (rzi.Count{Op: "<", Value: 15}) {
		t.Errorf("censored counts: %+v %+v", r[0].IE, r[0].EC)
	}
	if r[0].Date != "2026-06-02" || r[1].Date != "2026-08-14" {
		t.Errorf("text dates: %s %s", r[0].Date, r[1].Date)
	}
	if r[1].IE.Op != "=" || r[1].IE.Value != 30 || r[1].Remarks != "извън мониторинг" {
		t.Errorf("row 1: %+v", r[1])
	}
	if _, status := roundTrip(t, f); status != rzi.StatusOK {
		t.Errorf("status %s", status)
	}
}

func TestBadDateIsRowWarning(t *testing.T) {
	rows := []trow{
		ok(46160, 46160, 15, 15),
		{n(0, 0, 46174), s(0, 1, "32.06.2026г."), n(0, 2, 15), n(0, 3, 15), ""},
		{n(0, 0, 46188), s(0, 1, "2026-06=16"), n(0, 2, 15), n(0, 3, 15), ""},
	}
	f := extract(t, bookOpts{cells: std(rows)})
	if f.Error != "" || len(f.Blocks) != 1 || len(f.Blocks[0].Rows) != 1 || len(f.Warnings) != 2 {
		t.Fatalf("error %q rows %d warnings %v", f.Error, len(f.Blocks[0].Rows), f.Warnings)
	}
	for _, w := range f.Warnings {
		if strings.Contains(w, "2026") {
			t.Errorf("warning carries document text: %q", w)
		}
	}
	// Warnings make the validator hold the whole file for review.
	if _, status := roundTrip(t, f); status != rzi.StatusNeedsReview {
		t.Errorf("status %s", status)
	}
}

func TestRKAndDate1904(t *testing.T) {
	rows := []trow{{rk(0, 0, 46160-1462), rk(0, 1, 46160-1462), rk(0, 2, 15), rk(0, 3, 25), ""}}
	f := extract(t, bookOpts{cells: std(rows), date1904: true})
	if f.Error != "" || len(f.Blocks) != 1 {
		t.Fatalf("error %q", f.Error)
	}
	r := f.Blocks[0].Rows[0]
	if r.Date != "2026-05-18" || r.IE.Value != 15 || r.EC.Value != 25 {
		t.Errorf("row %+v", r)
	}
}

func TestSSTContinuation(t *testing.T) {
	rows := []trow{ok(46160, 46160, 15, 15)}
	cells := std(rows)
	// Cut inside the characters of each string in turn.
	for cut := 1; cut < 8; cut++ {
		for _, latin := range []bool{false, true} {
			f := extract(t, bookOpts{cells: cells, splitStr: cut, splitChar: 3, latin1: latin})
			if f.Error != "" || len(f.Blocks) != 1 || len(f.Warnings) != 0 {
				t.Fatalf("cut %d latin %v: error %q blocks %d warnings %v", cut, latin, f.Error, len(f.Blocks), f.Warnings)
			}
			if f.Blocks[0].Name != "Тестов плаж" {
				t.Errorf("cut %d latin %v: name %q", cut, latin, f.Blocks[0].Name)
			}
		}
	}
}

func TestSeveralBlocks(t *testing.T) {
	cells := std([]trow{ok(46160, 46160, 15, 15)})
	cells = append(cells, blockCells(20, "02002", `N 42° 48' 56"`, `E 27° 53' 12"`,
		[]trow{ok(46160, 46160, 30, 30)})...)
	f := extract(t, bookOpts{cells: cells})
	if f.Error != "" || len(f.Blocks) != 2 || f.Blocks[1].Point != "02002" {
		t.Fatalf("error %q blocks %+v", f.Error, f.Blocks)
	}
	if samples, status := roundTrip(t, f); status != rzi.StatusOK || len(samples) != 2 {
		t.Fatalf("status %s samples %d", status, len(samples))
	}
}

func TestSanitisedRefusals(t *testing.T) {
	// A workbook with no block labels is refused with a fixed text.
	f := extract(t, bookOpts{cells: []tcell{s(0, 0, "something else")}})
	if f.Error == "" || len(f.Blocks) != 0 {
		t.Fatalf("expected refusal, got %+v", f)
	}
	if _, status := roundTrip(t, f); status != rzi.StatusRejected {
		t.Errorf("status %s", status)
	}
}

// I8: a FAT that loops must end in an error, quickly.
func TestFATCycleTerminates(t *testing.T) {
	stream := buildStream(bookOpts{cells: std([]trow{ok(46160, 46160, 15, 15)})})
	cyc := func(fat []uint32) {
		// Last stream sector points back to the first one.
		last := 0
		for i, v := range fat {
			if v == sectEnd && i > 1 {
				last = i
			}
		}
		fat[last] = 2 // first stream sector is 2 with one FAT sector
	}
	data := buildCFB(stream, cfbOpts{fatMut: cyc})
	done := make(chan rzi.File, 1)
	go func() { done <- Extract(data) }()
	select {
	case f := <-done:
		if f.Error != "corrupt sector chain" {
			t.Fatalf("error %q", f.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("extract did not return within 1 s")
	}
	_, err := workbookStream(data)
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("err = %v, want ErrCycle", err)
	}
}

func TestSelfLoopAndOutOfRangeChain(t *testing.T) {
	stream := buildStream(bookOpts{cells: std([]trow{ok(46160, 46160, 15, 15)})})
	self := func(fat []uint32) { fat[2] = 2 }
	if _, err := workbookStream(buildCFB(stream, cfbOpts{fatMut: self})); !errors.Is(err, ErrCycle) {
		t.Errorf("self loop: %v", err)
	}
	far := func(fat []uint32) { fat[3] = 100000 }
	if _, err := workbookStream(buildCFB(stream, cfbOpts{fatMut: far})); !errors.Is(err, ErrChain) {
		t.Errorf("out of range: %v", err)
	}
}

func TestCaps(t *testing.T) {
	base := std([]trow{ok(46160, 46160, 15, 15)})
	cases := []struct {
		name string
		data func() []byte
		want string
	}{
		{"input over cap", func() []byte { return make([]byte, MaxInputBytes+1) }, "input over size cap"},
		{"declared stream over cap", func() []byte {
			return buildCFB(buildStream(bookOpts{cells: base}), cfbOpts{dirSize: MaxStreamBytes + 1})
		}, "not a readable BIFF8 workbook"},
		{"too many sheets", func() []byte {
			return buildCFB(buildStream(bookOpts{cells: base, sheets: MaxSheets}), cfbOpts{})
		}, "not a readable BIFF8 workbook"},
		{"too many records", func() []byte {
			return buildCFB(buildStream(bookOpts{cells: base, extraRecs: MaxRecords}), cfbOpts{})
		}, "not a readable BIFF8 workbook"},
		{"too many cells", func() []byte {
			var cs []tcell
			for i := 0; i <= MaxCells/200; i++ {
				for c := 0; c < 200; c++ {
					cs = append(cs, n(i, c, 1))
				}
			}
			return buildCFB(buildStream(bookOpts{cells: cs}), cfbOpts{})
		}, "not a readable BIFF8 workbook"},
		{"string over cap", func() []byte {
			cs := append([]tcell{s(40, 0, strings.Repeat("x", MaxStringChars+1))}, base...)
			return buildCFB(buildStream(bookOpts{cells: cs}), cfbOpts{})
		}, "not a readable BIFF8 workbook"},
		{"huge declared SST", func() []byte {
			st := buildStream(bookOpts{cells: base})
			// SST record follows the globals BOF (20 bytes); unique count at +8.
			i := bytes.Index(st, []byte{0xFC, 0x00})
			copy(st[i+8:], []byte{0xFF, 0xFF, 0xFF, 0x7F})
			return buildCFB(st, cfbOpts{})
		}, "not a readable BIFF8 workbook"},
		{"sst chars over cap", func() []byte {
			var cs []tcell
			for i := 0; i < 600; i++ {
				cs = append(cs, s(i, 0, strings.Repeat(string(rune('a'+i%26)), MaxStringChars-10)+fmt.Sprintf("%010d", i)))
			}
			return buildCFB(buildStream(bookOpts{cells: cs, latin1: true}), cfbOpts{})
		}, "not a readable BIFF8 workbook"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := Extract(c.data())
			if f.Error != c.want || len(f.Blocks) != 0 {
				t.Fatalf("error %q, want %q", f.Error, c.want)
			}
		})
	}
}

func TestTooManyBlocksAndRows(t *testing.T) {
	var cs []tcell
	for i := 0; i <= rzi.MaxBlocks; i++ {
		cs = append(cs, s(i, 0, "ПУНКТ ЗА ВЗЕМАНЕ НА ПРОБИ №"))
	}
	if f := Extract(buildCFB(buildStream(bookOpts{cells: cs}), cfbOpts{})); f.Error != "document over a structural cap" {
		t.Errorf("blocks: %q", f.Error)
	}
	rows := make([]trow, rzi.MaxRowsPerBlock+1)
	for i := range rows {
		rows[i] = ok(46160, 46160, 15, 15)
	}
	if f := extract(t, bookOpts{cells: std(rows)}); f.Error != "document over a structural cap" {
		t.Errorf("rows: %q", f.Error)
	}
}

func TestTruncationNeverPanics(t *testing.T) {
	data := buildCFB(buildStream(bookOpts{cells: std([]trow{ok(46160, 46160, 15, 15)})}), cfbOpts{})
	for n := 0; n < len(data); n += 7 {
		f := Extract(data[:n])
		if f.Error == "" {
			t.Fatalf("truncated to %d bytes was accepted", n)
		}
	}
	// A record whose length field runs past the end of the stream.
	st := buildStream(bookOpts{cells: std([]trow{ok(46160, 46160, 15, 15)})})
	for i := 0; i+4 < len(st); i += 3 {
		mut := bytes.Clone(st)
		mut[i+2], mut[i+3] = 0xFF, 0x1F
		Extract(buildCFB(mut, cfbOpts{}))
	}
}

func TestNotAnXLS(t *testing.T) {
	for _, in := range [][]byte{nil, []byte("PK\x03\x04"), bytes.Repeat([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, 100)} {
		if f := Extract(in); f.Error == "" {
			t.Errorf("accepted %d bytes", len(in))
		}
	}
}

// Opt-in: RZI_BURGAS_CORPUS is a directory of real Burgas files. The real
// files are not committed (decision D3).
func TestRealCorpus(t *testing.T) {
	dir := os.Getenv("RZI_BURGAS_CORPUS")
	if dir == "" {
		t.Skip("RZI_BURGAS_CORPUS not set")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.xls"))
	for _, p := range files {
		if strings.HasPrefix(filepath.Base(p), "kategoria") {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f := Extract(data)
		t.Logf("%s: error %q blocks %d warnings %d", filepath.Base(p), f.Error, len(f.Blocks), len(f.Warnings))
		// 02042 has a coordinate typo in the 2025 and 2026 files.
		if f.Error != "" || len(f.Blocks) < 48 {
			t.Errorf("%s: error %q blocks %d", p, f.Error, len(f.Blocks))
		}
		for _, w := range f.Warnings {
			t.Logf("  warning: %s", w)
		}
	}
}
