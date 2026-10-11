package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kanarche.eu/internal/docparse/pdf"
	"kanarche.eu/internal/upstream/rzi"
)

// fakeRunner stands in for pdfinfo and pdftotext.
type fakeRunner struct {
	info    string
	text    []byte
	textErr error
	calls   []string
}

func (f *fakeRunner) Run(_ context.Context, argv []string, _ int) ([]byte, error) {
	f.calls = append(f.calls, argv[0])
	if argv[0] == "pdfinfo" {
		return []byte(f.info), nil
	}
	return f.text, f.textErr
}

const okInfo = "Encrypted:      no\nJavaScript:     no\nPages:          3\n"

func excerpt(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "internal", "upstream", "rzi", "testdata", "varna-2026-excerpt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func sum(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

type env struct {
	t       *testing.T
	in, out string
}

func newEnv(t *testing.T) env {
	return env{t: t, in: t.TempDir(), out: t.TempDir()}
}

// put writes data as <sha256>.<ext> and returns the sha.
func (e env) put(data []byte, ext string) string {
	e.t.Helper()
	h := sum(data)
	if err := os.WriteFile(filepath.Join(e.in, h+"."+ext), data, 0o644); err != nil {
		e.t.Fatal(err)
	}
	return h
}

func (e env) record(h string) rzi.File {
	e.t.Helper()
	b, err := os.ReadFile(filepath.Join(e.out, h+".json"))
	if err != nil {
		e.t.Fatal(err)
	}
	f, err := rzi.Decode(bytes.NewReader(b))
	if err != nil {
		e.t.Fatalf("record does not decode: %v", err)
	}
	return f
}

func (e env) outNames() []string {
	ents, _ := os.ReadDir(e.out)
	var n []string
	for _, d := range ents {
		n = append(n, d.Name())
	}
	return n
}

func pdfBytes(tail string) []byte { return []byte("%PDF-1.7\n" + tail) }

func pdfKinds(f *fakeRunner) map[string]kind {
	return newKinds(pdf.Tools{Runner: f})
}

func TestPDFExtractsVarnaRecord(t *testing.T) {
	e := newEnv(t)
	h := e.put(pdfBytes("a"), "pdf")
	f := &fakeRunner{info: okInfo, text: excerpt(t)}
	if code := process(e.in, e.out, pdfKinds(f)); code != 0 {
		t.Fatalf("exit %d", code)
	}
	rec := e.record(h)
	if rec.Error != "" || len(rec.Blocks) == 0 {
		t.Fatalf("error %q blocks %d", rec.Error, len(rec.Blocks))
	}
	if rec.SHA256 != h || rec.Source != rzi.SourceVarna || rec.ExtractorVersion != varnaVersion || rec.Schema != 1 {
		t.Fatalf("header %+v", rec)
	}
	if got := e.outNames(); len(got) != 1 || got[0] != h+".json" {
		t.Fatalf("out = %v", got)
	}
}

func TestSkipsUnvalidatedNames(t *testing.T) {
	e := newEnv(t)
	h := sum([]byte("x"))
	for _, n := range []string{
		"readme.pdf",
		strings.ToUpper(h) + ".pdf",
		h + ".txt",
		h + ".pdf.bak",
		h[:63] + ".pdf",
		"-" + h[1:] + ".pdf",
	} {
		if err := os.WriteFile(filepath.Join(e.in, n), pdfBytes("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(e.in, sum([]byte("d"))+".pdf"), 0o755); err != nil {
		t.Fatal(err)
	}
	f := &fakeRunner{info: okInfo, text: excerpt(t)}
	if code := process(e.in, e.out, pdfKinds(f)); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := e.outNames(); len(got) != 0 || len(f.calls) != 0 {
		t.Fatalf("out %v, tool calls %v", got, f.calls)
	}
}

func TestSkipsSymlinks(t *testing.T) {
	e := newEnv(t)
	target := filepath.Join(t.TempDir(), "t")
	if err := os.WriteFile(target, pdfBytes("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(e.in, sum([]byte("y"))+".pdf")); err != nil {
		t.Skip("no symlinks")
	}
	f := &fakeRunner{info: okInfo, text: excerpt(t)}
	process(e.in, e.out, pdfKinds(f))
	if len(e.outNames()) != 0 || len(f.calls) != 0 {
		t.Fatal("symlink was processed")
	}
}

func TestMagicMismatchRejected(t *testing.T) {
	cases := map[string][]byte{
		"pdf":  []byte("<html>not a pdf"),
		"xls":  []byte("%PDF-1.7 in an xls"),
		"xlsx": []byte("%PDF-1.7 in an xlsx"),
	}
	for ext, data := range cases {
		e := newEnv(t)
		h := e.put(data, ext)
		f := &fakeRunner{info: okInfo, text: excerpt(t)}
		if code := process(e.in, e.out, pdfKinds(f)); code != 0 {
			t.Fatalf("%s: exit %d", ext, code)
		}
		rec := e.record(h)
		if rec.Error != "magic mismatch" || len(rec.Blocks) != 0 {
			t.Fatalf("%s: %+v", ext, rec)
		}
		if len(f.calls) != 0 {
			t.Fatalf("%s: tool ran", ext)
		}
	}
}

// I9 at unit level: the gate verdict becomes a rejected record.
func TestPDFGateRejects(t *testing.T) {
	cases := map[string]string{
		"encrypted":  "Encrypted:      yes\nJavaScript:     no\nPages:          3\n",
		"javascript": "Encrypted:      no\nJavaScript:     yes\nPages:          3\n",
		"pages41":    "Encrypted:      no\nJavaScript:     no\nPages:          41\n",
	}
	for name, info := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			h := e.put(pdfBytes(name), "pdf")
			f := &fakeRunner{info: info, text: excerpt(t)}
			if code := process(e.in, e.out, pdfKinds(f)); code != 0 {
				t.Fatalf("exit %d", code)
			}
			rec := e.record(h)
			if rec.Error == "" || len(rec.Blocks) != 0 {
				t.Fatalf("not rejected: %+v", rec)
			}
			for _, c := range f.calls {
				if c == "pdftotext" {
					t.Fatal("pdftotext ran")
				}
			}
		})
	}
}

func TestPDFTextCapRejected(t *testing.T) {
	e := newEnv(t)
	h := e.put(pdfBytes("cap"), "pdf")
	f := &fakeRunner{info: okInfo, textErr: pdf.ErrOutputCap}
	process(e.in, e.out, pdfKinds(f))
	if got := e.record(h).Error; got != pdf.ErrTextCap.Error() {
		t.Fatalf("error %q", got)
	}
}

func TestXLSXUnsupportedFormat(t *testing.T) {
	e := newEnv(t)
	h := e.put([]byte("PK\x03\x04rest"), "xlsx")
	if code := process(e.in, e.out, pdfKinds(&fakeRunner{})); code != 0 {
		t.Fatalf("exit %d", code)
	}
	rec := e.record(h)
	if rec.Error != "unsupported format" || rec.Source != rzi.SourceDobrich {
		t.Fatalf("%+v", rec)
	}
}

func TestXLSGarbageGetsRecord(t *testing.T) {
	e := newEnv(t)
	h := e.put(append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, make([]byte, 64)...), "xls")
	if code := process(e.in, e.out, pdfKinds(&fakeRunner{})); code != 0 {
		t.Fatalf("exit %d", code)
	}
	rec := e.record(h)
	if rec.Error == "" || rec.Source != rzi.SourceBurgas || rec.SHA256 != h {
		t.Fatalf("%+v", rec)
	}
}

