package web

import (
	"strings"
	"testing"
	"time"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
)

// acFixture is the SEO6 test snapshot: a covered city inside its oblast, a
// Sofia district inside city "sofia" inside "sofiya-grad-oblast", a second
// district for the nearest-list, an oblast with no city child
// ("sofiyska-oblast", per the plan's per-kind link table), and an uncovered
// city for the no-data example.
func acFixture() *snapshot.Snapshot {
	return &snapshot.Snapshot{
		GeneratedAt: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC),
		KnownSlugs: map[string]snapshot.AreaMeta{
			"plovdiv-oblast": {Slug: "plovdiv-oblast", Kind: "oblast", NameBG: "Пловдив", NameEN: "Plovdiv",
				CentroidLon: 24.75, CentroidLat: 42.14, DefaultZoom: 9, Covered: true, SensorCount: 137},
			"plovdiv": {Slug: "plovdiv", Kind: "city", NameBG: "Пловдив", NameEN: "Plovdiv",
				ParentSlug: "plovdiv-oblast", CentroidLon: 24.75, CentroidLat: 42.14, DefaultZoom: 11,
				Covered: true, SensorCount: 5, Values: map[string]float64{"P2": 12.3}},
			"sofiya-grad-oblast": {Slug: "sofiya-grad-oblast", Kind: "oblast", NameBG: "София-град", NameEN: "Sofia-City",
				CentroidLon: 23.32, CentroidLat: 42.69, DefaultZoom: 9, Covered: true, SensorCount: 412},
			"sofia": {Slug: "sofia", Kind: "city", NameBG: "София", NameEN: "Sofia",
				ParentSlug: "sofiya-grad-oblast", CentroidLon: 23.32, CentroidLat: 42.69, DefaultZoom: 11,
				Covered: true, SensorCount: 30, Values: map[string]float64{"P2": 8.0}},
			"mladost": {Slug: "mladost", Kind: "neighbourhood", NameBG: "Младост", NameEN: "Mladost",
				ParentSlug: "sofia", CentroidLon: 23.37, CentroidLat: 42.65, DefaultZoom: 13,
				Covered: true, SensorCount: 4, Values: map[string]float64{"P2": 6.0}},
			"lozenets": {Slug: "lozenets", Kind: "neighbourhood", NameBG: "Лозенец", NameEN: "Lozenets",
				ParentSlug: "sofia", CentroidLon: 23.32, CentroidLat: 42.68, DefaultZoom: 13,
				Covered: true, SensorCount: 6, Values: map[string]float64{"P2": 9.5}},
			"sofiyska-oblast": {Slug: "sofiyska-oblast", Kind: "oblast", NameBG: "Софийска", NameEN: "Sofia",
				CentroidLon: 23.7, CentroidLat: 42.5, DefaultZoom: 9, Covered: true, SensorCount: 40},
			"silistra-oblast": {Slug: "silistra-oblast", Kind: "oblast", NameBG: "Силистра", NameEN: "Silistra",
				CentroidLon: 27.26, CentroidLat: 44.12, DefaultZoom: 9, Covered: false, SensorCount: 0},
			"silistra": {Slug: "silistra", Kind: "city", NameBG: "Силистра", NameEN: "Silistra",
				ParentSlug: "silistra-oblast", CentroidLon: 27.26, CentroidLat: 44.12, DefaultZoom: 11,
				Covered: false, SensorCount: 0},
			"kremikovtsi": {Slug: "kremikovtsi", Kind: "neighbourhood", NameBG: "Кремиковци", NameEN: "Kremikovtsi",
				ParentSlug: "sofia", CentroidLon: 23.5, CentroidLat: 42.75, DefaultZoom: 13,
				Covered: false, SensorCount: 1},
		},
	}
}

