package integration

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// Positive controls: inputs a working image turns into an ok record. They
// prove the hostile refusals are not a broken poppler or a broken sandbox.
const (
	varnaExcerpt   = "../../upstream/rzi/testdata/varna-2026-excerpt.txt"
	burgasControl  = "../../upstream/rzi/testdata/burgas-control.xls"
	fontSize       = 8
	courierAdvance = 0.6 * fontSize // Courier is 600/1000 em wide
	leading        = 10
)

// textPDF lays text out on one page in Courier, one word per Td at its rune
// column, so pdftotext -layout gives back the same columns. Non-ASCII runes
// get codes from 128 up, named in /Differences and mapped in /ToUnicode.
func textPDF(text string) ([]byte, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\f", ""), "\n")
	codes, order, err := codeBook(lines)
	if err != nil {
		return nil, err
	}
	width, height := 1300, leading*(len(lines)+4)
	var c bytes.Buffer
	fmt.Fprintf(&c, "BT /F1 %d Tf\n", fontSize)
	for i, l := range lines {
		y := height - leading*(i+2)
		for _, w := range words([]rune(l)) {
			fmt.Fprintf(&c, "1 0 0 1 %.1f %d Tm <", 20+float64(w.col)*courierAdvance, y)
			for _, r := range w.runes {
				fmt.Fprintf(&c, "%02X", codes[r])
			}
			c.WriteString("> Tj\n")
		}
	}
	c.WriteString("ET")

	var diff, cmap strings.Builder
	for _, r := range order {
		if r >= 0x80 {
			fmt.Fprintf(&diff, " %d /uni%04X", codes[r], r)
		}
		fmt.Fprintf(&cmap, "<%02X> <%04X>\n", codes[r], r)
	}
	toUnicode := "/CIDInit /ProcSet findresource begin 12 dict begin begincmap\n" +
		"/CMapName /Kanarche-UCS def /CMapType 2 def\n" +
		"1 begincodespacerange <00> <FF> endcodespacerange\n" +
		fmt.Sprintf("%d beginbfchar\n%sendbfchar\n", len(order), cmap.String()) +
		"endcmap CMapName currentdict /CMap defineresource pop end end"
	widths := strings.TrimSpace(strings.Repeat("600 ", 256))
	return buildPDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [3 0 R] /Count 1 /MediaBox [0 0 %d %d] >>", width, height),
		"<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		stream("", c.Bytes()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Courier /FirstChar 0 /LastChar 255 /Widths [" + widths + "]" +
			" /Encoding << /Type /Encoding /BaseEncoding /WinAnsiEncoding /Differences [" + diff.String() + " ] >>" +
			" /ToUnicode 6 0 R >>",
		stream("", []byte(toUnicode)),
	}, ""), nil
}

// codeBook gives ASCII runes their own code and the rest 128 up, in rune
// order so the output is deterministic.
func codeBook(lines []string) (map[rune]int, []rune, error) {
	seen := map[rune]bool{}
	for _, l := range lines {
		for _, r := range l {
			if r != ' ' {
				seen[r] = true
			}
		}
	}
	order := make([]rune, 0, len(seen))
	for r := range seen {
		order = append(order, r)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	codes, next := map[rune]int{}, 0x80
	for _, r := range order {
		switch {
		case r > 0x20 && r < 0x7F:
			codes[r] = int(r)
		case next > 0xFF:
			return nil, nil, fmt.Errorf("more than 128 non-ASCII runes")
		case r > 0xFFFF:
			return nil, nil, fmt.Errorf("rune %U outside the BMP", r)
		default:
			codes[r] = next
			next++
		}
	}
	return codes, order, nil
}

// word is a run of non-space runes starting at rune column col.
type word struct {
	col   int
	runes []rune
}

func words(l []rune) []word {
	var out []word
	for i := 0; i < len(l); {
		if l[i] == ' ' {
			i++
			continue
		}
		j := i
		for j < len(l) && l[j] != ' ' {
			j++
		}
		out = append(out, word{i, l[i:j]})
		i = j
	}
	return out
}

// varnaControl is the Varna test excerpt as a PDF.
func varnaControl(t *testing.T) ([]byte, string) {
	t.Helper()
	raw, err := os.ReadFile(varnaExcerpt)
	if err != nil {
		t.Fatal(err)
	}
	data, err := textPDF(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return data, string(raw)
}

func TestTextPDFIsDeterministicAndSmall(t *testing.T) {
	a, _ := varnaControl(t)
	b, _ := varnaControl(t)
	if !bytes.Equal(a, b) {
		t.Fatal("textPDF output differs between runs")
	}
	if !bytes.HasPrefix(a, []byte("%PDF-")) || len(a) > 1<<20 {
		t.Fatalf("varna control: %d bytes", len(a))
	}
	for _, bad := range []string{"/Encrypt", "/JavaScript", "/JS"} {
		if bytes.Contains(a, []byte(bad)) {
			t.Errorf("varna control contains %s", bad)
		}
	}
}

func TestCodeBookCoversEveryRune(t *testing.T) {
	lines := []string{"Пункт № 03002", "MPN/100cm3 (x)"}
	codes, order, err := codeBook(lines)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		for _, r := range l {
			if _, ok := codes[r]; r != ' ' && !ok {
				t.Errorf("rune %q has no code", r)
			}
		}
	}
	if codes['M'] != 'M' || codes['П'] < 0x80 || len(order) != len(codes) {
		t.Errorf("codes %v", codes)
	}
}

func TestBurgasControlIsCommitted(t *testing.T) {
	d, err := os.ReadFile(burgasControl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(d, []byte("\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1")) || len(d) > 64<<10 {
		t.Fatalf("burgas control: %d bytes", len(d))
	}
}
