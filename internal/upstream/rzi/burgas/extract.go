// Package burgas extracts bathing samples from the RZI Burgas monitoring
// workbook (BIFF8 in an OLE2 container) into the rzi outbox contract. The
// input is hostile: sizes are capped, lengths are bounds-checked, and no
// input makes Extract panic.
package burgas

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"kanarche.eu/internal/upstream/rzi"
)

// ExtractorVersion is written into every outbox record.
const ExtractorVersion = "burgas-xls/1"

const (
	maxWarnings = 200
	labelText   = "пункт за вземане на проби"
	// headerWindow is how many rows below a block label the table header may sit.
	headerWindow = 12
)

// Extract turns one Burgas workbook into an outbox record. Refusals are
// reported in File.Error with a fixed text, never with document content.
func Extract(data []byte) (f rzi.File) {
	sum := sha256.Sum256(data)
	f = rzi.File{
		Schema:           rzi.SchemaVersion,
		Source:           rzi.SourceBurgas,
		SHA256:           hex.EncodeToString(sum[:]),
		ExtractorVersion: ExtractorVersion,
		Blocks:           []rzi.Block{},
		Warnings:         []string{},
	}
	defer func() {
		if recover() != nil {
			f.Blocks, f.Warnings = []rzi.Block{}, []string{}
			f.Error = "internal extractor error"
		}
	}()
	fail := func(msg string) rzi.File {
		f.Blocks = []rzi.Block{}
		f.Error = msg
		return f
	}
	stream, err := workbookStream(data)
	if err != nil {
		return fail(errText(err))
	}
	sh, err := parseWorkbook(stream)
	if err != nil {
		return fail(errText(err))
	}
	w := &walker{sh: sh}
	blocks, err := w.blocks()
	if err != nil {
		return fail(errText(err))
	}
	if len(blocks) == 0 {
		return fail("no sampling point blocks found")
	}
	f.Blocks = blocks
	if w.warnings != nil {
		f.Warnings = w.warnings
	}
	if out, err := json.Marshal(f); err != nil || len(out) > rzi.MaxRecordBytes {
		return fail("output over size cap")
	}
	return f
}

// errText maps an internal error to one of a few fixed refusal texts.
func errText(err error) string {
	switch {
	case errors.Is(err, ErrTooLarge):
		return "input over size cap"
	case errors.Is(err, ErrCycle), errors.Is(err, ErrChain):
		return "corrupt sector chain"
	case errors.Is(err, ErrContainer):
		return "not a readable OLE2 container"
	case errors.Is(err, ErrBIFF):
		return "not a readable BIFF8 workbook"
	case errors.Is(err, errCap):
		return "document over a structural cap"
	}
	return "unreadable document"
}

var errCap = errors.New("burgas: structural cap")

type walker struct {
	sh       *sheet
	warnings []string
}

func (w *walker) warn(format string, a ...any) {
	switch {
	case len(w.warnings) < maxWarnings:
		w.warnings = append(w.warnings, fmt.Sprintf(format, a...))
	case len(w.warnings) == maxWarnings:
		w.warnings = append(w.warnings, "more warnings suppressed")
	}
}

func (w *walker) str(row, col int) string {
	c, ok := w.sh.at(row, col)
	if !ok || c.isNum {
		return ""
	}
	return strings.TrimSpace(c.str)
}

func norm(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// labelRows lists the rows whose first column holds the block label.
func (w *walker) labelRows() []int {
	var rows []int
	for r := 0; r <= w.sh.maxRow; r++ {
		if strings.Contains(norm(w.str(r, 0)), labelText) {
			rows = append(rows, r)
		}
	}
	return rows
}

func (w *walker) blocks() ([]rzi.Block, error) {
	labels := w.labelRows()
	if len(labels) > rzi.MaxBlocks {
		return nil, errCap
	}
	var out []rzi.Block
	for i, lr := range labels {
		end := w.sh.maxRow + 1
		if i+1 < len(labels) {
			end = labels[i+1]
		}
		b, ok, err := w.block(lr, end)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, b)
		}
	}
	return out, nil
}

// cols are the column positions found from the header row text.
type cols struct {
	planned, date, ie, ec, info, measures int
	unit                                  string
}

