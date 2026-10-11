package varna

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kanarche.eu/internal/upstream/rzi"
)

func fixture(t testing.TB, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// mutate replaces the first occurrence of old and fails if there is none.
func mutate(t testing.TB, s, old, repl string) string {
	t.Helper()
	if !strings.Contains(s, old) {
		t.Fatalf("fixture has no %q", old)
	}
	return strings.Replace(s, old, repl, 1)
}

func findBlock(f rzi.File, point string) (rzi.Block, bool) {
	for _, b := range f.Blocks {
		if b.Point == point {
			return b, true
		}
	}
	return rzi.Block{}, false
}

// mustDrop asserts the point was refused with a warning and no record error.
func mustDrop(t *testing.T, f rzi.File, point string) {
	t.Helper()
	if f.Error != "" && len(f.Blocks) != 0 {
		t.Fatalf("refused record carries blocks: %s", f.Error)
	}
	if len(f.Warnings) == 0 {
		t.Fatal("no warning")
	}
	if _, ok := findBlock(f, point); ok {
		t.Fatalf("block %s kept despite warnings %v", point, f.Warnings)
	}
}

// cellAt places s at rune column col.
type cellAt struct {
	col int
	s   string
}

func line(cells ...cellAt) string {
	var out []rune
	for _, c := range cells {
		for len(out) < c.col {
			out = append(out, ' ')
		}
		out = append(out, []rune(c.s)...)
	}
	return string(out)
}

// Column layout of the 03002 header in the 2026 excerpt.
const (
	colPlanned = 0
	colDate    = 23
	colIE      = 44
	colEC      = 57
	colRemark  = 69
)

func row(planned, date, ie, ec, rest string) string {
	return line(cellAt{colPlanned, planned}, cellAt{colDate, date}, cellAt{colIE, ie}, cellAt{colEC, ec}, cellAt{colRemark, rest})
}

// head is the 03002 block header of the 2026 excerpt, up to and including
// the unit line, with the point number replaced.
func head(t testing.TB, point string) string {
	t.Helper()
	lines := strings.Split(fixture(t, "varna-2026-excerpt.txt"), "\n")
	start := -1
	for i, l := range lines {
		if strings.Contains(l, "№ 03002") {
			start = i
		}
		if start >= 0 && strings.Contains(l, "MPN/100cm3 MPN/100cm3") {
			h := strings.Join(lines[start:i+1], "\n") + "\n"
			return strings.Replace(h, "03002", point, 1)
		}
	}
	t.Fatal("no 03002 header in fixture")
	return ""
}

func synth(t testing.TB, point string, rows ...string) string {
	return head(t, point) + strings.Join(rows, "\n") + "\n"
}

func TestParseExcerpt2026(t *testing.T) {
	f := Parse(fixture(t, "varna-2026-excerpt.txt"))
	if f.Error != "" || len(f.Warnings) != 0 {
		t.Fatalf("error %q warnings %v", f.Error, f.Warnings)
	}
	if f.Schema != rzi.SchemaVersion || f.Source != rzi.SourceVarna {
		t.Fatalf("envelope %+v", f)
	}
	if len(f.Blocks) != 2 {
		t.Fatalf("%d blocks, want 2", len(f.Blocks))
	}
	b := f.Blocks[0]
	if b.Point != "03002" || b.Name != `Казино "Морско око", Златни пясъци (Морско казино)` {
		t.Fatalf("block 0 = %q %q", b.Point, b.Name)
	}
	if !near(b.Lat, 43+17.0/60+23.0/3600) || !near(b.Lon, 28+2.0/60+49.0/3600) {
		t.Fatalf("coordinates %v %v", b.Lat, b.Lon)
	}
	if len(b.Rows) != 9 {
		t.Fatalf("%d rows, want 9", len(b.Rows))
	}
	want := rzi.Row{Planned: "2026-05-27", Date: "2026-05-27", IE: rzi.Count{Op: "<", Value: 15}, EC: rzi.Count{Op: "<", Value: 15}, Unit: "MPN/100ml"}
	if b.Rows[0] != want {
		t.Fatalf("row 0 = %+v", b.Rows[0])
	}
	r := b.Rows[6]
	if r.Date != "2026-08-18" || r.IE != (rzi.Count{Op: "=", Value: 92}) || r.EC != (rzi.Count{Op: "=", Value: 266}) {
		t.Fatalf("row 6 = %+v", r)
	}
	if r := b.Rows[4]; r.IE.Op != "<" || r.EC != (rzi.Count{Op: "=", Value: 15}) {
		t.Fatalf("row 4 mixes censored and plain: %+v", r)
	}
}

func TestParseThousandsByColumn(t *testing.T) {
	f := Parse(fixture(t, "varna-2026-excerpt.txt"))
	b, ok := findBlock(f, "03011")
	if !ok {
		t.Fatal("no 03011")
	}
	if len(b.Rows) != 14 {
		t.Fatalf("%d rows, want 14", len(b.Rows))
	}
	r := b.Rows[2]
	if r.Date != "2026-06-16" || r.IE.Value != 4124 || r.EC.Value != 34659 || r.IE.Op != "=" {
		t.Fatalf("thousands row = %+v", r)
	}
	if r.Remarks != "след пороен дъжд Издадени 3 броя предписания за поставяне на табели в зоната за къпане" {
		t.Fatalf("remarks = %q", r.Remarks)
	}
}

func TestParseExtraAndReplacementRows(t *testing.T) {
	f := Parse(fixture(t, "varna-2026-excerpt.txt"))
	b, _ := findBlock(f, "03011")
	extra := b.Rows[3]
	if extra.Planned != "" || extra.Date != "2026-06-19" || extra.IE.Value != 30 || extra.EC.Value != 61 ||
		extra.Remarks != "за да се потвърди, че инцидентът е приключил (до 72ч.)" {
		t.Fatalf("extra row = %+v", extra)
	}
	repl := b.Rows[5]
	if repl.Planned != "" || repl.Date != "2026-06-26" || repl.IE.Value != 30 || repl.EC.Value != 30 ||
		repl.Remarks != "7 дни след края на краткотрайното замърсяване" {
		t.Fatalf("replacement row = %+v", repl)
	}
	moved := b.Rows[7]
	if moved.Planned != "2026-07-14" || moved.Date != "2026-07-17" ||
		moved.Remarks != "пробата е изместена поради лоши метеорологични условия" {
		t.Fatalf("moved row = %+v", moved)
	}
	if b.Rows[1].Remarks != "отсъства отсъства" {
		t.Fatalf("trailing columns = %q", b.Rows[1].Remarks)
	}
}

func TestParseExcerpt2025(t *testing.T) {
	f := Parse(fixture(t, "varna-2025-excerpt.txt"))
	if f.Error != "" || len(f.Warnings) != 0 || len(f.Blocks) != 1 {
		t.Fatalf("error %q warnings %v blocks %d", f.Error, f.Warnings, len(f.Blocks))
	}
	b := f.Blocks[0]
	if b.Point != "03001" || b.Name != "Панорама под ПСОВ Златни пясъци" {
		t.Fatalf("block = %q %q", b.Point, b.Name)
	}
	if !near(b.Lat, 43+18.0/60+12.0/3600) || !near(b.Lon, 28+3.0/60+12.0/3600) {
		t.Fatalf("degree-as-0 coordinates %v %v", b.Lat, b.Lon)
	}
	if len(b.Rows) != 9 {
		t.Fatalf("%d rows", len(b.Rows))
	}
	r := b.Rows[1]
	if r.IE != (rzi.Count{Op: "=", Value: 30}) || r.EC != (rzi.Count{Op: "<", Value: 15}) || r.Unit != "MPN/100ml" {
		t.Fatalf("row 1 = %+v", r)
	}
	if r := b.Rows[3]; r.IE.Op != "<" || r.EC != (rzi.Count{Op: "=", Value: 15}) {
		t.Fatalf("row 3 = %+v", r)
	}
}

func TestParseRoundTripsThroughContract(t *testing.T) {
	f := Parse(fixture(t, "varna-2026-excerpt.txt"))
	f.SHA256 = strings.Repeat("a", 64)
	f.ExtractorVersion = "rzi-extract/1"
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	got, err := rzi.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Blocks) != 2 || got.Blocks[1].Rows[2].EC.Value != 34659 {
		t.Fatalf("round trip lost data: %+v", got.Blocks)
	}
}

