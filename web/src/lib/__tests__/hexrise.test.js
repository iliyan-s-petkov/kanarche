import { describe, it, expect, vi } from 'vitest'
import {
  columnHeight, riseFactor, hexExtrusionPaint, installHexRise,
  HEX_RISE_FULL_PITCH, HEX_HEIGHT_PER_CELL, HEX_EXTRUDED_OPACITY,
} from '../hexrise.js'
import { hexFeatures } from '../hexes.js'
import { rampColour } from '../ramp.js'
import { HEX_SOURCE_ID, HEX_COLUMN_SOURCE_ID, HEX_EXTRUSION_LAYER_ID, PAINT_EVENT } from '../mapids.js'

const BANDS = [
  { upper: 10, colour: '#00ff00' },
  { upper: 25, colour: '#ffff00' },
  { upper: 50, colour: '#ff0000', ceiling: 100 },
]

describe('columnHeight', () => {
  it('is zero without a reading or a scale', () => {
    expect(columnHeight(null, BANDS, 1)).toBe(0)
    expect(columnHeight(undefined, BANDS, 1)).toBe(0)
    expect(columnHeight(20, [], 1)).toBe(0)
  })

  it('grows with the value on the colour axis', () => {
    const lo = columnHeight(5, BANDS, 1)
    const mid = columnHeight(20, BANDS, 1)
    const hi = columnHeight(60, BANDS, 1)
    expect(lo).toBeGreaterThan(0)
    expect(mid).toBeGreaterThan(lo)
    expect(hi).toBeGreaterThan(mid)
  })

  it('caps at the scale top, so a spike is no taller than the ceiling', () => {
    const cap = HEX_HEIGHT_PER_CELL * 1000
    expect(columnHeight(100, BANDS, 1)).toBeCloseTo(cap)
    expect(columnHeight(5000, BANDS, 1)).toBeCloseTo(cap)
  })

  it('scales with the cell size', () => {
    expect(columnHeight(100, BANDS, 25)).toBeCloseTo(HEX_HEIGHT_PER_CELL * 25000)
    expect(columnHeight(20, BANDS, 0.5) * 2).toBeCloseTo(columnHeight(20, BANDS, 1))
  })
})

describe('riseFactor', () => {
  it('is flat at pitch 0 and full from the ramp end', () => {
    expect(riseFactor(0)).toBe(0)
    expect(riseFactor(HEX_RISE_FULL_PITCH)).toBe(1)
    expect(riseFactor(60)).toBe(1)
  })

  it('ramps monotonically in between', () => {
    const steps = [2, 8, 15, 22, 30].map((p) => riseFactor(p))
    for (const f of steps) {
      expect(f).toBeGreaterThan(0)
      expect(f).toBeLessThan(1)
    }
    for (let i = 1; i < steps.length; i++) expect(steps[i]).toBeGreaterThan(steps[i - 1])
  })

  it('ends by about 30-40 degrees', () => {
    expect(HEX_RISE_FULL_PITCH).toBeGreaterThanOrEqual(30)
    expect(HEX_RISE_FULL_PITCH).toBeLessThanOrEqual(40)
  })

  it('steps with no ramp under reduced motion', () => {
    expect(riseFactor(0, true)).toBe(0)
    expect(riseFactor(5, true)).toBe(0)
    expect(riseFactor(HEX_RISE_FULL_PITCH / 2, true)).toBe(1)
    expect(riseFactor(25, true)).toBe(1)
  })
})

describe('hexFeatures height', () => {
  it('carries a capped column height per cell', () => {
    const body = { resolution_km: 2, hexes: [
      { lon: 23.3, lat: 42.7, n: 1, values: { pm25: 5 } },
      { lon: 23.4, lat: 42.7, n: 1, values: { pm25: 900 } },
      { lon: 23.5, lat: 42.7, n: 1, values: {} },
    ] }
    const f = hexFeatures(body, 'pm25', BANDS, '#999', rampColour)
    const by = (v) => f.find((x) => x.properties.value === v).properties.height
    expect(by(null)).toBe(0)
    expect(by(5)).toBeCloseTo(columnHeight(5, BANDS, 2))
    expect(by(900)).toBeCloseTo(HEX_HEIGHT_PER_CELL * 2000)
  })
})

