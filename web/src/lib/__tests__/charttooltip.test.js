// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { placeTooltip, focusSeries, formatStamp, tooltipContent, chartTooltip } from '../charttooltip.js'

const plot = { left: 0, top: 0, width: 400, height: 200 }
const card = { width: 100, height: 50 }

describe('placeTooltip', () => {
  it('centres the card 12px above the point', () => {
    const at = placeTooltip({ x: 200, y: 120, ...card, plot })
    expect(at).toMatchObject({ left: 150, top: 58, placement: 'above' })
    expect(at.arrow).toBe(50)
  })

  it('flips below the point when there is no room above', () => {
    const at = placeTooltip({ x: 200, y: 30, ...card, plot })
    expect(at.placement).toBe('below')
    expect(at.top).toBe(42)
  })

  it('stays above when the card exactly fits', () => {
    expect(placeTooltip({ x: 200, y: 62, ...card, plot }).placement).toBe('above')
  })

  it('clamps to the left edge and points the arrow back at the point', () => {
    const at = placeTooltip({ x: 10, y: 120, ...card, plot })
    expect(at.left).toBe(0)
    expect(at.arrow).toBe(10)
  })

  it('clamps to the right edge', () => {
    const at = placeTooltip({ x: 395, y: 120, ...card, plot })
    expect(at.left).toBe(300)
    expect(at.arrow).toBe(90)
  })

  it('works in a plot that does not start at the origin', () => {
    const at = placeTooltip({ x: 40, y: 90, ...card, plot: { left: 30, top: 20, width: 300, height: 150 } })
    expect(at.left).toBe(30)
    expect(at.top).toBe(28)
  })

  it('pins a card wider than the plot to the left edge', () => {
    const at = placeTooltip({ x: 100, y: 120, width: 500, height: 50, plot })
    expect(at.left).toBe(0)
    expect(at.arrow).toBe(100)
  })

  it('keeps the card inside a plot too short for either side', () => {
    const at = placeTooltip({ x: 200, y: 40, width: 100, height: 50, plot: { left: 0, top: 0, width: 400, height: 80 } })
    expect(at.top).toBeGreaterThanOrEqual(0)
    expect(at.top + 50).toBeLessThanOrEqual(80)
  })
})

describe('focusSeries', () => {
  it('picks the point nearest the cursor row', () => {
    expect(focusSeries([{ index: 0, y: 10 }, { index: 1, y: 90 }, { index: 2, y: 60 }], 70)).toBe(2)
  })
  it('is null when no series has a reading', () => {
    expect(focusSeries([], 70)).toBeNull()
  })
})

describe('formatStamp', () => {
  const t = Date.UTC(2026, 9, 4, 7, 45) / 1000
  it('writes day, month and 24-hour time in English', () => {
    expect(formatStamp(t, 'en', 'UTC')).toBe('4 Oct 07:45')
  })
  it('writes the Bulgarian month name', () => {
    expect(formatStamp(t, 'bg', 'UTC')).toBe('4 октомври 07:45')
  })
})

describe('tooltipContent', () => {
  const t = Date.UTC(2026, 9, 4, 7, 45) / 1000
  const pm = { name: 'PM2.5', colour: '#111', unit: 'µg/m³', value: 7.2 }

  it('reads value and unit, then metric and time, for one series', () => {
    const c = tooltipContent({ time: t, lang: 'en', series: [pm], timeZone: 'UTC' })
    expect(c.single).toBe(true)
    expect(c.rows[0].text).toBe('7.2 µg/m³')
    expect(c.caption).toBe('PM2.5 · 4 Oct 07:45')
    expect(c.live).toBe('PM2.5 7.2 µg/m³, 4 Oct 07:45')
  })

  it('lists the hovered series first and keeps the rest in order', () => {
    const series = [pm, { name: 'Median', unit: 'µg/m³', value: 5 }, { name: 'High', unit: 'µg/m³', value: 9 }]
    const c = tooltipContent({ time: t, lang: 'en', series, focus: 2, timeZone: 'UTC' })
    expect(c.single).toBe(false)
    expect(c.rows.map((r) => r.name)).toEqual(['High', 'PM2.5', 'Median'])
    expect(c.stamp).toBe('4 Oct 07:45')
  })

  it('shows a dash for a series with no reading and none when all are missing', () => {
    const series = [pm, { name: 'Temp', unit: '°C', value: null }]
    expect(tooltipContent({ time: t, lang: 'en', series, timeZone: 'UTC' }).rows[1].text).toBe('—')
    expect(tooltipContent({ time: t, lang: 'en', series: [{ ...pm, value: null }], timeZone: 'UTC' })).toBeNull()
  })

  it('formats numbers and time per language', () => {
    const c = tooltipContent({ time: t, lang: 'bg', series: [pm], timeZone: 'UTC' })
    expect(c.rows[0].text).toBe('7,2 µg/m³')
    expect(c.caption).toBe('PM2.5 · 4 октомври 07:45')
  })

  it('treats zero as a reading', () => {
    expect(tooltipContent({ time: t, lang: 'en', series: [{ ...pm, value: 0 }], timeZone: 'UTC' }).rows[0].text).toBe('0 µg/m³')
  })
})

