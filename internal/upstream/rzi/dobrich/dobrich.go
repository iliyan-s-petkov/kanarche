// Package dobrich extracts bathing samples from the RZI Dobrich monitoring
// workbook (XLSX, one sheet per sampling point) into the rzi outbox
// contract. The input is hostile: the container and sheet caps come from
// internal/xlsx, the structural caps from the contract, and no input makes
// Extract panic.
package dobrich

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"kanarche.eu/internal/upstream/rzi"
	"kanarche.eu/internal/xlsx"
)

// ExtractorVersion is written into every outbox record.
const ExtractorVersion = "dobrich-xlsx/1"

const (
	maxWarnings = 200
	// headerWindow is how many rows from the top the label, the coordinates
	// and the table header must appear in.
	headerWindow = 20
	labelText    = "пункт за вземане на проби"
)

var (
	errCap  = errors.New("dobrich: structural cap")
	errStop = errors.New("dobrich: stop reading sheet")
)

// Extract turns one Dobrich workbook into an outbox record. Refusals are
// reported in File.Error with a fixed text, never with document content.
func Extract(data []byte) (f rzi.File) {
	sum := sha256.Sum256(data)
	f = rzi.File{
		Schema:           rzi.SchemaVersion,
		Source:           rzi.SourceDobrich,
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
		f.Blocks, f.Warnings = []rzi.Block{}, []string{}
		f.Error = msg
		return f
	}
	book, err := xlsx.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fail(errText(err))
	}
	// One sheet per point: more sheets than the contract has blocks is a cap.
	names := book.Sheets()
	if len(names) > rzi.MaxBlocks {
		return fail(errText(errCap))
	}
	w := &walker{book: book}
	for i, name := range names {
		b, ok, err := w.sheet(i+1, name)
		if err != nil {
			return fail(errText(err))
		}
		if ok {
			f.Blocks = append(f.Blocks, b)
		}
	}
	if len(f.Blocks) == 0 {
		return fail("no sampling point blocks found")
	}
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
	case isAny(err, errCap,
		xlsx.ErrTooManyEntries, xlsx.ErrPartTooLarge, xlsx.ErrTotalTooLarge, xlsx.ErrRatio,
		xlsx.ErrDepth, xlsx.ErrTokenTooLarge, xlsx.ErrSharedStrings, xlsx.ErrRowCap, xlsx.ErrCellsPerRow):
		return "document over a structural cap"
	case isAny(err, xlsx.ErrFormula, xlsx.ErrCellType, xlsx.ErrMacro, xlsx.ErrBinaryPart,
		xlsx.ErrExternal, xlsx.ErrNestedZip, xlsx.ErrDirective):
		return "unsupported workbook content"
	}
	return "unreadable document"
}

func isAny(err error, targets ...error) bool {
	for _, t := range targets {
		if errors.Is(err, t) {
			return true
		}
	}
	return false
}