describe('paint', () => {
  it('extrudes height times the rise state, hidden by default', () => {
    const p = hexExtrusionPaint()
    expect(p['fill-extrusion-color']).toEqual(['get', 'colour'])
    expect(JSON.stringify(p['fill-extrusion-height'])).toContain('"rise"')
    expect(JSON.stringify(p['fill-extrusion-height'])).toContain('"height"')
    expect(p['fill-extrusion-opacity']).toBe(0)
  })

})

function fakeMap(pitch = 0) {
  const handlers = {}
  const container = new EventTarget()
  return {
    pitch,
    getPitch() { return this.pitch },
    on: (type, fn) => { (handlers[type] ??= []).push(fn) },
    fire(type) { for (const fn of handlers[type] ?? []) fn() },
    getContainer: () => container,
    getLayer: () => ({}),
    setFeatureState: vi.fn(),
    setPaintProperty: vi.fn(),
    columns: { setData: vi.fn() },
    getSource(id) { return id === HEX_COLUMN_SOURCE_ID ? this.columns : undefined },
  }
}

const paint = (map, ids) => map.getContainer().dispatchEvent(new CustomEvent(PAINT_EVENT, {
  detail: { source: HEX_SOURCE_ID, features: ids.map((id) => ({ id, geometry: { type: 'Polygon' } })) },
}))

describe('installHexRise', () => {
  const sync = (fn) => fn()

  it('follows the pitch onto every painted cell', () => {
    const map = fakeMap()
    installHexRise(map, { raf: sync, reducedMotion: () => false })
    paint(map, [1, 2])
    expect(map.setFeatureState).not.toHaveBeenCalled()

    map.pitch = 50
    map.fire('pitch')
    expect(map.setFeatureState).toHaveBeenCalledWith({ source: HEX_COLUMN_SOURCE_ID, id: 1 }, { rise: 1 })
    expect(map.setFeatureState).toHaveBeenCalledWith({ source: HEX_COLUMN_SOURCE_ID, id: 2 }, { rise: 1 })
    expect(map.setPaintProperty).toHaveBeenLastCalledWith(HEX_EXTRUSION_LAYER_ID, 'fill-extrusion-opacity', HEX_EXTRUDED_OPACITY)
    expect(map.columns.setData.mock.lastCall[0].features.map((f) => f.id)).toEqual([1, 2])

    map.setFeatureState.mockClear()
    map.pitch = 0
    map.fire('pitch')
    expect(map.setFeatureState).toHaveBeenCalledWith({ source: HEX_COLUMN_SOURCE_ID, id: 1 }, { rise: 0 })
    expect(map.setPaintProperty).toHaveBeenLastCalledWith(HEX_EXTRUSION_LAYER_ID, 'fill-extrusion-opacity', 0)
    expect(map.columns.setData.mock.lastCall[0].features).toEqual([])
  })

  it('keeps the column source empty while flat', () => {
    const map = fakeMap()
    installHexRise(map, { raf: (fn) => fn(), reducedMotion: () => false })
    paint(map, [1, 2])
    expect(map.columns.setData).not.toHaveBeenCalled()
  })

  it('gives newly painted cells the current rise', () => {
    const map = fakeMap(50)
    installHexRise(map, { raf: sync, reducedMotion: () => false })
    paint(map, [7])
    expect(map.setFeatureState).toHaveBeenCalledWith({ source: HEX_COLUMN_SOURCE_ID, id: 7 }, { rise: 1 })
  })

  it('coalesces pitch events into one frame and skips unchanged factors', () => {
    const map = fakeMap()
    const frames = []
    installHexRise(map, { raf: (fn) => frames.push(fn), reducedMotion: () => false })
    paint(map, [1])
    map.pitch = 50
    map.fire('pitch'); map.fire('pitch'); map.fire('pitch')
    expect(frames).toHaveLength(1)
    frames.shift()()
    expect(map.setFeatureState).toHaveBeenCalledTimes(1)
    map.fire('pitch')
    frames.shift()()
    expect(map.setFeatureState).toHaveBeenCalledTimes(1)
  })

  it('steps rather than ramps under reduced motion', () => {
    const map = fakeMap()
    installHexRise(map, { raf: sync, reducedMotion: () => true })
    paint(map, [1])
    map.pitch = 10
    map.fire('pitch')
    expect(map.setFeatureState).not.toHaveBeenCalled()
    map.pitch = 20
    map.fire('pitch')
    expect(map.setFeatureState).toHaveBeenCalledWith({ source: HEX_COLUMN_SOURCE_ID, id: 1 }, { rise: 1 })
  })
})