func acRenderer(t *testing.T) *Renderer {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	t.Setenv(config.DatabaseURLEnv, "postgres://user:pass@localhost:5432/airbg")
	cfg, err := config.LoadFile("../../airbg.yaml")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	cfg.Listen.BaseURL = "https://airbg.org"
	h := snapshot.NewHolder(cfg.Series, config.Wind{})
	h.Store(acFixture())
	rr, err := NewRenderer(cat, h, cfg)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	return rr
}

// acPageData builds a PageData carrying the catalogue and Sofia zone a real
// request would have, with GeneratedAt/Now pinned independently so staleness
// is a deliberate input rather than a race against the wall clock.
func acPageData(rr *Renderer, lang string, generatedAt, now time.Time) PageData {
	return PageData{
		Lang: lang, DefaultMetric: "P2",
		GeneratedAt: generatedAt, Now: now,
		cat: rr.cat, sofiaLoc: rr.sofiaLoc,
	}
}

func TestAreaNowSentenceCovered(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()
	t0 := snap.GeneratedAt
	p := acPageData(rr, i18n.DefaultLang, t0, t0)
	row := rr.rowFrom(snap.KnownSlugs["plovdiv"], i18n.DefaultLang)

	got := string(p.areaNowHTML(row))
	wants := []string{"Пловдив", "12,3", "µg/m³", "задоволително", "5 сензора", `<time datetime="`}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("areaNowHTML() = %q, want substring %q", got, want)
		}
	}
}

func TestAreaNowBandBoundaries(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{0, "good"}, {5, "good"},
		{5.1, "fair"}, {15, "fair"},
		{15.1, "moderate"}, {50, "moderate"},
		{50.1, "poor"}, {90, "poor"},
		{90.1, "very_poor"}, {140, "very_poor"},
		{140.1, "extremely_poor"}, {1000, "extremely_poor"},
	}
	for _, c := range cases {
		if got := bandLabel("P2", c.value); got != c.want {
			t.Errorf("bandLabel(P2, %v) = %q, want %q", c.value, got, c.want)
		}
	}
}

func TestAreaNowPlural(t *testing.T) {
	rr := acRenderer(t)
	p := acPageData(rr, i18n.DefaultLang, time.Now(), time.Now())
	if got := p.sensorsText(1); got != "1 сензор" {
		t.Errorf("sensorsText(1) = %q, want %q", got, "1 сензор")
	}
	if got := p.sensorsText(5); got != "5 сензора" {
		t.Errorf("sensorsText(5) = %q, want %q", got, "5 сензора")
	}

	pEn := acPageData(rr, "en", time.Now(), time.Now())
	if got := pEn.sensorsText(1); got != "1 sensor" {
		t.Errorf("en sensorsText(1) = %q, want %q", got, "1 sensor")
	}
	if got := pEn.sensorsText(5); got != "5 sensors" {
		t.Errorf("en sensorsText(5) = %q, want %q", got, "5 sensors")
	}
}

// TestAreaNowStates walks the four-state table: fresh+covered, stale+covered,
// no sensors at all, and "few" (some sensors, no default-metric value / not
// enough for coverage).
func TestAreaNowStates(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()
	t0 := snap.GeneratedAt
	fresh := t0
	stale := t0.Add(31 * time.Minute)

	covered := rr.rowFrom(snap.KnownSlugs["plovdiv"], i18n.DefaultLang)
	noSensors := rr.rowFrom(snap.KnownSlugs["silistra"], i18n.DefaultLang)
	few := AreaRow{Name: "Тест", Kind: "city", SensorCount: 2, Covered: false, HasValue: false}
	noMetric := AreaRow{Name: "Тест", Kind: "city", SensorCount: 137, Covered: true, HasValue: false}

	cases := []struct {
		name string
		row  AreaRow
		now  time.Time
		key  string
	}{
		{"fresh covered", covered, fresh, "сега:"},
		{"stale covered", covered, stale, "последните данни са от"},
		{"no sensors", noSensors, fresh, "няма сензор със скорошни данни."},
		{"few", few, fresh, "със скорошни данни. Стойност"},
		{"covered, no value for the metric", noMetric, fresh, "137 сензора със скорошни данни, но нито един не измерва ФПЧ2.5."},
	}
	for _, c := range cases {
		p := acPageData(rr, i18n.DefaultLang, t0, c.now)
		got := string(p.areaNowHTML(c.row))
		if !strings.Contains(got, c.key) {
			t.Errorf("%s: areaNowHTML() = %q, want substring %q", c.name, got, c.key)
		}
	}

	// Uncovered area, stale clock: the fifth state, area.now_none_stale.
	p := acPageData(rr, i18n.DefaultLang, t0, stale)
	got := string(p.areaNowHTML(noSensors))
	if !strings.Contains(got, "няма сензор със скорошни данни към") {
		t.Errorf("uncovered+stale areaNowHTML() = %q, want the none_stale wording", got)
	}
}

