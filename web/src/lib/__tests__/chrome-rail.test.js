// @vitest-environment jsdom
//
// The map's control layout: one rail top-right, one bar along the bottom,
// the wind note inside the layers panel.
import { describe, it, expect } from 'vitest'
import { mountChrome } from '../chrome.js'

const cfg = (over = {}) => ({
  t: {
    tier: {},
    zoomIn: 'Zoom in',
    zoomOut: 'Zoom out',
    zoomReset: 'Reset view',
    orientLabel: 'Orientation',
    orientNorth: 'North',
    tableLink: 'Data table',
    locateButton: 'Find me',
    layersButton: 'Layers',
  },
  noDataColour: '#999',
  lang: 'en',
  metric: 'P2',
  metricLabels: { P2: 'PM2.5' },
  metricUnits: { P2: 'µg/m³' },
  tableHref: '#sensors-h',
  ...over,
})

// The shell as the templates render it: the server puts the freshness overlay in the bar.
const page = ({ bar = true } = {}) => {
  const shell = document.createElement('div')
  shell.className = 'map-shell'
  const el = document.createElement('div')
  el.className = 'map'
  el.id = 'map'
  shell.appendChild(el)
  const fresh = document.createElement('div')
  fresh.className = 'map-freshness'
  if (bar) {
    const b = document.createElement('div')
    b.className = 'map-bar'
    b.appendChild(fresh)
    shell.appendChild(b)
  } else {
    shell.appendChild(fresh)
  }
  document.body.appendChild(shell)
  return { shell, el, fresh }
}

const kind = (n) => n.className.split(' ')[0]

describe('the control rail', () => {
  it('stacks zoom, compass, fullscreen, table and locate in that order', () => {
    const { el } = page()
    mountChrome(el, cfg())
    const rail = el.querySelector(':scope > .map-rail')
    expect(rail, 'no rail on the frame').toBeTruthy()
    expect([...rail.children].map(kind)).toEqual(['map-zoom', 'map-orient', 'map__full', 'map-table', 'map-locate'])
  })

  it('keeps only zoom in and out in the stack', () => {
    const { el } = page()
    mountChrome(el, cfg())
    const acts = [...el.querySelectorAll('.map-zoom [data-act]')].map((b) => b.getAttribute('data-act'))
    expect(acts).toEqual(['in', 'out'])
  })

  it('puts reset view in the compass panel and hands it to the camera wiring', () => {
    const { el } = page()
    const c = mountChrome(el, cfg())
    const reset = c.orient.panel.querySelector('[data-act="reset"]')
    expect(reset?.textContent).toBe('Reset view')
    expect(c.zoomButtons.reset).toBe(reset)
    expect(Object.keys(c.zoomButtons).sort()).toEqual(['in', 'out', 'reset'])
  })

  it('links the table to the configured target and leaves it out without one', () => {
    const a = page()
    mountChrome(a.el, cfg())
    expect(a.el.querySelector('.map-rail .map-table').getAttribute('href')).toBe('#sensors-h')
    const b = page()
    mountChrome(b.el, cfg({ tableHref: '' }))
    expect(b.el.querySelector('.map-table')).toBeNull()
  })
})

describe('the bottom bar', () => {
  it('leads the server-rendered bar with the legend, the time controls after it', () => {
    const { shell, el, fresh } = page()
    mountChrome(el, cfg())
    const bar = shell.querySelector(':scope > .map-bar')
    expect(bar.firstElementChild.matches('details.scale--onmap')).toBe(true)
    expect(bar.contains(fresh)).toBe(true)
    expect(shell.querySelectorAll('.map-bar')).toHaveLength(1)
  })

  it('builds a bar when the page has none', () => {
    const { shell, el } = page({ bar: false })
    mountChrome(el, cfg())
    const bar = shell.querySelector(':scope > .map-bar')
    expect(bar, 'no bar built').toBeTruthy()
    expect(bar.querySelector('details.scale--onmap')).toBeTruthy()
  })

  it('carries the unscaled-metric note as a line in the bar, not a chip on the map', () => {
    const { shell, el } = page()
    const c = mountChrome(el, cfg())
    expect(el.querySelector('.map-note')).toBeNull()
    c.showNote('No scale for this metric')
    const note = shell.querySelector('.map-bar .map-bar__note')
    expect(note.textContent).toBe('No scale for this metric')
    expect(note.hidden).toBe(false)
    c.showNote('')
    expect(note.hidden).toBe(true)
  })

  it('rides into the frame in fullscreen and back out', () => {
    const { shell, el } = page()
    mountChrome(el, cfg())
    const bar = shell.querySelector('.map-bar')
    el.querySelector('.map__full').click()
    expect(bar.parentElement).toBe(el)
    el.querySelector('.map__full').click()
    expect(bar.parentElement).toBe(shell)
  })
})

describe('the wind note', () => {
  it('sits at the bottom of the layers panel, shown only with the arrows', () => {
    const { el } = page()
    const c = mountChrome(el, cfg())
    expect(el.ownerDocument.querySelector('.map-wind-label')).toBeNull()
    const note = c.layersUI.panel.querySelector('.map-layers__wind')
    expect(note, 'no wind note in the layers panel').toBeTruthy()
    expect(c.layersUI.panel.lastElementChild).toBe(note)
    expect(note.hidden).toBe(true)
    c.showWind(true, 'Wind forecast, valid now')
    expect(note.hidden).toBe(false)
    expect(note.textContent).toBe('Wind forecast, valid now')
    c.showWind(false, '')
    expect(note.hidden).toBe(true)
  })
})