type walker struct {
	book     *xlsx.Book
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

// sheetName is "КР №08001": the point number is the trailing five digits.
var sheetName = regexp.MustCompile(`^КР\s*№\s*(\d{5})$`)

// sheet reads one worksheet into a block. ok is false when the sheet is not
// usable as a block; the reason is then in the warnings. An error is a
// refusal of the whole document.
func (w *walker) sheet(n int, name string) (rzi.Block, bool, error) {
	m := sheetName.FindStringSubmatch(strings.TrimSpace(name))
	if m == nil {
		w.warn("sheet %d: not a point sheet name", n)
		return rzi.Block{}, false, nil
	}
	point := m[1]
	s := &sheetReader{w: w, point: point}
	err := w.book.Cells(name, s.row)
	if err != nil && !errors.Is(err, errStop) {
		return rzi.Block{}, false, err
	}
	switch {
	case !s.labelSeen:
		w.warn("point %s: point label missing", point)
	case s.labelPoint != point:
		w.warn("point %s: label point number differs from the sheet name", point)
	case !s.coordsOK:
		w.warn("point %s: coordinates missing or unreadable", point)
	case s.hdr == hdrNone:
		w.warn("point %s: table header missing", point)
	case s.hdr == hdrBad:
		w.warn("point %s: table header not recognised", point)
	default:
		return rzi.Block{
			Point: point, Name: s.name, Lat: s.lat, Lon: s.lon,
			Rows: append([]rzi.Row{}, s.rows...),
		}, true, nil
	}
	return rzi.Block{}, false, nil
}

const (
	hdrNone = iota // not found yet
	hdrBad         // found but not usable
	hdrOK
)

// cols are the column positions found from the header row text.
type cols struct {
	planned, date, ie, ec, info, measures, visual int
	unit                                          string
}

// sheetReader is the state of one sheet while its rows stream by.
type sheetReader struct {
	w     *walker
	point string

	labelSeen  bool
	labelPoint string
	name       string
	coordsOK   bool
	lat, lon   float64

	hdr  int
	cols cols
	rows []rzi.Row
}

// row handles one worksheet row. Everything above the table header is
// searched by text, not by fixed row numbers.
func (s *sheetReader) row(r xlsx.GridRow) error {
	if s.hdr != hdrNone {
		return s.dataRow(r)
	}
	if r.Num > headerWindow {
		return errStop
	}
	text := rowText(r)
	low := norm(text)
	switch {
	case !s.labelSeen && strings.Contains(low, labelText):
		s.labelSeen = true
		s.labelPoint, s.name = parseLabel(text)
	case !s.coordsOK && strings.Contains(low, "координати"):
		s.lat, s.lon, s.coordsOK = parseCoords(text)
	case strings.Contains(low, "ентерококи") && strings.Contains(low, "ешерихия"):
		s.cols, s.hdr = header(r)
		if s.hdr == hdrBad {
			return errStop
		}
	}
	return nil
}

func rowText(r xlsx.GridRow) string {
	parts := make([]string, 0, len(r.Cells))
	for _, c := range r.Cells {
		parts = append(parts, c.Val)
	}
	return strings.Join(parts, " ")
}

func norm(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

var (
	labelPointRe = regexp.MustCompile(`№\s*\.?\s*(\d{5})(?:\D|$)`)
	labelNameRe  = regexp.MustCompile(`(?i)наименование\s*(.*)$`)
	nameQuotes   = strings.NewReplacer(`"`, "", "„", "", "“", "", "”", "", "«", "", "»", "")
)

// parseLabel reads the point number and the name from the label row. The
// number is "" when it is not five digits.
func parseLabel(text string) (point, name string) {
	if m := labelPointRe.FindStringSubmatch(text); m != nil {
		point = m[1]
	}
	if m := labelNameRe.FindStringSubmatch(text); m != nil {
		name = clip(strings.Join(strings.Fields(nameQuotes.Replace(m[1])), " "), rzi.MaxNameRunes)
	}
	return point, name
}

var (
	hemiRe = regexp.MustCompile(`([NEНЕ])([^NEНЕ]*)`)
	numRe  = regexp.MustCompile(`\d+(?:[.,]\d+)?`)
)

// parseCoords reads `N 43º 43' 36" ... E 28º 34' 25"`. The hemisphere
// letters may be Latin or Cyrillic look-alikes.
func parseCoords(text string) (lat, lon float64, ok bool) {
	var gotLat, gotLon bool
	for _, m := range hemiRe.FindAllStringSubmatch(text, -1) {
		v, vok := dms(m[2])
		if !vok {
			continue
		}
		switch m[1] {
		case "N", "Н":
			if !gotLat && v <= 90 {
				lat, gotLat = v, true
			}
		default:
			if !gotLon && v <= 180 {
				lon, gotLon = v, true
			}
		}
	}
	return lat, lon, gotLat && gotLon
}

// dms reads degrees with optional minutes and seconds.
func dms(s string) (float64, bool) {
	nums := numRe.FindAllString(s, 4)
	if len(nums) < 1 || len(nums) > 3 {
		return 0, false
	}
	var v float64
	div := 1.0
	for i, n := range nums {
		f, err := strconv.ParseFloat(strings.Replace(n, ",", ".", 1), 64)
		// Minutes and seconds are below 60.
		if err != nil || f >= 1000 || i > 0 && f >= 60 {
			return 0, false
		}
		v += f / div
		div *= 60
	}
	return v, true
}

var unitRe = regexp.MustCompile(`(?i)\b(mpn|cfu)\s*/\s*100\s*(?:cm3|cm³|ml)`)

// header finds the data columns from the header row text. A column named
// twice, a missing column or differing units make the header unusable.
func header(r xlsx.GridRow) (cols, int) {
	c := cols{planned: -1, date: -1, ie: -1, ec: -1, info: -1, measures: -1, visual: -1}
	var ieUnit, ecUnit string
	dup := false
	set := func(p *int, col int) {
		dup = dup || *p >= 0
		*p = col
	}
	for _, cell := range r.Cells {
		h := norm(cell.Val)
		switch {
		case strings.Contains(h, "планирана дата"):
			set(&c.planned, cell.Col)
		case strings.Contains(h, "дата на проб"):
			set(&c.date, cell.Col)
		case strings.Contains(h, "ентерококи"):
			set(&c.ie, cell.Col)
			ieUnit = unitOf(h)
		case strings.Contains(h, "ешерихия"):
			set(&c.ec, cell.Col)
			ecUnit = unitOf(h)
		case strings.Contains(h, "допълнителна информация"):
			set(&c.info, cell.Col)
		case strings.Contains(h, "предприети мерки"):
			set(&c.measures, cell.Col)
		case strings.Contains(h, "визуалното наблюдение"):
			set(&c.visual, cell.Col)
		}
	}
	if dup || c.date < 0 || c.ie < 0 || c.ec < 0 || ieUnit == "" || ieUnit != ecUnit {
		return c, hdrBad
	}
	c.unit = ieUnit
	return c, hdrOK
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

// cellAt returns the cell of row r in column col.
func cellAt(r xlsx.GridRow, col int) (xlsx.GridCell, bool) {
	if col < 0 {
		return xlsx.GridCell{}, false
	}
	i := sort.Search(len(r.Cells), func(i int) bool { return r.Cells[i].Col >= col })
	if i < len(r.Cells) && r.Cells[i].Col == col {
		return r.Cells[i], true
	}
	return xlsx.GridCell{}, false
}

// dataRow reads one row under the header. Rows without a sample date or
// values are blank, planned-only or footnotes and are skipped quietly.
func (s *sheetReader) dataRow(r xlsx.GridRow) error {
	c := s.cols
	dc, hasDate := cellAt(r, c.date)
	ic, hasIE := cellAt(r, c.ie)
	ec, hasEC := cellAt(r, c.ec)
	if !hasDate && !hasIE && !hasEC {
		return nil
	}
	if !hasDate {
		s.w.warn("point %s row %d: sample date missing", s.point, r.Num)
		return nil
	}
	date, ok := parseDate(dc)
	if !ok {
		s.w.warn("point %s row %d: unreadable sample date", s.point, r.Num)
		return nil
	}
	row := rzi.Row{Date: date, Unit: c.unit}
	if pc, ok := cellAt(r, c.planned); ok {
		if row.Planned, ok = parseDate(pc); !ok {
			s.w.warn("point %s row %d: unreadable planned date", s.point, r.Num)
			return nil
		}
	}
	if !hasIE || !hasEC {
		s.w.warn("point %s row %d: value missing", s.point, r.Num)
		return nil
	}
	var ok1, ok2 bool
	row.IE, ok1 = parseCount(ic)
	row.EC, ok2 = parseCount(ec)
	if !ok1 || !ok2 {
		s.w.warn("point %s row %d: unreadable value", s.point, r.Num)
		return nil
	}
	var parts []string
	for _, col := range []int{c.info, c.measures, c.visual} {
		if cell, ok := cellAt(r, col); ok {
			if t := strings.TrimSpace(cell.Val); t != "" {
				parts = append(parts, t)
			}
		}
	}
	row.Remarks = clip(strings.Join(parts, "; "), rzi.MaxRemarksRunes)
	if len(s.rows) >= rzi.MaxRowsPerBlock {
		return errCap
	}
	s.rows = append(s.rows, row)
	return nil
}

var (
	isoDate  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	textDate = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.(\d{4})\s*(?:г\.?)?$`)
)

// parseDate reads an Excel serial date, `YYYY-MM-DD` text or `dd.mm.yyyyг.`
// text into YYYY-MM-DD. Text such as `2026-06=02` is refused.
func parseDate(c xlsx.GridCell) (string, bool) {
	if c.Num {
		v, err := strconv.ParseFloat(c.Val, 64)
		// 61 is 1900-03-01, after Excel's invented leap day.
		if err != nil || v < 61 || v > 100000 || v != math.Trunc(v) {
			return "", false
		}
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		return base.AddDate(0, 0, int(v)).Format(time.DateOnly), true
	}
	txt := strings.TrimSpace(c.Val)
	if isoDate.MatchString(txt) {
		t, err := time.Parse(time.DateOnly, txt)
		return t.Format(time.DateOnly), err == nil && t.Year() >= 1990
	}
	m := textDate.FindStringSubmatch(txt)
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

// parseCount reads a number, or text such as `<15` (below detection) or `15`.
func parseCount(c xlsx.GridCell) (rzi.Count, bool) {
	if c.Num {
		v, err := strconv.ParseFloat(c.Val, 64)
		if err != nil || v < 0 || v > rzi.MaxCount || v != math.Trunc(v) {
			return rzi.Count{}, false
		}
		return rzi.Count{Op: "=", Value: int64(v)}, true
	}
	m := textCount.FindStringSubmatch(strings.TrimSpace(c.Val))
	if m == nil {
		return rzi.Count{}, false
	}
	v, err := strconv.ParseInt(m[2], 10, 64)
	if err != nil {
		return rzi.Count{}, false
	}
	if m[1] != "" {
		// "<0" has no meaning; below detection starts at 1.
		if v < 1 {
			return rzi.Count{}, false
		}
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
