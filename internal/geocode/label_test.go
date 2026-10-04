package geocode_test

import (
	"strings"
	"testing"

	"airbg.org/internal/geocode"
)

// Two OSM ways of one street with different postcodes and adjacent bboxes.
const salzaBody = `[
 {"name":"Salza","display_name":"Salza, кв. Любово, Габрово град, Gabrovo, 5310, Bulgaria","lat":"42.8700","lon":"25.3100",
  "boundingbox":["42.8690","42.8710","25.3090","25.3110"],
  "address":{"road":"Salza","suburb":"кв. Любово","municipality":"Габрово град","city":"Gabrovo","postcode":"5310","country":"Bulgaria","country_code":"bg"}},
 {"name":"Salza","display_name":"Salza, кв. Любово, Габрово град, Gabrovo, 5301, Bulgaria","lat":"42.8720","lon":"25.3130",
  "boundingbox":["42.8710","42.8730","25.3110","25.3140"],
  "address":{"road":"Salza","suburb":"кв. Любово","municipality":"Габрово град","city":"Gabrovo","postcode":"5301","country":"Bulgaria","country_code":"bg"}}
]`

func TestSearchAsksForAddressDetailsAndMoreRows(t *testing.T) {
	up := newStub(t, 200, salzaBody)
	if _, err := geocode.New(cfgFor(up.URL)).Search(t.Context(), "salza 6 gabrovo", "en"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	q := up.last.Load().URL.Query()
	if q.Get("addressdetails") != "1" || q.Get("limit") != "8" {
		t.Errorf("addressdetails=%q limit=%q, want 1 and 8", q.Get("addressdetails"), q.Get("limit"))
	}
}

func TestSameStreetWaysMergeIntoOneResult(t *testing.T) {
	up := newStub(t, 200, salzaBody)
	res, err := geocode.New(cfgFor(up.URL)).Search(t.Context(), "salza 6 gabrovo", "en")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("got %d results, want 1: %+v", len(res), res)
	}
	r := res[0]
	if r.Label != "Salza, кв. Любово, Gabrovo, Габрово град" {
		t.Errorf("label = %q", r.Label)
	}
	for _, bad := range []string{"5310", "5301", "Bulgaria"} {
		if strings.Contains(r.Label, bad) {
			t.Errorf("label %q still carries %q", r.Label, bad)
		}
	}
	// W,S,E,N of the union; the point stays the first way's own point.
	if want := [4]float64{25.309, 42.869, 25.314, 42.873}; r.BBox != want {
		t.Errorf("bbox = %v, want %v", r.BBox, want)
	}
	if r.Lat != 42.87 || r.Lon != 25.31 {
		t.Errorf("point = %v,%v, want the first result's", r.Lat, r.Lon)
	}
}

func TestMergeKeepsFirstSeenOrderAndTrimsToFive(t *testing.T) {
	var rows []string
	// a, b, a, c, d, e, f, g: merging leaves a b c d e f g, trimmed to five.
	for _, n := range []string{"a", "b", "a", "c", "d", "e", "f", "g"} {
		rows = append(rows, `{"display_name":"`+n+`, X","lat":"42","lon":"23","boundingbox":["42","42","23","23"],"address":{"road":"`+n+`","city":"X"}}`)
	}
	up := newStub(t, 200, "["+strings.Join(rows, ",")+"]")
	res, err := geocode.New(cfgFor(up.URL)).Search(t.Context(), "many", "en")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	var got []string
	for _, r := range res {
		got = append(got, r.Label)
	}
	if want := "a, X|b, X|c, X|d, X|e, X"; strings.Join(got, "|") != want {
		t.Errorf("labels = %v, want %s", got, want)
	}
}

func TestLabelRules(t *testing.T) {
	cases := []struct {
		name, row, want string
	}{
		{"house number and road, suburb, city",
			`"display_name":"d","address":{"house_number":"6","road":"ул. Сълза","suburb":"Любово","city":"Габрово","postcode":"5300","country":"България"}`,
			"ул. Сълза 6, Любово, Габрово"},
		{"municipality or county repeating the city adds nothing",
			`"display_name":"d","address":{"road":"Vitosha Blvd","city":"Sofia","municipality":"Sofia","county":"Sofia-grad"}`,
			"Vitosha Blvd, Sofia"},
		{"municipality containing the city adds nothing",
			`"display_name":"d","address":{"road":"Salza","town":"Gabrovo","municipality":"Gabrovo Municipality"}`,
			"Salza, Gabrovo"},
		{"municipality that adds information is kept",
			`"display_name":"d","address":{"road":"Main","village":"Dolna","municipality":"Pernik"}`,
			"Main, Dolna, Pernik"},
		{"quarter used when no suburb; village as locality",
			`"display_name":"d","address":{"road":"Main","quarter":"Q1","village":"Dolna Banya"}`,
			"Main, Q1, Dolna Banya"},
		{"place without a road uses its name",
			`"display_name":"d","name":"Rila Monastery","address":{"village":"Rila","municipality":"Rila","country":"Bulgaria"}`,
			"Rila Monastery"},
		{"no address falls back to display_name",
			`"display_name":"Some place, 1000, Bulgaria"`,
			"Some place, 1000, Bulgaria"},
		{"address without head falls back to display_name",
			`"display_name":"Whole region, Bulgaria","address":{"country":"Bulgaria","postcode":"1"}`,
			"Whole region, Bulgaria"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := `[{` + c.row + `,"lat":"42","lon":"23","boundingbox":["42","42","23","23"]}]`
			up := newStub(t, 200, body)
			res, err := geocode.New(cfgFor(up.URL)).Search(t.Context(), "q q q", "en")
			if err != nil || len(res) != 1 {
				t.Fatalf("Search: %v, %d results", err, len(res))
			}
			if res[0].Label != c.want {
				t.Errorf("label = %q, want %q", res[0].Label, c.want)
			}
		})
	}
}

func TestMergeIgnoresMissingBBoxes(t *testing.T) {
	body := `[{"display_name":"d","lat":"42","lon":"23","address":{"road":"R","city":"C"}},
	 {"display_name":"d","lat":"42.1","lon":"23.1","boundingbox":["42","42.2","23","23.2"],"address":{"road":"R","city":"C"}}]`
	up := newStub(t, 200, body)
	res, err := geocode.New(cfgFor(up.URL)).Search(t.Context(), "r c", "en")
	if err != nil || len(res) != 1 {
		t.Fatalf("Search: %v, %d results", err, len(res))
	}
	if want := [4]float64{23, 42, 23.2, 42.2}; res[0].BBox != want {
		t.Errorf("bbox = %v, want %v", res[0].BBox, want)
	}
}
