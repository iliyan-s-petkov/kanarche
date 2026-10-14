// Pure builders for the bathing-water layer; mapsea.js drives the map with them.

export const SEA_SOURCE_ID = 'sea-sites'
export const SEA_LAYER_ID = 'sea-sites'
export const SEA_IMAGE_ID = 'sea-square'

// Order of frontend.sea_class_colours.
export const SEA_CLASSES = ['excellent', 'good', 'sufficient', 'poor', 'not_classified']

// The marker class for an API quality. The legacy combined class shares sufficient's marker.
export function seaClass(quality) {
  if (quality === 'good_or_sufficient') return 'sufficient'
  return SEA_CLASSES.includes(quality) ? quality : 'not_classified'
}

export function seaColours(list) {
  return Object.fromEntries(SEA_CLASSES.map((k, i) => [k, list?.[i] || '']))
}

export function seaFeatures(body, colours) {
  const features = (body?.sites ?? []).map((s) => ({
    type: 'Feature',
    geometry: { type: 'Point', coordinates: [s.lon, s.lat] },
    properties: { id: s.id, quality: s.quality ?? null, colour: colours[seaClass(s.quality)] },
  }))
  return { type: 'FeatureCollection', features }
}

// 0 within the excellent limit, 1 over it, 2 over the good limit. Censored values are never over.
export function seaLevel(value, belowDetection, [excellent, good]) {
  if (belowDetection) return 0
  if (value > good) return 2
  if (value > excellent) return 1
  return 0
}

// EEA names are upper case; shown in title case, in the page language when it has one.
export function seaName(site, lang) {
  const raw = (lang === 'bg' ? site.name_bg || site.name_en : site.name_en || site.name_bg) || ''
  const locale = lang === 'bg' ? 'bg' : 'en'
  return raw.toLocaleLowerCase(locale)
    .replace(/(^|[\s\-(])(\p{L})/gu, (_, sep, ch) => sep + ch.toLocaleUpperCase(locale))
}

export function fillSeason(template, season) {
  return String(template || '').replace('{season}', String(season))
}

// Fills {name} slots from `values`; a slot with no value stays as written.
export function fillSlots(template, values) {
  return String(template || '').replace(/\{(\w+)\}/g, (slot, name) => (name in values ? String(values[name]) : slot))
}

// The API's YYYY-MM-DD as DD.MM.YYYY, the form the RZI credit uses. Anything else is returned as sent.
export function creditDate(iso) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso || '')
  return m ? `${m[3]}.${m[2]}.${m[1]}` : String(iso ?? '')
}

export const isRZISource = (source) => typeof source === 'string' && source.startsWith('rzi_')

// Only an absolute http(s) results page is linked. A path that names a file is not a page.
export function safePageURL(url) {
  let u
  try {
    u = new URL(String(url ?? ''))
  } catch {
    return ''
  }
  if (u.protocol !== 'http:' && u.protocol !== 'https:') return ''
  if (u.username || u.password) return ''
  if (/\.(pdf|xlsx?|csv|docx?)$/i.test(u.pathname)) return ''
  return u.href
}

// The credit line for one RZI sample. Empty when the template or the source name is missing,
// and callers then show no value from that sample.
export function rziCredit(t, source, date) {
  const region = t.sources?.[source] || ''
  if (!region || !t.creditRzi) return ''
  return fillSlots(t.creditRzi, { region, date: creditDate(date) })
}

export const SEA_METRICS = ['e_coli', 'enterococci']
export const LEVEL_MARKS = ['✓', '▲', '▲▲']

const metricKey = (metric) => (metric === 'enterococci' ? 'enterococci' : 'e_coli')

// Seasons that have at least one sample, newest first.
export function seasonsWithSamples(samples) {
  return [...new Set((samples ?? []).map((s) => s.season))].sort((a, b) => b - a)
}

// One metric of one season as chart points, oldest first.
export function samplePoints(samples, season, metric) {
  const key = metricKey(metric)
  return (samples ?? [])
    .filter((s) => s.season === season)
    .map((s) => ({ date: s.date, value: s[key], below: !!s[`${key}_below_detection`], preSeason: !!s.pre_season, source: s.source ?? '' }))
    .sort((a, b) => (a.date < b.date ? -1 : a.date > b.date ? 1 : 0))
}

// The worse of the two indicators: 0 within, 1 over excellent, 2 over good.
export function chipLevel(sample, limits) {
  return Math.max(
    seaLevel(sample.e_coli, sample.e_coli_below_detection, limits.e_coli),
    seaLevel(sample.enterococci, sample.enterococci_below_detection, limits.enterococci),
  )
}