// bg time.layout ends in "ч.", so a sentence ending in {time} must not add a second period.
func TestAreaNowNoDoublePeriodAfterTime(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()
	t0 := snap.GeneratedAt
	covered := rr.rowFrom(snap.KnownSlugs["plovdiv"], i18n.DefaultLang)
	noSensors := rr.rowFrom(snap.KnownSlugs["silistra"], i18n.DefaultLang)
	for _, lang := range []string{"bg", "en"} {
		for _, c := range []struct {
			name string
			row  AreaRow
			now  time.Time
		}{{"fresh covered", covered, t0}, {"uncovered stale", noSensors, t0.Add(31 * time.Minute)}} {
			got := string(acPageData(rr, lang, t0, c.now).areaNowHTML(c.row))
			if strings.Contains(got, "..") || strings.Contains(got, ".</time>.") {
				t.Errorf("%s %s: areaNowHTML() = %q has a double period", lang, c.name, got)
			}
			if text := strings.NewReplacer("</time>", "", "</p>", "").Replace(got); !strings.HasSuffix(text, ".") {
				t.Errorf("%s %s: areaNowHTML() = %q does not end the sentence", lang, c.name, got)
			}
		}
	}
}

func TestAreaDayRange(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()
	t0 := snap.GeneratedAt

	withDay := rr.rowFrom(snap.KnownSlugs["plovdiv"], i18n.DefaultLang)
	withDay.Day = &snapshot.DayRange{
		Min: 4.0, Max: 18.5,
		MinAt: t0.Add(-2 * time.Hour), MaxAt: t0.Add(-20 * time.Hour),
		Buckets: 40,
	}
	noDay := rr.rowFrom(snap.KnownSlugs["plovdiv"], i18n.DefaultLang) // Day stays nil: <6h or never built

	p := acPageData(rr, i18n.DefaultLang, t0, t0)
	got := string(p.areaDayHTML(withDay))
	for _, want := range []string{"4,0", "18,5", "µg/m³"} {
		if !strings.Contains(got, want) {
			t.Errorf("areaDayHTML() = %q, want substring %q", got, want)
		}
	}
	if got := p.areaDayHTML(noDay); got != "" {
		t.Errorf("areaDayHTML() with nil Day = %q, want empty", got)
	}

	// Stale clock suppresses the day sentence even with a Day present.
	stale := acPageData(rr, i18n.DefaultLang, t0, t0.Add(31*time.Minute))
	if got := stale.areaDayHTML(withDay); got != "" {
		t.Errorf("areaDayHTML() while stale = %q, want empty", got)
	}
}

