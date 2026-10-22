package web

import (
	"kanarche.eu/internal/i18n"
	"strings"
	"testing"
)

// A bare PageData has no AboutFAQ; datasetJSONLD must still work and leave the FAQPage out.
func TestDatasetJSONLDWithoutAboutFAQ(t *testing.T) {
	cat, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	js, err := PageData{cat: cat, Lang: "en"}.datasetJSONLD()
	if err != nil {
		t.Fatalf("datasetJSONLD: %v", err)
	}
	if !strings.Contains(string(js), `"Dataset"`) {
		t.Errorf("no Dataset node: %s", js)
	}
	if strings.Contains(string(js), "FAQPage") {
		t.Errorf("FAQPage present without AboutFAQ: %s", js)
	}
}
