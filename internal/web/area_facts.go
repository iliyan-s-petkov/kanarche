package web

import (
	"sort"
	"strconv"
	"strings"

	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/upstream/eea"
)

// minRankPeers is the fewest same-kind areas a ranking sentence needs.
const minRankPeers = 5

// rankMetric is the metric the rank block orders by.
const rankMetric = "P2"

// municipalityCities are the 14 cities whose boundary is the whole municipality (about.cities.list).
var municipalityCities = map[string]bool{
	"blagoevgrad": true, "vratsa": true, "kardzhali": true, "lovech": true, "montana": true,
	"pazardzhik": true, "pernik": true, "pleven": true, "plovdiv": true, "sliven": true,
	"smolyan": true, "haskovo": true, "stara-zagora": true, "yambol": true,
}

// AreaFacts is the data-driven section above the FAQ: place, monitoring network and rank.
// An empty block is not rendered.
type AreaFacts struct {
	Heading string
	Place   string
	Network string
	Rank    string
	// rankFAQ is the rank block without its closing note, the start of the first FAQ answer.
	rankFAQ   string
	kindPlain string
}

// areaPlain is the area's name inside a sentence: "област Смолян", "район Младост", "Smolyan", "Mladost district".
func (p PageData) areaPlain(row AreaRow) string {
	switch row.Kind {
	case "oblast":
		return p.oblastFormKey(row, "seo.oblast.inline")
	case "city":
		return row.Name
	default:
		return strings.ReplaceAll(p.T("area.plain.district"), "{name}", row.Name)
	}
}

// factText fills a facts key with the area placeholders plus the given pairs.
func (p PageData) factText(key string, pairs ...string) string {
	row := *p.Area
	in := p.areaIn()
	base := []string{
		"{subject}", p.areaSubject(row),
		"{area_in}", in,
		"{area_in_cap}", capFirst(in),
		"{area_plain}", p.areaPlain(row),
	}
	return strings.NewReplacer(append(base, pairs...)...).Replace(p.T(key))
}

// countText picks key.one or key.other for n and fills {n}.
func (p PageData) countText(key string, n int) string {
	return strings.ReplaceAll(p.T(key+"."+pluralOf(n, "one", "other")), "{n}", strconv.Itoa(n))
}

// sameKind lists the areas of one kind.
func sameKind(snap *snapshot.Snapshot, kind string) []snapshot.AreaMeta {
	var out []snapshot.AreaMeta
	for _, m := range snap.KnownSlugs {
		if m.Kind == kind {
			out = append(out, m)
		}
	}
	return out
}

// areaFacts builds the three blocks for one area page from the snapshot alone.
func (rr *Renderer) areaFacts(p PageData, meta snapshot.AreaMeta, snap *snapshot.Snapshot) *AreaFacts {
	f := &AreaFacts{Heading: p.factText("area.facts.heading")}
	f.Place = rr.placeFact(p, meta, snap)
	f.Network = p.networkFact(meta, snap)
	f.Rank, f.rankFAQ = p.rankFact(meta, snap)
	if f.Rank != "" {
		f.kindPlain = p.T("area.facts.kind_plain." + meta.Kind)
	}
	return f
}

// placeFact says what the page's boundary is and how big it is.
func (rr *Renderer) placeFact(p PageData, meta snapshot.AreaMeta, snap *snapshot.Snapshot) string {
	km2 := ""
	if meta.Km2 > 0 {
		km2 = formatMeasure(meta.Km2, p.Lang)
	}
	var parts []string
	switch meta.Kind {
	case "city":
		parts = rr.cityPlace(p, meta, snap, km2)
	case "oblast":
		parts = p.sizedPlace(meta, snap, km2, "area.facts.place.oblast_lead", "area.facts.place.size_rank_oblast")
	case "neighbourhood":
		parts = p.sizedPlace(meta, snap, km2, "area.facts.place.district_lead", "area.facts.place.size_rank_district")
	}
	return strings.Join(parts, " ")
}

