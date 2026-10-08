package datahub

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"kanarche.eu/internal/xlsx"
	"kanarche.eu/internal/xlsx/xlsxtest"
)

var now = time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)

var hdr = []string{"countryCode", "bathingWaterIdentifier", "bathingWaterName", "season", "quality"}

func row(cc, id string, season, q string) []string {
	return []string{cc, id, "name", season, q}
}

func book(t *testing.T, rows ...[]string) *xlsx.Book {
	t.Helper()
	data, err := xlsxtest.TableBook(append([][]string{hdr}, rows...))
	if err != nil {
		t.Fatal(err)
	}
	b, err := xlsx.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// small relaxes the newest-season floor so fixtures stay tiny.
var small = Options{MaxRejectRatio: 0.01, MinNewestSites: 1}

func TestParseKeepsOnlyCountry(t *testing.T) {
	b := book(t,
		row("BG", "BG0000000000000001", "2025", "1 - Excellent"),
		row("DK", "DK0000000000000001", "2025", "2 - Good"),
		row("BG", "BG0000000000000002", "2024", " 2 - Good "))
	snap, rej, err := ParseWith(b, "BG", now, small)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Classes) != 2 || rej.OtherCountry != 1 || rej.Country != 2 || rej.Rejected() != 0 {
		t.Fatalf("snap=%v rej=%+v", snap.Classes, rej)
	}
	want := []Class{{"BG0000000000000001", 2025, "excellent"}, {"BG0000000000000002", 2024, "good"}}
	for i, c := range want {
		if snap.Classes[i] != c {
			t.Fatalf("class %d = %v, want %v", i, snap.Classes[i], c)
		}
	}
}

// reject checks one bad row among many good ones is counted under reason and dropped.
func reject(t *testing.T, bad []string, reason Reason) {
	t.Helper()
	rows := [][]string{bad}
	for i := 0; i < 200; i++ {
		rows = append(rows, row("BG", fmt.Sprintf("BG%016d", i), "2025", "1 - Excellent"))
	}
	snap, rej, err := ParseWith(book(t, rows...), "BG", now, small)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Classes) != 200 || rej.ByReason[reason] != 1 || rej.Rejected() != 1 {
		t.Fatalf("kept=%d rej=%+v", len(snap.Classes), rej)
	}
}

func TestParseRejectsBadClass(t *testing.T) {
	reject(t, row("BG", "BG0000000000009999", "2025", "9 - Superb"), ReasonQuality)
}

func TestParseRejectsBadID(t *testing.T) {
	reject(t, row("BG", "bg0000000000009999", "2025", "1 - Excellent"), ReasonID)
	reject(t, row("BG", "DK0000000000009999", "2025", "1 - Excellent"), ReasonID)
	reject(t, row("BG", "", "2025", "1 - Excellent"), ReasonID)
}

func TestParseRejectsSeasonRange(t *testing.T) {
	for _, s := range []string{"1989", "2006", "2027", "2099", "2025.5", "02025", " 2025", "abc", ""} {
		t.Run(s, func(t *testing.T) {
			reject(t, row("BG", "BG0000000000009999", s, "1 - Excellent"), ReasonSeason)
		})
	}
}

func TestParseAcceptsSeasonBounds(t *testing.T) {
	b := book(t,
		row("BG", "BG0000000000000001", "2007", "1 - Excellent"),
		row("BG", "BG0000000000000001", "2026", "1 - Excellent"))
	snap, _, err := ParseWith(b, "BG", now, small)
	if err != nil || len(snap.Classes) != 2 {
		t.Fatalf("snap=%v err=%v", snap.Classes, err)
	}
}

