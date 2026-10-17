// The home summary's second sentence once a Find me fix is in: the reading at the
// nearest sensor and where that sensor is. The paragraph and its copy come from the
// server (HomeNowHTML in internal/web/spoken_units.go), so no string lives here.
import { number } from './areacards.js'

const DIR_ATTRS = ['tDirN', 'tDirNe', 'tDirE', 'tDirSe', 'tDirS', 'tDirSw', 'tDirW', 'tDirNw']

// readNearCopy(el) -> { km, m, dirs[8] } from the summary paragraph's data-t-* attributes.
export function readNearCopy(el) {
  const d = el.dataset
  return { km: d.tNearKm || '', m: d.tNearM || '', dirs: DIR_ATTRS.map((k) => d[k] || '') }
}

const fill = (template, values) => template.replace(/\{(\w+)\}/g, (_, key) => values[key] ?? '')

// Rounded the way a person says it: tenths of a kilometre, tens of metres, never zero.
function distance(metres, lang) {
  if (metres >= 1000) {
    const km = metres < 10000 ? Math.round(metres / 100) / 10 : Math.round(metres / 1000)
    return { template: 'km', n: number(km, lang) }
  }
  return { template: 'm', n: number(Math.max(10, Math.round(metres / 10) * 10), lang) }
}

// unit is the spoken form: this text is for the speech layer.
export function nearSummaryText({ t, metric, value, unit, band, metres, octant, lang }) {
  const far = distance(metres, lang)
  const text = fill(t[far.template], {
    metric, value: number(value, lang), unit, band, n: far.n, dir: t.dirs[octant],
  })
  // A metric with no scale has no band, which would leave ", ." in the sentence.
  return band ? text : text.replace(/ ?, \./, '.').replace(/,\s*\./, '.')
}

// applyNearSummary writes the sentence into the page. False when the page has no summary.
export function applyNearSummary(doc, reading) {
  const host = doc.querySelector('[data-home-summary]')
  const second = host?.querySelector('[data-home-second]')
  if (!host || !second) return false
  second.textContent = nearSummaryText({ ...reading, t: readNearCopy(host) })
  return true
}
