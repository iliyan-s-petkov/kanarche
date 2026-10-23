// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, unmount, flushSync } from 'svelte'
import SensorPanel from '../SensorPanel.svelte'
import SensorChart from '../SensorChart.svelte'
import { createPanelLink } from '../../lib/panellink.svelte.js'
import { clearCache } from '../../lib/api.js'
import { setScales, setSensors } from '../../lib/sensors.svelte.js'

vi.mock('uplot', () => ({
  default: vi.fn(function () {
    this.setSize = vi.fn()
  }),
}))

const model = { fraction: 0.4, colour: '#fa0', stops: [{ fraction: 1, colour: '#fa0' }], range: { min: 0, max: 50 } }
const rows = [
  { metric: 'P2', label: 'PM2.5', value: 12.4, unit: 'µg/m³', missing: false, model },
  { metric: 'P1', label: 'PM10', value: 20, unit: 'µg/m³', missing: false, model },
]
const details = [{ key: 'devices', label: 'Devices', value: '42, 43' }]

const chartProps = {
  stationId: 42,
  sources: { P2: 42, P1: 42 },
  options: [{ metric: 'P2', label: 'PM2.5' }, { metric: 'P1', label: 'PM10' }],
  periods: ['24h', '7d', '30d', '1y'],
  periodLabels: ['24 hours', '7 days', '30 days', '1 year'],
  periodShortLabels: ['24h', '7d', '30d', '1y'],
  initialPeriod: '24h',
  initialMetric: 'P2',
  metricLegend: 'Metric', periodLegend: 'Period', customLabel: 'Custom range',
  fromLabel: 'From', toLabel: 'To', nowLabel: 'Now', resetLabel: 'Reset chart', rangeInvalid: 'bad',
  nearbyLegend: 'Nearby sensors', nearbyOff: 'off', nearbySingleOnly: 'one metric',
  nearbyLabels: { low: 'Low', median: 'Median', high: 'High' },
  moreLabel: 'More actions', aboutLabel: 'About this station', shareLabel: 'Share', embedLabel: 'Embed',
  shareDone: 'Link copied', embedDone: 'Embed code copied', copyFailed: 'Could not copy',
  colours: ['#111', '#222', '#333'],
  timeLabel: 'Time', empty: 'none', unavailable: 'unavailable',
}

let comps = []
function teardown() {
  comps.forEach((c) => unmount(c))
  comps = []
  document.body.innerHTML = ''
}
afterEach(() => {
  clearCache()
  teardown()
  setScales(null)
  setSensors(null, null)
  vi.restoreAllMocks()
})

function host() {
  const t = document.createElement('div')
  document.body.appendChild(t)
  return t
}

function renderPanel(extra = {}) {
  const target = host()
  const onclose = vi.fn()
  const link = createPanelLink()
  comps.push(mount(SensorPanel, {
    target,
    props: {
      rows, title: 'Sensor 42', flagText: '', closeLabel: 'Close', noValue: 'no reading', onclose,
      details, detailsLabel: 'About this station', source: 'sensor.community',
      updated: Date.parse('2026-09-28T11:56:00Z'), now: Date.parse('2026-09-28T12:00:00Z'), locale: 'en',
      link, ...extra,
    },
  }))
  return { target, onclose, link }
}

function renderChart(extra = {}, link = createPanelLink()) {
  vi.spyOn(globalThis, 'fetch').mockImplementation(async () =>
    new Response(JSON.stringify({ t: ['2026-08-14T00:00:00Z'], v: [1] }), { status: 200 }))
  const target = host()
  comps.push(mount(SensorChart, { target, props: { ...chartProps, link, ...extra } }))
  return { target, link }
}

const urls = () => globalThis.fetch.mock.calls.map((c) => String(c[0]))
const items = (target) => [...target.querySelectorAll('[role="menuitem"]')]
function openMenu(target) {
  target.querySelector('.panel-more').click()
  flushSync()
}

