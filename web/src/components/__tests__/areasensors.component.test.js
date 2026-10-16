// @vitest-environment jsdom
import { describe, it, expect, afterEach, vi } from 'vitest'
import { mount, unmount, flushSync } from 'svelte'
import AreaSensors from '../AreaSensors.svelte'
import { setSensors, setScales } from '../../lib/sensors.svelte.js'

const texts = {
  summary: 'All sensors in this area ({n})',
  summaryOne: 'The sensor in this area',
  caption: 'Sensors in {area} and their latest readings',
  colSensor: 'Sensor',
  colDistrict: 'District',
  colValue: '{metric}, {unit}',
  colBand: 'Air quality',
  colUpdated: 'Updated',
  open: 'Show sensor {id} on the map',
  filter: 'Filter sensors',
  filterPlaceholder: 'Sensor number',
  filterHint: 'Type a sensor number to narrow the list.',
  status: 'Showing {from} to {to} of {total} sensors',
  rangeOne: 'Showing 1 sensor',
  empty: 'No sensor matches this filter.',
  none: 'No live sensors in this area.',
  noValue: 'no reading',
  page: 'Page',
  pageOf: 'of',
  pageFirst: 'First page',
  pagePrev: 'Previous page',
  pageNext: 'Next page',
  pageLast: 'Last page',
  pagerLabel: 'Table pages',
  perPage: 'Rows per page',
  perPageAll: 'All',
  metrics: { P2: 'PM2.5', P1: 'PM10' },
  units: { P2: 'µg/m³', P1: 'µg/m³' },
}
const scales = [
  { metric: 'P2', bands: [{ label: 'Good', upper: 10 }, { label: 'Fair', upper: null }] },
  { metric: 'P1', bands: [{ label: 'Good', upper: 20 }, { label: 'Fair', upper: null }] },
]
const names = { Good: 'good', Fair: 'fair' }

const bodyOf = (n) => ({
  sensors: {
    id: Array.from({ length: n }, (_, i) => i + 1),
    P2: Array.from({ length: n }, (_, i) => i),
    P1: Array.from({ length: n }, () => 5),
    last_seen: Array.from({ length: n }, () => '2026-10-16T09:00:00Z'),
  },
})

let component
let host
afterEach(() => {
  if (component) unmount(component)
  component = null
  host?.remove()
  setSensors(null)
  setScales(null)
})

function render({ districts = {}, onopen = () => {} } = {}) {
  host = document.createElement('div')
  document.body.append(host)
  component = mount(AreaSensors, {
    target: host,
    props: { texts, names, area: 'Sofia', slug: 'sofia', districts, lang: 'en', onopen },
  })
  flushSync()
  return host
}

const rows = (el) => [...el.querySelectorAll('tbody tr')]
const status = (el) => el.querySelector('[role="status"]').textContent.trim()
function type(el, value) {
  const input = el.querySelector('input[type="search"]')
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
  flushSync()
}