func near(a, b float64) bool {
	d := a - b
	return d < 1e-9 && d > -1e-9
}

func TestPointHeaderForms(t *testing.T) {
	for _, hdr := range []string{"№03002", "№.03002", "№ 03002", "№. 03002"} {
		txt := mutate(t, fixture(t, "varna-2026-excerpt.txt"), "№ 03002", hdr)
		f := Parse(txt)
		if _, ok := findBlock(f, "03002"); !ok || len(f.Warnings) != 0 {
			t.Errorf("%q: warnings %v", hdr, f.Warnings)
		}
	}
}

func TestCoordinateForms(t *testing.T) {
	cases := []struct {
		in       string
		lat, lon float64
	}{
		{`N .43 17'23'' . дължина E 28002'49''`, 43 + 17.0/60 + 23.0/3600, 28 + 2.0/60 + 49.0/3600},
		{`N 43 16'11'' . дължина E .28002'21''.`, 43 + 16.0/60 + 11.0/3600, 28 + 2.0/60 + 21.0/3600},
		{`N .43014'34''. дължина E .28001'10''.`, 43 + 14.0/60 + 34.0/3600, 28 + 1.0/60 + 10.0/3600},
		{`N .42057'29''. дължина E .27053'56''.`, 42 + 57.0/60 + 29.0/3600, 27 + 53.0/60 + 56.0/3600},
		{`N 43 12'31''.. дължина E 27056'20''`, 43 + 12.0/60 + 31.0/3600, 27 + 56.0/60 + 20.0/3600},
	}
	for _, c := range cases {
		txt := mutate(t, fixture(t, "varna-2026-excerpt.txt"), `N .43 17'23'' . дължина E 28002'49''`, c.in)
		b, ok := findBlock(Parse(txt), "03002")
		if !ok || !near(b.Lat, c.lat) || !near(b.Lon, c.lon) {
			t.Errorf("%q = %v %v ok=%v", c.in, b.Lat, b.Lon, ok)
		}
	}
}

