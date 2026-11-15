// The visitors chart on /about. Fetches the daily unique visitors and draws
// them with uPlot, the chart library the rest of the site uses. Only the empty
// and failed states carry text; the heading and footnote are server-rendered.
import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'
import { getJSON } from '../lib/api.js'
import { tickValues } from '../lib/timeaxis.js'
import { visitorSeries } from '../lib/visitors.js'

const URL = '/api/v1/visitors?days=30'
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
})

function message(el, text) {
  const p = document.createElement('p')
  p.className = 'chart-message'
  p.textContent = text
  el.replaceChildren(p)
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

  const data = visitorSeries(body)
  if (!data) {
    message(el, d.tEmpty || '')
    return
  }

  const host = document.createElement('div')
  host.className = 'visitors-chart'
  el.replaceChildren(host)

  const c = colours()
  const plot = new uPlot({
    width: host.clientWidth || 600,
    height: HEIGHT,
    // The legend doubles as the hover readout; it is hidden until the cursor
    // is on a point (see .visitors-chart in app.css).
    series: [
      { label: d.tDate },
      { label: d.tTitle, stroke: c.line, width: 2, spanGaps: false },
    ],
    axes: [
      { values: tickValues(data[0], document.documentElement.lang || undefined), stroke: c.axis, ticks: { stroke: c.axis }, grid: { stroke: c.grid } },
      { stroke: c.axis, incrs: INCRS, ticks: { stroke: c.axis }, grid: { stroke: c.grid } },
    ],
    scales: { x: { time: true }, y: { range: (u, min, max) => [0, Math.max(max * 1.1, 1)] } },
    hooks: { setCursor: [(u) => host.classList.toggle('is-live', u.cursor.idx != null)] },
  }, data, host)

  new ResizeObserver(() => {
    if (host.clientWidth > 0) plot.setSize({ width: host.clientWidth, height: HEIGHT })
  }).observe(host)

  // uPlot paints once and never re-reads CSS, so a theme change needs a repaint.
  const restyle = () => {
    const n = colours()
    // uPlot calls strokes as functions on redraw. A plain string throws.
    plot.axes.forEach((a) => {
      a.stroke = () => n.axis
      a.ticks.stroke = () => n.axis
      a.grid.stroke = () => n.grid
    })
    plot.series[1].stroke = () => n.line
    plot.redraw()
  }
  new MutationObserver(restyle).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
  matchMedia('(prefers-color-scheme: dark)').addEventListener?.('change', restyle)
}
