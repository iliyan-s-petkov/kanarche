import { beforeEach, describe, expect, it } from 'vitest'
import { flagsAt, isFaultyAt, metricColumnsOf } from '../stations.js'
import { sensorFeatures } from '../mapfeatures.js'
import { countSensors, sensorCountLine } from '../sensorcount.js'
import {
  filterFaulty, filterSensorFeatures, getShowFaulty, onShowFaultyChange, resetFaultyFilterForTests, setShowFaulty,
} from '../faultyfilter.svelte.js'

beforeEach(() => resetFaultyFilterForTests())

// Station A: PM box 1 (healthy) + climate box 2 (temperature dead) at one address.
// Station B: one PM box whose P2 is flagged. Station C: healthy.
const body = {
  sensors: {
    id: [1, 2, 3, 4],
    station: [1, 1, 3, 4],
    lon: [23.1, 23.1, 23.2, 23.3],
    lat: [42.1, 42.1, 42.2, 42.3],
    quality: ['ok', 'out_of_range', 'stuck', 'ok'],
    flags: [{}, { temperature: 'out_of_range' }, { P2: 'stuck' }, {}],
    P2: [5, null, null, 7],
    temperature: [null, null, null, 20],
  },
}
const scales = []

describe('flags column', () => {
  it('is not mistaken for a metric column', () => {
    expect(metricColumnsOf(body)).toEqual(['P2', 'temperature'])
  })

  it('merges the members of one station', () => {
    expect(flagsAt(body, [0, 1])).toEqual({ temperature: 'out_of_range' })
  })

  it('is empty for a body served before the column existed', () => {
    expect(flagsAt({ sensors: { id: [1] } }, [0])).toEqual({})
  })
})

describe('isFaultyAt', () => {
  it('is per metric: a dead climate chip does not condemn the PM layer', () => {
    expect(isFaultyAt(body, [0, 1], 'P2')).toBe(false)
    expect(isFaultyAt(body, [0, 1], 'temperature')).toBe(true)
  })

  it('needs the metric flagged AND without a usable reading', () => {
    expect(isFaultyAt(body, [2], 'P2')).toBe(true)
    expect(isFaultyAt(body, [2], 'temperature')).toBe(false)
    // A usable value from any member rescues the station.
    const rescued = { sensors: { ...body.sensors, P2: [5, 6, null, 7], flags: [{}, { P2: 'stuck' }, {}, {}] } }
    expect(isFaultyAt(rescued, [0, 1], 'P2')).toBe(false)
  })

  it('never calls a usable flag faulty', () => {
    const b = { sensors: { id: [1], flags: [{ P2: 'no_neighbours' }], P2: [null] } }
    expect(isFaultyAt(b, [0], 'P2')).toBe(false)
  })

  it('treats clamped as faulty', () => {
    const b = { sensors: { id: [1], flags: [{ P2: 'clamped' }], P2: [null] } }
    expect(isFaultyAt(b, [0], 'P2')).toBe(true)
  })
})

// Station D: PM box 5 the server marks faulty for P2 + climate box 6.
// Station E: two PM boxes, only box 7 faulty for P2.
const served = {
  sensors: {
    id: [5, 6, 7, 8],
    station: [5, 5, 7, 7],
    lon: [23.4, 23.4, 23.5, 23.5],
    lat: [42.4, 42.4, 42.5, 42.5],
    quality: ['ok', 'ok', 'ok', 'ok'],
    flags: [{}, {}, {}, {}],
    faulty: [['P2'], [], ['P2'], []],
    measures: [['P2'], ['temperature'], ['P2'], ['P2']],
    P2: [900, null, 800, 9],
    temperature: [null, 20, null, null],
  },
}

describe('isFaultyAt with the server faulty column', () => {
  it('is not mistaken for a metric column', () => {
    expect(metricColumnsOf(served)).toEqual(['P2', 'temperature'])
  })

  it('is faulty when every member measuring the metric is, even with a value', () => {
    expect(isFaultyAt(served, [0, 1], 'P2')).toBe(true)
  })

  it('stays per metric', () => {
    expect(isFaultyAt(served, [0, 1], 'temperature')).toBe(false)
  })

  it('is not faulty while one member measuring the metric is healthy', () => {
    expect(isFaultyAt(served, [2, 3], 'P2')).toBe(false)
  })

  it('is not faulty for a metric no member measures', () => {
    expect(isFaultyAt(served, [1], 'P2')).toBe(false)
  })

  it('counts a server-faulty station with a value as faulty, not active', () => {
    expect(countSensors(served, 'P2', { showFaulty: true })).toEqual({ total: 2, active: 1, silent: 1, faulty: 1 })
    expect(countSensors(served, 'P2', { showFaulty: false })).toEqual({ total: 1, active: 1, silent: 0, faulty: 0 })
  })

  it('marks the map feature', () => {
    expect(sensorFeatures(served, 'P2', scales, '#999').map((f) => f.properties.faulty)).toEqual([true, false])
  })
})

describe('sensorFeatures', () => {
  it('marks faulty stations for the layer being drawn', () => {
    const p2 = sensorFeatures(body, 'P2', scales, '#999').map((f) => f.properties.faulty)
    expect(p2).toEqual([false, true, false])
    const t = sensorFeatures(body, 'temperature', scales, '#999').map((f) => f.properties.faulty)
    expect(t).toEqual([true, false, false])
  })
})

describe('the faulty filter', () => {
  it('draws faulty stations even under the with-data status, once asked for', () => {
    const f = [
      { properties: { faulty: false, value: 5 } },
      { properties: { faulty: false, value: null } },
      { properties: { faulty: true, value: null } },
    ]
    expect(filterSensorFeatures(f, 'active', false)).toHaveLength(1)
    expect(filterSensorFeatures(f, 'active', true)).toHaveLength(2)
    expect(filterSensorFeatures(f, 'inactive', true)).toHaveLength(2)
    expect(filterSensorFeatures(f, 'inactive', false)).toHaveLength(1)
  })

  const features = [{ properties: { faulty: true } }, { properties: { faulty: false } }, { properties: {} }]

  it('hides faulty stations by default', () => {
    expect(getShowFaulty()).toBe(false)
    expect(filterFaulty(features, getShowFaulty())).toHaveLength(2)
  })

  it('brings them back when switched on, and tells subscribers', () => {
    let seen = null
    onShowFaultyChange((v) => { seen = v })
    setShowFaulty(true)
    expect(seen).toBe(true)
    expect(filterFaulty(features, getShowFaulty())).toHaveLength(3)
  })
})

describe('countSensors', () => {
  it('excludes faulty stations while they are hidden', () => {
    expect(countSensors(body, 'P2', { showFaulty: false })).toEqual({ total: 2, active: 2, silent: 0, faulty: 0 })
  })

  it('counts them as silent while shown', () => {
    expect(countSensors(body, 'P2', { showFaulty: true })).toEqual({ total: 3, active: 2, silent: 1, faulty: 1 })
  })

  it('keeps the old answer when no option is passed', () => {
    expect(countSensors(body, 'P2')).toEqual({ total: 3, active: 2, silent: 1, faulty: 1 })
  })

  it('adds the drawn faulty stations to the with-data line', () => {
    const texts = { shown: 'Showing', of: 'of', sensors: 'sensors', silent: 'silent' }
    const counts = countSensors(body, 'P2', { showFaulty: true })
    expect(sensorCountLine(texts, counts, 'active')).toBe('Showing 3 of 3 sensors — 1 silent')
  })
})
