//go:build rzicorpus

package rzi

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// corpusDir holds the full real files. They are never committed (D3).
func corpusDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("KANARCHE_RZI_CORPUS")
	if dir == "" {
		dir = "/tmp/airbg-verify/rzi"
	}
	return dir
}

// TestCorpusPDFMagic checks the 2026 Varna file is present and is a PDF.
func TestCorpusPDFMagic(t *testing.T) {
	path := filepath.Join(corpusDir(t), "mvodi-2026.pdf")
	head, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("corpus file missing: %v", err)
	}
	if !bytes.HasPrefix(head, []byte("%PDF-")) {
		t.Fatalf("%s: missing %%PDF- magic", path)
	}
}
