package varna

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"kanarche.eu/internal/upstream/rzi"
)

func FuzzVarna(f *testing.F) {
	f.Add(fixture(f, "varna-2026-excerpt.txt"))
	f.Add(fixture(f, "varna-2025-excerpt.txt"))
	f.Add(synth(f, "03002", row("2026-06-09", "2026-06-09", "15", "под 15", "бележка")))
	f.Add(synth(f, "03002", remarkAt("горе"), row("2026-06-09", "2026-06-09", "4 124", "34 659", "")))
	f.Add(head(f, "03002") + "заместваща проба\n")
	f.Add("")
	f.Add("\f\f№03001\n")
	f.Fuzz(func(t *testing.T, text string) {
		out := Parse(text)
		if out.Warnings == nil {
			t.Fatal("nil warnings")
		}
		if len(out.Warnings) > maxWarnings+1 {
			t.Fatalf("%d warnings", len(out.Warnings))
		}
		if len(out.Blocks) > rzi.MaxBlocks {
			t.Fatalf("%d blocks", len(out.Blocks))
		}
		for _, b := range out.Blocks {
			if len(b.Rows) == 0 || len(b.Rows) > rzi.MaxRowsPerBlock {
				t.Fatalf("block %s has %d rows", b.Point, len(b.Rows))
			}
			for _, r := range b.Rows {
				if r.Unit != "MPN/100ml" || r.IE.Value < 0 || r.EC.Value < 0 {
					t.Fatalf("bad row %+v", r)
				}
			}
		}
		for _, w := range out.Warnings {
			if len(w) > 200 {
				t.Fatalf("long warning %q", w)
			}
		}
		raw, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rzi.Decode(bytes.NewReader(raw)); err != nil && strings.TrimSpace(out.Error) == "" && len(out.Warnings) == 0 {
			t.Fatalf("clean output does not decode: %v", err)
		}
	})
}