describe('AreaSensors', () => {
  it('is a closed disclosure naming how many sensors it holds', () => {
    setScales(scales)
    setSensors(bodyOf(3), 'sofia')
    const el = render()
    const details = el.querySelector('details')
    expect(details.open).toBe(false)
    expect(details.querySelector('summary').textContent.trim()).toBe('All sensors in this area (3)')
  })

  it('is a real data table with a caption and column headers', () => {
    setScales(scales)
    setSensors(bodyOf(3), 'sofia')
    const el = render({ districts: { 2: 'Mladost' } })
    expect(el.querySelector('caption').textContent.trim()).toBe('Sensors in Sofia and their latest readings')
    const heads = [...el.querySelectorAll('thead th')].map((th) => [th.getAttribute('scope'), th.textContent.trim()])
    expect(heads).toEqual([
      ['col', 'Sensor'], ['col', 'District'], ['col', 'PM2.5, µg/m³'], ['col', 'PM10, µg/m³'], ['col', 'Air quality'], ['col', 'Updated'],
    ])
    const second = rows(el)[1]
    expect(second.querySelector('th[scope="row"]')).not.toBeNull()
    expect([...second.querySelectorAll('td')].map((td) => td.textContent.trim())).toEqual(['Mladost', '1', '5', 'good', expect.any(String)])
    expect(second.querySelector('time').getAttribute('datetime')).toBe('2026-10-16T09:00:00Z')
  })

  it('drops the district column outside Sofia', () => {
    setSensors(bodyOf(2), 'sofia')
    const el = render()
    expect([...el.querySelectorAll('thead th')].map((th) => th.textContent.trim())).not.toContain('District')
  })

  it('opens a sensor from a button named for what it does', () => {
    setSensors(bodyOf(3), 'sofia')
    const onopen = vi.fn()
    const el = render({ onopen })
    const button = rows(el)[2].querySelector('button')
    expect(button.textContent.trim()).toBe('3')
    expect(button.getAttribute('aria-label')).toBe('Show sensor 3 on the map')
    button.click()
    expect(onopen).toHaveBeenCalledWith(expect.objectContaining({ id: 3 }))
  })

  it('says a missing reading in words', () => {
    const body = bodyOf(1)
    body.sensors.P1 = [null]
    setSensors(body, 'sofia')
    const el = render()
    expect(rows(el)[0].querySelectorAll('td')[1].textContent.trim()).toBe('no reading')
  })

  it('filters by sensor number and announces the result', () => {
    setSensors(bodyOf(30), 'sofia')
    const el = render()
    expect(status(el)).toBe('Showing 1 to 25 of 30 sensors')
    type(el, '12')
    expect(rows(el)).toHaveLength(1)
    expect(status(el)).toBe('Showing 1 sensor')
    type(el, '999')
    expect(status(el)).toBe('No sensor matches this filter.')
    expect(rows(el)).toHaveLength(1)
    expect(rows(el)[0].textContent.trim()).toBe('No sensor matches this filter.')
  })

  it('pages with named buttons and announces the new range', () => {
    setSensors(bodyOf(30), 'sofia')
    const el = render()
    const nav = el.querySelector('nav')
    expect(nav.getAttribute('aria-label')).toBe('Table pages')
    nav.querySelector('button[aria-label="Next page"]').click()
    flushSync()
    expect(status(el)).toBe('Showing 26 to 30 of 30 sensors')
    expect(rows(el)).toHaveLength(5)
    const select = el.querySelector('select')
    select.value = 'all'
    select.dispatchEvent(new Event('change', { bubbles: true }))
    flushSync()
    expect(rows(el)).toHaveLength(30)
  })

  it('keeps each row button across a data refresh, so focus can return to it', () => {
    setSensors(bodyOf(3), 'sofia')
    const el = render()
    const before = rows(el)[1].querySelector('button')
    setSensors(bodyOf(3), 'sofia')
    flushSync()
    expect(rows(el)[1].querySelector('button')).toBe(before)
  })

  it('keeps a row button with its sensor when an earlier sensor drops out', () => {
    setSensors(bodyOf(3), 'sofia')
    const el = render()
    const second = rows(el)[1].querySelector('button')
    const b = bodyOf(3)
    setSensors({ sensors: { id: b.sensors.id.slice(1), P2: b.sensors.P2.slice(1), P1: b.sensors.P1.slice(1), last_seen: b.sensors.last_seen.slice(1) } }, 'sofia')
    flushSync()
    expect(rows(el)[0].querySelector('button')).toBe(second)
    expect(second.textContent).toBe('2')
  })

  // No "(0)" while this area loads, nor for a body from another area the map adopted.
  it('renders nothing until this area has loaded', () => {
    const el = render()
    expect(el.querySelector('details')).toBeNull()
    setSensors(bodyOf(3), 'elsewhere')
    flushSync()
    expect(el.querySelector('details')).toBeNull()
    setSensors(bodyOf(3), 'sofia')
    flushSync()
    expect(el.querySelector('summary').textContent.trim()).toBe('All sensors in this area (3)')
  })

  it('tells the reader when this area has no live sensors', () => {
    setSensors({ sensors: { id: [], P2: [], P1: [], last_seen: [] } }, 'sofia')
    const el = render()
    expect(status(el)).toBe('No live sensors in this area.')
    expect(rows(el)[0].textContent.trim()).toBe('No live sensors in this area.')
  })

  it('reports ready once this area has loaded', () => {
    const onready = vi.fn()
    host = document.createElement('div')
    document.body.append(host)
    component = mount(AreaSensors, { target: host, props: { texts, names, area: 'Sofia', slug: 'sofia', lang: 'en', onopen: () => {}, onready } })
    flushSync()
    expect(onready).not.toHaveBeenCalled()
    setSensors(bodyOf(2), 'sofia')
    flushSync()
    expect(onready).toHaveBeenCalled()
  })
})
