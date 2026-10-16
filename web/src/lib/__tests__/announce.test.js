import { describe, it, expect } from 'vitest'
import { viewedCategory, categoryChange, changeText, refreshStatus, watchViewedCategory } from '../announce.js'
import { setSensors, setScales } from '../sensors.svelte.js'

const scales = [{ metric: 'P2', bands: [{ label: 'Good', upper: 10 }, { label: 'Fair', upper: 20 }, { label: 'Poor', upper: null }] }]
const names = { Good: 'good', Fair: 'fair', Poor: 'poor' }
const body = (p2) => ({ sensors: { id: [1, 2, 3], station: [1, 2, 3], P2: p2 } })
const base = { slug: 'sofia', area: 'sofia', sensorId: null, metric: 'P2', scales, names }

describe('viewedCategory', () => {
  it('rates the open sensor by its own reading', () => {
    expect(viewedCategory({ ...base, body: body([5, 15, 25]), sensorId: 2 })).toEqual({ kind: 'sensor', id: 2, band: 'fair' })
  })

  it('rates the area page by the median station when no sensor is open', () => {
    expect(viewedCategory({ ...base, body: body([5, 15, 25]) })).toEqual({ kind: 'area', id: 'sofia', band: 'fair' })
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
  const a = (band) => ({ kind: 'area', id: 'sofia', band })

  it('reports a band that moved for the same subject', () => {
    expect(categoryChange(a('good'), a('fair'))).toEqual({ kind: 'area', id: 'sofia', band: 'fair', was: 'good' })
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
    expect(probe()).toEqual({ kind: 'sensor', id: 3, band: 'poor' })
    stop()
    expect(probe).toBeNull()
    setSensors(null)
  })
})

describe('refreshStatus', () => {
  const t = { changed: '{subject}: air quality is now {band}, was {was}.', sensor: 'Sensor', subject: 'Sofia' }
  const change = { kind: 'area', band: 'fair', was: 'good' }

  it('replaces the plain line after a refresh that moved the band', () => {
    expect(refreshStatus({ changed: change }, t, 'Data from 10:31')).toBe('Sofia: air quality is now fair, was good.')
  })

  it('keeps the plain line otherwise', () => {
    expect(refreshStatus({ changed: null }, t, 'Data from 10:31')).toBe('Data from 10:31')
    expect(refreshStatus({ changed: change, busy: true }, t, 'Loading')).toBe('Loading')
    expect(refreshStatus({ changed: change }, { ...t, changed: '' }, 'Data from 10:31')).toBe('Data from 10:31')
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
