package burgas

import "testing"

// Real blocks exist where the date title cell is blank.
func TestBlankDateTitle(t *testing.T) {
	var cells []tcell
	for _, c := range std([]trow{ok(46160, 46160, 15, 15)}) {
		if !(c.row == 5 && c.col == 1) {
			cells = append(cells, c)
		}
	}
	f := extract(t, bookOpts{cells: cells})
	if f.Error != "" || len(f.Warnings) != 0 || len(f.Blocks) != 1 || f.Blocks[0].Rows[0].Date != "2026-05-18" {
		t.Fatalf("%+v", f)
	}
}