func TestAreaOblastSubjectOverrides(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()
	p := acPageData(rr, i18n.DefaultLang, snap.GeneratedAt, snap.GeneratedAt)

	sgo := rr.rowFrom(snap.KnownSlugs["sofiya-grad-oblast"], i18n.DefaultLang)
	if got := p.areaSubject(sgo); got != "Област София-град" {
		t.Errorf("areaSubject(sofiya-grad-oblast) = %q, want %q", got, "Област София-град")
	}
	so := rr.rowFrom(snap.KnownSlugs["sofiyska-oblast"], i18n.DefaultLang)
	if got := p.areaSubject(so); got != "Софийска област" {
		t.Errorf("areaSubject(sofiyska-oblast) = %q, want %q", got, "Софийска област")
	}
	pd := rr.rowFrom(snap.KnownSlugs["plovdiv-oblast"], i18n.DefaultLang)
	if got := p.areaSubject(pd); got != "Област Пловдив" {
		t.Errorf("areaSubject(plovdiv-oblast) = %q, want %q", got, "Област Пловдив")
	}
	district := rr.rowFrom(snap.KnownSlugs["mladost"], i18n.DefaultLang)
	if got := p.areaSubject(district); got != "Район Младост" {
		t.Errorf("areaSubject(mladost) = %q, want %q", got, "Район Младост")
	}
}

func TestAreaTierDistrict(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()
	row := rr.rowFrom(snap.KnownSlugs["mladost"], i18n.DefaultLang)
	p := acPageData(rr, i18n.DefaultLang, snap.GeneratedAt, snap.GeneratedAt)
	p.Area = &row
	if got := p.AreaTier(); got != "медиана за района" {
		t.Errorf("AreaTier() for a district = %q, want %q", got, "медиана за района")
	}
}