// block reads one block between its label row and the next label row.
func (w *walker) block(lr, end int) (rzi.Block, bool, error) {
	point, ok := w.pointID(lr)
	if !ok {
		w.warn("block at row %d: unreadable point number", lr+1)
		return rzi.Block{}, false, nil
	}
	b := rzi.Block{Point: point, Rows: []rzi.Row{}}
	b.Name = clip(w.firstStr(lr, 4, 5, 3), rzi.MaxNameRunes)
	hr := -1
	for r := lr + 1; r < end && r <= lr+headerWindow; r++ {
		if w.hasText(r, "ешерихия") {
			hr = r
			break
		}
	}
	foundCoords := false
	for r := lr + 1; r < end && r <= lr+headerWindow; r++ {
		if strings.Contains(norm(w.str(r, 0)), "координати") {
			b.Lat, b.Lon, foundCoords = w.coords(r)
			break
		}
	}
	if !foundCoords {
		w.warn("block %s: coordinates missing or unreadable", point)
		return rzi.Block{}, false, nil
	}
	if hr < 0 {
		w.warn("block %s: table header missing", point)
		return rzi.Block{}, false, nil
	}
	c, ok := w.header(hr)
	if !ok {
		w.warn("block %s: table header not recognised", point)
		return rzi.Block{}, false, nil
	}
	for r := hr + 1; r < end; r++ {
		row, ok := w.row(point, r, c)
		if !ok {
			continue
		}
		if len(b.Rows) >= rzi.MaxRowsPerBlock {
			return rzi.Block{}, false, errCap
		}
		b.Rows = append(b.Rows, row)
	}
	return b, true, nil
}

// hasText reports whether any cell of the row contains sub (lower case).
func (w *walker) hasText(row int, sub string) bool {
	for col := 0; col < 16; col++ {
		if strings.Contains(norm(w.str(row, col)), sub) {
			return true
		}
	}
	return false
}

func (w *walker) firstStr(row int, cols ...int) string {
	for _, c := range cols {
		if s := w.str(row, c); s != "" && !strings.HasSuffix(norm(s), ":") {
			return s
		}
	}
	return ""
}

var digits5 = regexp.MustCompile(`^\D*(\d{5})\D*$`)

func (w *walker) pointID(row int) (string, bool) {
	for col := 1; col <= 3; col++ {
		c, ok := w.sh.at(row, col)
		if !ok {
			continue
		}
		if c.isNum {
			if c.num >= 1 && c.num < 100000 && c.num == math.Trunc(c.num) {
				return fmt.Sprintf("%05d", int(c.num)), true
			}
			continue
		}
		if m := digits5.FindStringSubmatch(c.str); m != nil {
			return m[1], true
		}
	}
	return "", false
}

var numRe = regexp.MustCompile(`\d+(?:[.,]\d+)?`)

// coords reads "N 42° 49' 14\"" and "E 27° 53' 07\"" cells from one row.
func (w *walker) coords(row int) (lat, lon float64, ok bool) {
	var gotLat, gotLon bool
	for col := 0; col < 8; col++ {
		s := w.str(row, col)
		if s == "" {
			continue
		}
		v, hemi, vok := dms(s)
		switch {
		case vok && hemi == 'N' && !gotLat:
			lat, gotLat = v, v <= 90
		case vok && hemi == 'E' && !gotLon:
			lon, gotLon = v, v <= 180
		}
	}
	return lat, lon, gotLat && gotLon
}

// dms parses a hemisphere letter with up to three numbers (deg, min, sec).
func dms(s string) (float64, rune, bool) {
	s = strings.TrimSpace(s)
	r, _ := utf8.DecodeRuneInString(s)
	hemi := r
	if hemi != 'N' && hemi != 'E' {
		return 0, 0, false
	}
	nums := numRe.FindAllString(s, 4)
	if len(nums) < 1 || len(nums) > 3 {
		return 0, 0, false
	}
	var v, div float64 = 0, 1
	for _, n := range nums {
		f, err := strconv.ParseFloat(strings.Replace(n, ",", ".", 1), 64)
		if err != nil || f < 0 || f >= 1000 {
			return 0, 0, false
		}
		v += f / div
		div *= 60
	}
	return v, hemi, true
}

var unitRe = regexp.MustCompile(`(?i)\b(mpn|cfu)\s*/\s*100\s*(?:cm3|cm³|ml)`)

