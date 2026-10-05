import { describe, it, expect, vi } from 'vitest'
import { POLLEN_HIDES, setPollen } from '../mappollen.js'
import { POLLEN_FILL_LAYER_ID, POLLEN_LINE_LAYER_ID } from '../pollen.js'

const cfg = { noDataColour: '#cccccc' }
const boundaries = {
  type: 'FeatureCollection',
  features: [{ type: 'Feature', geometry: { type: 'Polygon', coordinates: [] }, properties: { slug: 'sofia-oblast' } }],
}
const pollen = { date: '2026-10-06', areas: [{ slug: 'sofia-oblast', level: 'high', species: 'ragweed' }] }
const theme = { '--pollen-high': '#f1c21b' }
const read = (n) => theme[n] ?? ''

function fakeMap() {
  const data = []
  const vis = {}
  const paint = {}
  return {
    data, vis, paint,
    getLayer: () => true,
    getSource: () => ({ setData: (d) => data.push(d) }),
    setLayoutProperty: (id, k, v) => { vis[id] = v },
    setPaintProperty: (id, k, v) => { paint[k] = v },
  }
}
const fakeChrome = () => ({ showPollen: vi.fn() })
const fetcher = () => vi.fn(async (url) => (url === '/api/v1/boundaries' ? boundaries : pollen))

describe('setPollen', () => {
  it('fetches once, colours the provinces, hides the grid and shows the key', async () => {
    const map = fakeMap()
    const chrome = fakeChrome()
    const st = {}
    const fetchJSON = fetcher()
    expect(await setPollen(map, cfg, chrome, st, true, fetchJSON, read)).toBe(true)
    expect(fetchJSON).toHaveBeenCalledWith('/api/v1/pollen')
    expect(map.data[0].features[0].properties).toEqual({ slug: 'sofia-oblast', level: 'high' })
    expect(map.paint['fill-color']).toContain('#f1c21b')
    expect(map.vis[POLLEN_FILL_LAYER_ID]).toBe('visible')
    expect(map.vis[POLLEN_LINE_LAYER_ID]).toBe('visible')
    for (const id of POLLEN_HIDES) expect(map.vis[id]).toBe('none')
    expect(chrome.showPollen).toHaveBeenLastCalledWith(true)
    await setPollen(map, cfg, chrome, st, false, fetchJSON, read)
    await setPollen(map, cfg, chrome, st, true, fetchJSON, read)
    expect(fetchJSON).toHaveBeenCalledTimes(2)
  })

  it('restores the grid and hides the key when switched off', async () => {
    const map = fakeMap()
    const chrome = fakeChrome()
    const st = {}
    await setPollen(map, cfg, chrome, st, true, fetcher(), read)
    expect(await setPollen(map, cfg, chrome, st, false)).toBe(false)
    expect(map.vis[POLLEN_FILL_LAYER_ID]).toBe('none')
    for (const id of POLLEN_HIDES) expect(map.vis[id]).toBe('visible')
    expect(chrome.showPollen).toHaveBeenLastCalledWith(false)
    expect(st.on).toBe(false)
  })

  // No forecast (503) unticks the box rather than leaving the grid hidden under nothing.
  it('reports off and keeps the grid when the fetch fails', async () => {
    const map = fakeMap()
    const chrome = fakeChrome()
    const st = {}
    expect(await setPollen(map, cfg, chrome, st, true, async () => { throw new Error('503') }, read)).toBe(false)
    expect(map.vis[POLLEN_FILL_LAYER_ID]).toBe('none')
    for (const id of POLLEN_HIDES) expect(map.vis[id]).toBe('visible')
    expect(st.on).toBeFalsy()
  })
})
