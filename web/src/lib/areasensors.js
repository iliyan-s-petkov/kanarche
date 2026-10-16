// The area page's sensor table: rows, filter and paging as pure functions over the registry body.
import { stationsOf, readingAt } from './stations.js'
import { pageSlice } from './table.js'

export const METRICS = ['P2', 'P1']
export const PER_PAGE_OPTIONS = [10, 25, 50]
export const DEFAULT_PER_PAGE = 25

// Index of the band a value falls in, -1 for no value. Same rule as bandName.
function bandIndex(value, bands) {
  if (typeof value !== 'number' || !Array.isArray(bands)) return -1
  return bands.findIndex((b) => b.upper === null || b.upper === undefined || value <= b.upper)
}

// sensorRows(body, {districts, bands: {P2, P1}, names}) -> one row per station, by number.
// The band is the worse of the two particulate bands, as the EAQI rates a site.
export function sensorRows(body, { districts = {}, bands = {}, names = {} } = {}) {
  const cols = body?.sensors ?? {}
  const ids = cols.id ?? []
  const rows = stationsOf(body).map(({ station, indices }) => {
    const members = indices.map((i) => Number(ids[i]))
    const row = { id: Number(station), members }
    let worst = { i: -1, label: '' }
    for (const m of METRICS) {
      const { value } = readingAt(body, indices, m)
      row[m] = value
      const i = bandIndex(value, bands[m])
      if (i > worst.i) worst = { i, label: bands[m][i].label }
    }
    row.band = worst.i < 0 ? '' : names[worst.label] || ''
    row.district = [row.id, ...members].map((id) => districts[id]).find(Boolean) ?? ''
    row.updated = indices.map((i) => cols.last_seen?.[i]).filter(Boolean).sort().at(-1) ?? null
    return row
  })
  return rows.sort((a, b) => a.id - b.id)
}

// A typed number matches any box at the station. Non-digits are ignored.
export function filterSensorRows(rows, query) {
  const q = String(query ?? '').replace(/\D/g, '')
  if (!q) return rows
  return rows.filter((r) => r.members.some((id) => String(id).includes(q)) || String(r.id).includes(q))
}

// pageView: the page on screen plus the 1-based ranks it covers, for the status line.
export function pageView(rows, perPage, page) {
  const view = pageSlice(rows, perPage, page)
  const total = rows.length
  const size = perPage === 'all' ? total : perPage
  const from = total ? (view.page - 1) * size + 1 : 0
  return { ...view, from, to: total ? from + view.rows.length - 1 : 0, total }
}

const fill = (s, vars) => Object.entries(vars).reduce((out, [k, v]) => out.replaceAll(`{${k}}`, String(v)), s || '')

export function statusText(t, { from, to, total }) {
  return total ? fill(t.status, { from, to, total }) : t.empty || ''
}

export function summaryText(t, n) {
  return n === 1 ? t.summaryOne || '' : fill(t.summary, { n })
}

export { fill }
