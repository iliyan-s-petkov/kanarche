// Pure builders for the pollen layer; mappollen.js drives the map with them.

export const POLLEN_SOURCE_ID = 'pollen-areas'
export const POLLEN_FILL_LAYER_ID = 'pollen-fill'
export const POLLEN_LINE_LAYER_ID = 'pollen-line'

// The server's snapshot.PollenLevels, in rank order.
export const POLLEN_LEVELS = ['low', 'moderate', 'high']

// Low and moderate share the area page's accent; opacity separates them, as the bar width does there.
export const POLLEN_OPACITY = { low: 0.35, moderate: 0.7, high: 0.7 }

// The theme token for a level.
export function pollenVar(level) {
  return `--pollen-${level}`
}

// Level to colour, read from the theme; `read` returns a custom property's value.
export function pollenColours(read, fallback) {
  return Object.fromEntries(POLLEN_LEVELS.map((l) => [l, String(read(pollenVar(l)) || '').trim() || fallback]))
}

// The province outlines with today's level joined on slug; null where the forecast has none.
export function pollenFeatures(boundaries, payload) {
  const levels = new Map((payload?.areas ?? []).map((a) => [a.slug, a.level]))
  const features = (boundaries?.features ?? []).map((f) => ({
    type: 'Feature',
    geometry: f.geometry,
    properties: { slug: f.properties?.slug ?? '', level: levels.get(f.properties?.slug) ?? null },
  }))
  return { type: 'FeatureCollection', features }
}

// A ['match', level, ...] picking one value per level; `other` for no data.
function byLevel(pick, other) {
  return ['match', ['coalesce', ['get', 'level'], ''], ...POLLEN_LEVELS.flatMap((l) => [l, pick(l)]), other]
}

// No-data provinces stay unfilled but still take the click.
export function pollenFillPaint(colours, noData) {
  return {
    'fill-color': byLevel((l) => colours[l], noData),
    'fill-opacity': byLevel((l) => POLLEN_OPACITY[l], 0),
  }
}

// Where a click on a province goes: its area page, in the page's language.
export function pollenHref(feature, langPrefix) {
  const slug = feature?.properties?.slug
  if (!slug) return null
  return `${langPrefix || ''}/area/${encodeURIComponent(slug)}`
}

// The credit for the map's attribution control, as the HTML MapLibre expects.
export function pollenAttribution(text, url) {
  if (!text) return ''
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`)
  return url ? `<a href="${esc(url)}" rel="noopener noreferrer">${esc(text)}</a>` : esc(text)
}

// The data-t-pollen-* attributes, grouped. Missing ones read as ''.
export function readPollenTexts(d) {
  const s = (k) => d[k] || ''
  return {
    toggle: s('tPollenToggle'),
    legend: s('tPollenLegend'),
    credit: s('tPollenCredit'),
    levels: {
      low: s('tPollenLevelLow'),
      moderate: s('tPollenLevelModerate'),
      high: s('tPollenLevelHigh'),
    },
  }
}
