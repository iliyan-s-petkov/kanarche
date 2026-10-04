// The reading card shown at the hovered point of a uPlot chart, shared by every chart.
// placeTooltip, focusSeries and tooltipContent are pure; chartTooltip wires them to uPlot.

const DASH = '—'
// The card sits this far from the point; its arrow fills the gap.
export const GAP = 12
// The arrow never slides closer than this to a card corner.
const ARROW_INSET = 10
// A reading is announced once the cursor has rested this long on one point.
const LIVE_DELAY = 300

const clamp = (v, lo, hi) => Math.min(Math.max(v, lo), hi)

// Card top-left for a point, all in the plot's own coordinates. Above by default, below when
// the top is short of room, and always inside the plot box horizontally.
export function placeTooltip({ x, y, width, height, plot, gap = GAP }) {
  const right = plot.left + plot.width
  const bottom = plot.top + plot.height
  // A card wider than the plot cannot fit; it is pinned to the left edge.
  const left = width >= plot.width ? plot.left : clamp(x - width / 2, plot.left, right - width)
  const above = y - gap - height
  const below = y + gap
  let placement
  let top
  if (above >= plot.top) {
    placement = 'above'
    top = above
  } else if (below + height <= bottom) {
    placement = 'below'
    top = below
  } else {
    // Neither side fits: take the roomier one and keep the card inside the plot.
    placement = y - plot.top >= bottom - y ? 'above' : 'below'
    top = clamp(placement === 'above' ? above : below, plot.top, Math.max(plot.top, bottom - height))
  }
  return { left, top, placement, arrow: clamp(x - left, ARROW_INSET, Math.max(ARROW_INSET, width - ARROW_INSET)) }
}

// The series whose point is closest to the cursor, among those with a reading.
// points: [{ index, y }] with y in the same units as cursorY. Null when none has one.
export function focusSeries(points, cursorY) {
  let best = null
  for (const p of points) {
    if (best === null || Math.abs(p.y - cursorY) < Math.abs(best.y - cursorY)) best = p
  }
  return best ? best.index : null
}

export function formatReading(value, lang) {
  if (value == null || Number.isNaN(value)) return DASH
  return new Intl.NumberFormat(lang, { maximumFractionDigits: 1 }).format(value)
}

// "4 Oct 07:45": day, month, 24-hour time, in the page language; seconds are epoch seconds.
// Bulgarian has no short month name in ICU (it yields "10"), so a numeric result falls back to the long name.
export function formatStamp(seconds, lang, timeZone) {
  const date = new Date(seconds * 1000)
  const part = (opts, type) => new Intl.DateTimeFormat(lang, { ...opts, timeZone }).formatToParts(date).find((p) => p.type === type)?.value ?? ''
  let month = part({ month: 'short' }, 'month').replace(/\.$/, '')
  if (/^\d+$/.test(month)) month = part({ month: 'long' }, 'month')
  const time = new Intl.DateTimeFormat(lang, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZone }).format(date)
  return `${part({ day: 'numeric' }, 'day')} ${month} ${time}`
}

// What the card says. series: the visible lines at the hovered index as { name, colour, unit, value };
// focus indexes the one the cursor is on. Null when no visible line has a reading there.
export function tooltipContent({ time, lang, series, focus = 0, timeZone }) {
  if (!series.some((s) => s.value != null && !Number.isNaN(s.value))) return null
  const stamp = formatStamp(time, lang, timeZone)
  const ordered = [series[focus], ...series.filter((_, i) => i !== focus)].filter(Boolean)
  const rows = ordered.map((s) => {
    const value = formatReading(s.value, lang)
    const text = s.value == null || Number.isNaN(s.value) ? DASH : `${value} ${s.unit ?? ''}`.trim()
    return { name: s.name, colour: s.colour, value, unit: s.unit ?? '', text }
  })
  const single = series.length === 1
  const live = single
    ? `${rows[0].name} ${rows[0].text}, ${stamp}`
    : `${rows.map((r) => `${r.name} ${r.text}`).join(', ')}, ${stamp}`
  return { single, stamp, caption: `${rows[0].name} · ${stamp}`, rows, live }
}

const el = (tag, cls, parent) => {
  const node = document.createElement(tag)
  node.className = cls
  parent?.appendChild(node)
  return node
}