func TestCoordinatesFailClosed(t *testing.T) {
	for _, bad := range []string{
		`N .43 17'23'' . дължина E 28002'61''`, // seconds
		`N .43 77'23'' . дължина E 28002'49''`, // minutes
		`N .43 17'23'' . дължина E 38002'49''`, // longitude out of range
		`N .53 17'23'' . дължина E 28002'49''`, // latitude out of range
		`N .43 17'23 . дължина E 28002'49''`,   // missing quote
		`N .4317'23'' . дължина E 28002'49''`,  // no degree separator
		`N .43 17.5'23'' . дължина E 28002'49''`,
		`E .43 17'23'' . дължина N 28002'49''`, // swapped axes
	} {
		txt := mutate(t, fixture(t, "varna-2026-excerpt.txt"), `N .43 17'23'' . дължина E 28002'49''`, bad)
		mustDrop(t, Parse(txt), "03002")
	}
}

func TestMissingStructureDropsBlock(t *testing.T) {
	src := fixture(t, "varna-2026-excerpt.txt")
	cases := map[string]string{
		"coordinates": "Географски координати: ширина N .43 17'23'' . дължина E 28002'49''",
		"season":      "Времетраене на сезона за къпане: от 1.06.2026 г. до 30.09.2026 г.",
		"column head": "Резултат от Резултат от",
		"unit line":   "MPN/100cm3 MPN/100cm3",
	}
	for name, cut := range cases {
		f := Parse(mutate(t, src, cut, ""))
		mustDrop(t, f, "03002")
		if _, ok := findBlock(f, "03011"); !ok {
			t.Errorf("%s: the other block was dropped too", name)
		}
	}
}

func TestSecondCoordinatesAndSeasonRefused(t *testing.T) {
	src := fixture(t, "varna-2026-excerpt.txt")
	co := "Географски координати: ширина N .43 17'23'' . дължина E 28002'49''"
	mustDrop(t, Parse(mutate(t, src, co, co+"\n"+co)), "03002")
	se := "Времетраене на сезона за къпане: от 1.06.2026 г. до 30.09.2026 г."
	mustDrop(t, Parse(mutate(t, src, se, se+"\n"+se)), "03002")
	mustDrop(t, Parse(mutate(t, src, "до 30.09.2026 г.", "до 30.09.2027 г.")), "03002")
}

