// The text alternative of a chart: a one-line summary and the readings as table rows.
// Pure; Chart.svelte renders it. Values and times are formatted as the hover card formats them.
import { formatDay, formatReading, formatStamp } from './charttooltip.js'

// "{metric}" style placeholders, filled from a map of strings.
const fill = (template, values) => template.replace(/\{(\w+)\}/g, (_, key) => values[key] ?? '')

const withUnit = (value, lang, unit) => `${formatReading(value, lang)} ${unit ?? ''}`.trim()

// speak maps a printed unit to its spoken words. It applies to the summary a
// screen reader gets, not to the column head, which is read as text on screen.
const noSpeech = (unit) => unit

// Latest, lowest and highest of one column, ignoring gaps. Null when it holds no reading.
function stats(values) {
  const real = values.filter((v) => v != null && !Number.isNaN(v))
  if (real.length === 0) return null
  return { latest: real[real.length - 1], min: Math.min(...real), max: Math.max(...real) }
}

// Up to this span the table has a row per clock hour, past it a row per day.
export const HOURLY_SPAN_SECONDS = 48 * 3600

const keyFormat = (unit, timeZone) => new Intl.DateTimeFormat('en-CA', {
  year: 'numeric', month: '2-digit', day: '2-digit', timeZone,
  ...(unit === 'hour' ? { hour: '2-digit', hourCycle: 'h23' } : {}),
})

// Seconds into the local hour of a time, so a bucket can be labelled by its hour and not its first reading.
function intoHour(seconds, timeZone) {
  const parts = new Intl.DateTimeFormat('en', { minute: 'numeric', second: 'numeric', timeZone }).formatToParts(new Date(seconds * 1000))
  const get = (type) => Number(parts.find((p) => p.type === type)?.value ?? 0)
  return get('minute') * 60 + get('second')
}

// One row per local hour or day: the label, then each column's mean over the readings in that bucket (gaps skipped).
function bucketRows(data, columns, unit, lang, timeZone) {
  const key = keyFormat(unit, timeZone)
  const buckets = []
  data[0].forEach((time, row) => {
    const k = key.format(new Date(time * 1000))
    let bucket = buckets[buckets.length - 1]
    if (bucket?.key !== k) {
      bucket = { key: k, time, sums: columns.map(() => ({ sum: 0, n: 0 })) }
      buckets.push(bucket)
    }
    columns.forEach((_, i) => {
      const v = data[i + 1]?.[row]
      if (v != null && !Number.isNaN(v)) { bucket.sums[i].sum += v; bucket.sums[i].n += 1 }
    })
  })
  return buckets.map((b) => [
    unit === 'hour' ? formatStamp(b.time - intoHour(b.time, timeZone), lang, timeZone) : formatDay(b.time, lang, timeZone),
    ...b.sums.map((c) => formatReading(c.n ? c.sum / c.n : null, lang)),
  ])
}

// lines: [{ name, unit }]. data: uPlot's [times, ...columns], times in epoch seconds.
// text: { summary, none, caption } templates. Empty data gives the "none" sentence and no rows.
export function chartText({ lines, data, periodLabel, lang, text, timeLabel, timeZone, speak = noSpeech }) {
  const summary = lines.map((line, i) => {
    const s = stats(data[i + 1] ?? [])
    const values = { metric: line.name, period: periodLabel }
    if (!s) return fill(text.none, values)
    return fill(text.summary, {
      ...values,
      latest: withUnit(s.latest, lang, speak(line.unit)),
      min: withUnit(s.min, lang, speak(line.unit)),
      max: withUnit(s.max, lang, speak(line.unit)),
    })
  }).join(' ')

  const times = data[0] ?? []
  const daily = times.length > 1 && times[times.length - 1] - times[0] > HOURLY_SPAN_SECONDS
  const unit = daily ? 'day' : 'hour'
  const caption = fill(daily ? text.captionDaily : text.captionHourly, { metric: lines.map((l) => l.name).join(', '), period: periodLabel })
  const head = [timeLabel, ...lines.map((l) => (l.unit ? `${l.name} (${l.unit})` : l.name))]
  const rows = times.length ? bucketRows(data, lines, unit, lang, timeZone) : []
  return { summary, caption, head, rows }
}