// cityPlace names the province and says which kind of boundary the city has.
func (rr *Renderer) cityPlace(p PageData, meta snapshot.AreaMeta, snap *snapshot.Snapshot, km2 string) []string {
	lead := p.factText("area.facts.place.city_lead_alone")
	if parent, ok := snap.KnownSlugs[meta.ParentSlug]; ok && parent.Kind == "oblast" {
		province := p.oblastFormKey(rr.rowFrom(parent, p.Lang), "seo.oblast.inline")
		lead = p.factText("area.facts.place.city_lead", "{province}", province)
	}
	parts := []string{lead}
	switch {
	case municipalityCities[meta.Slug]:
		parts = append(parts, p.factText("area.facts.place.city_muni"))
		if km2 != "" {
			parts = append(parts, p.factText("area.facts.place.city_muni_size", "{km2}", km2))
		}
	case km2 != "":
		parts = append(parts, p.factText("area.facts.place.city_own", "{km2}", km2))
	}
	return parts
}

// sizedPlace is the district and oblast shape: how many there are, the size, and the place by size.
func (p PageData) sizedPlace(meta snapshot.AreaMeta, snap *snapshot.Snapshot, km2, leadKey, rankKey string) []string {
	peers := sameKind(snap, meta.Kind)
	parts := []string{p.factText(leadKey, "{total}", strconv.Itoa(len(peers)))}
	if km2 == "" {
		return parts
	}
	parts = append(parts, p.factText("area.facts.place.size", "{km2}", km2))
	var sizes []float64
	for _, m := range peers {
		if m.Km2 > 0 {
			sizes = append(sizes, m.Km2)
		}
	}
	if len(sizes) >= minRankPeers {
		parts = append(parts, p.factText(rankKey, "{rank}", ordinal(p.Lang, rankDescending(meta.Km2, sizes))))
	}
	return parts
}

// citizenSensors is the citizen sensor count: all stations minus the official ones.
func citizenSensors(meta snapshot.AreaMeta) int {
	official := 0
	switch {
	case meta.BySource["eea"].N > 0:
		official = meta.BySource["eea"].N
	case meta.Source == "eea":
		official = meta.SensorCount
	case !meta.Covered:
		// An uncovered area publishes no source split, so assume each station inside is counted.
		official = min(len(meta.Stations), meta.SensorCount)
	}
	return max(meta.SensorCount-official, 0)
}

// stationName is the station's name as printed: Bulgarian as committed, Latin letters for every other language.
func stationName(info eea.StationInfo, lang string) string {
	name := strings.TrimSpace(info.Name)
	if lang == i18n.DefaultLang {
		return name
	}
	return transliterateBG(name)
}

// joinNames writes "a, b and c" with the language's own word for "and".
func (p PageData) joinNames(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + p.T("area.facts.and") + names[len(names)-1]
}

// classWord is a station class word in the page's language. Unknown words are dropped except in Bulgarian, where they are already right.
func (p PageData) classWord(bg string) string {
	if id := stationWordIDs[bg]; id != "" && p.cat.Has(p.Lang, "area.stype."+id) {
		return p.T("area.stype." + id)
	}
	if p.Lang == i18n.DefaultLang {
		return bg
	}
	return ""
}

// stationType is "АИС Шумен е градска фонова станция", or "" when the station has no class.
func (p PageData) stationType(info eea.StationInfo) string {
	var words []string
	for _, w := range []string{normStationType(info.Type), normStationArea(info.Area)} {
		if cw := p.classWord(w); cw != "" {
			words = append(words, cw)
		}
	}
	if len(words) == 0 {
		return ""
	}
	joined := strings.Join(words, " ")
	return p.factText("area.facts.net.type", "{name}", stationName(info, p.Lang), "{words}", joined, "{article}", aOrAn(joined))
}