func TestUnknownPointHeaderWarns(t *testing.T) {
	src := fixture(t, "varna-2026-excerpt.txt")
	for _, h := range []string{
		"Пункт за вземане на проби № 0301 Наименование X",
		"Пункт за вземане на проби № 030011 Наименование X",
		"Пункт за вземане на проби № 03О11 Наименование X",
		"Пункт за вземане на проби № 03011",
		"Пунктове за вземане на проби № 03011 Наименование X",
	} {
		txt := mutate(t, src, "Пункт за вземане на проби № 03011       Наименование Град Варна Офицерски плаж", h)
		f := Parse(txt)
		if len(f.Warnings) == 0 {
			t.Errorf("%q: no warning", h)
		}
		if _, ok := findBlock(f, "03011"); ok {
			t.Errorf("%q: block kept", h)
		}
		if _, ok := findBlock(f, "03002"); !ok {
			t.Errorf("%q: unrelated block lost", h)
		}
	}
}

func TestUnknownLineShapesWarn(t *testing.T) {
	src := fixture(t, "varna-2026-excerpt.txt")
	for name, txt := range map[string]string{
		"in header":  mutate(t, src, "Времетраене на сезона", "Нещо ново\nВреметраене на сезона"),
		"after rows": mutate(t, src, "2026-09-16             2026-09-16            61         под 15", "2026-09-16             2026-09-16            61         под 15\nЗабележка: данните са предварителни"),
		"header num": mutate(t, src, "MPN/100cm3 MPN/100cm3", "17 18\n                                         MPN/100cm3 MPN/100cm3"),
	} {
		t.Run(name, func(t *testing.T) { mustDrop(t, Parse(txt), "03002") })
	}
}

func TestIgnoredPageFurniture(t *testing.T) {
	src := fixture(t, "varna-2026-excerpt.txt")
	title := "                                              РЕЗУЛТАТИ ОТ МОНИТОРИНГА НА МОРСКИ ВОДИ ОТ ЗОНИТЕ ЗА КЪПАНЕ-РЕГИОН ВАРНА 2026 Г."
	txt := mutate(t, src, "2026-07-21             2026-07-21          под 15         15",
		"2026-07-21             2026-07-21          под 15         15\n                                                     стр.2\n\f"+title+"\n")
	f := Parse(txt)
	if len(f.Warnings) != 0 {
		t.Fatalf("furniture warned: %v", f.Warnings)
	}
	if b, _ := findBlock(f, "03002"); len(b.Rows) != 9 {
		t.Fatalf("rows lost across the page break: %d", len(b.Rows))
	}
	for _, bad := range []string{"стр.", "стр.12345", "стр.1 2", "РЕЗУЛТАТИ ОТ МОНИТОРИНГА НА ВОДИ"} {
		mustDrop(t, Parse(mutate(t, src, "2026-07-21             2026-07-21          под 15         15",
			"2026-07-21             2026-07-21          под 15         15\n"+bad)), "03002")
	}
}

func TestCountGrammar(t *testing.T) {
	ok := map[string]rzi.Count{
		"под 15":    {Op: "<", Value: 15},
		"0":         {Op: "=", Value: 0},
		"7":         {Op: "=", Value: 7},
		"4 124":     {Op: "=", Value: 4124},
		"34 659":    {Op: "=", Value: 34659},
		"1 234 567": {Op: "=", Value: 1234567},
	}
	for in, want := range ok {
		f := Parse(synth(t, "03002", row("2026-06-09", "2026-06-09", in, "10", "")))
		b, found := findBlock(f, "03002")
		if !found || b.Rows[0].IE != want {
			t.Errorf("%q = %+v (warnings %v)", in, b.Rows, f.Warnings)
		}
	}
	for _, bad := range []string{
		"<15", "< 15", "≤15", ">15", "под15", "под  15", "под 0", "под",
		"15 30", "4 12", "4 1245", "4  124", "015", "00", "+15", "-15", "1,5", "15.0", "15*", "n/a",
		"1234567890", "1 234 567 890", "под 4 12", "4 124", "１５",
	} {
		mustDrop(t, Parse(synth(t, "03002", row("2026-06-09", "2026-06-09", bad, "10", ""))), "03002")
		mustDrop(t, Parse(synth(t, "03002", row("2026-06-09", "2026-06-09", "10", bad, ""))), "03002")
	}
}