describe('SensorPanel phone header and gauges', () => {
  it('shows network and relative age under the title', () => {
    const { target } = renderPanel()
    const sub = target.querySelector('.panel-sub')
    expect(sub.textContent).toContain('sensor.community')
    expect(sub.textContent).toMatch(/4 min/)
  })

  it('formats gauge values with the page locale', () => {
    const fr = [{ metric: 'P2', label: 'PM2.5', value: 8.85, unit: 'µg/m³', missing: false, model }]
    const bg = renderPanel({ rows: fr, locale: 'bg' }).target.querySelector('button.gauge')
    expect(bg.querySelector('.gauge__value').textContent).toBe('8,9 µg/m³')
    expect(bg.hasAttribute('aria-label')).toBe(false)
    const en = renderPanel({ rows: fr, locale: 'en' }).target.querySelector('button.gauge')
    expect(en.querySelector('.gauge__value').textContent).toBe('8.9 µg/m³')
  })

  it('renders gauges as buttons, pressed for the chart metric', () => {
    const link = createPanelLink()
    link.metrics = ['P1']
    const { target } = renderPanel({ link })
    const gauges = target.querySelectorAll('button.gauge')
    expect(gauges.length).toBe(2)
    expect(gauges[0].getAttribute('aria-pressed')).toBe('false')
    expect(gauges[1].getAttribute('aria-pressed')).toBe('true')
  })

  it('a gauge tap selects that metric alone', () => {
    const { target, link } = renderPanel()
    link.metrics = ['P1', 'P2']
    flushSync()
    target.querySelectorAll('button.gauge')[0].click()
    flushSync()
    expect(link.metrics).toEqual(['P2'])
  })

  it('the info button opens the about sheet, Escape closes only the sheet', () => {
    const { target, onclose } = renderPanel()
    expect(target.querySelector('[role="dialog"]')).toBeNull()
    target.querySelector('.panel-info').click()
    flushSync()
    const dialog = target.querySelector('[role="dialog"]')
    expect(dialog.getAttribute('aria-label')).toBe('About this station')
    expect(dialog.textContent).toContain('Devices')
    dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    flushSync()
    expect(target.querySelector('[role="dialog"]')).toBeNull()
    expect(onclose).not.toHaveBeenCalled()
  })

  it('opens the sheet when the link says so', () => {
    const { target, link } = renderPanel()
    link.aboutOpen = true
    flushSync()
    expect(target.querySelector('[role="dialog"]')).not.toBeNull()
  })
})

