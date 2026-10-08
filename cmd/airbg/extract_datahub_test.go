package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/upstream/bathing/datahub"
	"kanarche.eu/internal/xlsx/xlsxtest"
)

// extractFixture builds a workbook with 60 BG sites in 2024 and 2025 (the
// newest-season floor is 50) plus other countries, and a config pinning it.
type extractFixture struct {
	dir, file, cfg string
}

func newExtractFixture(t *testing.T) extractFixture {
	t.Helper()
	rows := [][]string{{"bathingWaterIdentifier", "countryCode", "quality", "season"}}
	for i := 0; i < 60; i++ {
		id := fmt.Sprintf("BG%016d", i)
		for _, s := range []string{"2024", "2025"} {
			rows = append(rows, []string{id, "BG", "1 - Excellent", s})
		}
		rows = append(rows, []string{fmt.Sprintf("DK%016d", i), "DK", "2 - Good", "2025"})
	}
	data, err := xlsxtest.TableBook(rows)
	if err != nil {
		t.Fatal(err)
	}
	return pinFixture(t, data)
}

func pinFixture(t *testing.T, data []byte) extractFixture {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "in.xlsx")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return extractFixture{dir: dir, file: file, cfg: writePinnedConfig(t, dir, hex.EncodeToString(sum[:]), int64(len(data)))}
}

// writePinnedConfig copies the committed airbg.yaml with other pins.
func writePinnedConfig(t *testing.T, dir, sha string, size int64) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "airbg.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	old := "    sha256: \"39a54c81bcbd30f8770327d2dc271cf68f3c90955d0fca98c3c6890bcfd683b3\"\n    size: 43096327\n"
	if !strings.Contains(s, old) {
		t.Fatal("airbg.yaml pins moved, update the fixture")
	}
	s = strings.Replace(s, old, fmt.Sprintf("    sha256: %q\n    size: %d\n", sha, size), 1)
	path := filepath.Join(dir, "airbg.yaml")
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// run points the config at fx and clears every database variable.
func (fx extractFixture) run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	t.Setenv(config.PathEnv, fx.cfg)
	t.Setenv(config.DatabaseURLEnv, "")
	t.Setenv(config.DatabaseURLFileEnv, "")
	var out, errOut bytes.Buffer
	code = runExtractDatahub(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestExtractNeedsNoDatabase(t *testing.T) {
	fx := newExtractFixture(t)
	out := filepath.Join(fx.dir, "bg.json")
	code, _, stderr := fx.run(t, "--file", fx.file, "--out", out)
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	var f datahub.File
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Classes) != 120 || f.Header.Country != "BG" || f.Header.SHA256 == "" || f.Header.Size == 0 {
		t.Fatalf("header=%+v classes=%d", f.Header, len(f.Classes))
	}
	for _, c := range f.Classes {
		if !strings.HasPrefix(c.SiteID, "BG") {
			t.Fatalf("non-BG class %v in the snapshot", c)
		}
	}
}

func TestExtractIsByteIdenticalAcrossRuns(t *testing.T) {
	fx := newExtractFixture(t)
	var first []byte
	for i := 0; i < 2; i++ {
		out := filepath.Join(fx.dir, "bg"+strconv.Itoa(i)+".json")
		if code, _, stderr := fx.run(t, "--file", fx.file, "--out", out); code != 0 {
			t.Fatalf("run %d exit %d:\n%s", i, code, stderr)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = data
		} else if !bytes.Equal(first, data) {
			t.Fatal("second run differs from the first")
		}
	}
}

// The wrong file is refused before its bytes are parsed: a body that is not a
// zip must be reported as a pin mismatch, not as a zip error.
func TestExtractRefusesWrongShaBeforeParsing(t *testing.T) {
	fx := newExtractFixture(t)
	bad := filepath.Join(fx.dir, "bad.xlsx")
	st, _ := os.Stat(fx.file)
	if err := os.WriteFile(bad, bytes.Repeat([]byte("x"), int(st.Size())), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(fx.dir, "bg.json")
	code, _, stderr := fx.run(t, "--file", bad, "--out", out)
	if code == 0 {
		t.Fatal("wrong file accepted")
	}
	if !strings.Contains(stderr, "sha256 differs from the pin") {
		t.Errorf("stderr = %q, want the sha256 pin error", stderr)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("output written on failure")
	}
}

func TestExtractRefusesWrongSize(t *testing.T) {
	fx := newExtractFixture(t)
	data, _ := os.ReadFile(fx.file)
	sum := sha256.Sum256(data)
	fx.cfg = writePinnedConfig(t, fx.dir, hex.EncodeToString(sum[:]), int64(len(data))+1)
	out := filepath.Join(fx.dir, "bg.json")
	code, _, stderr := fx.run(t, "--file", fx.file, "--out", out)
	if code == 0 || !strings.Contains(stderr, "size differs from the pin") {
		t.Fatalf("exit %d, stderr = %q, want the size pin error", code, stderr)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("output written on failure")
	}
}

// A file that passes the pins but fails validation leaves no snapshot behind.
func TestExtractWritesNothingWhenParseFails(t *testing.T) {
	data, err := xlsxtest.TableBook([][]string{
		{"bathingWaterIdentifier", "countryCode", "quality", "season"},
		{"DK0000000000000001", "DK", "1 - Excellent", "2025"},
	})
	if err != nil {
		t.Fatal(err)
	}
	fx := pinFixture(t, data)
	out := filepath.Join(fx.dir, "bg.json")
	if code, _, _ := fx.run(t, "--file", fx.file, "--out", out); code == 0 {
		t.Fatal("a file with no BG rows was accepted")
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("output written on failure")
	}
}

func TestExtractNeedsExactlyOneSource(t *testing.T) {
	fx := newExtractFixture(t)
	if code, _, _ := fx.run(t); code == 0 {
		t.Error("no source accepted")
	}
	if code, _, _ := fx.run(t, "--file", fx.file, "--fetch"); code == 0 {
		t.Error("both sources accepted")
	}
}
