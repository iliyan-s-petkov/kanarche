// The area page's sensor table: rows, filter and paging as pure functions over the registry body.
import { stationsOf, readingAt } from './stations.js'
import { pageSlice } from './table.js'
import { METRICS, compositeBand } from './compositeband.js'

export { METRICS }
export const PER_PAGE_OPTIONS = [10, 25, 50]
export const DEFAULT_PER_PAGE = 25

// sensorRows(body, {districts, bands: {P2, P1}, names}) -> one row per station, by number.
// The band is the worse of the two particulate bands, as the EAQI rates a site.
export function sensorRows(body, { districts = {}, bands = {}, names = {} } = {}) {
  const cols = body?.sensors ?? {}
  const ids = cols.id ?? []
  const rows = stationsOf(body).map(({ station, indices }) => {
    const members = indices.map((i) => Number(ids[i]))
    const row = { id: Number(station), members }
    for (const m of METRICS) row[m] = readingAt(body, indices, m).value
    row.band = compositeBand(row, bands, names)
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

// available: rows before the filter, so an area with no sensors is not told its filter matched nothing.
export function statusText(t, { from, to, total }, available) {
  if (total === 1 && t.rangeOne) return t.rangeOne
  if (total) return fill(t.status, { from, to, total })
  return (available ? t.empty : t.none) || ''
}

export function summaryText(t, n) {
  return n === 1 ? t.summaryOne || '' : fill(t.summary, { n })
}

export { fill }