func TestRowShapeFailsClosed(t *testing.T) {
	d := "2026-06-09"
	cases := map[string][]string{
		"no ec":            {line(cellAt{0, d}, cellAt{colDate, d}, cellAt{colIE, "15"})},
		"no ie":            {line(cellAt{0, d}, cellAt{colDate, d}, cellAt{colEC, "15"})},
		"two ie":           {line(cellAt{0, d}, cellAt{colDate, d}, cellAt{colIE, "15"}, cellAt{colIE + 4, "16"}, cellAt{colEC, "15"})},
		"two ec":           {line(cellAt{0, d}, cellAt{colDate, d}, cellAt{colIE, "15"}, cellAt{colEC, "15"}, cellAt{colEC + 4, "16"})},
		"bad date":         {row("2026-06-09", "2026-02-30", "15", "15", "")},
		"short date":       {row("2026-06-09", "2026-6-9", "15", "15", "")},
		"bad planned":      {row("2026-13-09", d, "15", "15", "")},
		"year differs":     {row("2025-06-09", "2025-06-09", "15", "15", "")},
		"planned year":     {row("2025-06-09", d, "15", "15", "")},
		"unknown label":    {row("някаква проба", d, "15", "15", "")},
		"label with date":  {row("допълнителна проба 2026-06-09", d, "15", "15", "")},
		"no planned cell":  {row("", d, "15", "15", "")},
		"two planned":      {line(cellAt{0, d}, cellAt{12, d}, cellAt{colDate, d}, cellAt{colIE, "1"}, cellAt{colEC, "1"})},
		"value no date":    {row("", "", "15", "15", "")},
		"tab":              {row(d, d, "15", "15", "a\tb")},
		"zero width":       {row(d, d, "15", "15", "a​b")},
		"repl no label":    {row("(2026-06-09)", d, "15", "15", "")},
		"repl wrong date":  {line(cellAt{0, labelRepl}), row("(2026-06-10)", d, "15", "15", "")},
		"label unused":     {line(cellAt{0, labelRepl}), row(d, d, "15", "15", "")},
		"label at end":     {row(d, d, "15", "15", ""), line(cellAt{0, labelRepl})},
		"two labels":       {line(cellAt{0, labelRepl}), line(cellAt{0, labelDop}), row("(2026-06-09)", d, "15", "15", "")},
		"extra with label": {line(cellAt{0, labelRepl}), row(labelDop, d, "15", "15", "")},
		"orphan remark":    {row(d, d, "15", "15", ""), "", line(cellAt{colRemark, "стои сам"})},
		"orphan first":     {line(cellAt{colRemark, "стои сам"}), "", row(d, d, "15", "15", "")},
		"two anchors run": {
			row(d, d, "15", "15", "първи"),
			row("2026-06-10", "2026-06-10", "15", "15", "втори"),
		},
		"ec crosses remark edge": {line(cellAt{0, d}, cellAt{colDate, d}, cellAt{colIE, "15"}, cellAt{64, "34 659"})},
		"ie crosses ec edge":     {line(cellAt{0, d}, cellAt{colDate, d}, cellAt{50, "15 000"}, cellAt{colEC + 2, "30"})},
		"ie in date zone":        {line(cellAt{0, d}, cellAt{colDate, d}, cellAt{34, "15"}, cellAt{colEC, "30"})},
		"date moved right":       {line(cellAt{0, d}, cellAt{40, d}, cellAt{colIE + 10, "15"}, cellAt{colEC + 4, "30"})},
		"huge remark": {
			row(d, d, "15", "15", strings.Repeat("я", 180)),
			line(cellAt{colRemark, strings.Repeat("я", 180)}),
			line(cellAt{colRemark, strings.Repeat("я", 180)}),
		},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) { mustDrop(t, Parse(synth(t, "03002", rows...)), "03002") })
	}
	t.Run("no rows", func(t *testing.T) { mustDrop(t, Parse(synth(t, "03002")), "03002") })
}

