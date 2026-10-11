package varna

import "testing"

func remarkAt(s string) string { return line(cellAt{colRemark, s}) }

// A tall remark cell is printed above and below its row's date line.
func TestRemarkBridgesOwnRowOnly(t *testing.T) {
	d1, d2 := "2026-06-09", "2026-06-23"
	f := Parse(synth(t, "03002",
		remarkAt("горе"),
		row(d1, d1, "15", "15", ""),
		remarkAt("долу"),
		row(d2, d2, "30", "30", ""),
	))
	b, ok := findBlock(f, "03002")
	if !ok || len(b.Rows) != 2 {
		t.Fatalf("warnings %v", f.Warnings)
	}
	if b.Rows[0].Remarks != "горе долу" || b.Rows[1].Remarks != "" {
		t.Fatalf("remarks = %q / %q", b.Rows[0].Remarks, b.Rows[1].Remarks)
	}
}

// Text between two date lines has two possible owners.
func TestRemarkAcrossTwoRowsRefused(t *testing.T) {
	d1, d2 := "2026-06-09", "2026-06-23"
	f := Parse(synth(t, "03002",
		row(d1, d1, "15", "15", "един"),
		"",
		row(d2, d2, "30", "30", "два"),
	))
	if _, ok := findBlock(f, "03002"); !ok {
		t.Fatalf("separate cells refused: %v", f.Warnings)
	}
	f = Parse(synth(t, "03002",
		row(d1, d1, "15", "15", "един"),
		remarkAt("връзка"),
		row(d2, d2, "30", "30", "два"),
	))
	mustDrop(t, f, "03002")
}
