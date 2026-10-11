package integration

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// These run without Docker: they pin the shape of each generated fixture so
// the container tests cannot pass on an input that lost its hostile part.

func corpusByName(t *testing.T) map[string]fixture {
	t.Helper()
	m := map[string]fixture{}
	for _, f := range hostileCorpus() {
		if _, dup := m[f.name]; dup {
			t.Fatalf("duplicate fixture %s", f.name)
		}
		m[f.name] = f
	}
	return m
}

func TestCorpusIsTheTask8List(t *testing.T) {
	want := []string{
		"xref-loop.pdf", "deep-nesting.pdf", "encrypted.pdf", "js-action.pdf",
		"1000-pages.pdf", "huge-stream.pdf", "not-a-pdf.pdf", "zip-bomb.xlsx",
		"xml-depth.xlsx", "fat-cycle.xls", "5mb-plus-one.pdf",
	}
	got := corpusByName(t)
	for _, n := range want {
		if _, ok := got[n]; !ok {
			t.Errorf("missing fixture %s", n)
		}
	}
	if len(got) != len(want) {
		t.Errorf("corpus has %d fixtures, want %d", len(got), len(want))
	}
}

func TestCorpusIsDeterministic(t *testing.T) {
	a, b := hostileCorpus(), buildCorpus()
	for i := range a {
		if !bytes.Equal(a[i].data, b[i].data) {
			t.Errorf("%s differs between runs", a[i].name)
		}
	}
}

func TestFixtureMagicAndExt(t *testing.T) {
	magic := map[string]string{"pdf": "%PDF-", "xls": "\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1", "xlsx": "PK\x03\x04"}
	for _, f := range hostileCorpus() {
		if !strings.HasSuffix(f.name, "."+f.ext) {
			t.Errorf("%s: ext %q", f.name, f.ext)
		}
		has := bytes.HasPrefix(f.data, []byte(magic[f.ext]))
		if f.name == "not-a-pdf.pdf" {
			if has {
				t.Errorf("%s starts with the PDF magic", f.name)
			}
			continue
		}
		if !has {
			t.Errorf("%s lacks the %s magic", f.name, f.ext)
		}
	}
}

func TestFixtureSizes(t *testing.T) {
	for _, f := range hostileCorpus() {
		switch f.name {
		case "5mb-plus-one.pdf":
			if len(f.data) != 5<<20+1 {
				t.Errorf("%s is %d bytes", f.name, len(f.data))
			}
		default:
			if len(f.data) > 1<<20 {
				t.Errorf("%s is %d bytes; hostile inputs here stay small", f.name, len(f.data))
			}
		}
	}
}

func TestPDFFixturesCarryTheirFeature(t *testing.T) {
	c := corpusByName(t)
	for name, needle := range map[string]string{
		"encrypted.pdf":    "/Encrypt",
		"js-action.pdf":    "/JavaScript",
		"1000-pages.pdf":   "/Count 1000",
		"huge-stream.pdf":  "/FlateDecode",
		"xref-loop.pdf":    "/Prev",
		"deep-nesting.pdf": strings.Repeat("[", 1000),
	} {
		if !bytes.Contains(c[name].data, []byte(needle)) {
			t.Errorf("%s lacks %q", name, needle)
		}
	}
	if n := bytes.Count(c["1000-pages.pdf"].data, []byte("/Type /Page ")); n != 1000 {
		t.Errorf("1000-pages.pdf has %d page objects", n)
	}
}

// The control must reach pdftotext: no gate trigger, one page.
func TestControlPDFPassesTheGate(t *testing.T) {
	c := controlPDF()
	if !bytes.HasPrefix(c.data, []byte("%PDF-")) || c.want != refNoBlocks {
		t.Fatalf("control fixture: want %q", c.want)
	}
	for _, bad := range []string{"/Encrypt", "/JavaScript", "/JS"} {
		if bytes.Contains(c.data, []byte(bad)) {
			t.Errorf("control.pdf contains %s", bad)
		}
	}
	if !bytes.Contains(c.data, []byte("/Count 1 ")) {
		t.Error("control.pdf is not one page")
	}
}

func TestZipBombInflatesFarPastCaps(t *testing.T) {
	f := corpusByName(t)["zip-bomb.xlsx"]
	zr, err := zip.NewReader(bytes.NewReader(f.data), int64(len(f.data)))
	if err != nil {
		t.Fatal(err)
	}
	var total uint64
	for _, e := range zr.File {
		total += e.UncompressedSize64
	}
	if total < 64<<20 {
		t.Fatalf("zip-bomb inflates to %d bytes", total)
	}
	if ratio := total / uint64(len(f.data)); ratio < 100 {
		t.Fatalf("ratio %d is under the xlsx cap", ratio)
	}
}

func TestXMLDepthIsDeep(t *testing.T) {
	f := corpusByName(t)["xml-depth.xlsx"]
	zr, err := zip.NewReader(bytes.NewReader(f.data), int64(len(f.data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range zr.File {
		if e.Name == "xl/workbook.xml" && e.UncompressedSize64 > 100000*3 {
			return
		}
	}
	t.Fatal("no deep xl/workbook.xml")
}

func TestFATCycleSelfLinks(t *testing.T) {
	d := corpusByName(t)["fat-cycle.xls"].data
	// FAT sector is sector 0 at file offset 512; entry 2 must point at 2.
	if len(d) < 1024 || le32(d[512+8:]) != 2 {
		t.Fatal("FAT entry 2 does not loop to itself")
	}
}

func le32(b []byte) uint32 { return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24 }
