package burgas

import (
	"bytes"
	"os"
	"testing"
)

// controlFixture is the positive control for the docparse image tests
// (internal/docparse/integration). It is committed so the image test can use
// this package's workbook builder without importing a _test.go file.
const controlFixture = "../testdata/burgas-control.xls"

func controlBook() []byte {
	return buildCFB(buildStream(bookOpts{cells: std([]trow{
		ok(46160, 46160, 15, 15),
		ok(46174, 46177, 77, 46),
	})}), cfbOpts{})
}

// TestControlFixtureIsCurrent fails when the committed file drifts from the
// builder. RZI_WRITE_FIXTURE=1 rewrites it.
func TestControlFixtureIsCurrent(t *testing.T) {
	want := controlBook()
	if os.Getenv("RZI_WRITE_FIXTURE") == "1" {
		if err := os.WriteFile(controlFixture, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(controlFixture)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; rerun with RZI_WRITE_FIXTURE=1", controlFixture)
	}
	f := Extract(got)
	if f.Error != "" || len(f.Blocks) != 1 || len(f.Blocks[0].Rows) != 2 {
		t.Fatalf("control fixture extracts to error %q blocks %+v", f.Error, f.Blocks)
	}
}
