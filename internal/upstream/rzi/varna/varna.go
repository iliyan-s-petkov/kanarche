// Package varna parses `pdftotext -layout` text of the RZI Varna bathing
// water PDF into the rzi outbox contract. It parses text only and fails
// closed: anything it does not recognise becomes a warning, and a block
// with a warning is dropped.
package varna

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"kanarche.eu/internal/upstream/rzi"
)

// Input caps. The real 2026 file is 57 KB, 768 lines, 248 columns.
const (
	MaxInputBytes = 1 << 20
	MaxLines      = 20000
	MaxLineRunes  = 400
	maxWarnings   = 100
	maxHeaderRows = 8
	// colDrift is how far a block's header columns may move from the first
	// block's.
	colDrift = 6
	// resultHdr is the rune length of "Резултат от".
	resultHdr = 11
)

const (
	unit      = "MPN/100ml"
	labelDop  = "допълнителна проба"
	labelRepl = "заместваща проба"
)

var (
	reTitle  = regexp.MustCompile(`^РЕЗУЛТАТИ ОТ МОНИТОРИНГА НА МОРСКИ ВОДИ ОТ ЗОНИТЕ ЗА КЪПАНЕ-РЕГИОН ВАРНА \d{4} Г\.$`)
	reFooter = regexp.MustCompile(`^стр\.\d{1,4}$`)
	rePoint  = regexp.MustCompile(`^Пункт за вземане на проби №\s?\.?\s?(\d{5})\s+Наименование:?\s+(\S.*)$`)
	reCoords = regexp.MustCompile(`^Географски координати: ширина N (\S.*?) дължина E (\S.*)$`)
	reDMS    = regexp.MustCompile(`^(\d{2})(?: |0)(\d{2})'(\d{2})''$`)
	reSeason = regexp.MustCompile(`^Времетраене на сезона за къпане: от \d{1,2}\.\d{2}\.(\d{4}) г\. до \d{1,2}\.\d{2}\.(\d{4}) г\.$`)
	reUnits  = regexp.MustCompile(`^MPN/100cm(3?) MPN/100cm(3?)$`)
	reSpaces = regexp.MustCompile(`\s+`)
)

// Parse converts one document's text into an outbox record. The caller
// sets SHA256 and ExtractorVersion. Error is set when the input is refused
// as a whole; the record then carries no blocks.
func Parse(text string) rzi.File {
	f := rzi.File{Schema: rzi.SchemaVersion, Source: rzi.SourceVarna, Blocks: []rzi.Block{}, Warnings: []string{}}
	refuse := func(msg string) rzi.File {
		f.Error = msg
		return f
	}
	if len(text) > MaxInputBytes {
		return refuse("input over size cap")
	}
	if !utf8.ValidString(text) {
		return refuse("input is not valid UTF-8")
	}
	if strings.Count(text, "\n") >= MaxLines {
		return refuse("input over line cap")
	}
	raw := strings.Split(text, "\n")
	p := &parser{}
	for i, l := range raw {
		l = strings.TrimLeft(l, "\f")
		if utf8.RuneCountInString(l) > MaxLineRunes {
			return refuse("line over length cap")
		}
		p.line(i+1, []rune(l))
	}
	p.endBlock()
	f.Warnings = append(f.Warnings, p.warnings...)
	if len(p.blocks) == 0 {
		return refuse("no point blocks found")
	}
	f.Blocks = p.blocks
	return f
}

type parser struct {
	warnings []string
	blocks   []rzi.Block
	cur      *block
	seenCols bool
	cols     [4]int // first block's header columns, for drift
	skipping bool   // inside a point header that was refused
}

// zone holds the column bounds, in runes, derived from a block's header row.
type zone struct {
	// Column bounds on the centre of a cell.
	plannedHi int // the date zone starts here
	ieLo      int
	boundary  int // IE/EC split
	ecHi      int // remarks start here
	extraLo   int // Sal. and NAG columns start here, 0 when absent
	textMid   [nTextCols]int
}

type tline struct {
	no    int
	cells []cell
}

type block struct {
	point, name string
	lat, lon    float64
	haveCoords  bool
	season      int
	haveCols    bool
	z           zone
	h1, h2      int
	unitsDone   bool
	unitNeeds3  bool
	headerRows  int
	orderOK     [2]bool
	lines       []tline
	bad         bool
	startNo     int
}