// nearestStation is the committed station closest to the area's centre, or false when the snapshot has no positions.
func nearestStation(meta snapshot.AreaMeta, snap *snapshot.Snapshot) (eea.StationInfo, snapshot.OfficialStation, float64, bool) {
	if meta.CentroidLat == 0 && meta.CentroidLon == 0 {
		return eea.StationInfo{}, snapshot.OfficialStation{}, 0, false
	}
	codes := make([]string, 0, len(snap.Stations))
	for code := range snap.Stations {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	var (
		best     eea.StationInfo
		bestPos  snapshot.OfficialStation
		bestDist float64
		found    bool
	)
	for _, code := range codes {
		info, ok := eea.LookupStationInfo(code)
		if !ok {
			continue
		}
		pos := snap.Stations[code]
		d := haversineKM(meta.CentroidLat, meta.CentroidLon, pos.Lat, pos.Lon)
		if !found || d < bestDist {
			best, bestPos, bestDist, found = info, pos, d, true
		}
	}
	return best, bestPos, bestDist, found
}

// networkFact says how many citizen sensors and official stations the area has, and the nearest official one when none is inside.
func (p PageData) networkFact(meta snapshot.AreaMeta, snap *snapshot.Snapshot) string {
	var infos []eea.StationInfo
	var names []string
	for _, code := range meta.Stations {
		if info, ok := eea.LookupStationInfo(code); ok {
			infos = append(infos, info)
			names = append(names, stationName(info, p.Lang))
		}
	}
	n, k := citizenSensors(meta), len(infos)
	pairs := []string{
		"{sensors}", p.countText("area.facts.net.sensors", n),
		"{stations}", p.countText("area.facts.net.stations", k),
		"{names}", p.joinNames(names),
	}
	var key string
	switch {
	case n > 0 && k > 0:
		key = "area.facts.net.both"
	case n > 0:
		key = "area.facts.net.citizen_only"
	case k > 0:
		key = "area.facts.net.official_only"
	default:
		key = "area.facts.net.neither"
	}
	parts := []string{p.factText(key, pairs...)}
	switch {
	case k == 1:
		parts = append(parts, p.stationType(infos[0]))
	case k == 0:
		if info, pos, dist, ok := nearestStation(meta, snap); ok {
			dir := p.T(directionKeys[bearingSector(meta.CentroidLat, meta.CentroidLon, pos.Lat, pos.Lon)])
			parts = append(parts,
				p.factText("area.facts.net.nearest", "{name}", stationName(info, p.Lang), "{km}", formatMeasure(dist, p.Lang), "{dir}", dir),
				p.stationType(info))
		}
	}
	return joinSentences(parts)
}

// joinSentences joins the non-empty sentences with a space.
func joinSentences(parts []string) string {
	var kept []string
	for _, s := range parts {
		if s != "" {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, " ")
}

// rankFact is the area's place by PM2.5 among the same kind. Both results are empty without a current value
// or with fewer than minRankPeers areas that have one. The second is the block without its closing note.
func (p PageData) rankFact(meta snapshot.AreaMeta, snap *snapshot.Snapshot) (full, forFAQ string) {
	v, ok := meta.Values[rankMetric]
	if !ok || !p.cat.Has(p.Lang, "area.facts.kind."+meta.Kind) {
		return "", ""
	}
	var vals []float64
	for _, m := range sameKind(snap, meta.Kind) {
		if pv, has := m.Values[rankMetric]; has {
			vals = append(vals, pv)
		}
	}
	if len(vals) < minRankPeers {
		return "", ""
	}
	rank := rankAscending(v, vals)
	// medianOf sorts in place, so it gets its own copy.
	median := medianOf(append([]float64(nil), vals...))
	pairs := []string{
		"{rank}", ordinal(p.Lang, rank),
		"{total}", strconv.Itoa(len(vals)),
		"{kind}", p.T("area.facts.kind." + meta.Kind),
		"{value}", formatValue(v, p.Lang),
		"{median}", formatValue(median, p.Lang),
	}
	leadKey := "area.facts.rank.lead"
	if rank == 1 {
		leadKey = "area.facts.rank.first"
	}
	forFAQ = joinSentences([]string{p.factText(leadKey, pairs...), p.factText("area.facts.rank.value", pairs...)})
	return joinSentences([]string{forFAQ, p.T("area.facts.rank.note")}), forFAQ
}
