import { describe, it, expect } from 'vitest'
import { viewedCategory, categoryChange, changeText, announceText, watchViewedCategory } from '../announce.js'
import { setSensors, setScales } from '../sensors.svelte.js'

const scales = [{ metric: 'P2', bands: [{ label: 'Good', upper: 10 }, { label: 'Fair', upper: 20 }, { label: 'Poor', upper: null }] }]
const names = { Good: 'good', Fair: 'fair', Poor: 'poor' }
const body = (p2) => ({ sensors: { id: [1, 2, 3], station: [1, 2, 3], P2: p2 } })
const base = { slug: 'sofia', area: 'sofia', sensorId: null, metric: 'P2', scales, names }

describe('viewedCategory', () => {
  it('rates the open sensor by its own reading', () => {
    expect(viewedCategory({ ...base, body: body([5, 15, 25]), sensorId: 2 })).toEqual({ kind: 'sensor', id: 2, metric: 'P2', band: 'fair' })
  })

  it('rates the area page by the median station when no sensor is open', () => {
    expect(viewedCategory({ ...base, body: body([5, 15, 25]) })).toEqual({ kind: 'area', id: 'sofia', metric: 'P2', band: 'fair' })
  })

  it('has no area category on the home map or for a body from another area', () => {
    expect(viewedCategory({ ...base, slug: '', body: body([5, 15, 25]) })).toBeNull()
    expect(viewedCategory({ ...base, area: 'plovdiv', body: body([5, 15, 25]) })).toBeNull()
  })

  it('has no category for a sensor with no reading', () => {
    expect(viewedCategory({ ...base, body: body([5, null, 25]), sensorId: 2 })).toBeNull()
  })
})

describe('categoryChange', () => {
  const a = (band, metric = 'P2') => ({ kind: 'area', id: 'sofia', metric, band })

  it('reports a band that moved for the same subject', () => {
    expect(categoryChange(a('good'), a('fair'))).toEqual({ kind: 'area', id: 'sofia', metric: 'P2', band: 'fair', was: 'good' })
  })

  it('is silent when the metric was switched during the refresh', () => {
    expect(categoryChange(a('good', 'P2'), a('fair', 'P1'))).toBeNull()
  })

  it('is silent when only the value moved', () => {
    expect(categoryChange(a('good'), a('good'))).toBeNull()
  })

  it('is silent when the reader looks at something else after the refresh', () => {
    expect(categoryChange(a('good'), { kind: 'sensor', id: 2, band: 'fair' })).toBeNull()
    expect(categoryChange({ kind: 'sensor', id: 1, band: 'good' }, { kind: 'sensor', id: 2, band: 'fair' })).toBeNull()
    expect(categoryChange(null, a('fair'))).toBeNull()
  })
})

describe('watchViewedCategory', () => {
  it('reports what the map shows from the registry and the view state', () => {
    let probe = null
    const fresh = { watch: (fn) => { probe = fn; return () => { probe = null } } }
    setSensors(body([5, 15, 25]), 'sofia')
    setScales(scales)
    const stop = watchViewedCategory({ fresh, vs: { sensorId: 3, metric: 'P2' }, cfg: { slug: 'sofia', bandNames: names } })
    expect(probe()).toEqual({ kind: 'sensor', id: 3, metric: 'P2', band: 'poor' })
    stop()
    expect(probe).toBeNull()
    setSensors(null)
  })
})

describe('announceText', () => {
  const t = { changed: '{subject}: air quality is now {band}, was {was}.', sensor: 'Sensor', subject: 'Sofia', failed: 'Could not refresh' }
  const now = { kind: 'area', id: 'sofia', metric: 'P2', band: 'fair' }
  const change = { ...now, was: 'good' }
  const fresh = (over) => ({ changed: change, busy: false, failed: false, current: () => now, ...over })

  it('says the change sentence after a refresh that moved the band', () => {
    expect(announceText(fresh(), t)).toBe('Sofia: air quality is now fair, was good.')
  })

  // The time of a value-only refresh is never spoken: the live region stays empty.
  it('says nothing for a value-only refresh or while one is in flight', () => {
    expect(announceText(fresh({ changed: null }), t)).toBe('')
    expect(announceText(fresh({ busy: true }), t)).toBe('')
    expect(announceText(fresh(), { ...t, changed: '' })).toBe('')
  })

  it('says the failure', () => {
    expect(announceText(fresh({ changed: null, failed: true }), t)).toBe('Could not refresh')
  })

  it('drops a change the reader has since moved away from', () => {
    expect(announceText(fresh({ current: () => null }), t)).toBe('')
    expect(announceText(fresh({ current: () => ({ ...now, kind: 'sensor', id: 7 }) }), t)).toBe('')
    expect(announceText(fresh({ current: () => ({ ...now, metric: 'P1' }) }), t)).toBe('')
    expect(announceText(fresh({ current: () => ({ ...now, band: 'poor' }) }), t)).toBe('')
  })
})

describe('changeText', () => {
  const t = { changed: '{subject}: air quality is now {band}, was {was}.', sensor: 'Sensor', subject: 'Sofia' }

  it('names the area', () => {
    expect(changeText(t, { kind: 'area', band: 'fair', was: 'good' })).toBe('Sofia: air quality is now fair, was good.')
  })

  it('names the sensor as its panel does', () => {
    expect(changeText(t, { kind: 'sensor', id: 101, band: 'poor', was: 'fair' })).toBe('Sensor 101: air quality is now poor, was fair.')
  })

  it('says nothing without a template or an area name', () => {
    expect(changeText({ ...t, changed: '' }, { kind: 'area', band: 'fair', was: 'good' })).toBe('')
    expect(changeText({ ...t, subject: '' }, { kind: 'area', band: 'fair', was: 'good' })).toBe('')
  })
})
