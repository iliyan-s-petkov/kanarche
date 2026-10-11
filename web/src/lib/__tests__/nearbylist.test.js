import { describe, it, expect } from 'vitest'
import { buildNearby, readingText, describeReading } from '../nearbylist.js'

const pt = (id, lon, lat, value) => ({
  id: id * 10,
  geometry: { type: 'Point', coordinates: [lon, lat] },
  properties: { sensorId: id, value },
})
const base = {
  centre: [23.5, 42.5],
  bounds: [23, 42, 24, 43],
  limit: 3,
  nameOf: (id) => `Sensor ${id}`,
  readingOf: (value) => `${value} µg/m³ · good`,
}

describe('buildNearby', () => {
  it('sorts by distance from the view centre', () => {
    const out = buildNearby([pt(1, 23.9, 42.9, 5), pt(2, 23.5, 42.51, 6), pt(3, 23.6, 42.6, 7)], base)
    expect(out.items.map((i) => i.id)).toEqual([2, 3, 1])
  })

  it('caps the list and counts the rest', () => {
    const feats = [1, 2, 3, 4, 5].map((n) => pt(n, 23.5 + n * 0.01, 42.5, n))
    const out = buildNearby(feats, base)
    expect(out.items.map((i) => i.id)).toEqual([1, 2, 3])
    expect(out.total).toBe(5)
    expect(out.rest).toBe(2)
  })

  it('carries name and reading text on each item', () => {
    const out = buildNearby([pt(7, 23.5, 42.5, 12)], base)
    expect(out.items[0]).toMatchObject({ id: 7, name: 'Sensor 7', reading: '12 µg/m³ · good' })
  })

  it('drops cells outside the view', () => {
    const out = buildNearby([pt(1, 25, 42.5, 5), pt(2, 23.5, 42.5, 5)], base)
    expect(out.items.map((i) => i.id)).toEqual([2])
  })

  it('drops cells that name no single sensor or have no reading', () => {
    const bin = { id: 1, geometry: { type: 'Point', coordinates: [23.5, 42.5] }, properties: { value: 9 } }
    const out = buildNearby([bin, pt(2, 23.5, 42.5, null), pt(3, 23.5, 42.5, 4)], base)
    expect(out.items.map((i) => i.id)).toEqual([3])
  })

  it('lists a station once when its devices share a position', () => {
    const out = buildNearby([pt(4, 23.5, 42.5, 5), pt(4, 23.5, 42.5, 5)], base)
    expect(out.total).toBe(1)
  })

  it('uses the centre of a polygon cell', () => {
    const ring = [[23.4, 42.4], [23.6, 42.4], [23.6, 42.6], [23.4, 42.6], [23.4, 42.4]]
    const cell = { id: 1, geometry: { type: 'Polygon', coordinates: [ring] }, properties: { sensorId: 9, value: 3 } }
    const out = buildNearby([cell], base)
    expect(out.items[0].lon).toBeCloseTo(23.5)
    expect(out.items[0].lat).toBeCloseTo(42.5)
  })

  it('returns the empty state for no features', () => {
    expect(buildNearby([], base)).toEqual({ items: [], total: 0, rest: 0 })
    expect(buildNearby(undefined, base)).toEqual({ items: [], total: 0, rest: 0 })
  })
})

describe('readingText', () => {
  it('joins value, unit and band name', () => {
    expect(readingText({ value: 12.34, unit: 'µg/m³', band: 'good', lang: 'en' })).toBe('12.3 µg/m³ · good')
  })

  it('uses the page language for the number', () => {
    expect(readingText({ value: 12.5, unit: 'µg/m³', band: 'добро', lang: 'bg' })).toBe('12,5 µg/m³ · добро')
  })

  it('leaves the band off for a metric with no scale', () => {
    expect(readingText({ value: 21, unit: '°C', band: '', lang: 'en' })).toBe('21 °C')
  })
})

describe('describeReading', () => {
  const read = {
    bands: [{ label: 'Good', upper: 5 }, { label: 'Fair', upper: null }],
    names: { Good: 'good', Fair: 'fair' },
    unit: 'µg/m³',
    lang: 'en',
  }

  it('joins the value to the name of its band', () => {
    expect(describeReading(3, read)).toBe('3 µg/m³ · good')
    expect(describeReading(9, read)).toBe('9 µg/m³ · fair')
  })

  it('gives the value alone when the metric has no bands', () => {
    expect(describeReading(9, { ...read, bands: [] })).toBe('9 µg/m³')
  })
})