// A stand-in for the uPlot instance: the plugin only touches these members.
function fakeChart(data, extra = {}) {
  const root = document.createElement('div')
  const over = document.createElement('div')
  root.appendChild(over)
  document.body.appendChild(root)
  Object.defineProperty(over, 'clientWidth', { value: 400 })
  Object.defineProperty(over, 'clientHeight', { value: 200 })
  return {
    root, over, data,
    series: [{}, { scale: 'y' }, ...(data.length > 2 ? [{ scale: 'y' }] : [])],
    cursor: { idx: null, top: 100, left: 100 },
    valToPos: (v, scale) => (scale === 'x' ? v : 200 - v * 10),
    setCursor: vi.fn(),
    ...extra,
  }
}

describe('chartTooltip plugin', () => {
  afterEach(() => { vi.useRealTimers(); document.body.replaceChildren() })
  const line = { name: 'PM2.5', colour: '#111', unit: 'µg/m³' }

  function setup(data, lines = [line], opts = {}) {
    const u = fakeChart(data)
    const plugin = chartTooltip({ lang: 'en', lines, timeZone: 'UTC', ...opts })
    plugin.hooks.init(u)
    return { u, plugin, card: u.over.querySelector('.chart-tip'), live: u.root.querySelector('[aria-live]') }
  }

  it('shows the card on a reading and hides it when the cursor leaves', () => {
    const { u, plugin, card } = setup([[1000, 2000], [7.2, 8]])
    expect(card.hidden).toBe(true)
    u.cursor.idx = 0
    plugin.hooks.setCursor(u)
    expect(card.hidden).toBe(false)
    expect(card.querySelector('.chart-tip__value').textContent).toBe('7.2 µg/m³')
    u.cursor.idx = null
    plugin.hooks.setCursor(u)
    expect(card.hidden).toBe(true)
  })

  it('hides the card from assistive tech and speaks through a polite region only', () => {
    const { card, live } = setup([[1000], [7.2]])
    expect(card.getAttribute('aria-hidden')).toBe('true')
    expect(live.getAttribute('aria-live')).toBe('polite')
  })

  it('announces once per index, after the cursor rests', () => {
    vi.useFakeTimers()
    const { u, plugin, live } = setup([[1000, 2000], [7.2, 8]])
    u.cursor.idx = 0
    plugin.hooks.setCursor(u)
    plugin.hooks.setCursor(u)
    expect(live.textContent).toBe('')
    u.cursor.idx = 1
    plugin.hooks.setCursor(u)
    vi.advanceTimersByTime(300)
    expect(live.textContent).toBe('PM2.5 8 µg/m³, 1 Jan 00:33')
  })

  it('leaves out a series the key has switched off', () => {
    const lines = [line, { name: 'Median', colour: '#222', unit: 'µg/m³' }]
    const { u, plugin, card } = setup([[1000], [7.2], [5]], lines)
    u.series[2].show = false
    u.cursor.idx = 0
    plugin.hooks.setCursor(u)
    expect(card.querySelector('.chart-tip__value').textContent).toBe('7.2 µg/m³')
  })

  it('anchors on the series nearest the cursor and lists it first', () => {
    const lines = [line, { name: 'Median', colour: '#222', unit: 'µg/m³' }]
    const { u, plugin, card } = setup([[1000], [7.2], [5]], lines)
    u.cursor.idx = 0
    u.cursor.top = 150
    plugin.hooks.setCursor(u)
    const names = [...card.querySelectorAll('.chart-tip__name')].map((n) => n.textContent)
    expect(names).toEqual(['Median', 'PM2.5'])
    expect(card.dataset.placement).toBe('above')
    expect(card.style.top).toBe(`${150 - 12 - 0}px`)
  })

  it('keeps a touched point until a tap outside the plot', () => {
    const { u, plugin, card } = setup([[1000], [7.2]])
    u.setCursor.mockImplementation(({ left }) => { u.cursor.idx = left < 0 ? null : 0; plugin.hooks.setCursor(u) })
    u.over.dispatchEvent(Object.assign(new Event('pointerdown', { bubbles: true }), { pointerType: 'touch', clientX: 10, clientY: 10 }))
    expect(card.hidden).toBe(false)
    u.cursor.idx = null
    plugin.hooks.setCursor(u)
    expect(card.hidden).toBe(false)
    document.body.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    expect(card.hidden).toBe(true)
  })

  it('ignores a mouse pointerdown for stickiness', () => {
    const { u, plugin, card } = setup([[1000], [7.2]])
    u.over.dispatchEvent(Object.assign(new Event('pointerdown', { bubbles: true }), { pointerType: 'mouse', clientX: 10, clientY: 10 }))
    expect(u.setCursor).not.toHaveBeenCalled()
    u.cursor.idx = null
    plugin.hooks.setCursor(u)
    expect(card.hidden).toBe(true)
  })
})