// TestAreaLinksPerKind checks the per-kind table in buildAreaLinks's doc
// comment: an oblast's children block names its областен град, a city named
// "sofia" gets the districts block instead, a district's parent chain is
// city-then-oblast, and an oblast with no city child gets no children block.
func TestAreaLinksPerKind(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()

	// Oblast: children is the one city; no parent block.
	poMeta := snap.KnownSlugs["plovdiv-oblast"]
	poRow := rr.rowFrom(poMeta, i18n.DefaultLang)
	p := acPageData(rr, i18n.DefaultLang, snap.GeneratedAt, snap.GeneratedAt)
	rr.buildAreaLinks(&p, poMeta, poRow, snap, i18n.DefaultLang)
	if p.AreaParentBlock != nil {
		t.Error("oblast page has a parent block, want none")
	}
	if p.AreaChildrenBlock == nil || len(p.AreaChildrenBlock.Items) != 1 || p.AreaChildrenBlock.Items[0].Slug != "plovdiv" {
		t.Errorf("oblast children block = %+v, want the one city plovdiv", p.AreaChildrenBlock)
	}

	// Oblast with no city child: no children block.
	soMeta := snap.KnownSlugs["sofiyska-oblast"]
	soRow := rr.rowFrom(soMeta, i18n.DefaultLang)
	p2 := acPageData(rr, i18n.DefaultLang, snap.GeneratedAt, snap.GeneratedAt)
	rr.buildAreaLinks(&p2, soMeta, soRow, snap, i18n.DefaultLang)
	if p2.AreaChildrenBlock != nil {
		t.Errorf("sofiyska-oblast children block = %+v, want nil (no city inside it)", p2.AreaChildrenBlock)
	}

	// City "sofia": children is the districts block, labelled area.children.districts.
	sofiaMeta := snap.KnownSlugs["sofia"]
	sofiaRow := rr.rowFrom(sofiaMeta, i18n.DefaultLang)
	p3 := acPageData(rr, i18n.DefaultLang, snap.GeneratedAt, snap.GeneratedAt)
	rr.buildAreaLinks(&p3, sofiaMeta, sofiaRow, snap, i18n.DefaultLang)
	if p3.AreaChildrenBlock == nil || p3.AreaChildrenBlock.Label != "Райони на София" {
		t.Errorf("sofia children block = %+v, want label %q", p3.AreaChildrenBlock, "Райони на София")
	}
	if p3.AreaParentBlock == nil || len(p3.AreaParentBlock.Items) != 1 || p3.AreaParentBlock.Items[0].Slug != "sofiya-grad-oblast" {
		t.Errorf("sofia parent block = %+v, want sofiya-grad-oblast", p3.AreaParentBlock)
	}

	// District "mladost": parent chain is sofia, then sofiya-grad-oblast; no children.
	mMeta := snap.KnownSlugs["mladost"]
	mRow := rr.rowFrom(mMeta, i18n.DefaultLang)
	p4 := acPageData(rr, i18n.DefaultLang, snap.GeneratedAt, snap.GeneratedAt)
	rr.buildAreaLinks(&p4, mMeta, mRow, snap, i18n.DefaultLang)
	if p4.AreaChildrenBlock != nil {
		t.Error("district page has a children block, want none")
	}
	if p4.AreaParentBlock == nil || len(p4.AreaParentBlock.Items) != 2 {
		t.Fatalf("mladost parent block = %+v, want a 2-item chain", p4.AreaParentBlock)
	}
	if p4.AreaParentBlock.Items[0].Slug != "sofia" || p4.AreaParentBlock.Items[1].Slug != "sofiya-grad-oblast" {
		t.Errorf("mladost parent chain slugs = %v, want [sofia sofiya-grad-oblast]",
			[]string{p4.AreaParentBlock.Items[0].Slug, p4.AreaParentBlock.Items[1].Slug})
	}
	// Nearest is the other covered district, lozenets (kremikovtsi is uncovered
	// but still same-kind, so nearestSameKind — ranked purely by distance,
	// coverage-blind — includes it too; both are same kind "neighbourhood").
	if p4.AreaNearestBlock == nil {
		t.Fatal("mladost nearest block is nil")
	}
	var sawLozenets bool
	for _, item := range p4.AreaNearestBlock.Items {
		if item.Slug == "lozenets" {
			sawLozenets = true
		}
		if item.Slug == "mladost" {
			t.Error("nearest block includes the area itself")
		}
	}
	if !sawLozenets {
		t.Errorf("mladost nearest block = %+v, want lozenets among the results", p4.AreaNearestBlock.Items)
	}

	// Breadcrumb chain for the district: Home, София, Област София-град, Младост.
	if len(p4.AreaCrumbs) != 4 {
		t.Fatalf("mladost breadcrumbs = %+v, want 4 crumbs", p4.AreaCrumbs)
	}
	wantNames := []string{"Карта", "Област София-град", "София", "Младост"}
	for i, want := range wantNames {
		if p4.AreaCrumbs[i].Name != want {
			t.Errorf("breadcrumb[%d].Name = %q, want %q", i, p4.AreaCrumbs[i].Name, want)
		}
	}
	if p4.AreaCrumbs[3].URL != "" {
		t.Errorf("last breadcrumb URL = %q, want empty (self, not a link)", p4.AreaCrumbs[3].URL)
	}
}

func TestAreasDirectoryLinksEveryArea(t *testing.T) {
	rr := acRenderer(t)
	snap := acFixture()
	groups := rr.buildDirectory(snap, i18n.DefaultLang)

	linked := map[string]bool{}
	for _, g := range groups {
		linked[g.Oblast.Slug] = true
		for _, c := range g.Cities {
			linked[c.Slug] = true
		}
		for _, d := range g.Districts {
			linked[d.Slug] = true
		}
	}
	for slug, meta := range snap.KnownSlugs {
		if !linked[slug] {
			t.Errorf("area %q (kind %s) is not linked from the /areas directory", slug, meta.Kind)
		}
	}
	// Sofia's districts are nested only under the sofia group, not duplicated
	// under every oblast — this directory has one city "sofia", so exactly one
	// group carries a non-empty Districts slice.
	withDistricts := 0
	for _, g := range groups {
		if len(g.Districts) > 0 {
			withDistricts++
			if g.Oblast.Slug != "sofiya-grad-oblast" {
				t.Errorf("group %q carries Districts, want only sofiya-grad-oblast to", g.Oblast.Slug)
			}
		}
	}
	if withDistricts != 1 {
		t.Errorf("groups carrying Districts = %d, want 1", withDistricts)
	}
}
