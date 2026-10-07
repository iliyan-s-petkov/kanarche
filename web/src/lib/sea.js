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
    // The legend (i) dialog, see infodialog.js.
    info: {
      label: s('tSeaInfoLabel'),
      title: s('tSeaInfoTitle'),
      body: s('tSeaInfoBody'),
      linkMap: s('tSeaInfoLinkMap'),
      linkEea: s('tSeaInfoLinkEea'),
      linkDatahub: s('tSeaInfoLinkDatahub'),
    },
  }
}