func TestRemarkAttribution(t *testing.T) {
	d1, d2, d3 := "2026-06-09", "2026-06-23", "2026-07-07"
	at := func(s string) string { return line(cellAt{colRemark, s}) }
	f := Parse(synth(t, "03002",
		row(d1, d1, "15", "15", ""),
		at("преди"), at("още преди"),
		row(d2, d2, "30", "30", "на реда"),
		at("след"),
		"",
		row(d3, d3, "15", "15", ""),
	))
	b, ok := findBlock(f, "03002")
	if !ok || len(b.Rows) != 3 {
		t.Fatalf("warnings %v", f.Warnings)
	}
	if b.Rows[0].Remarks != "" || b.Rows[2].Remarks != "" {
		t.Fatalf("remarks leaked to neighbours: %+v", b.Rows)
	}
	if b.Rows[1].Remarks != "преди още преди на реда след" {
		t.Fatalf("remarks = %q", b.Rows[1].Remarks)
	}
}

func TestRemarkTextIsNeverInterpreted(t *testing.T) {
	for _, rem := range []string{"под 15", "2026-06-10", "<15 > 20", "1 2 3", "стр.1"} {
		d := "2026-06-09"
		f := Parse(synth(t, "03002", row(d, d, "15", "15", rem)))
		b, ok := findBlock(f, "03002")
		if !ok || b.Rows[0].IE.Value != 15 || b.Rows[0].Date != d {
			t.Errorf("%q changed the row: %+v %v", rem, b.Rows, f.Warnings)
		}
	}
}

func TestColumnDrift(t *testing.T) {
	src := fixture(t, "varna-2026-excerpt.txt")
	// Shift the second block's whole header row ten columns right.
	old := "  Планирана дата за       Дата на        Резултат от Резултат от     Допълнителна"
	if !strings.Contains(src, old) {
		t.Fatal("fixture changed")
	}
	f := Parse(mutate(t, src, old, strings.Repeat(" ", 10)+old))
	mustDrop(t, f, "03011")
	if _, ok := findBlock(f, "03002"); !ok {
		t.Fatal("first block lost")
	}
}

func TestResultColumnOrder(t *testing.T) {
	src := mutate(t, fixture(t, "varna-2026-excerpt.txt"), "чревни    Ешерихия", "Ешерихия    чревни")
	mustDrop(t, Parse(src), "03002")
}

func TestHeaderDriftBetweenWordsAndRule(t *testing.T) {
	src := fixture(t, "varna-2026-excerpt.txt")
	mustDrop(t, Parse(mutate(t, src, "Резултат от Резултат от", "Резултат от    Резултат от")), "03002")
	mustDrop(t, Parse(mutate(t, src, "Резултат от Резултат от", "Резултат от Резултат от Резултат от")), "03002")
	mustDrop(t, Parse(mutate(t, src, "Дата на ", "Дата    ")), "03002")
}

func TestUnitLine(t *testing.T) {
	src := fixture(t, "varna-2026-excerpt.txt")
	for _, u := range []string{"MPN/100cm3 MPN/100cm", "cfu/100ml cfu/100ml", "MPN/100cm3", "MPN/100cm3 MPN/100cm3 MPN/100cm3"} {
		mustDrop(t, Parse(mutate(t, src, "MPN/100cm3 MPN/100cm3", u)), "03002")
	}
}

func TestWarningsDoNotEchoDocumentText(t *testing.T) {
	secret := "ТАЙНА-ВЪВ-ВАРНА"
	d := "2026-06-09"
	f := Parse(synth(t, "03002", row(secret, d, "15", "15", ""), row(d, d, secret, "15", "")))
	if len(f.Warnings) == 0 {
		t.Fatal("no warnings")
	}
	for _, w := range f.Warnings {
		if strings.Contains(w, secret) || strings.Contains(w, "Казино") {
			t.Fatalf("warning echoes document text: %q", w)
		}
	}
}