// A uPlot plugin. lines: one { name, colour, unit } per data series, in series order.
export function chartTooltip({ lang, lines, timeZone, liveDelay = LIVE_DELAY }) {
  let card
  let rowsEl
  let capEl
  let live
  let over
  let sticky = false
  let lastIdx = null
  let timer = null
  let chart
  const cleanups = []

  function hide() {
    card.hidden = true
    lastIdx = null
    clearTimeout(timer)
    live.textContent = ''
  }

  function announce(text) {
    clearTimeout(timer)
    timer = setTimeout(() => { live.textContent = text }, liveDelay)
  }

  function draw(u) {
    const idx = u.cursor.idx
    if (idx == null) {
      if (!sticky) hide()
      return
    }
    const visible = []
    lines.forEach((line, i) => {
      if (u.series[i + 1]?.show === false) return
      visible.push({ ...line, value: u.data[i + 1][idx], scale: u.series[i + 1].scale })
    })
    // Series are focused by the pixel row of their own point, so the cursor picks the nearest line.
    const points = []
    visible.forEach((s, index) => {
      if (s.value != null) points.push({ index, y: u.valToPos(s.value, s.scale) })
    })
    const focus = focusSeries(points, u.cursor.top)
    const time = u.data[0][idx]
    const content = focus === null ? null : tooltipContent({ time, lang, series: visible, focus, timeZone })
    if (!content) {
      hide()
      return
    }
    // Rebuilt from text nodes only; no markup comes from data.
    rowsEl.replaceChildren()
    card.classList.toggle('chart-tip--multi', !content.single)
    if (content.single) {
      const r = content.rows[0]
      const head = el('div', 'chart-tip__value', rowsEl)
      head.textContent = r.text
      capEl.textContent = content.caption
    } else {
      capEl.textContent = content.stamp
      content.rows.forEach((r, i) => {
        const row = el('div', i === 0 ? 'chart-tip__row chart-tip__row--focus' : 'chart-tip__row', rowsEl)
        const sw = el('span', 'chart-tip__swatch', row)
        sw.style.backgroundColor = r.colour || 'currentColor'
        el('span', 'chart-tip__name', row).textContent = r.name
        el('span', 'chart-tip__num', row).textContent = r.text
      })
    }
    card.hidden = false
    const x = u.valToPos(time, 'x')
    const y = points[points.findIndex((p) => p.index === focus)].y
    const plot = { left: 0, top: 0, width: over.clientWidth, height: over.clientHeight }
    const at = placeTooltip({ x, y, width: card.offsetWidth, height: card.offsetHeight, plot })
    card.style.left = `${at.left}px`
    card.style.top = `${at.top}px`
    card.style.setProperty('--tip-arrow', `${at.arrow}px`)
    card.dataset.placement = at.placement
    if (idx !== lastIdx) {
      lastIdx = idx
      announce(content.live)
    }
  }

  // Touch: a tap or drag on the plot places the cursor, and it stays until a tap elsewhere.
  function onTouch(e) {
    if (e.pointerType !== 'touch') return
    sticky = true
    const r = over.getBoundingClientRect()
    chart.setCursor({ left: e.clientX - r.left, top: e.clientY - r.top })
  }

  function onOutside(e) {
    if (!sticky || over.contains(e.target)) return
    sticky = false
    chart.setCursor({ left: -10, top: -10 })
    hide()
  }

  return {
    hooks: {
      init(u) {
        chart = u
        over = u.over
        card = el('div', 'chart-tip', over)
        card.hidden = true
        card.setAttribute('aria-hidden', 'true')
        rowsEl = el('div', 'chart-tip__rows', card)
        capEl = el('div', 'chart-tip__caption', card)
        live = el('div', 'visually-hidden', u.root)
        live.setAttribute('aria-live', 'polite')
        over.addEventListener('pointerdown', onTouch)
        over.addEventListener('pointermove', onTouch)
        document.addEventListener('pointerdown', onOutside)
        cleanups.push(() => {
          over.removeEventListener('pointerdown', onTouch)
          over.removeEventListener('pointermove', onTouch)
          document.removeEventListener('pointerdown', onOutside)
        })
      },
      setCursor: draw,
      destroy() {
        clearTimeout(timer)
        cleanups.forEach((fn) => fn())
      },
    },
  }
}
