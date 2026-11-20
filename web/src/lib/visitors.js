// Maps the visitors payload to uPlot's column arrays. Only `uniques` is plotted:
// requests and page views are not what the page claims to show, and the API
// deliberately carries no total.
const DAY_RE = /^\d{4}-\d{2}-\d{2}$/
const DAY_S = 86400
const AVERAGE_DAYS = 7

export const RANGES = [30, 90, 365]
export const DEFAULT_RANGE = 30

// Usable days, ascending, as {x, y, partial, fetchedAt}. x is epoch seconds at
// UTC midnight, as uPlot's time scale expects (see series.js).
export function visitorDays(body) {
  const rows = Array.isArray(body?.days) ? body.days : []
  const days = []
  for (const row of rows) {
    if (typeof row?.day !== 'string' || !DAY_RE.test(row.day)) continue
    if (!Number.isFinite(row.uniques)) continue
    days.push({
      x: Date.parse(`${row.day}T00:00:00Z`) / 1000,
      y: row.uniques,
      partial: row.partial === true,
      fetchedAt: typeof row.fetched_at === 'string' ? row.fetched_at : null,
    })
  }
  return days.sort((a, b) => a.x - b.x)
}

// Mean of the AVERAGE_DAYS calendar days ending at each complete day. The
// partial day is never an input and never gets a value, and a window with a
// missing day is a gap (null) instead of a mean over fewer days.
export function trailingAverage(days) {
  const complete = new Map()
  for (const d of days) if (!d.partial) complete.set(d.x, d.y)
  return days.map((d) => {
    if (d.partial) return null
    let sum = 0
    for (let i = 0; i < AVERAGE_DAYS; i++) {
      const y = complete.get(d.x - i * DAY_S)
      if (y === undefined) return null
      sum += y
    }
    return sum / AVERAGE_DAYS
  })
}

// Keeps the days inside `range` calendar days ending at the newest day. When
// fewer days exist, all of them are kept.
export function sliceRange(days, range) {
  if (days.length === 0) return days
  const cutoff = days[days.length - 1].x - (range - 1) * DAY_S
  return days.filter((d) => d.x >= cutoff)
}

// Columns for the chart, or null when there is nothing worth drawing. One point
// is a number, not a trend, so fewer than two days in range is empty.
//   complete: closed days, null at the partial day
//   today:    only the segment from the last closed day to the partial day
//   average:  trailing mean, computed over all stored days so a short range
//             still has averages at its left edge
// `partial` is the partial day's index in the columns, or -1.
export function chartData(body, range = DEFAULT_RANGE) {
  const all = visitorDays(body)
  const avg = trailingAverage(all)
  const average = new Map(all.map((d, i) => [d.x, avg[i]]))
  const days = sliceRange(all, range)
  if (days.length < 2) return null

  const last = days.length - 1
  const partial = days[last].partial ? last : -1
  const complete = days.map((d) => (d.partial ? null : d.y))
  const today = days.map(() => null)
  if (partial > 0) {
    today[partial] = days[partial].y
    // The dashed line starts at yesterday only when yesterday is adjacent.
    if (days[partial].x - days[partial - 1].x === DAY_S) today[partial - 1] = days[partial - 1].y
  }
  return {
    xs: days.map((d) => d.x),
    complete,
    today,
    average: days.map((d) => average.get(d.x) ?? null),
    partial,
    fetchedAt: partial >= 0 ? days[partial].fetchedAt : null,
  }
}

const sofiaTime = new Intl.DateTimeFormat('en-GB', {
  timeZone: 'Europe/Sofia',
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
})

// HH:MM in Europe/Sofia, 24 hour, whatever the reader's own zone is. Empty
// when the stamp is missing or unreadable.
export function sofiaClock(iso) {
  const d = new Date(iso)
  return iso && !Number.isNaN(d.getTime()) ? sofiaTime.format(d) : ''
}

// Fills {time} in the page-language template.
export function partialLabel(template, iso) {
  const time = sofiaClock(iso)
  return time ? template.replace('{time}', time) : ''
}

const dayEn = new Intl.DateTimeFormat('en-GB', { timeZone: 'UTC', day: 'numeric', month: 'short', year: 'numeric' })

// The UTC day behind an x value (epoch seconds at UTC midnight), for the hover
// readout. Always UTC: in a zone behind UTC the local date would be the day before.
export function dayLabel(seconds, lang = 'bg') {
  const d = new Date(seconds * 1000)
  if (Number.isNaN(d.getTime())) return ''
  if (lang.startsWith('bg')) {
    const p = (n) => String(n).padStart(2, '0')
    return `${p(d.getUTCDate())}.${p(d.getUTCMonth() + 1)}.${d.getUTCFullYear()}`
  }
  return dayEn.format(d)
}
