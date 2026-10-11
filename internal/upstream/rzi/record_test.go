package rzi

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func readFixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "outbox", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDecodeValidFixture(t *testing.T) {
	f, err := Decode(bytes.NewReader(readFixture(t, "varna-valid.json")))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Schema != SchemaVersion || f.Source != SourceVarna || len(f.Blocks) != 3 {
		t.Fatalf("unexpected file: %+v", f)
	}
	r := f.Blocks[1].Rows[0]
	if r.IE != (Count{Op: "=", Value: 4124}) || r.EC != (Count{Op: "=", Value: 34659}) {
		t.Fatalf("row values: %+v", r)
	}
}

func TestDecodeRejectedFixture(t *testing.T) {
	f, err := Decode(bytes.NewReader(readFixture(t, "rejected.json")))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Error != "magic mismatch" {
		t.Fatalf("Error = %q", f.Error)
	}
}

// mutateFixture swaps one exact substring in the valid fixture.
func mutateFixture(t *testing.T, old, repl string) string {
	t.Helper()
	s := string(readFixture(t, "varna-valid.json"))
	if !strings.Contains(s, old) {
		t.Fatalf("fixture lacks %q", old)
	}
	return strings.Replace(s, old, repl, 1)
}

func TestDecodeRejectsHostileInput(t *testing.T) {
	big := `{"schema":1,"source":"rzi_varna","warnings":["` + strings.Repeat("a", MaxRecordBytes) + `"]}`
	exact := `{"schema":1}`
	exact += strings.Repeat(" ", MaxRecordBytes-len(exact))
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"empty", ``, ErrRecord},
		{"not an object", `[1,2]`, ErrRecord},
		{"wrong schema version", mutateFixture(t, `"schema": 1`, `"schema": 2`), ErrSchema},
		{"missing schema", mutateFixture(t, `"schema": 1,`, ``), ErrSchema},
		{"schema as string", mutateFixture(t, `"schema": 1`, `"schema": "1"`), ErrRecord},
		{"unknown top field", mutateFixture(t, `"schema": 1,`, `"schema": 1, "status": "ok",`), ErrRecord},
		{"unknown row field", mutateFixture(t, `"unit": "MPN/100ml", "remarks": "след`, `"unit": "MPN/100ml", "html": "<b>", "remarks": "след`), ErrRecord},
		{"unknown count field", mutateFixture(t, `{"op": "=", "value": 4124}`, `{"op": "=", "value": 4124, "raw": "4 124"}`), ErrRecord},
		{"case variant key", mutateFixture(t, `"source": "rzi_varna"`, `"Source": "rzi_varna"`), ErrRecord},
		{"duplicate key", mutateFixture(t, `"source": "rzi_varna",`, `"source": "rzi_varna", "source": "rzi_burgas",`), ErrRecord},
		{"duplicate key in row", mutateFixture(t, `"ie": {"op": "=", "value": 4124}`, `"ie": {"op": "=", "value": 4124}, "ie": {"op": "=", "value": 1}`), ErrRecord},
		{"fractional count", mutateFixture(t, `"value": 4124`, `"value": 4124.5`), ErrRecord},
		{"exponent count", mutateFixture(t, `"value": 4124`, `"value": 4e3`), ErrRecord},
		{"overflow count", mutateFixture(t, `"value": 4124`, `"value": 99999999999999999999`), ErrRecord},
		{"NaN count", mutateFixture(t, `"value": 4124`, `"value": NaN`), ErrRecord},
		{"trailing value", string(readFixture(t, "varna-valid.json")) + `{}`, ErrRecord},
		{"trailing garbage", string(readFixture(t, "varna-valid.json")) + `x`, ErrRecord},
		{"truncated", string(readFixture(t, "varna-valid.json"))[:200], ErrRecord},
		{"too deep", `{"schema":1,"blocks":[{"rows":[{"ie":{"op":[[[[[[[[[]]]]]]]]]}}]}]}`, errTooDeep},
		{"oversized", big, ErrTooLarge},
		{"cap plus one", exact + " ", ErrTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(tc.in))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestDecodeAcceptsExactCap pins the cap boundary: MaxRecordBytes is allowed.
func TestDecodeAcceptsExactCap(t *testing.T) {
	in := `{"schema":1}`
	in += strings.Repeat(" ", MaxRecordBytes-len(in))
	if _, err := Decode(strings.NewReader(in)); err != nil {
		t.Fatalf("Decode at cap: %v", err)
	}
}

// TestSampleJSONOmitsRemarks keeps inspector remarks out of any JSON the
// API could build from a Sample (decision D5).
func TestSampleJSONOmitsRemarks(t *testing.T) {
	b, err := json.Marshal(Sample{SiteID: "x", Remarks: "след пороен дъжд"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("пороен")) || bytes.Contains(bytes.ToLower(b), []byte("remarks")) {
		t.Fatalf("remarks leaked into JSON: %s", b)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add(readFixture(f, "varna-valid.json"))
	f.Add(readFixture(f, "rejected.json"))
	f.Add([]byte(`{"schema":1,"blocks":[{"rows":[{}]}]}`))
	sites := testSites()
	now := testNow()
	f.Fuzz(func(t *testing.T, in []byte) {
		got, err := Decode(bytes.NewReader(in))
		if err != nil {
			return
		}
		// A decoded file re-encodes and decodes to the same value.
		b, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		again, err := Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("re-decode: %v\n%s", err, b)
		}
		if !reflect.DeepEqual(normalize(got), normalize(again)) {
			t.Fatalf("round trip differs:\n%+v\n%+v", got, again)
		}
		// Validate is total and fails closed on whatever decodes.
		acc, rej, st := Validate(got, sites, LastGood{}, now)
		if st != StatusOK && len(acc) != 0 {
			t.Fatalf("status %s with %d accepted rows", st, len(acc))
		}
		if st == StatusOK && len(rej) != 0 {
			t.Fatalf("ok with %d rejects", len(rej))
		}
		if len(rej) > MaxRejects {
			t.Fatalf("%d rejects over cap", len(rej))
		}
	})
}

// normalize maps nil and empty slices to one form for comparison.
func normalize(f File) File {
	if len(f.Blocks) == 0 {
		f.Blocks = nil
	}
	if len(f.Warnings) == 0 {
		f.Warnings = nil
	}
	for i := range f.Blocks {
		if len(f.Blocks[i].Rows) == 0 {
			f.Blocks[i].Rows = nil
		}
	}
	return f
}