export function classLabel(quality, t) {
  return t.classes[quality ?? 'not_classified'] || t.classes.not_classified || quality || ''
}

// The legacy combined class takes sufficient's initial, as it takes its colour.
export function classInitial(quality, t) {
  return t.initials?.[seaClass(quality)] ?? ''
}

// The credit line of one source. Empty when an RZI source has no name or the template is missing,
// and callers then show no value from that source.
export function sourceCredit(t, source) {
  if (!isRZISource(source)) return t.credit || ''
  const region = t.sources?.[source] || ''
  if (!region || !t.creditSource) return ''
  return fillSlots(t.creditSource, { region })
}

// A sample shows only with its credit line.
export function creditedSamples(samples, t) {
  return (samples ?? []).filter((s) => sourceCredit(t, s.source))
}

export function latestSamples(samples, t, n = 3) {
  return creditedSamples(samples, t)
    .sort((a, b) => (a.date < b.date ? 1 : a.date > b.date ? -1 : 0))
    .slice(0, n)
}

// Each source once, in order of first appearance. A sample without a source is the EEA's.
export function sampleSources(samples, t) {
  const seen = new Map()
  for (const s of samples ?? []) {
    const id = isRZISource(s.source) ? s.source : 'eea'
    if (seen.has(id)) continue
    const credit = sourceCredit(t, id)
    if (credit) seen.set(id, { id, name: id === 'eea' ? t.sourceEea : t.sources[id], credit })
  }
  return [...seen.values()]
}

// The text of a sample level. `within` is what level 0 reads as, empty where it needs no word.
export function levelText(t, level, within = '') {
  return level === 2 ? t.overGood : level === 1 ? t.overExcellent : within
}

// Scheme check only, for links that are often PDFs. safePageURL is the stricter one.
export function httpURL(url) {
  try {
    const u = new URL(String(url ?? ''))
    return u.protocol === 'http:' || u.protocol === 'https:' ? u.href : ''
  } catch {
    return ''
  }
}

// The closed-season note: the API's season_open flag, no latest result, and the year of the newest
// RZI sample says which results the table holds. `samples` are the credited ones.
export function offSeasonText(t, d, samples) {
  const year = (samples ?? []).find((s) => isRZISource(s.source))?.season ?? null
  if (d?.site?.rzi?.season_open !== false || d.latest || year === null || !t.latest.offSeason) return ''
  return fillSlots(t.latest.offSeason, { year })
}

// One cell per year from the first class to the last. A year with no class is not classified.
export function seasonCells(classes) {
  const bySeason = new Map((classes ?? []).map((c) => [c.season, c]))
  if (bySeason.size === 0) return []
  const years = [...bySeason.keys()]
  const cells = []
  for (let y = Math.min(...years); y <= Math.max(...years); y++) {
    const c = bySeason.get(y)
    cells.push({ season: y, quality: c?.quality ?? null, source: c?.source ?? '' })
  }
  return cells
}

export const cellName = (cell, t) => `${cell.season} ${classLabel(cell.quality, t)}`

// Letter colour on a swatch: whichever of white and black has the higher contrast with it.
export function inkFor(colour) {
  const m = /^#([0-9a-f]{6})$/i.exec(colour || '')
  if (!m) return 'dark'
  const [r, g, b] = [0, 2, 4]
    .map((i) => parseInt(m[1].slice(i, i + 2), 16) / 255)
    .map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b > 0.179 ? 'dark' : 'light'
}

// WAI-ARIA tabs: the index a key moves to, or null for a key the tabs ignore.
export function nextTab(index, key, count) {
  if (key === 'ArrowRight') return (index + 1) % count
  if (key === 'ArrowLeft') return (index - 1 + count) % count
  if (key === 'Home') return 0
  if (key === 'End') return count - 1
  return null
}

// The season of a datahub class whose lab samples are not published yet, else null.
export function pendingSeason(d) {
  const current = d?.classes?.[0]
  if (current?.source !== 'datahub') return null
  return (d.samples ?? []).some((s) => s.season === current.season) ? null : current.season
}

// An ISO day in the page language, as UTC. Anything else is returned as sent.
export function formatDay(iso, lang) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(iso || '')) return String(iso ?? '')
  return new Intl.DateTimeFormat(lang || 'bg', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' })
    .format(new Date(`${iso}T00:00:00Z`))
}

