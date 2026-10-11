package varna

import (
	"strings"
	"testing"
)

// A value that straddles the IE/EC boundary has no single column.
func TestValueCrossingColumnBoundaryRefused(t *testing.T) {
	d := "2026-06-09"
	ok := Parse(synth(t, "03002", row(d, d, "15", "15", "")))
	if _, found := findBlock(ok, "03002"); !found {
		t.Fatalf("control row refused: %v", ok.Warnings)
	}
	straddle := line(cellAt{colPlanned, d}, cellAt{colDate, d}, cellAt{51, "150"}, cellAt{colEC, "15"})
	mustDrop(t, Parse(synth(t, "03002", straddle)), "03002")
}

func TestRemarkCellTooTall(t *testing.T) {
	d := "2026-06-09"
	rows := []string{row(d, d, "15", "15", "t")}
	for i := 0; i < maxRunLines; i++ {
		rows = append(rows, remarkAt("t"))
	}
	mustDrop(t, Parse(synth(t, "03002", rows...)), "03002")
	rows = rows[:maxRunLines]
	if _, ok := findBlock(Parse(synth(t, "03002", rows...)), "03002"); !ok {
		t.Fatal("remark cell at the line cap refused")
	}
}

func TestCoordinateMinutesAndSecondsRange(t *testing.T) {
	src := fixture(t, "varna-2026-excerpt.txt")
	good := `N .43 17'23'' . дължина E 28002'49''`
	for _, bad := range []string{
		`N .42 60'00'' . дължина E 28002'49''`,
		`N .43 17'23'' . дължина E 27060'49''`,
	} {
		mustDrop(t, Parse(mutate(t, src, good, bad)), "03002")
	}
}

// The second block and all its rows move together, so only the drift
// rule can tell.
func TestColumnDriftWholeHeader(t *testing.T) {
	lines := strings.Split(fixture(t, "varna-2026-excerpt.txt"), "\n")
	seen := 0
	for i, l := range lines {
		if strings.Contains(l, "Планирана дата за") {
			seen++
			if seen == 2 {
				for j := i; j < len(lines) && !strings.ContainsRune(lines[j], '\f'); j++ {
					lines[j] = strings.Repeat(" ", 8) + lines[j]
				}
			}
		}
	}
	f := Parse(strings.Join(lines, "\n"))
	mustDrop(t, f, "03011")
	if _, ok := findBlock(f, "03002"); !ok {
		t.Fatal("first block lost")
	}
}

// The byte cap must trip before the line caps do.
func TestInputByteCapNotLineCap(t *testing.T) {
	filler := strings.Repeat(strings.Repeat(" ", 100)+"\n", MaxInputBytes/101+2)
	f := Parse(fixture(t, "varna-2026-excerpt.txt") + filler)
	if f.Error != "input over size cap" {
		t.Fatalf("error = %q", f.Error)
	}
}
