//go:build rzicorpus

package varna

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// corpusText reads pdftotext output of the full files, made outside Go:
//
//	pdftotext -layout -enc UTF-8 mvodi-2026.pdf mvodi-2026.txt
//
// The files are never committed (decision D3).
func corpusText(t *testing.T, name string) string {
	t.Helper()
	dir := os.Getenv("KANARCHE_RZI_CORPUS")
	if dir == "" {
		dir = "/tmp/airbg-verify/rzi"
	}
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Skipf("corpus file missing: %v", err)
	}
	return string(b)
}

func TestCorpusVarna2026(t *testing.T) {
	f := Parse(corpusText(t, "mvodi-2026.txt"))
	if f.Error != "" || len(f.Warnings) != 0 {
		t.Fatalf("error %q warnings %v", f.Error, f.Warnings)
	}
	if len(f.Blocks) != 23 {
		t.Fatalf("%d points parsed, want 23", len(f.Blocks))
	}
	rows := 0
	for i, b := range f.Blocks {
		if want := fmt.Sprintf("030%02d", i+1); b.Point != want {
			t.Errorf("block %d is point %s, want %s", i, b.Point, want)
		}
		if len(b.Rows) < 9 {
			t.Errorf("point %s has %d rows", b.Point, len(b.Rows))
		}
		rows += len(b.Rows)
	}
	t.Logf("23 points, %d rows", rows)
	b, _ := findBlock(f, "03011")
	var found bool
	for _, r := range b.Rows {
		found = found || (r.IE.Value == 4124 && r.EC.Value == 34659)
	}
	if !found {
		t.Error("03011 lost the 4 124 / 34 659 row")
	}
}

// The 2025 file has a "Sal" header without a dot and rows with no results.
// The grammar does not know them and must refuse, not guess.
func TestCorpusVarna2025(t *testing.T) {
	f := Parse(corpusText(t, "mvodi-2025.txt"))
	if len(f.Warnings) == 0 {
		t.Fatal("unknown 2025 shapes parsed without a warning")
	}
	if _, ok := findBlock(f, "03007"); ok {
		t.Fatal("03007 kept despite the unknown header")
	}
	t.Logf("%d points kept, %d warnings", len(f.Blocks), len(f.Warnings))
}