// header finds the data columns from the header row text.
func (w *walker) header(hr int) (cols, bool) {
	c := cols{planned: -1, date: -1, ie: -1, ec: -1, info: -1, measures: -1}
	var ieUnit, ecUnit string
	for col := 0; col < 16; col++ {
		h := norm(w.str(hr, col))
		switch {
		case h == "":
		case strings.Contains(h, "дата на пробонабиране"):
			c.date = col
		case strings.Contains(h, "планирана дата"):
			c.planned = col
		case strings.Contains(h, "ентерококи"):
			c.ie, ieUnit = col, unitOf(h)
		case strings.Contains(h, "ешерихия"):
			c.ec, ecUnit = col, unitOf(h)
		case strings.Contains(h, "допълнителна"):
			c.info = col
		case strings.Contains(h, "предприети"):
			c.measures = col
		}
	}
	// The date title cell is blank in some blocks: take the column between
	// the planned date and the first value.
	if c.date < 0 && c.planned >= 0 && c.planned+1 < c.ie {
		c.date = c.planned + 1
	}
	if c.date < 0 || c.ie < 0 || c.ec < 0 || ieUnit == "" || ieUnit != ecUnit {
		return c, false
	}
	c.unit = ieUnit
	return c, true
}

func unitOf(h string) string {
	m := unitRe.FindStringSubmatch(h)
	if m == nil {
		return ""
	}
	if strings.EqualFold(m[1], "cfu") {
		return "cfu/100ml"
	}
	return "MPN/100ml"
}

// row reads one data row. Rows without a sample date or values are blank
// or footnotes and are skipped without comment.
func (w *walker) row(point string, r int, c cols) (rzi.Row, bool) {
	dc, hasDate := w.sh.at(r, c.date)
	ic, hasIE := w.sh.at(r, c.ie)
	ec, hasEC := w.sh.at(r, c.ec)
	if !hasDate && !hasIE && !hasEC {
		return rzi.Row{}, false
	}
	if !hasDate {
		w.warn("block %s row %d: sample date missing", point, r+1)
		return rzi.Row{}, false
	}
	date, ok := w.date(dc)
	if !ok {
		w.warn("block %s row %d: unreadable sample date", point, r+1)
		return rzi.Row{}, false
	}
	row := rzi.Row{Date: date, Unit: c.unit}
	if c.planned >= 0 {
		if pc, ok := w.sh.at(r, c.planned); ok {
			if row.Planned, ok = w.date(pc); !ok {
				w.warn("block %s row %d: unreadable planned date", point, r+1)
				return rzi.Row{}, false
			}
		}
	}
	if !hasIE || !hasEC {
		w.warn("block %s row %d: value missing", point, r+1)
		return rzi.Row{}, false
	}
	var ok1, ok2 bool
	row.IE, ok1 = count(ic)
	row.EC, ok2 = count(ec)
	if !ok1 || !ok2 {
		w.warn("block %s row %d: unreadable value", point, r+1)
		return rzi.Row{}, false
	}
	var parts []string
	for _, col := range []int{c.info, c.measures} {
		if col >= 0 {
			if s := w.str(r, col); s != "" {
				parts = append(parts, s)
			}
		}
	}
	row.Remarks = clip(strings.Join(parts, "; "), rzi.MaxRemarksRunes)
	return row, true
}

var textDate = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.(\d{4})\s*(?:г\.?)?$`)

// date reads an Excel serial or a "dd.mm.yyyyг." text into YYYY-MM-DD.
func (w *walker) date(c cell) (string, bool) {
	if c.isNum {
		if math.IsNaN(c.num) || c.num < 61 || c.num > 100000 {
			return "", false
		}
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		if w.sh.date1904 {
			base = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		return base.AddDate(0, 0, int(c.num)).Format(time.DateOnly), true
	}
	m := textDate.FindStringSubmatch(strings.TrimSpace(c.str))
	if m == nil {
		return "", false
	}
	d, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	y, _ := strconv.Atoi(m[3])
	t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	if t.Day() != d || int(t.Month()) != mo || y < 1990 {
		return "", false
	}
	return t.Format(time.DateOnly), true
}

var textCount = regexp.MustCompile(`^(<)?\s*(\d{1,7})$`)

// count reads a number, or text such as "<10" (below detection) or "15".
func count(c cell) (rzi.Count, bool) {
	if c.isNum {
		if math.IsNaN(c.num) || c.num < 0 || c.num > rzi.MaxCount || c.num != math.Trunc(c.num) {
			return rzi.Count{}, false
		}
		return rzi.Count{Op: "=", Value: int64(c.num)}, true
	}
	m := textCount.FindStringSubmatch(strings.TrimSpace(c.str))
	if m == nil {
		return rzi.Count{}, false
	}
	v, err := strconv.ParseInt(m[2], 10, 64)
	if err != nil {
		return rzi.Count{}, false
	}
	if m[1] != "" {
		return rzi.Count{Op: "<", Value: v}, true
	}
	return rzi.Count{Op: "=", Value: v}, true
}

// clip cuts s to n runes.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
