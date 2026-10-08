package bathing_test

import (
	"bytes"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"kanarche.eu/internal/store"
	"kanarche.eu/internal/upstream/bathing"
)

const (
	siteA = "BG0000000000000001"
	siteB = "BG0000000000000002"
)

// base is what Build produced from Discodata: two active sites.
func base() store.BathingData {
	return store.BathingData{
		Sites: []store.BathingSite{{ID: siteA}, {ID: siteB}},
		Classes: []store.BathingClass{
			{SiteID: siteA, Season: 2023, Quality: "good", Source: store.SourceDiscodata},
			{SiteID: siteA, Season: 2024, Quality: "excellent", Source: store.SourceDiscodata},
			{SiteID: siteB, Season: 2024, Quality: "good", Source: store.SourceDiscodata},
		},
	}
}

func sup(cs ...bathing.SupplementClass) *bathing.Supplement {
	return &bathing.Supplement{Edition: "2025 v1.0", Classes: cs}
}

func sc(site string, season int, q string) bathing.SupplementClass {
	return bathing.SupplementClass{SiteID: site, Season: season, Quality: q}
}

// loose never trips a guard.
var loose = bathing.Guards{MaxDisagree: 1, MinSiteCoverage: 0.0001}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func classOf(d store.BathingData, site string, season int) (store.BathingClass, bool) {
	for _, c := range d.Classes {
		if c.SiteID == site && c.Season == season {
			return c, true
		}
	}
	return store.BathingClass{}, false
}

func TestSupplementFillsMissingSeasonOnly(t *testing.T) {
	d, st := bathing.Merge(base(), sup(
		sc(siteA, 2023, "good"),      // Discodata has it
		sc(siteA, 2025, "excellent"), // missing
		sc(siteB, 2025, "good"),      // missing
		sc(siteB, 2023, "poor"),      // missing, older than Discodata's newest
	), loose)
	if st.Applied != 3 || st.Shadowed != 1 {
		t.Errorf("stats = %+v, want 3 applied and 1 shadowed", st)
	}
	if len(d.Classes) != 6 {
		t.Fatalf("%d classes, want 3 from Discodata + 3 filled", len(d.Classes))
	}
	for _, k := range []struct {
		site   string
		season int
	}{{siteA, 2025}, {siteB, 2025}, {siteB, 2023}} {
		c, ok := classOf(d, k.site, k.season)
		if !ok || c.Source != store.SourceDatahub {
			t.Errorf("%s/%d = %+v, %v; want a datahub class", k.site, k.season, c, ok)
		}
	}
	if c, _ := classOf(d, siteA, 2023); c.Source != store.SourceDiscodata {
		t.Errorf("Discodata class relabelled: %+v", c)
	}
}

func TestDiscodataWinsOnOverlap(t *testing.T) {
	d, st := bathing.Merge(base(), sup(sc(siteA, 2024, "poor")), loose)
	c, _ := classOf(d, siteA, 2024)
	if c.Quality != "excellent" || c.Source != store.SourceDiscodata {
		t.Errorf("class = %+v, want Discodata's excellent", c)
	}
	if st.Applied != 0 || st.Shadowed != 1 || st.Disagree != 1 {
		t.Errorf("stats = %+v", st)
	}
}

func TestSupplementIgnoredForRetiredOrUnknownSite(t *testing.T) {
	// A retired site never reached d.Sites, so it looks like any unknown id.
	d, st := bathing.Merge(base(), sup(
		sc("BG0000000000000099", 2025, "good"),
		sc("BG0000000000000098", 2025, "good"),
		sc(siteA, 2025, "good"),
	), loose)
	if st.Applied != 1 || st.Inactive != 2 {
		t.Errorf("stats = %+v, want 1 applied and 2 inactive", st)
	}
	if _, ok := classOf(d, "BG0000000000000099", 2025); ok {
		t.Error("class for an unknown site was added")
	}
}

func TestSupplementEditionIsRecorded(t *testing.T) {
	d, _ := bathing.Merge(base(), sup(sc(siteA, 2025, "good")), loose)
	if d.SupplementEdition != "2025 v1.0" {
		t.Errorf("edition = %q", d.SupplementEdition)
	}
}

func TestNilSupplementIsToday(t *testing.T) {
	want := base()
	got, st := bathing.Merge(base(), nil, bathing.Guards{MaxDisagree: 0.005, MinSiteCoverage: 0.8})
	if !reflect.DeepEqual(got, want) || st != (bathing.SupplementStats{}) {
		t.Errorf("nil supplement changed the data: %+v, %+v", got, st)
	}
}

func TestMergeDoesNotMutateItsInput(t *testing.T) {
	in := base()
	bathing.Merge(in, sup(sc(siteA, 2025, "good")), loose)
	if !reflect.DeepEqual(in, base()) {
		t.Error("Merge changed its input")
	}
}

func TestSupplementDisagreementIsLoggedNotBlocking(t *testing.T) {
	log := captureLog(t)
	// 1 of 2 shared keys differs, limit 0.5%. Coverage is full.
	d, st := bathing.Merge(base(), sup(
		sc(siteA, 2024, "poor"), sc(siteB, 2024, "good"), sc(siteA, 2025, "good"), sc(siteB, 2025, "good"),
	), bathing.Guards{MaxDisagree: 0.005, MinSiteCoverage: 0.8})
	for _, want := range []string{"level=ERROR", "sea supplement guard: disagreement above limit", "shared=2", "disagree=1", "ratio=0.5", "max=0.005"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
	if strings.Contains(log.String(), "coverage") {
		t.Errorf("coverage logged though it is fine:\n%s", log)
	}
	if st.Applied != 2 || len(d.Classes) != 5 {
		t.Errorf("guard blocked the fill: %+v, %d classes", st, len(d.Classes))
	}
}

func TestSupplementLowCoverageIsLoggedNotBlocking(t *testing.T) {
	log := captureLog(t)
	// Newest season 2025 covers 1 of 2 active sites, limit 80%.
	d, st := bathing.Merge(base(), sup(sc(siteA, 2025, "good")),
		bathing.Guards{MaxDisagree: 0.005, MinSiteCoverage: 0.8})
	for _, want := range []string{"level=ERROR", "sea supplement guard: coverage below limit", "sites=2", "covered=1", "newest_season=2025", "ratio=0.5", "min=0.8"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
	if strings.Contains(log.String(), "disagreement") {
		t.Errorf("disagreement logged though there is none:\n%s", log)
	}
	if st.Applied != 1 || len(d.Classes) != 4 {
		t.Errorf("guard blocked the fill: %+v", st)
	}
}

func TestSupplementWithinLimitsLogsNothing(t *testing.T) {
	log := captureLog(t)
	bathing.Merge(base(), sup(
		sc(siteA, 2024, "excellent"), sc(siteA, 2025, "good"), sc(siteB, 2025, "good"),
	), bathing.Guards{MaxDisagree: 0.005, MinSiteCoverage: 0.8})
	if log.Len() != 0 {
		t.Errorf("unexpected log:\n%s", log)
	}
}

// Inactive sites do not count toward coverage: it is measured over kept sites.
func TestCoverageIgnoresInactiveSites(t *testing.T) {
	log := captureLog(t)
	bathing.Merge(base(), sup(
		sc(siteA, 2025, "good"), sc("BG0000000000000099", 2025, "good"),
	), bathing.Guards{MaxDisagree: 1, MinSiteCoverage: 0.8})
	if !strings.Contains(log.String(), "covered=1") {
		t.Errorf("an inactive site counted as covered:\n%s", log)
	}
}