describe('SensorChart phone controls', () => {
  it('has a segmented period control with the short labels', () => {
    const { target } = renderChart()
    const btns = [...target.querySelectorAll('.period-seg button')]
    expect(btns.map((b) => b.textContent.trim())).toEqual(['24h', '7d', '30d', '1y'])
    expect(btns[0].getAttribute('aria-pressed')).toBe('true')
  })

  it('a segment re-requests the series over that window', async () => {
    const { target } = renderChart()
    target.querySelectorAll('.period-seg button')[1].click()
    flushSync()
    await vi.waitFor(() => expect(urls().some((u) => u.includes('period=7d'))).toBe(true))
    expect(target.querySelectorAll('.period-seg button')[1].getAttribute('aria-pressed')).toBe('true')
  })

  it('follows the metric the link carries', async () => {
    const { link } = renderChart()
    link.metrics = ['P1']
    flushSync()
    await vi.waitFor(() => expect(urls().some((u) => u.includes('metric=P1'))).toBe(true))
  })

  it('the more menu holds the six items and opens on click', () => {
    const { target } = renderChart()
    const more = target.querySelector('.panel-more')
    expect(more.getAttribute('aria-expanded')).toBe('false')
    expect(target.querySelector('[role="menu"]')).toBeNull()
    openMenu(target)
    expect(more.getAttribute('aria-expanded')).toBe('true')
    const labels = items(target).map((i) => i.textContent.trim())
    expect(labels.length).toBe(6)
    expect(labels[0]).toContain('Nearby sensors')
    expect(labels[1]).toContain('Custom range')
    expect(labels.slice(2)).toEqual(['Reset chart', 'About this station', 'Share', 'Embed'])
  })

  it('Escape closes the menu, returns focus and does not reach the panel', () => {
    const { target } = renderChart()
    const more = target.querySelector('.panel-more')
    openMenu(target)
    const esc = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })
    const stop = vi.spyOn(esc, 'stopPropagation')
    items(target)[0].dispatchEvent(esc)
    flushSync()
    expect(target.querySelector('[role="menu"]')).toBeNull()
    expect(document.activeElement).toBe(more)
    expect(stop).toHaveBeenCalled()
  })

  it('arrow keys move between items', () => {
    const { target } = renderChart()
    openMenu(target)
    const list = items(target)
    list[0].focus()
    list[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }))
    expect(document.activeElement).toBe(list[1])
    list[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true }))
    expect(document.activeElement).toBe(list[0])
  })

  it('Custom range switches to the from/to fields and closes the menu', () => {
    const { target } = renderChart()
    openMenu(target)
    items(target)[1].click()
    flushSync()
    expect(target.querySelector('[role="menu"]')).toBeNull()
    expect(target.querySelectorAll('.chart-range input[type="datetime-local"]').length).toBe(2)
  })

  it('Reset chart returns the window to the opening one', () => {
    const { target } = renderChart()
    target.querySelectorAll('.period-seg button')[2].click()
    flushSync()
    openMenu(target)
    items(target)[2].click()
    flushSync()
    expect(target.querySelectorAll('.period-seg button')[0].getAttribute('aria-pressed')).toBe('true')
  })

  it('About this station asks the panel to open its sheet', () => {
    const { target, link } = renderChart()
    openMenu(target)
    items(target)[3].click()
    flushSync()
    expect(link.aboutOpen).toBe(true)
    expect(target.querySelector('[role="menu"]')).toBeNull()
  })

  it('Nearby sensors is disabled without an area', () => {
    const { target } = renderChart()
    openMenu(target)
    expect(items(target)[0].getAttribute('aria-disabled')).toBe('true')
  })

  it('Nearby sensors lists the three lines when the area is known', () => {
    setSensors({}, 'sofia')
    const { target } = renderChart()
    openMenu(target)
    items(target)[0].click()
    flushSync()
    expect(target.querySelectorAll('[role="menu"] input[type="checkbox"]').length).toBe(3)
  })

  it('Share copies the page address and says so', async () => {
    const writeText = vi.fn().mockResolvedValue()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    Object.defineProperty(navigator, 'share', { value: undefined, configurable: true })
    const { target } = renderChart()
    openMenu(target)
    items(target)[4].click()
    await vi.waitFor(() => expect(target.querySelector('[role="status"]').textContent).toContain('Link copied'))
    expect(writeText).toHaveBeenCalledWith(window.location.href)
  })

  it('Embed copies an iframe snippet and says so', async () => {
    const writeText = vi.fn().mockResolvedValue()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const { target } = renderChart()
    openMenu(target)
    items(target)[5].click()
    await vi.waitFor(() => expect(target.querySelector('[role="status"]').textContent).toContain('Embed code copied'))
    expect(writeText.mock.calls[0][0]).toMatch(/^<iframe src="http[^"]*\/embed\?metric=P2"/)
    expect(writeText.mock.calls[0][0]).toContain('title="Kanarche"')
  })

  it('titles the embed with the masthead brand of the page language', async () => {
    const writeText = vi.fn().mockResolvedValue()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const brand = document.createElement('a')
    brand.className = 'masthead__brand'
    brand.setAttribute('aria-label', 'Канарче')
    document.body.append(brand)
    try {
      const { target } = renderChart()
      openMenu(target)
      items(target)[5].click()
      await vi.waitFor(() => expect(writeText).toHaveBeenCalled())
      expect(writeText.mock.calls[0][0]).toContain('title="Канарче"')
    } finally {
      brand.remove()
    }
  })

  it('says so when copying fails', async () => {
    Object.defineProperty(navigator, 'clipboard', { value: { writeText: vi.fn().mockRejectedValue(new Error('no')) }, configurable: true })
    Object.defineProperty(navigator, 'share', { value: undefined, configurable: true })
    const { target } = renderChart()
    openMenu(target)
    items(target)[4].click()
    await vi.waitFor(() => expect(target.querySelector('[role="status"]').textContent).toContain('Could not copy'))
  })
})
