package rzi

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"
)

func validFile(t *testing.T) File {
	t.Helper()
	f, err := Decode(bytes.NewReader(readFixture(t, "varna-valid.json")))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestValidateAcceptsFixture(t *testing.T) {
	acc, rej, st := Validate(validFile(t), testSites(), LastGood{Blocks: 3}, testNow())
	if st != StatusOK || len(rej) != 0 {
		t.Fatalf("status %s, rejects %+v", st, rej)
	}
	if len(acc) != 4 {
		t.Fatalf("accepted %d rows, want 4", len(acc))
	}
	want := Sample{SiteID: "BG3310610135003001", Date: day(2026, 7, 17), Season: 2026,
		EC: 120, IE: 46, Unit: "MPN/100ml", Remarks: "пробата е изместена"}
	if acc[1] != want {
		t.Fatalf("row 2 = %+v, want %+v", acc[1], want)
	}
	if !acc[0].IEBelowDetection || acc[0].IE != 15 || acc[0].ECBelowDetection {
		t.Fatalf("censoring lost: %+v", acc[0])
	}
}

func TestValidateRejectedRecord(t *testing.T) {
	f, err := Decode(bytes.NewReader(readFixture(t, "rejected.json")))
	if err != nil {
		t.Fatal(err)
	}
	acc, rej, st := Validate(f, testSites(), LastGood{}, testNow())
	if st != StatusRejected || len(acc) != 0 || len(rej) == 0 {
		t.Fatalf("status %s, %d accepted, %d rejects", st, len(acc), len(rej))
	}
}

// stored is a last-good sample identical to the fixture's first row.
func stored() Sample {
	return Sample{SiteID: "BG3310610135003001", Date: day(2026, 6, 2), Season: 2026,
		EC: 30, IE: 15, IEBelowDetection: true, Unit: "MPN/100ml"}
}

// addBlocks appends valid blocks for 03004 and 03005, giving 5 blocks.
func addBlocks(f *File) {
	r := []Row{f.Blocks[2].Rows[0]}
	f.Blocks = append(f.Blocks,
		Block{Point: "03004", Lat: 43.25, Lon: 28.02, Rows: r},
		Block{Point: "03005", Lat: 43.24, Lon: 28.01, Rows: r})
}

// TestValidateFailsClosed: every case must give needs_review (or rejected)
// with zero accepted rows, even though the other rows are valid.
func TestValidateFailsClosed(t *testing.T) {
	row := func(f *File) *Row { return &f.Blocks[1].Rows[0] }
	cases := []struct {
		name   string
		mutate func(f *File)
		last   LastGood
		now    time.Time
		want   Status
	}{
		// Rows.
		{"future date", func(f *File) { row(f).Date = "2026-10-12" }, LastGood{}, day(2026, 10, 11).Add(12 * time.Hour), StatusNeedsReview},
		{"before 1 May", func(f *File) { row(f).Date = "2026-04-30" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"after 31 October", func(f *File) { row(f).Date = "2026-11-01" }, LastGood{}, day(2026, 12, 1), StatusNeedsReview},
		{"other year in file", func(f *File) { row(f).Date = "2025-07-01" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"season two years back", func(f *File) {
			for b := range f.Blocks {
				for r := range f.Blocks[b].Rows {
					f.Blocks[b].Rows[r].Date = "2024" + f.Blocks[b].Rows[r].Date[4:]
					f.Blocks[b].Rows[r].Planned = ""
				}
			}
		}, LastGood{}, time.Time{}, StatusNeedsReview},
		{"date typo", func(f *File) { row(f).Date = "2026-06=02" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"date not padded", func(f *File) { row(f).Date = "2026-6-2" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"date with time", func(f *File) { row(f).Date = "2026-06-02T00:00:00Z" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"impossible date", func(f *File) { row(f).Date = "2026-06-31" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"empty date", func(f *File) { row(f).Date = "" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"planned out of season", func(f *File) { row(f).Planned = "2026-12-01" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"planned garbage", func(f *File) { row(f).Planned = "soon" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"value -1", func(f *File) { row(f).EC.Value = -1 }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"value 1000001", func(f *File) { row(f).IE.Value = 1_000_001 }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"value huge", func(f *File) { row(f).IE.Value = math.MaxInt64 }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"op >", func(f *File) { row(f).EC.Op = ">" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"op empty", func(f *File) { row(f).EC = Count{} }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"below zero", func(f *File) { row(f).EC = Count{Op: "<", Value: 0} }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"unit mg/l", func(f *File) { row(f).Unit = "mg/l" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"unit case variant", func(f *File) { row(f).Unit = "mpn/100ml" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"remarks 501 runes", func(f *File) { row(f).Remarks = strings.Repeat("ж", 501) }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"remarks invalid utf8", func(f *File) { row(f).Remarks = "a\xffb" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"duplicate site date", func(f *File) {
			f.Blocks[1].Rows = append(f.Blocks[1].Rows, f.Blocks[1].Rows[0])
		}, LastGood{}, time.Time{}, StatusNeedsReview},
		{"duplicate block", func(f *File) {
			// Same point, other dates, so only the point repeats.
			dup := f.Blocks[2]
			dup.Rows = []Row{dup.Rows[0]}
			dup.Rows[0].Date = "2026-09-01"
			f.Blocks = append(f.Blocks, dup)
		}, LastGood{}, time.Time{}, StatusNeedsReview},
		{"too many rows", func(f *File) {
			// Distinct dates, so only the row cap can fire.
			b := &f.Blocks[0]
			tmpl := b.Rows[0]
			b.Rows = nil
			for d := day(2026, 5, 1); len(b.Rows) <= MaxRowsPerBlock; d = d.AddDate(0, 0, 1) {
				r := tmpl
				r.Planned, r.Date = "", d.Format(time.DateOnly)
				b.Rows = append(b.Rows, r)
			}
		}, LastGood{}, time.Time{}, StatusNeedsReview},
		{"rows at cap allowed", func(f *File) {
			b := &f.Blocks[0]
			tmpl := b.Rows[0]
			b.Rows = nil
			for d := day(2026, 5, 1); len(b.Rows) < MaxRowsPerBlock; d = d.AddDate(0, 0, 1) {
				r := tmpl
				r.Planned, r.Date = "", d.Format(time.DateOnly)
				b.Rows = append(b.Rows, r)
			}
		}, LastGood{}, time.Time{}, StatusOK},
		{"block without rows", func(f *File) { f.Blocks[2].Rows = nil }, LastGood{}, time.Time{}, StatusNeedsReview},
		// Blocks and site mapping.
		{"wrong site id", func(f *File) { f.Blocks[1].Point = "03099" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"short point", func(f *File) { f.Blocks[1].Point = "3001" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"ambiguous point", func(f *File) {
			f.Blocks[1].Point, f.Blocks[1].Lat, f.Blocks[1].Lon = "03007", 43.2, 28.0
		}, LastGood{}, time.Time{}, StatusNeedsReview},
		{"coord 300 m off", func(f *File) { f.Blocks[1].Lat = offsetNorth(43.29, 300) }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"NaN coord", func(f *File) { f.Blocks[1].Lon = math.NaN() }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"point of other source", func(f *File) {
			f.Blocks[1].Point, f.Blocks[1].Lat, f.Blocks[1].Lon = "02001", 42.65, 27.72
		}, LastGood{}, time.Time{}, StatusNeedsReview},
		{"name too long", func(f *File) { f.Blocks[0].Name = strings.Repeat("x", 201) }, LastGood{}, time.Time{}, StatusNeedsReview},
		// File level.
		{"unknown source", func(f *File) { f.Source = "rzi_sofia" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"bad sha", func(f *File) { f.SHA256 = strings.ToUpper(f.SHA256) }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"empty extractor version", func(f *File) { f.ExtractorVersion = "" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"control in extractor version", func(f *File) { f.ExtractorVersion = "v1\n" }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"wrong schema", func(f *File) { f.Schema = 2 }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"extractor warning", func(f *File) { f.Warnings = []string{"unknown header"} }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"zero blocks", func(f *File) { f.Blocks = nil }, LastGood{}, time.Time{}, StatusNeedsReview},
		{"too many blocks", func(f *File) {
			for len(f.Blocks) <= MaxBlocks {
				f.Blocks = append(f.Blocks, f.Blocks[0])
			}
		}, LastGood{}, time.Time{}, StatusNeedsReview},
		{"error with blocks", func(f *File) { f.Error = "magic mismatch" }, LastGood{}, time.Time{}, StatusRejected},
		// Drift against the last good file.
		{"block count drop by 3", func(f *File) {}, LastGood{Blocks: 6}, time.Time{}, StatusNeedsReview},
		{"no last good file", func(f *File) {}, LastGood{}, time.Time{}, StatusOK},
		{"block count rise by 3", addBlocks, LastGood{Blocks: 2}, time.Time{}, StatusNeedsReview},
		{"rise by 2 allowed", addBlocks, LastGood{Blocks: 3}, time.Time{}, StatusOK},
		{"changed value", func(f *File) {}, LastGood{Blocks: 3, Samples: []Sample{func() Sample {
			s := stored()
			s.EC = 31
			return s
		}()}}, time.Time{}, StatusNeedsReview},
		{"changed censoring", func(f *File) {}, LastGood{Blocks: 3, Samples: []Sample{func() Sample {
			s := stored()
			s.IEBelowDetection = false
			return s
		}()}}, time.Time{}, StatusNeedsReview},
		{"same value", func(f *File) {}, LastGood{Blocks: 3, Samples: []Sample{stored()}}, time.Time{}, StatusOK},
		{"drop by 2 allowed", func(f *File) { f.Blocks = f.Blocks[:1] }, LastGood{Blocks: 3}, time.Time{}, StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := validFile(t)
			tc.mutate(&f)
			now := tc.now
			if now.IsZero() {
				now = testNow()
			}
			acc, rej, st := Validate(f, testSites(), tc.last, now)
			if st != tc.want {
				t.Fatalf("status %s, want %s; rejects %+v", st, tc.want, rej)
			}
			if st != StatusOK && len(acc) != 0 {
				t.Fatalf("%d rows accepted with status %s", len(acc), st)
			}
			if st != StatusOK && len(rej) == 0 {
				t.Fatalf("status %s without a reject reason", st)
			}
			if len(rej) > MaxRejects {
				t.Fatalf("%d rejects over cap", len(rej))
			}
		})
	}
}

// TestValidateRejectsCapped bounds the reject list on a file of all-bad rows.
func TestValidateRejectsCapped(t *testing.T) {
	f := validFile(t)
	b := &f.Blocks[0]
	for len(b.Rows) < MaxRowsPerBlock {
		b.Rows = append(b.Rows, Row{Date: "bad"})
	}
	for len(f.Blocks) < MaxBlocks {
		f.Blocks = append(f.Blocks, *b)
	}
	_, rej, st := Validate(f, testSites(), LastGood{}, testNow())
	if st != StatusNeedsReview || len(rej) != MaxRejects {
		t.Fatalf("status %s, %d rejects", st, len(rej))
	}
}

// TestValidateTodayInSofia: a sample dated today in Sofia is not in the
// future even while UTC is still on the previous day.
func TestValidateTodayInSofia(t *testing.T) {
	f := validFile(t)
	f.Blocks[2].Rows[0].Date = "2026-09-17"
	now := time.Date(2026, 9, 16, 22, 30, 0, 0, time.UTC) // 01:30 on the 17th in Sofia
	if _, rej, st := Validate(f, testSites(), LastGood{}, now); st != StatusOK {
		t.Fatalf("status %s, rejects %+v", st, rej)
	}
	now = time.Date(2026, 9, 16, 20, 30, 0, 0, time.UTC) // 23:30 on the 16th in Sofia
	if _, _, st := Validate(f, testSites(), LastGood{}, now); st != StatusNeedsReview {
		t.Fatalf("status %s for a date one day ahead", st)
	}
}

// TestValidateStripsControlChars keeps remarks text but drops control and
// bidi format characters.
func TestValidateStripsControlChars(t *testing.T) {
	f := validFile(t)
	f.Blocks[1].Rows[0].Remarks = "a\x00b\u202ec\r\nd\u200b"
	acc, rej, st := Validate(f, testSites(), LastGood{}, testNow())
	if st != StatusOK {
		t.Fatalf("status %s, rejects %+v", st, rej)
	}
	if got := acc[2].Remarks; got != "abcd" {
		t.Fatalf("remarks = %q", got)
	}
}

// TestRejectDoesNotEchoInput keeps hostile text out of reject reasons.
func TestRejectDoesNotEchoInput(t *testing.T) {
	f := validFile(t)
	f.Blocks[1].Point = "<script>"
	f.Blocks[1].Rows[0].Unit = "<img src=x>"
	f.Blocks[1].Rows[0].Date = "<b>"
	_, rej, _ := Validate(f, testSites(), LastGood{}, testNow())
	for _, r := range rej {
		s := r.Point + r.Date + r.Reason
		if strings.ContainsAny(s, "<>") {
			t.Fatalf("reject echoes input: %+v", r)
		}
	}
}