func TestParseDuplicateKeyFailsFile(t *testing.T) {
	b := book(t,
		row("BG", "BG0000000000000001", "2025", "1 - Excellent"),
		row("BG", "BG0000000000000001", "2025", "2 - Good"))
	if _, _, err := ParseWith(b, "BG", now, small); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseFailsOnZeroRows(t *testing.T) {
	b := book(t, row("DK", "DK0000000000000001", "2025", "1 - Excellent"))
	if _, _, err := ParseWith(b, "BG", now, small); !errors.Is(err, ErrNoRows) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseFailsWhenAllRowsRejected(t *testing.T) {
	b := book(t, row("BG", "BG0000000000000001", "2025", "nope"))
	if _, _, err := ParseWith(b, "BG", now, small); !errors.Is(err, ErrNoRows) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseFailsOverRejectRatio(t *testing.T) {
	build := func(bad int) *xlsx.Book {
		var rows [][]string
		for i := 0; i < 100; i++ {
			q := "1 - Excellent"
			if i < bad {
				q = "bad"
			}
			rows = append(rows, row("BG", fmt.Sprintf("BG%016d", i), "2025", q))
		}
		return book(t, rows...)
	}
	// exactly 1% is tolerated, 2% is not
	if _, rej, err := ParseWith(build(1), "BG", now, small); err != nil || rej.Rejected() != 1 {
		t.Fatalf("1%%: rej=%+v err=%v", rej, err)
	}
	if _, _, err := ParseWith(build(2), "BG", now, small); !errors.Is(err, ErrTooManyRejects) {
		t.Fatalf("2%%: err = %v", err)
	}
}

func TestParseFailsThinNewestSeason(t *testing.T) {
	sites := func(n int) *xlsx.Book {
		var rows [][]string
		for i := 0; i < n; i++ {
			rows = append(rows, row("BG", fmt.Sprintf("BG%016d", i), "2025", "1 - Excellent"))
			rows = append(rows, row("BG", fmt.Sprintf("BG%016d", i), "2024", "1 - Excellent"))
		}
		return book(t, rows...)
	}
	if _, _, err := Parse(sites(49), "BG", now); !errors.Is(err, ErrThinNewest) {
		t.Fatalf("49 sites: err = %v", err)
	}
	if snap, _, err := Parse(sites(50), "BG", now); err != nil || len(snap.Classes) != 100 {
		t.Fatalf("50 sites: n=%d err=%v", len(snap.Classes), err)
	}
}

func TestParseSortsBySiteThenSeason(t *testing.T) {
	b := book(t,
		row("BG", "BG0000000000000002", "2025", "1 - Excellent"),
		row("BG", "BG0000000000000001", "2025", "1 - Excellent"),
		row("BG", "BG0000000000000001", "2024", "1 - Excellent"))
	snap, _, err := ParseWith(b, "BG", now, small)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range snap.Classes {
		got = append(got, c.SiteID[len(c.SiteID)-1:]+"/"+strconv.Itoa(c.Season))
	}
	if fmt.Sprint(got) != "[1/2024 1/2025 2/2025]" {
		t.Fatalf("order = %v", got)
	}
}

func TestParseMissingColumnIsError(t *testing.T) {
	data, _ := xlsxtest.TableBook([][]string{{"countryCode", "season"}, {"BG", "2025"}})
	b, err := xlsx.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ParseWith(b, "BG", now, small); !errors.Is(err, xlsx.ErrHeader) {
		t.Fatalf("err = %v", err)
	}
}

// TestParseRealFile runs against a downloaded workbook when AIRBG_REAL_XLSX
// names one. It is skipped otherwise so the suite never needs the file.
func TestParseRealFile(t *testing.T) {
	path := os.Getenv("AIRBG_REAL_XLSX")
	if path == "" {
		t.Skip("AIRBG_REAL_XLSX not set")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	b, err := xlsx.Open(f, st.Size())
	if err != nil {
		t.Fatal(err)
	}
	snap, rej, err := Parse(b, "BG", now)
	t.Logf("accepted=%d country=%d other=%d reasons=%v err=%v", len(snap.Classes), rej.Country, rej.OtherCountry, rej.ByReason, err)
	if err != nil {
		t.Fatal(err)
	}
}

// The committed sample is cut from the real EEA workbook by
// TestEmitSampleFromSource, with no hand edits.
func TestParseRealSample(t *testing.T) {
	data, err := os.ReadFile("testdata/datahub_bg_sample.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	b, err := xlsx.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	snap, rej, err := ParseWith(b, "BG", now, small)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Classes) != 42 || rej.Country != 42 || rej.OtherCountry != 5 || rej.Rejected() != 0 {
		t.Fatalf("classes=%d rej=%+v, want 42 classes, 5 other-country rows, 0 rejects", len(snap.Classes), rej)
	}
	perSeason := map[int]int{}
	for _, c := range snap.Classes {
		perSeason[c.Season]++
	}
	for _, s := range []int{2023, 2024, 2025} {
		if perSeason[s] != 14 {
			t.Errorf("season %d has %d classes, want 14", s, perSeason[s])
		}
	}
	got := map[Class]bool{}
	for _, c := range snap.Classes {
		got[c] = true
	}
	for _, want := range []Class{
		{"BG3242661710017001", 2023, "excellent"},
		{"BG3242661710017001", 2024, "excellent"},
		{"BG3412181178002022", 2025, "good"},
		{"BG3412758356002032", 2025, "excellent"},
	} {
		if !got[want] {
			t.Errorf("class %v missing", want)
		}
	}
	if first := snap.Classes[0]; first.SiteID != "BG3242661710017001" || first.Season != 2023 {
		t.Errorf("first class = %v, want the sorted first", first)
	}
}
