// The text alternative of a chart: a one-line summary and the readings as table rows.
// Pure; Chart.svelte renders it. Values and times are formatted as the hover card formats them.
import { formatReading, formatStamp } from './charttooltip.js'

// "{metric}" style placeholders, filled from a map of strings.
const fill = (template, values) => template.replace(/\{(\w+)\}/g, (_, key) => values[key] ?? '')

const withUnit = (value, lang, unit) => `${formatReading(value, lang)} ${unit ?? ''}`.trim()

// Latest, lowest and highest of one column, ignoring gaps. Null when it holds no reading.
function stats(values) {
  const real = values.filter((v) => v != null && !Number.isNaN(v))
  if (real.length === 0) return null
  return { latest: real[real.length - 1], min: Math.min(...real), max: Math.max(...real) }
}

// lines: [{ name, unit }]. data: uPlot's [times, ...columns], times in epoch seconds.
// text: { summary, none, caption } templates. Empty data gives the "none" sentence and no rows.
export function chartText({ lines, data, periodLabel, lang, text, timeLabel, timeZone }) {
  const summary = lines.map((line, i) => {
    const s = stats(data[i + 1] ?? [])
    const values = { metric: line.name, period: periodLabel }
    if (!s) return fill(text.none, values)
    return fill(text.summary, {
      ...values,
      latest: withUnit(s.latest, lang, line.unit),
      min: withUnit(s.min, lang, line.unit),
      max: withUnit(s.max, lang, line.unit),
    })
  }).join(' ')

  const caption = fill(text.caption, { metric: lines.map((l) => l.name).join(', '), period: periodLabel })
  const head = [timeLabel, ...lines.map((l) => (l.unit ? `${l.name} (${l.unit})` : l.name))]
  const rows = (data[0] ?? []).map((time, row) => [
    formatStamp(time, lang, timeZone),
    ...lines.map((_, i) => formatReading(data[i + 1]?.[row], lang)),
  ])
  return { summary, caption, head, rows }
}
