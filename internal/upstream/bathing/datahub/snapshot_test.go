package datahub

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"airbg.org/internal/xlsx"
	"airbg.org/internal/xlsx/xlsxtest"
)

var testHeader = Header{
	SourceURL: "https://sdi.eea.europa.eu/x.xlsx",
	SHA256:    strings.Repeat("a", 64),
	Size:      10,
	Edition:   "2025 v1.0",
	Published: "2026-06-02",
	Licence:   "CC BY 4.0",
	Country:   "BG",
}

func TestSnapshotStableOrder(t *testing.T) {
	a := []Class{
		{"BG2", 2024, "good"}, {"BG1", 2025, "excellent"}, {"BG2", 2023, "poor"}, {"BG1", 2024, "good"},
	}
	b := []Class{a[3], a[2], a[1], a[0]}
	ea, err := NewFile(testHeader, Snapshot{Classes: a}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	eb, err := NewFile(testHeader, Snapshot{Classes: b}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ea, eb) {
		t.Fatalf("input order changed the output:\n%s\n%s", ea, eb)
	}
	var f File
	if err := json.Unmarshal(ea, &f); err != nil {
		t.Fatal(err)
	}
	want := []Class{{"BG1", 2024, "good"}, {"BG1", 2025, "excellent"}, {"BG2", 2023, "poor"}, {"BG2", 2024, "good"}}
	for i, c := range want {
		if f.Classes[i] != c {
			t.Fatalf("class %d = %v, want %v", i, f.Classes[i], c)
		}
	}
	if f.Header != testHeader {
		t.Errorf("header = %+v, want %+v", f.Header, testHeader)
	}
}

func TestEncodeLeavesInputUntouched(t *testing.T) {
	in := []Class{{"BG2", 2024, "good"}, {"BG1", 2025, "excellent"}}
	if _, err := NewFile(testHeader, Snapshot{Classes: in}).Encode(); err != nil {
		t.Fatal(err)
	}
	if in[0].SiteID != "BG2" {
		t.Error("Encode sorted the caller's slice")
	}
}

func TestEncodeOneClassPerLine(t *testing.T) {
	out, err := NewFile(testHeader, Snapshot{Classes: []Class{{"BG1", 2024, "good"}, {"BG1", 2025, "good"}}}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if out[len(out)-1] != '\n' || strings.Count(string(out), `"site_id"`) != 2 {
		t.Fatalf("unexpected layout:\n%s", out)
	}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, `"site_id"`) && !strings.HasSuffix(strings.TrimSuffix(l, ","), "}") {
			t.Errorf("class split over lines: %q", l)
		}
	}
}

func writeFile(t *testing.T, data []byte) (path, sum string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "x.xlsx")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(data)
	return path, hex.EncodeToString(h[:])
}

func TestVerifyFileAcceptsPinnedFile(t *testing.T) {
	path, sum := writeFile(t, []byte("PK\x03\x04body"))
	m, err := VerifyFile(path, sum, 8)
	if err != nil || m.SHA256 != sum || m.Size != 8 || m.Path != path {
		t.Fatalf("VerifyFile = %+v, %v", m, err)
	}
}

func TestVerifyFileRejectsWrongSHA(t *testing.T) {
	path, _ := writeFile(t, []byte("PK\x03\x04body"))
	_, err := VerifyFile(path, strings.Repeat("0", 64), 8)
	if !errors.Is(err, ErrSHA256) {
		t.Fatalf("err = %v, want ErrSHA256", err)
	}
}

func TestVerifyFileRejectsWrongSize(t *testing.T) {
	path, sum := writeFile(t, []byte("PK\x03\x04body"))
	_, err := VerifyFile(path, sum, 9)
	if !errors.Is(err, ErrSize) {
		t.Fatalf("err = %v, want ErrSize", err)
	}
}

func TestVerifyFileRejectsBadPin(t *testing.T) {
	path, _ := writeFile(t, []byte("PK\x03\x04body"))
	if _, err := VerifyFile(path, "ABC", 8); !errors.Is(err, ErrBadPin) {
		t.Fatalf("err = %v, want ErrBadPin", err)
	}
	if _, err := VerifyFile(path, strings.Repeat("0", 64), 0); !errors.Is(err, ErrBadLimits) {
		t.Fatalf("size 0: err = %v, want ErrBadLimits", err)
	}
}

func TestVerifyFileMissing(t *testing.T) {
	if _, err := VerifyFile(filepath.Join(t.TempDir(), "none"), strings.Repeat("0", 64), 1); err == nil {
		t.Fatal("missing file accepted")
	}
}

// A source book for sampling: BG rows over several seasons, plus other countries.
func sampleSource(t *testing.T) *xlsx.Book {
	t.Helper()
	cols := []string{"bathingWaterIdentifier", "countryCode", "lat", "quality", "season"}
	rows := [][]string{cols}
	for i := 0; i < 10; i++ {
		cc := "DK"
		if i%2 == 0 {
			cc = "BG"
		}
		rows = append(rows, []string{"X" + string(rune('A'+i)), cc, "42.5", "1 - Excellent", "2024"})
	}
	rows = append(rows, []string{"BG0000000000000001", "BG", "42.5", "2 - Good", "2019"})
	for i := 0; i < 4; i++ {
		rows = append(rows, []string{"Y" + string(rune('A'+i)), "BG", "42.5", "1 - Excellent", "2025"})
	}
	data, err := xlsxtest.TableBook(rows)
	if err != nil {
		t.Fatal(err)
	}
	b, err := xlsx.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestEmitSampleCutsByCountryAndSeason(t *testing.T) {
	var out bytes.Buffer
	if err := EmitSample(sampleSource(t), "BG", SampleOptions{MinSeason: 2023, MaxPerSeason: 3, MaxOther: 2}, &out); err != nil {
		t.Fatal(err)
	}
	b, err := xlsx.Open(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var bg, other int
	err = b.Rows(b.Sheets()[0], []string{"countryCode", "season"}, func(r xlsx.Row) error {
		if r["countryCode"] == "BG" {
			bg++
			if r["season"] < "2023" {
				t.Errorf("old season %s kept", r["season"])
			}
		} else {
			other++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if bg != 6 || other != 2 {
		t.Fatalf("bg=%d other=%d, want 6 (3 per season) and 2", bg, other)
	}
}

func TestEmitSampleDeterministic(t *testing.T) {
	var a, b bytes.Buffer
	opt := SampleOptions{MinSeason: 2023, MaxPerSeason: 3, MaxOther: 2}
	if err := EmitSample(sampleSource(t), "BG", opt, &a); err != nil {
		t.Fatal(err)
	}
	if err := EmitSample(sampleSource(t), "BG", opt, &b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("two cuts of the same book differ")
	}
}

// The committed file is the canonical encoding of its own contents, so a
// regenerated snapshot differs only by real data changes.
func TestCommittedSnapshotIsCanonical(t *testing.T) {
	raw, err := os.ReadFile("bg.json")
	if err != nil {
		t.Fatal(err)
	}
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	again, err := NewFile(f.Header, Snapshot{Classes: f.Classes}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, again) {
		t.Error("bg.json is not in canonical form")
	}
	n2025 := 0
	for _, c := range f.Classes {
		if c.Season == 2025 {
			n2025++
		}
	}
	if len(f.Classes) != 1795 || n2025 != 96 {
		t.Errorf("classes=%d in 2025=%d, want 1795 and 96", len(f.Classes), n2025)
	}
}
