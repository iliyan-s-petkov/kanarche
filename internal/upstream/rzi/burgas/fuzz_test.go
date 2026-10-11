package burgas

import (
	"encoding/json"
	"testing"
	"time"

	"kanarche.eu/internal/upstream/rzi"
)

// FuzzBurgas feeds arbitrary bytes both as a whole file and as the Workbook
// stream inside a valid container, so mutations reach the BIFF reader.
func FuzzBurgas(f *testing.F) {
	good := buildStream(bookOpts{cells: std([]trow{
		ok(46160, 46160, 15, 15),
		{s(0, 0, "02.06.2026г."), s(0, 1, "02.06.2026г."), s(0, 2, "<10"), n(0, 3, 12), "x"},
	}), splitStr: 3, splitChar: 2})
	f.Add(buildCFB(good, cfbOpts{}))
	f.Add(good)
	f.Add(buildCFB(good, cfbOpts{fatMut: func(fat []uint32) { fat[2] = 2 }}))
	f.Add(buildCFB(good, cfbOpts{dirSize: MaxStreamBytes + 1}))
	f.Add(buildCFB(good, cfbOpts{})[:600])
	f.Add(buildCFB(buildStream(bookOpts{cells: std(nil), sheets: MaxSheets}), cfbOpts{}))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		start := time.Now()
		defer func() {
			if d := time.Since(start); d > 2*time.Second {
				t.Errorf("input took %v", d)
			}
		}()
		// Direct calls run without Extract's recover, so a panic fails here.
		checkChains(data)
		parseWorkbook(data)
		check(t, Extract(data))
		check(t, Extract(buildCFB(data, cfbOpts{})))
	})
}

// check holds the invariants every result must keep.
func check(t *testing.T, f rzi.File) {
	t.Helper()
	if f.Schema != rzi.SchemaVersion || f.Source != rzi.SourceBurgas || f.Blocks == nil || f.Warnings == nil {
		t.Fatalf("bad envelope: %+v", f)
	}
	if f.Error != "" && len(f.Blocks) != 0 {
		t.Fatal("error record carries blocks")
	}
	raw, err := json.Marshal(f)
	if err != nil || len(raw) > rzi.MaxRecordBytes {
		t.Fatalf("marshal: %v, %d bytes", err, len(raw))
	}
	// Validate must not panic whatever the extractor produced.
	rzi.Validate(f, sites, rzi.LastGood{}, time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC))
}
