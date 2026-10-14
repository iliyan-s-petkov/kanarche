// Pure geometry for the sea sample chart. SeaSampleChart.svelte only draws what this returns.
import { fillSlots, seaLevel } from './sea.js'

const WIDTH = 320
const HEIGHT = 190
const MARGIN = { left: 46, right: 12, top: 12, bottom: 28 }
// Keeps the first and last dot off the plot edge.
const INSET = 8

// points: oldest first, as samplePoints returns them. limits: [excellent, good].
// The y axis is log10 and starts at 1, so a reading of 0 or less than 1 is drawn at 1.
// It ends at the decade above twice the good limit or the largest reading, so both limits always show.
export function chartLayout(points, [excellent, good], dateLabel) {
  const plot = { x: MARGIN.left, y: MARGIN.top, w: WIDTH - MARGIN.left - MARGIN.right, h: HEIGHT - MARGIN.top - MARGIN.bottom }
  const peak = Math.max(good * 2, ...points.map((p) => p.value))
  const decades = Math.ceil(Math.log10(peak))
  const y = (v) => plot.y + plot.h * (1 - Math.log10(Math.max(v, 1)) / decades)

  const times = points.map((p) => Date.parse(`${p.date}T00:00:00Z`))
  const first = Math.min(...times)
  const span = Math.max(...times) - first
  const x = (time) => (span === 0 ? plot.x + plot.w / 2 : plot.x + INSET + ((plot.w - 2 * INSET) * (time - first)) / span)

  const yExcellent = y(excellent)
  const yGood = y(good)
  const bottom = plot.y + plot.h
  const labels = points.length > 1 && span > 0 ? [0, points.length - 1] : [0]
  const byTime = [...points.keys()].sort((a, b) => times[a] - times[b])

  return {
    width: WIDTH,
    height: HEIGHT,
    plot,
    bands: [
      { kind: 'within', y: yExcellent, h: bottom - yExcellent },
      { kind: 'between', y: yGood, h: yExcellent - yGood },
      { kind: 'above', y: plot.y, h: yGood - plot.y },
    ],
    limitLines: [{ y: yExcellent, value: excellent }, { y: yGood, value: good }],
    ticks: Array.from({ length: decades + 1 }, (_, i) => ({ y: y(10 ** i), label: String(10 ** i) })),
    dots: points.map((p, i) => ({ cx: x(times[i]), cy: y(p.value), below: p.below, label: dateLabel(p.date) })),
    xLabels: labels.map((i) => (i === 0 ? byTime[0] : byTime[byTime.length - 1])).map((i) => ({ x: x(times[i]), label: dateLabel(points[i].date) })),
  }
}

// The text alternative of the chart. "Above the excellent limit" counts every sample over it,
// including those over the good limit.
export function chartSummary({ points, limits, template, metric, season }) {
  const levels = points.map((p) => seaLevel(p.value, p.below, limits))
  return fillSlots(template, {
    metric,
    season,
    count: points.length,
    over_excellent: levels.filter((l) => l >= 1).length,
    over_good: levels.filter((l) => l === 2).length,
  })
}
