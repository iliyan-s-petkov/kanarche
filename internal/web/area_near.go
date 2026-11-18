package web

import (
	"math"
	"strconv"

	"kanarche.eu/internal/snapshot"
)

// nearbyCount is how many neighbours the nearby sentence lists, and nearbyMin the fewest it needs.
const (
	nearbyCount = 3
	nearbyMin   = 2
)

// densityPer100 is citizen sensors per 100 km2, or false without a size or a citizen sensor.
func densityPer100(meta snapshot.AreaMeta) (float64, bool) {
	n := citizenSensors(meta)
	if meta.Km2 <= 0 || n == 0 {
		return 0, false
	}
	return float64(n) / meta.Km2 * 100, true
}

// densityFact is how dense the citizen sensor network is in the boundary, with its place among the same kind
// when at least minRankPeers areas of that kind have a size and a citizen sensor.
func (p PageData) densityFact(meta snapshot.AreaMeta, snap *snapshot.Snapshot) string {
	d, ok := densityPer100(meta)
	if !ok {
		return ""
	}
	parts := []string{p.factText("area.facts.density.line", "{density}", formatMeasure(d, p.Lang))}
	var all []float64
	for _, m := range peersOf(snap, meta) {
		if pd, has := densityPer100(m); has {
			all = append(all, pd)
		}
	}
	if len(all) >= minRankPeers && p.cat.Has(p.Lang, "area.facts.kind."+meta.Kind) {
		parts = append(parts, p.factText("area.facts.density.rank",
			"{rank}", ordinal(p.Lang, rankDescending(d, all)),
			"{total}", strconv.Itoa(len(all)),
			"{kind}", p.factText(districtKey("area.facts.kind."+meta.Kind, p.Area.DistrictCity))))
	}
	return joinSentences(parts)
}

// nearbyFact lists the nearest areas of the same kind that have a PM2.5 value, each with distance, direction and
// value, and says how many of them have cleaner air now. It is empty with fewer than nearbyMin such areas.
func (rr *Renderer) nearbyFact(p PageData, meta snapshot.AreaMeta, snap *snapshot.Snapshot) string {
	if !p.cat.Has(p.Lang, "area.facts.kind_plain."+meta.Kind) {
		return ""
	}
	v, ok := meta.Values[rankMetric]
	if !ok {
		return ""
	}
	// Ask for every same-kind area: the nearest ones may have no value, and the list skips those.
	var items []string
	cleaner := 0
	for _, row := range rr.nearestSameKind(meta, snap, p.Lang, len(snap.KnownSlugs)) {
		m := snap.KnownSlugs[row.Slug]
		nv, has := m.Values[rankMetric]
		if !has {
			continue
		}
		km := haversineKM(meta.CentroidLat, meta.CentroidLon, m.CentroidLat, m.CentroidLon)
		dir := p.T(directionKeys[bearingSector(meta.CentroidLat, meta.CentroidLon, m.CentroidLat, m.CentroidLon)])
		items = append(items, p.factText("area.facts.near.item",
			"{name}", row.Name, "{km}", formatMeasure(km, p.Lang), "{dir}", dir, "{value}", formatValue(nv, p.Lang)))
		// Compare at the one decimal the page prints, so two equal printed values do not count as cleaner.
		if math.Round(nv*10) < math.Round(v*10) {
			cleaner++
		}
		if len(items) == nearbyCount {
			break
		}
	}
	if len(items) < nearbyMin {
		return ""
	}
	var verdict string
	switch {
	case cleaner == 0:
		verdict = p.factText("area.facts.near.none")
	case cleaner == len(items):
		verdict = p.factText("area.facts.near.all")
	default:
		verdict = p.factText("area.facts.near.cleaner."+pluralOf(cleaner, "one", "other"), "{n}", strconv.Itoa(cleaner))
	}
	kindPlain := p.factText(districtKey("area.facts.kind_plain."+meta.Kind, p.Area.DistrictCity))
	return joinSentences([]string{
		p.factText("area.facts.near.lead", "{kind_plain}", kindPlain, "{list}", p.joinNames(items)),
		verdict,
	})
}