// warn records a reason and marks the block bad. Messages never contain
// document text.
func (p *parser) warn(no int, b *block, msg string) {
	if b != nil {
		b.bad = true
	}
	if len(p.warnings) > maxWarnings {
		return
	}
	if len(p.warnings) == maxWarnings {
		p.warnings = append(p.warnings, "further warnings suppressed")
		return
	}
	if b != nil && b.point != "" {
		p.warnings = append(p.warnings, fmt.Sprintf("point %s line %d: %s", b.point, no, msg))
		return
	}
	p.warnings = append(p.warnings, fmt.Sprintf("line %d: %s", no, msg))
}

func (p *parser) line(no int, l []rune) {
	for _, r := range l {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			if p.cur != nil {
				p.warn(no, p.cur, "control character")
			}
			return
		}
	}
	trim := strings.TrimSpace(string(l))
	switch {
	case reFooter.MatchString(trim), reTitle.MatchString(trim):
		return
	case strings.HasPrefix(string(l), "Пункт"):
		p.endBlock()
		p.startBlock(no, trim)
		return
	case strings.HasPrefix(trim, "Пункт") && p.cur != nil && !p.skipping:
		// Real headers start at column 0. An indented one sits in a table
		// column and must not end or forge a block.
		p.warn(no, p.cur, "point header inside a table")
		return
	}
	if p.skipping || p.cur == nil {
		// Before the first point (the method page) and inside a refused
		// point header the text is not parsed.
		return
	}
	p.blockLine(no, l, trim)
}

func (p *parser) startBlock(no int, trim string) {
	m := rePoint.FindStringSubmatch(trim)
	if m == nil {
		p.skipping = true
		p.warn(no, nil, "unknown point header")
		return
	}
	p.skipping = false
	name := strings.TrimSpace(reSpaces.ReplaceAllString(m[2], " "))
	p.cur = &block{point: m[1], name: name, startNo: no}
	if utf8.RuneCountInString(name) > rzi.MaxNameRunes {
		p.warn(no, p.cur, "point name too long")
	}
}

func (p *parser) blockLine(no int, l []rune, trim string) {
	b := p.cur
	if b.unitsDone {
		b.lines = append(b.lines, tline{no: no, cells: splitCells(l)})
		return
	}
	if trim == "" {
		return
	}
	switch {
	case reCoords.MatchString(trim):
		p.coords(no, b, reCoords.FindStringSubmatch(trim))
	case trim == "0":
		// The degree sign of the coordinates, extracted on its own line.
	case reSeason.MatchString(trim):
		p.season(no, b, reSeason.FindStringSubmatch(trim))
	case strings.HasPrefix(trim, "ДРУГИ ДОПЪЛНИТЕЛНО ИЗСЛЕДВАНИ ПРИ НЕОБХОДИМОСТ ПОКАЗАТЕЛИ"):
	case !b.haveCols && strings.Contains(trim, "Резултат от"):
		p.colHeader(no, b, l)
	case b.haveCols:
		p.headerRest(no, b, l, trim)
	default:
		p.warn(no, b, "unknown line shape")
	}
}

func (p *parser) coords(no int, b *block, m []string) {
	if b.haveCoords {
		p.warn(no, b, "second coordinates line")
		return
	}
	lat, ok1 := dms(m[1])
	lon, ok2 := dms(m[2])
	if !ok1 || !ok2 || lat < 41 || lat > 44 || lon < 22 || lon > 29 {
		p.warn(no, b, "unreadable coordinates")
		return
	}
	b.lat, b.lon, b.haveCoords = lat, lon, true
}

// dms reads 43 17'23” and 43017'23” (the degree sign extracted as "0").
func dms(s string) (float64, bool) {
	m := reDMS.FindStringSubmatch(strings.Trim(s, ". "))
	if m == nil {
		return 0, false
	}
	d, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	se, _ := strconv.Atoi(m[3])
	if mi > 59 || se > 59 {
		return 0, false
	}
	return float64(d) + float64(mi)/60 + float64(se)/3600, true
}

func (p *parser) season(no int, b *block, m []string) {
	if b.season != 0 {
		p.warn(no, b, "second season line")
		return
	}
	if m[1] != m[2] {
		p.warn(no, b, "season spans two years")
		return
	}
	b.season, _ = strconv.Atoi(m[1])
}

// runeIndex is the rune column of the first sub at or after rune from.
func runeIndex(l []rune, sub string, from int) int {
	if from > len(l) {
		return -1
	}
	rest := string(l[from:])
	i := strings.Index(rest, sub)
	if i < 0 {
		return -1
	}
	return from + utf8.RuneCountInString(rest[:i])
}

