// The visitors chart on /about. Fetches the daily unique visitors once and
// draws them with uPlot, the chart library the rest of the site uses. The range
// buttons slice that one response on the client. Only the empty and failed
// states, the range buttons and the "today so far" label carry text of their
// own; the heading and footnote are server-rendered.
import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'
import { getJSON } from '../lib/api.js'
import { tickValues } from '../lib/timeaxis.js'
import { DEFAULT_RANGE, RANGES, chartData, partialLabel } from '../lib/visitors.js'

// One request for the longest range. The API caps days at 365.
const URL = '/api/v1/visitors?days=365'
const HEIGHT = 200
// Whole visitors only: uPlot's default ticks would land on 0.5 for a small count.
const INCRS = [1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 25000, 50000, 100000]

// Token reads, not colour literals; fallbacks are the light theme as rgb().
function token(name, fallback) {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fallback
}
const colours = () => ({
  axis: token('--fg-2', 'rgb(82, 82, 82)'),
  grid: token('--border-faint', 'rgb(224, 224, 224)'),
  line: token('--accent', 'rgb(15, 98, 254)'),
  bg: token('--bg', 'rgb(255, 255, 255)'),
})

function message(el, text) {
  const p = document.createElement('p')
  p.className = 'chart-message'
  p.textContent = text
  el.replaceChildren(p)
}

// The text alternative names the range and, while a day is still counting, the
// partial day and its time. role="img" hides the drawn chart from assistive
// technology, so this string is the only description it gets.
function describe(d, rangeText, todayText) {
  const parts = [`${d.label}: ${rangeText}`]
  if (todayText) parts.push(todayText)
  parts.push(d.tAvg)
  return parts.join(', ')
}

export async function mount(el) {
  const d = el.dataset
  let body
  try {
    body = await getJSON(URL)
  } catch (err) {
    message(el, d.tUnavailable || '')
    console.error('visitors:', err)
    return
  }

  if (!chartData(body, 365)) {
    message(el, d.tEmpty || '')
    return
  }

  // dataset camel-casing skips a dash followed by a digit, so read these directly.
  const ranges = Object.fromEntries(RANGES.map((r) => [r, el.getAttribute(`data-t-range-${r}`)]))
  const label = { label: el.getAttribute('aria-label') || '', tAvg: d.tAvg || '' }
  let c = colours()
  let range = DEFAULT_RANGE
  let plot = null
  let observer = null

  const group = document.createElement('div')
  group.className = 'about-stats__range sea-seg'
  group.setAttribute('role', 'group')
  group.setAttribute('aria-label', d.tRangeLegend || '')
  const buttons = new Map()
  for (const r of RANGES) {
    const b = document.createElement('button')
    b.type = 'button'
    b.className = 'sea-seg__btn'
    b.textContent = ranges[r] || String(r)
    b.addEventListener('click', () => {
      if (r === range) return
      range = r
      render()
    })
    buttons.set(r, b)
    group.append(b)
  }
  el.before(group)

  function render() {
    for (const [r, b] of buttons) b.setAttribute('aria-pressed', String(r === range))
    const data = chartData(body, range)
    plot?.destroy()
    observer?.disconnect()
    if (!data) {
      message(el, d.tEmpty || '')
      plot = null
      return
    }

    const todayText = data.partial >= 0 ? partialLabel(d.tToday || '', data.fetchedAt) : ''
    el.setAttribute('aria-label', describe(label, ranges[range] || '', todayText))

    const host = document.createElement('div')
    host.className = 'visitors-chart'
    el.replaceChildren(host)
    if (todayText) {
      const p = document.createElement('p')
      p.className = 'visitors-today'
      p.textContent = todayText
      el.append(p)
    }

    plot = new uPlot({
      width: host.clientWidth || 600,
      height: HEIGHT,
      // The legend doubles as the hover readout; it is hidden until the cursor
      // is on a point (see .visitors-chart in app.css). Strokes are functions so
      // a theme change only has to redraw.
      series: [
        { label: d.tDate },
        { label: d.tTitle, stroke: () => c.line, width: 2, spanGaps: false, points: { show: false } },
        {
          // Yesterday to today, dashed, with a hollow marker on today only.
          label: todayText || d.tTitle,
          stroke: () => c.line,
          width: 2,
          dash: [4, 4],
          spanGaps: false,
          points: {
            show: true,
            size: 9,
            width: 2,
            stroke: () => c.line,
            fill: () => c.bg,
            filter: () => (data.partial >= 0 ? [data.partial] : null),
          },
        },
        { label: d.tAvg, stroke: () => c.axis, width: 1.5, spanGaps: false, points: { show: false }, value: (u, v) => (v == null ? '--' : Math.round(v)) },
      ],
      axes: [
        { values: tickValues(data.xs, document.documentElement.lang || undefined), stroke: () => c.axis, ticks: { stroke: () => c.axis }, grid: { stroke: () => c.grid } },
        { stroke: () => c.axis, incrs: INCRS, ticks: { stroke: () => c.axis }, grid: { stroke: () => c.grid } },
      ],
      scales: { x: { time: true }, y: { range: (u, min, max) => [0, Math.max(max * 1.1, 1)] } },
      hooks: { setCursor: [(u) => host.classList.toggle('is-live', u.cursor.idx != null)] },
    }, [data.xs, data.complete, data.today, data.average], host)

    const current = plot
    observer = new ResizeObserver(() => {
      if (host.clientWidth > 0) current.setSize({ width: host.clientWidth, height: HEIGHT })
    })
    observer.observe(host)
  }

  // uPlot paints once and never re-reads CSS, so a theme change needs a repaint.
  const restyle = () => {
    c = colours()
    plot?.redraw()
  }
  new MutationObserver(restyle).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
  matchMedia('(prefers-color-scheme: dark)').addEventListener?.('change', restyle)

  render()
}