func TestCaps(t *testing.T) {
	src := fixture(t, "varna-2026-excerpt.txt")
	t.Run("input size", func(t *testing.T) {
		f := Parse(src + strings.Repeat(" ", MaxInputBytes))
		if f.Error == "" || len(f.Blocks) != 0 {
			t.Fatalf("oversize input accepted: %+v", f.Error)
		}
	})
	t.Run("at size cap", func(t *testing.T) {
		pad := MaxInputBytes - len(src)
		f := Parse(src + strings.Repeat(strings.Repeat(" ", 100)+"\n", pad/101) + strings.Repeat(" ", pad%101))
		if f.Error != "" {
			t.Fatalf("input at the byte cap refused: %s", f.Error)
		}
	})
	t.Run("line length", func(t *testing.T) {
		f := Parse(src + strings.Repeat("я", MaxLineRunes+1) + "\n")
		if f.Error == "" {
			t.Fatal("long line accepted")
		}
	})
	t.Run("line length at cap", func(t *testing.T) {
		f := Parse(src + strings.Repeat("\f", 3) + strings.Repeat(" ", MaxLineRunes) + "\n")
		if f.Error != "" {
			t.Fatalf("line at the cap refused: %s", f.Error)
		}
	})
	t.Run("line count", func(t *testing.T) {
		f := Parse(strings.Repeat("\n", MaxLines) + src)
		if f.Error == "" {
			t.Fatal("too many lines accepted")
		}
	})
	t.Run("invalid utf8", func(t *testing.T) {
		if f := Parse(src + "\xff\xfe\n"); f.Error == "" {
			t.Fatal("invalid UTF-8 accepted")
		}
	})
	t.Run("rows per block", func(t *testing.T) {
		var rows []string
		for i := 0; i <= rzi.MaxRowsPerBlock; i++ {
			day := fmt.Sprintf("2026-07-%02d", i%28+1)
			rows = append(rows, row(day, day, "15", "15", ""))
		}
		mustDrop(t, Parse(synth(t, "03002", rows...)), "03002")
		f := Parse(synth(t, "03002", rows[:rzi.MaxRowsPerBlock]...))
		if b, ok := findBlock(f, "03002"); !ok || len(b.Rows) != rzi.MaxRowsPerBlock {
			t.Fatalf("rows at the cap refused: %v", f.Warnings)
		}
	})
	t.Run("blocks", func(t *testing.T) {
		var sb strings.Builder
		for i := 0; i <= rzi.MaxBlocks; i++ {
			sb.WriteString(synth(t, fmt.Sprintf("%05d", i), row("2026-06-09", "2026-06-09", "15", "15", "")))
		}
		f := Parse(sb.String())
		if len(f.Blocks) > rzi.MaxBlocks || len(f.Warnings) == 0 {
			t.Fatalf("%d blocks, %d warnings", len(f.Blocks), len(f.Warnings))
		}
	})
	t.Run("warnings", func(t *testing.T) {
		var rows []string
		for i := 0; i < 500; i++ {
			rows = append(rows, "\t")
		}
		f := Parse(synth(t, "03002", rows...))
		if len(f.Warnings) > maxWarnings+1 {
			t.Fatalf("%d warnings", len(f.Warnings))
		}
	})
	t.Run("name", func(t *testing.T) {
		long := strings.Repeat("я", rzi.MaxNameRunes+1)
		mustDrop(t, Parse(mutate(t, src, "Казино", long)), "03002")
	})
}

func TestNoBlocksIsAnError(t *testing.T) {
	for _, in := range []string{"", "\n\n", "нищо тук\nстр.1\n"} {
		f := Parse(in)
		if f.Error == "" || len(f.Blocks) != 0 || f.Blocks == nil || f.Warnings == nil {
			t.Errorf("%q: %+v", in, f)
		}
	}
}

func TestDuplicatePointBlocksBothParse(t *testing.T) {
	// Duplicates are the validator's call, not the grammar's.
	d := "2026-06-09"
	f := Parse(synth(t, "03002", row(d, d, "1", "1", "")) + synth(t, "03002", row(d, d, "1", "1", "")))
	if len(f.Blocks) != 2 {
		t.Fatalf("%d blocks", len(f.Blocks))
	}
}
