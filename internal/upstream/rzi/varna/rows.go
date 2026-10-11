package varna

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"kanarche.eu/internal/upstream/rzi"
)

// maxRunLines caps the lines of one remark cell.
const maxRunLines = 12

// nTextCols is the count of free-text columns: additional information,
// measures taken, visual observation.
const nTextCols = 3

var (
	reISO   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reParen = regexp.MustCompile(`^\((\d{4}-\d{2}-\d{2})\)$`)
	// reCount is one result cell: plain, "под N", or thousands grouped with
	// single spaces. A space between digit groups is read as a thousands
	// separator only when every later group has exactly three digits.
	reCount = regexp.MustCompile(`^(под )?(0|[1-9]\d{0,8}|[1-9]\d{0,2}(?: \d{3}){1,2})$`)
)

// cell is a run of text in one layout line. lo and hi are rune columns.
type cell struct {
	lo, hi int
	text   string
}

// splitCells cuts a line into cells at runs of two or more spaces, so
// "под 15" and "4 124" stay whole.
func splitCells(l []rune) []cell {
	var out []cell
	start, lastText := -1, -1
	for i, r := range l {
		if r == ' ' {
			if start >= 0 && i-lastText >= 2 {
				out = append(out, cell{start, lastText + 1, string(l[start : lastText+1])})
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
		lastText = i
	}
	if start >= 0 {
		out = append(out, cell{start, lastText + 1, string(l[start : lastText+1])})
	}
	return out
}

// lineCells is one table line sorted into the block's columns.
type lineCells struct {
	planned, date, ie, ec, rest, extra []cell
}

// sortCells assigns each cell to the column its centre falls in. Value
// cells sit at different offsets under the header from block to block, so
// the centre is used, not the start. A cell spanning the IE/EC midline is
// refused: it could belong to either column.
func sortCells(cells []cell, z zone) (lineCells, bool) {
	var lc lineCells
	ok := true
	for _, c := range cells {
		mid := (c.lo + c.hi) / 2
		switch {
		case mid < z.plannedHi:
			lc.planned = append(lc.planned, c)
		case mid < z.ieLo:
			lc.date = append(lc.date, c)
		case mid < z.boundary:
			lc.ie = append(lc.ie, c)
		case mid < z.ecHi:
			lc.ec = append(lc.ec, c)
		case z.extraLo > 0 && c.lo >= z.extraLo:
			lc.extra = append(lc.extra, c)
		default:
			lc.rest = append(lc.rest, c)
		}
		if mid >= z.plannedHi && mid < z.ecHi && c.lo < z.boundary && c.hi > z.boundary {
			ok = false
		}
	}
	return lc, ok
}

func parseCount(s string) (rzi.Count, bool) {
	m := reCount.FindStringSubmatch(s)
	if m == nil {
		return rzi.Count{}, false
	}
	v, err := strconv.ParseInt(strings.ReplaceAll(m[2], " ", ""), 10, 64)
	if err != nil {
		return rzi.Count{}, false
	}
	if m[1] != "" {
		if v < 1 {
			return rzi.Count{}, false
		}
		return rzi.Count{Op: "<", Value: v}, true
	}
	return rzi.Count{Op: "=", Value: v}, true
}

func validDay(s string, year int) bool {
	if !reISO.MatchString(s) {
		return false
	}
	t, err := time.Parse(time.DateOnly, s)
	return err == nil && t.Year() == year
}

func joinCells(cs []cell) string {
	texts := make([]string, len(cs))
	for i, c := range cs {
		texts[i] = c.text
	}
	return strings.Join(texts, " ")
}

// anchor is a data line: it has the actual sampling date.
type anchor struct {
	line  int
	row   rzi.Row
	extra string // text of the Sal. and NAG columns
}

// rows turns the table lines of a block into rows. Every problem warns and
// marks the block bad.
func (p *parser) rows(b *block) []rzi.Row {
	infos := make([]remarkLine, len(b.lines))
	var anchors []anchor
	pending := ""
	var sp *splitHead
	for i, tl := range b.lines {
		infos[i].anchor = -1
		if len(tl.cells) == 0 {
			if sp != nil {
				p.warn(sp.no, b, "replacement row cut by a blank line")
				sp = nil
			}
			continue
		}
		lc, inCols := sortCells(tl.cells, b.z)
		if !inCols {
			p.warn(tl.no, b, "cell crosses a column edge")
			continue
		}
		for _, c := range lc.rest {
			col := textCol(c, b.z)
			infos[i].text[col] = strings.TrimSpace(infos[i].text[col] + " " + c.text)
		}
		if sp != nil {
			a, ok := p.splitTail(tl.no, b, lc, *sp)
			if ok {
				infos[sp.idx].anchor, infos[i].anchor = len(anchors), len(anchors)
				a.extra = strings.TrimSpace(sp.extra + " " + joinCells(lc.extra))
				anchors = append(anchors, a)
			}
			sp = nil
			continue
		}
		if h, ok := splitHeadOf(lc, b.season, pending, i, tl.no); ok {
			sp = &h
			continue
		}
		if len(lc.date) == 0 {
			if len(lc.extra) > 0 {
				p.warn(tl.no, b, "extra column text without a date")
			}
			pending = p.freeLine(tl.no, b, lc, pending)
			continue
		}
		a, ok := p.dataLine(tl.no, b, lc, pending)
		pending = ""
		if ok {
			infos[i].anchor = len(anchors)
			a.extra = joinCells(lc.extra)
			anchors = append(anchors, a)
		}
	}
	if pending != "" || sp != nil {
		p.warn(b.startNo, b, "label without a row")
	}
	switch {
	case len(anchors) == 0:
		p.warn(b.startNo, b, "no rows")
		return nil
	case len(anchors) > rzi.MaxRowsPerBlock:
		p.warn(b.startNo, b, "too many rows")
		return nil
	}
	p.attachRemarks(b, infos, anchors)
	rows := make([]rzi.Row, len(anchors))
	for i, a := range anchors {
		rows[i] = a.row
	}
	return rows
}

// remarkLine is what attachRemarks needs of one table line.
type remarkLine struct {
	anchor int // index into anchors, -1 when not a data line
	text   [nTextCols]string
}

// textCol is the remark column a cell belongs to: the header whose centre
// is nearest to the cell's centre.
func textCol(c cell, z zone) int {
	mid, best := (c.lo+c.hi)/2, 0
	for i := 1; i < nTextCols; i++ {
		if abs(mid-z.textMid[i]) < abs(mid-z.textMid[best]) {
			best = i
		}
	}
	return best
}

// attachRemarks gives each run of consecutive lines that carry text in one
// remark column to the one data line inside it. Runs are per column, so a
// tall cell of one row does not swallow the next row's text. A run with no
// data line, or with two, cannot be attributed and is refused.
func (p *parser) attachRemarks(b *block, lines []remarkLine, anchors []anchor) {
	parts := make([][nTextCols][]string, len(anchors))
	for col := 0; col < nTextCols; col++ {
		for i := 0; i < len(lines); {
			if lines[i].text[col] == "" {
				i++
				continue
			}
			// A data line without text in this column bridges the run only
			// when no data line is owned yet and text follows: tall cells
			// are printed around their row's date line.
			j := i
			owner, owners := -1, 0
			var texts []string
			for j < len(lines) {
				l := lines[j]
				if l.text[col] == "" && !(l.anchor >= 0 && owners == 0 && j+1 < len(lines) && lines[j+1].text[col] != "") {
					break
				}
				if l.anchor >= 0 && l.anchor != owner {
					owner = l.anchor
					owners++
				}
				if l.text[col] != "" {
					texts = append(texts, l.text[col])
				}
				j++
			}
			switch {
			case owners != 1:
				p.warn(b.lines[i].no, b, "remark text not attributable to one row")
			case j-i > maxRunLines:
				p.warn(b.lines[i].no, b, "remark too long")
			default:
				parts[owner][col] = texts
			}
			i = j
		}
	}
	for i := range anchors {
		var all []string
		for col := 0; col < nTextCols; col++ {
			all = append(all, parts[i][col]...)
		}
		all = append(all, anchors[i].extra)
		r := strings.TrimSpace(reSpaces.ReplaceAllString(strings.Join(all, " "), " "))
		if utf8.RuneCountInString(r) > rzi.MaxRemarksRunes {
			p.warn(anchors[i].line, b, "remark too long")
		}
		anchors[i].row.Remarks = r
	}
}

// freeLine handles a line without a date: a row label in the planned
// column, remark text, or nothing else. It returns the pending label.
func (p *parser) freeLine(no int, b *block, lc lineCells, pending string) string {
	if len(lc.ie) > 0 || len(lc.ec) > 0 {
		p.warn(no, b, "result without a date")
		return pending
	}
	switch len(lc.planned) {
	case 0:
		return pending
	case 1:
		if t := lc.planned[0].text; t == labelDop || t == labelRepl {
			if pending != "" {
				p.warn(no, b, "two row labels in a row")
			}
			return t
		}
	}
	p.warn(no, b, "unknown line shape")
	return pending
}

// splitHead is the first line of a replacement row printed on two lines:
// the label and the date on the first, "(date)" and the results below.
type splitHead struct {
	idx, no int
	date    string
	extra   string
}

func splitHeadOf(lc lineCells, season int, pending string, idx, no int) (splitHead, bool) {
	if len(lc.date) != 1 || len(lc.planned) != 1 || lc.planned[0].text != labelRepl ||
		len(lc.ie) != 0 || len(lc.ec) != 0 || pending != "" || !validDay(lc.date[0].text, season) {
		return splitHead{}, false
	}
	return splitHead{idx: idx, no: no, date: lc.date[0].text, extra: joinCells(lc.extra)}, true
}

// splitTail reads the results line of a replacement row.
func (p *parser) splitTail(no int, b *block, lc lineCells, h splitHead) (anchor, bool) {
	m := []string(nil)
	if len(lc.planned) == 1 {
		m = reParen.FindStringSubmatch(lc.planned[0].text)
	}
	if m == nil || m[1] != h.date || len(lc.date) != 0 || len(lc.ie) != 1 || len(lc.ec) != 1 {
		p.warn(no, b, "replacement row does not match its label line")
		return anchor{}, false
	}
	r := rzi.Row{Date: h.date, Unit: unit}
	var ok bool
	if r.IE, ok = parseCount(lc.ie[0].text); !ok {
		p.warn(no, b, "unreadable enterococci value")
		return anchor{}, false
	}
	if r.EC, ok = parseCount(lc.ec[0].text); !ok {
		p.warn(no, b, "unreadable E. coli value")
		return anchor{}, false
	}
	return anchor{line: h.no, row: r}, true
}

// dataLine reads one row from a line with an actual sampling date.
func (p *parser) dataLine(no int, b *block, lc lineCells, pending string) (anchor, bool) {
	bad := func(msg string) (anchor, bool) {
		p.warn(no, b, msg)
		return anchor{}, false
	}
	if len(lc.date) != 1 || !validDay(lc.date[0].text, b.season) {
		return bad("unreadable sample date")
	}
	r := rzi.Row{Date: lc.date[0].text, Unit: unit}
	if len(lc.planned) > 1 {
		return bad("unexpected text before the sample date")
	}
	label := ""
	if len(lc.planned) == 1 {
		label = lc.planned[0].text
	}
	switch m := reParen.FindStringSubmatch(label); {
	case validDay(label, b.season):
		if pending != "" {
			return bad("label does not match the row")
		}
		r.Planned = label
	case label == labelDop || (label == "" && pending == labelDop):
		if pending != "" && label != "" {
			return bad("label does not match the row")
		}
	case m != nil && pending == labelRepl && m[1] == r.Date:
	default:
		return bad("unreadable planned date")
	}
	if len(lc.ie) != 1 || len(lc.ec) != 1 {
		return bad("result columns do not hold one value each")
	}
	var ok bool
	if r.IE, ok = parseCount(lc.ie[0].text); !ok {
		return bad("unreadable enterococci value")
	}
	if r.EC, ok = parseCount(lc.ec[0].text); !ok {
		return bad("unreadable E. coli value")
	}
	return anchor{line: no, row: r}, true
}