// The data-t-sea-* attributes, grouped. Missing ones read as ''.
export function readSeaTexts(d) {
  const s = (k) => d[k] || ''
  return {
    toggle: s('tSeaToggle'),
    legend: s('tSeaLegend'),
    classes: {
      excellent: s('tSeaClassExcellent'),
      good: s('tSeaClassGood'),
      sufficient: s('tSeaClassSufficient'),
      poor: s('tSeaClassPoor'),
      not_classified: s('tSeaClassNotClassified'),
      good_or_sufficient: s('tSeaClassGoodOrSufficient'),
    },
    zones: { coastal: s('tSeaZoneCoastal'), lake: s('tSeaZoneLake') },
    season: s('tSeaSeason'),
    history: s('tSeaHistory'),
    samples: s('tSeaSamples'),
    date: s('tSeaDate'),
    eColi: s('tSeaEColi'),
    enterococci: s('tSeaEnterococci'),
    unit: s('tSeaUnit'),
    limits: s('tSeaLimits'),
    overExcellent: s('tSeaOverExcellent'),
    overGood: s('tSeaOverGood'),
    belowDetection: s('tSeaBelowDetection'),
    preSeason: s('tSeaPreSeason'),
    noSamples: s('tSeaNoSamples'),
    note: s('tSeaNote'),
    credit: s('tSeaCredit'),
    profile: s('tSeaProfile'),
    close: s('tSeaClose'),
    failed: s('tSeaFailed'),
    // Provenance of classes that come from the EEA annual dataset instead of Discodata.
    classSourceDatahub: s('tSeaClassSourceDatahub'),
    samplesPending: s('tSeaSamplesPending'),
    supplementNote: s('tSeaSupplementNote'),
    // The RZI lab results shown next to the class. Keyed by the API's source id.
    latest: {
      heading: s('tSeaLatestHeading'),
      singleSampleNote: s('tSeaLatestSingleSampleNote'),
      stale: s('tSeaLatestStale'),
      offSeason: s('tSeaLatestOffSeason'),
    },
    creditRzi: s('tSeaCreditRzi'),
    noticeUnofficial: s('tSeaNoticeUnofficial'),
    sources: { rzi_varna: s('tSeaSourceRziVarna'), rzi_burgas: s('tSeaSourceRziBurgas'), rzi_dobrich: s('tSeaSourceRziDobrich') },
    sourceLink: s('tSeaSourceLink'),
    unitMpn: s('tSeaUnitMpn'),
    newTab: s('tSeaNewTab'),
    // The legend (i) dialog, see infodialog.js.
    info: {
      label: s('tSeaInfoLabel'),
      title: s('tSeaInfoTitle'),
      body: s('tSeaInfoBody'),
      rzi: s('tSeaInfoRzi'),
      linkMap: s('tSeaInfoLinkMap'),
      linkEea: s('tSeaInfoLinkEea'),
      linkDatahub: s('tSeaInfoLinkDatahub'),
    },
    initials: {
      excellent: s('tSeaInitialExcellent'),
      good: s('tSeaInitialGood'),
      sufficient: s('tSeaInitialSufficient'),
      poor: s('tSeaInitialPoor'),
    },
    officialClass: s('tSeaOfficialClass'),
    chips: {
      heading: s('tSeaChipsHeading'),
      keyWithin: s('tSeaChipsKeyWithin'),
      none: s('tSeaChipsNone'),
      infoLabel: s('tSeaChipsInfoLabel'),
    },
    seasonsSince: s('tSeaSeasonsSince'),
    details: s('tSeaDetails'),
    back: s('tSeaBack'),
    tabs: { label: s('tSeaTabsLabel'), samples: s('tSeaTabsSamples'), seasons: s('tSeaTabsSeasons'), about: s('tSeaTabsAbout') },
    metricLabel: s('tSeaMetricLabel'),
    seasonPicker: s('tSeaSeasonPicker'),
    chart: {
      summary: s('tSeaChartSummary'),
      caption: s('tSeaChartCaption'),
      showTable: s('tSeaChartShowTable'),
      showChart: s('tSeaChartShowChart'),
      bandWithin: s('tSeaChartBandWithin'),
      bandBetween: s('tSeaChartBandBetween'),
      bandAbove: s('tSeaChartBandAbove'),
      hollow: s('tSeaChartHollow'),
    },
    creditSource: s('tSeaCreditSource'),
    sourceEea: s('tSeaSourcesEea'),
    sourcesInfoLabel: s('tSeaSourcesInfoLabel'),
  }
}