func TestOversizeInputRejected(t *testing.T) {
	e := newEnv(t)
	h := e.put(pdfBytes(strings.Repeat("a", maxInputBytes)), "pdf")
	f := &fakeRunner{info: okInfo, text: excerpt(t)}
	process(e.in, e.out, pdfKinds(f))
	if got := e.record(h).Error; got != "input over size cap" {
		t.Fatalf("error %q", got)
	}
	if len(f.calls) != 0 {
		t.Fatal("tool ran")
	}
}

func TestNameHashMismatchRejected(t *testing.T) {
	e := newEnv(t)
	name := sum([]byte("something else"))
	if err := os.WriteFile(filepath.Join(e.in, name+".pdf"), pdfBytes("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := &fakeRunner{info: okInfo, text: excerpt(t)}
	process(e.in, e.out, pdfKinds(f))
	if got := e.record(name).Error; got != "name does not match content hash" {
		t.Fatalf("error %q", got)
	}
}

// bigRecord is an extractor result that marshals to over 2 MB.
func bigRecord(data []byte) rzi.File {
	return rzi.File{
		Schema: rzi.SchemaVersion, Source: rzi.SourceBurgas, SHA256: sum(data), ExtractorVersion: "test/1",
		Blocks:   []rzi.Block{},
		Warnings: []string{strings.Repeat("w", 3<<20)},
	}
}

// I10: a record over 2 MB is replaced by a rejected record.
func TestOutputOverCapRejected(t *testing.T) {
	e := newEnv(t)
	data := append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, 'z')
	h := e.put(data, "xls")
	ks := pdfKinds(&fakeRunner{})
	k := ks["xls"]
	k.extract = func(_ context.Context, _ string, data []byte) rzi.File { return bigRecord(data) }
	ks["xls"] = k
	if code := process(e.in, e.out, ks); code != 0 {
		t.Fatalf("exit %d", code)
	}
	st, err := os.Stat(filepath.Join(e.out, h+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() > rzi.MaxRecordBytes {
		t.Fatalf("output is %d bytes", st.Size())
	}
	if got := e.record(h).Error; got != "output over size cap" {
		t.Fatalf("error %q", got)
	}
}

func TestExtractorPanicRejected(t *testing.T) {
	e := newEnv(t)
	h := e.put(pdfBytes("p"), "pdf")
	ks := pdfKinds(&fakeRunner{})
	k := ks["pdf"]
	k.extract = func(context.Context, string, []byte) rzi.File { panic("boom secret") }
	ks["pdf"] = k
	if code := process(e.in, e.out, ks); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := e.record(h).Error; got != "internal extractor error" {
		t.Fatalf("error %q", got)
	}
}

func TestRecordIsWrittenViaTmpRename(t *testing.T) {
	e := newEnv(t)
	e.put(pdfBytes("t"), "pdf")
	process(e.in, e.out, pdfKinds(&fakeRunner{info: okInfo, text: excerpt(t)}))
	for _, n := range e.outNames() {
		if strings.HasSuffix(n, ".tmp") {
			t.Fatalf("leftover %s", n)
		}
	}
}

func TestExitOneWhenWriteFails(t *testing.T) {
	e := newEnv(t)
	e.put(pdfBytes("w"), "pdf")
	missing := filepath.Join(e.out, "no", "such", "dir")
	if code := process(e.in, missing, pdfKinds(&fakeRunner{info: okInfo, text: excerpt(t)})); code != 1 {
		t.Fatalf("exit %d", code)
	}
}

func TestExitOneWhenInputDirMissing(t *testing.T) {
	e := newEnv(t)
	if code := process(filepath.Join(e.in, "nope"), e.out, pdfKinds(&fakeRunner{})); code != 1 {
		t.Fatalf("exit %d", code)
	}
}

func TestRejectedRecordsDoNotLeakContent(t *testing.T) {
	e := newEnv(t)
	h := e.put(pdfBytes("TOPSECRET"), "pdf")
	f := &fakeRunner{info: "Encrypted:      yes\nTitle: TOPSECRET\n"}
	process(e.in, e.out, pdfKinds(f))
	b, _ := os.ReadFile(filepath.Join(e.out, h+".json"))
	if bytes.Contains(b, []byte("TOPSECRET")) {
		t.Fatal("record leaks document content")
	}
}
