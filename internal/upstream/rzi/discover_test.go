package rzi

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func page(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "pages", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustSpec(t *testing.T, id string) Spec {
	t.Helper()
	spec, ok := specFor(id)
	if !ok {
		t.Fatalf("no spec for %s", id)
	}
	return spec
}

func TestDiscoverPerSource(t *testing.T) {
	tests := []struct {
		id, file, base, year, want string
	}{
		{SourceVarna, "varna-section.html", "https://rzi-varna.com/?page_id=2635", "2026",
			"https://www.rzi-varna.com/health/mvodi-2026.pdf"},
		{SourceVarna, "varna-section.html", "https://rzi-varna.com/?page_id=2635", "2025",
			"https://www.rzi-varna.com/health/mvodi-2025.pdf"},
		{SourceBurgas, "burgas-section.html", "https://rzi-burgas.bg/kachestvo-na-vodite/", "2026",
			"https://rzi-burgas.bg/wp-content/uploads/2026/09/tablici-monitoring-more-mz-2026.xls"},
		// Irregular names the site has used: -20249 and -2023-1.
		{SourceBurgas, "burgas-section.html", "https://rzi-burgas.bg/kachestvo-na-vodite/", "2024",
			"https://rzi-burgas.bg/wp-content/uploads/2024/10/tablici-monitoring-more-mz-20249.xls"},
		{SourceBurgas, "burgas-section.html", "https://rzi-burgas.bg/kachestvo-na-vodite/", "2023",
			"https://rzi-burgas.bg/wp-content/uploads/2023/09/tablici-monitoring-more-mz-2023-1.xls"},
		// The page links /wcm/myconnect/ (a login redirect to an internal host).
		// The fetcher asks /wcm/connect/ and drops the CONVERT_TO parameter.
		{SourceDobrich, "dobrich-section.html", "https://rzi-dobrich.egov.bg/wps/portal/rzi.dobrich/main-activities/public-health/swimming-zone", "2026",
			"https://rzi-dobrich.egov.bg/wps/wcm/connect/rzi-dobrich.egov.bg-3026/d340442c-83a4-4708-b547-0143ce079330/RZI-Dobrich-morski+vodi+2026-Prilojenie+2.xlsx?MOD=AJPERES"},
	}
	for _, tt := range tests {
		t.Run(tt.id+"/"+tt.year, func(t *testing.T) {
			base, _ := url.Parse(tt.base)
			got, err := discover(page(t, tt.file), base, mustSpec(t, tt.id), tt.year)
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != tt.want {
				t.Errorf("got %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestDiscoverIgnoresSchedulePDF(t *testing.T) {
	// grafik_mvodi-2026.pdf sits next to mvodi-2026.pdf and must not count.
	base, _ := url.Parse("https://rzi-varna.com/?page_id=2635")
	got, err := discover(page(t, "varna-section.html"), base, mustSpec(t, SourceVarna), "2026")
	if err != nil || strings.Contains(got.Path, "grafik") {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDiscoverNoMatch(t *testing.T) {
	base, _ := url.Parse("https://rzi-varna.com/?page_id=2635")
	_, err := discover(page(t, "varna-section.html"), base, mustSpec(t, SourceVarna), "2031")
	if !errors.Is(err, ErrNoMatch) {
		t.Fatalf("err = %v, want ErrNoMatch", err)
	}
}

func TestDiscoverTwoMatches(t *testing.T) {
	html := `<a href="https://www.rzi-varna.com/health/mvodi-2026.pdf">a</a>
<a href="https://www.rzi-varna.com/old/mvodi-2026.pdf">b</a>`
	base, _ := url.Parse("https://rzi-varna.com/?page_id=2635")
	_, err := discover([]byte(html), base, mustSpec(t, SourceVarna), "2026")
	if !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("err = %v, want ErrAmbiguous", err)
	}
}

func TestDiscoverSameLinkTwiceIsOneMatch(t *testing.T) {
	html := `<a href="/health/mvodi-2026.pdf">a</a><a href="/health/mvodi-2026.pdf">b</a>`
	base, _ := url.Parse("https://www.rzi-varna.com/x")
	if _, err := discover([]byte(html), base, mustSpec(t, SourceVarna), "2026"); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverRefusesForeignHost(t *testing.T) {
	html := `<a href="https://evil.example/health/mvodi-2026.pdf">a</a>`
	base, _ := url.Parse("https://rzi-varna.com/?page_id=2635")
	_, err := discover([]byte(html), base, mustSpec(t, SourceVarna), "2026")
	if !errors.Is(err, ErrHost) {
		t.Fatalf("err = %v, want ErrHost", err)
	}
}

func TestDiscoverRefusesPlainHTTP(t *testing.T) {
	html := `<a href="http://www.rzi-varna.com/health/mvodi-2026.pdf">a</a>`
	base, _ := url.Parse("https://rzi-varna.com/?page_id=2635")
	_, err := discover([]byte(html), base, mustSpec(t, SourceVarna), "2026")
	if !errors.Is(err, ErrHost) {
		t.Fatalf("err = %v, want ErrHost", err)
	}
}

func TestDiscoverErrorsNeverEchoPage(t *testing.T) {
	html := `<a href="https://evil.example/health/mvodi-2026.pdf">SECRETTEXT</a>`
	base, _ := url.Parse("https://rzi-varna.com/?page_id=2635")
	_, err := discover([]byte(html), base, mustSpec(t, SourceVarna), "2026")
	if err == nil || strings.Contains(err.Error(), "evil") || strings.Contains(err.Error(), "SECRETTEXT") {
		t.Fatalf("err = %v", err)
	}
}
