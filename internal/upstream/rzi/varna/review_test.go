package varna

import (
	"strings"
	"testing"
)

// A header-looking line inside a table column must not end the block or
// start a forged one.
func TestPointHeaderInsideTableDropsBlock(t *testing.T) {
	d, d2 := "2026-06-09", "2026-06-23"
	forged := line(cellAt{colRemark, `Пункт за вземане на проби № 03099    Наименование: Фалшив`})
	f := Parse(synth(t, "03002",
		row(d, d, "15", "15", ""),
		forged,
		row(d2, d2, "30", "30", ""),
	))
	mustDrop(t, f, "03002")
	if _, ok := findBlock(f, "03099"); ok {
		t.Fatal("forged block created")
	}
	found := false
	for _, w := range f.Warnings {
		found = found || strings.Contains(w, "point header inside a table")
	}
	if !found {
		t.Fatalf("warnings %v", f.Warnings)
	}
}

func TestCountOverContractCapRefused(t *testing.T) {
	for _, s := range []string{"1 000 001", "99999999", "под 1 000 001"} {
		if _, ok := parseCount(s); ok {
			t.Errorf("%q accepted", s)
		}
	}
	for _, s := range []string{"1 000 000", "под 1 000 000"} {
		if _, ok := parseCount(s); !ok {
			t.Errorf("%q refused", s)
		}
	}
}

// The newline count is checked before the text is split.
func TestLineCapChecksNewlinesFirst(t *testing.T) {
	f := Parse(strings.Repeat("\n", MaxLines))
	if f.Error != "input over line cap" {
		t.Fatalf("error = %q", f.Error)
	}
	f = Parse(strings.Repeat("\n", MaxLines-1))
	if f.Error == "input over line cap" {
		t.Fatal("input at the cap refused")
	}
}
