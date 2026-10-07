import { describe, it, expect, vi } from 'vitest'
import { setSea, seaLayout, seaPaint } from '../mapsea.js'
import { SEA_IMAGE_ID, SEA_LAYER_ID, seaColours } from '../sea.js'

const cfg = {
  markerStrokeColour: '#ffffff',
  seaColours: seaColours(['#0b4f9c', '#3a8fd9', '#8cc5e8', '#8e3a9c', '#9ca3af']),
}
const body = { imported_at: null, limits: {}, sites: [{ id: 'BG1', lat: 43, lon: 28, zone: 'coastal', season: 2024, quality: 'excellent' }] }

function fakeMap() {
  const data = []
  const vis = {}
  return {
    data, vis,
    getLayer: () => true,
    getSource: () => ({ setData: (d) => data.push(d) }),
    setLayoutProperty: (id, k, v) => { vis[id] = v },
  }
}
const fakeChrome = () => ({ showSea: vi.fn(), setSeaSupplement: vi.fn() })

describe('setSea supplement', () => {
  it('hands the supplement url to the legend, and clears it when the body has none', async () => {
    const chrome = fakeChrome()
    const sup = { edition: '2025 v1.0', published: '2026-09-30', url: 'https://example.test/d' }
    await setSea(fakeMap(), cfg, chrome, {}, true, async () => ({ ...body, supplement: sup }))
    expect(chrome.setSeaSupplement).toHaveBeenLastCalledWith('https://example.test/d')
    await setSea(fakeMap(), cfg, chrome, {}, true, async () => body)
    expect(chrome.setSeaSupplement).toHaveBeenLastCalledWith('')
  })
})

describe('setSea', () => {
  it('fetches once, paints the sites and shows the key', async () => {
    const map = fakeMap()
    const chrome = fakeChrome()
    const st = {}
    const fetchJSON = vi.fn(async () => body)
    expect(await setSea(map, cfg, chrome, st, true, fetchJSON)).toBe(true)
    expect(fetchJSON).toHaveBeenCalledWith('/api/v1/sea/sites')
    expect(map.data[0].features[0].properties.colour).toBe('#0b4f9c')
    expect(map.vis[SEA_LAYER_ID]).toBe('visible')
    expect(chrome.showSea).toHaveBeenLastCalledWith(true)
    await setSea(map, cfg, chrome, st, false, fetchJSON)
    await setSea(map, cfg, chrome, st, true, fetchJSON)
    expect(fetchJSON).toHaveBeenCalledTimes(1)
  })

  it('hides the layer, the key and the open panel when switched off', async () => {
    const map = fakeMap()
    const chrome = fakeChrome()
    const closePanel = vi.fn()
    const st = { closePanel }
    await setSea(map, cfg, chrome, st, true, async () => body)
    expect(await setSea(map, cfg, chrome, st, false)).toBe(false)
    expect(map.vis[SEA_LAYER_ID]).toBe('none')
    expect(chrome.showSea).toHaveBeenLastCalledWith(false)
    expect(closePanel).toHaveBeenCalled()
    expect(st.on).toBe(false)
  })

  // A failed fetch unticks the box rather than leaving it ticked over nothing.
  it('reports off when the fetch fails', async () => {
    const map = fakeMap()
    const chrome = fakeChrome()
    const st = {}
    expect(await setSea(map, cfg, chrome, st, true, async () => { throw new Error('503') })).toBe(false)
    expect(map.vis[SEA_LAYER_ID]).toBe('none')
    expect(st.on).toBeFalsy()
  })
})

describe('seaLayout and seaPaint', () => {
  it('draws the square marker in the class colour with the marker halo', () => {
    expect(seaLayout()['icon-image']).toBe(SEA_IMAGE_ID)
    expect(seaLayout()['icon-allow-overlap']).toBe(true)
    expect(seaPaint(cfg)).toEqual({
      'icon-color': ['get', 'colour'],
      'icon-halo-color': '#ffffff',
      'icon-halo-width': 1,
    })
  })
})
