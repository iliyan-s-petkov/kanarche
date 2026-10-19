// @vitest-environment jsdom
import { describe, it, expect, afterEach, vi } from 'vitest'
import { flushSync } from 'svelte'
import { installAreaSensors } from '../mapareasensors.js'
import { setSensors } from '../sensors.svelte.js'
import { DEEP_LINK_ZOOM } from '../placement.js'

let stop
let el
afterEach(() => {
  stop?.()
  stop = null
  el?.remove()
  setSensors(null)
})

function host(attrs = {}) {
  el = document.createElement('div')
  el.hidden = true
  Object.assign(el.dataset, {
    area: 'Sofia', tSummary: 'All sensors in this area ({n})', tOpen: 'Show sensor {id} on the map',
    tColValue: '{metric}, {unit}', tP2: 'PM2.5', tUnitP2: 'µg/m³', tBandGood: 'good', ...attrs,
  })
  document.body.append(el)
  return el
}

function fakeMap(zoom = 10) {
  const container = document.createElement('div')
  container.scrollIntoView = vi.fn()
  return { getZoom: () => zoom, jumpTo: vi.fn(), getContainer: () => container }
}

const body = { sensors: { id: [7, 8], lon: [23.3, 23.4], lat: [42.6, 42.7], P2: [3, 4] } }

describe('installAreaSensors', () => {
  it('fills the server shell, shows it and reads its labels', () => {
    setSensors(body, 'sofia')
    stop = installAreaSensors({ map: fakeMap(), el: host({ districts: '{"8":"Mladost"}', tColDistrict: 'District' }), cfg: { slug: 'sofia', lang: 'en' }, onopen: () => {} })
    flushSync()
    expect(el.hidden).toBe(false)
    expect(el.querySelector('summary').textContent.trim()).toBe('All sensors in this area (2)')
    expect(el.querySelector('thead').textContent).toContain('PM2.5, µg/m³')
    expect(el.querySelector('tbody tr:nth-child(2)').textContent).toContain('Mladost')
  })

  it('brings the map to the sensor before opening it', () => {
    setSensors(body, 'sofia')
    const map = fakeMap(10)
    const onopen = vi.fn()
    stop = installAreaSensors({ map, el: host(), cfg: { slug: 'sofia', lang: 'en' }, onopen })
    flushSync()
    el.querySelector('tbody tr:nth-child(2) button').click()
    expect(map.jumpTo).toHaveBeenCalledWith({ center: [23.4, 42.7], zoom: DEEP_LINK_ZOOM })
    expect(map.getContainer().scrollIntoView).toHaveBeenCalled()
    expect(onopen).toHaveBeenCalledWith(8)
  })

  it('keeps a closer zoom the reader already chose', () => {
    setSensors(body, 'sofia')
    const map = fakeMap(DEEP_LINK_ZOOM + 3)
    stop = installAreaSensors({ map, el: host(), cfg: { slug: 'sofia', lang: 'en' }, onopen: () => {} })
    flushSync()
    el.querySelector('tbody button').click()
    expect(map.jumpTo.mock.calls[0][0].zoom).toBe(DEEP_LINK_ZOOM + 3)
  })

  it('keeps the shell hidden until this area has loaded', () => {
    stop = installAreaSensors({ map: fakeMap(), el: host(), cfg: { slug: 'sofia', lang: 'en' }, onopen: () => {} })
    flushSync()
    expect(el.hidden).toBe(true)
    setSensors(body, 'elsewhere')
    flushSync()
    expect(el.hidden).toBe(true)
    setSensors(body, 'sofia')
    flushSync()
    expect(el.hidden).toBe(false)
  })

  it('reads the empty-area and singular range labels', () => {
    setSensors({ sensors: { id: [] } }, 'sofia')
    stop = installAreaSensors({ map: fakeMap(), el: host({ tNone: 'No live sensors in this area.' }), cfg: { slug: 'sofia', lang: 'en' }, onopen: () => {} })
    flushSync()
    expect(el.querySelector('[role="status"]').textContent.trim()).toBe('No live sensors in this area.')
  })

  it('survives a malformed district attribute', () => {
    setSensors(body, 'sofia')
    stop = installAreaSensors({ map: fakeMap(), el: host({ districts: '{oops', tColDistrict: 'District' }), cfg: { slug: 'sofia', lang: 'en' }, onopen: () => {} })
    flushSync()
    expect(el.querySelector('thead').textContent).not.toContain('District')
  })
})