// colHeader derives the column bounds from the header row that carries the
// planned date, the actual date and the two result headers.
func (p *parser) colHeader(no int, b *block, l []rune) {
	h1 := runeIndex(l, "Резултат от", 0)
	h2 := -1
	if h1 >= 0 {
		h2 = runeIndex(l, "Резултат от", h1+resultHdr)
	}
	pl := runeIndex(l, "Планирана дата за", 0)
	d := runeIndex(l, "Дата на", 0)
	if h2 < 0 || pl < 0 || d < 0 || runeIndex(l, "Резултат от", h2+resultHdr) >= 0 ||
		!(pl < d && d < h1) || h2-h1 < 10 || h2-h1 > 14 {
		p.warn(no, b, "unreadable column header")
		return
	}
	cols := [4]int{pl, d, h1, h2}
	if !p.seenCols {
		p.seenCols, p.cols = true, cols
	}
	for i := range cols {
		if abs(cols[i]-p.cols[i]) > colDrift {
			p.warn(no, b, "column drift")
			return
		}
	}
	// Header and value centres: the date column is centred about d+2 and
	// each result header is resultHdr wide.
	h1c, h2c := h1+resultHdr/2, h2+resultHdr/2
	b.z = zone{plannedHi: (pl + d) / 2, ieLo: (d + 2 + h1c) / 2, boundary: (h1c + h2c) / 2, ecHi: h2c + 8}
	ta := runeIndex(l, "Допълнителна", 0)
	tm := runeIndex(l, "Предприети", 0)
	tv := runeIndex(l, "Данни от", 0)
	if !(h2 < ta && ta < tm && tm < tv) {
		p.warn(no, b, "unreadable text column headers")
		return
	}
	b.z.textMid = [nTextCols]int{ta + 6, tm + 5, tv + 4}
	sal, nag := runeIndex(l, "Sal.", 0), runeIndex(l, "NAG", 0)
	if (sal < 0) != (nag < 0) || (sal >= 0 && !(h2 < sal && sal < nag)) {
		p.warn(no, b, "unreadable extra column header")
		return
	}
	if sal >= 0 {
		b.z.extraLo = sal - 6
	}
	b.h1, b.h2, b.haveCols = h1, h2, true
	p.checkOrder(b, l)
}

// checkOrder confirms enterococci sit over the first result column and
// E. coli over the second.
func (p *parser) checkOrder(b *block, l []rune) {
	if i := runeIndex(l, "чревни", 0); i >= b.h1-4 && i <= b.h1+8 && i >= 0 {
		b.orderOK[0] = true
	}
	if i := runeIndex(l, "Ешерихия", 0); i >= b.h2-8 && i <= b.h2+8 && i >= 0 {
		b.orderOK[1] = true
	}
}

// headerRest handles the lines between the column header and the first
// data row: more header words, then the unit line.
func (p *parser) headerRest(no int, b *block, l []rune, trim string) {
	if b.unitNeeds3 {
		if trim != "3" {
			p.warn(no, b, "unit line not completed")
			return
		}
		b.unitNeeds3, b.unitsDone = false, true
		return
	}
	if m := reUnits.FindStringSubmatch(trim); m != nil {
		if !b.orderOK[0] || !b.orderOK[1] {
			p.warn(no, b, "result columns not in the expected order")
			return
		}
		b.unitNeeds3 = m[1] == "" || m[2] == ""
		b.unitsDone = !b.unitNeeds3
		return
	}
	b.headerRows++
	if b.headerRows > maxHeaderRows {
		p.warn(no, b, "header too long")
		return
	}
	if strings.ContainsAny(trim, "0123456789") {
		p.warn(no, b, "unknown header text")
		return
	}
	p.checkOrder(b, l)
}

func (p *parser) endBlock() {
	b := p.cur
	p.cur = nil
	if b == nil {
		return
	}
	switch {
	case b.bad:
		return
	case len(p.blocks) >= rzi.MaxBlocks:
		p.warn(b.startNo, b, "too many blocks")
	case !b.haveCoords:
		p.warn(b.startNo, b, "no coordinates")
	case b.season == 0:
		p.warn(b.startNo, b, "no season line")
	case !b.haveCols:
		p.warn(b.startNo, b, "no column header")
	case !b.unitsDone:
		p.warn(b.startNo, b, "no unit line")
	}
	if b.bad {
		return
	}
	rows := p.rows(b)
	if b.bad {
		return
	}
	p.blocks = append(p.blocks, rzi.Block{Point: b.point, Name: b.name, Lat: b.lat, Lon: b.lon, Rows: rows})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
