import { describe, it, expect } from 'vitest'
import { bandName, readBandNames } from '../bandname.js'

const bands = [
  { label: 'Good', upper: 5, colour: '#50f0e6' },
  { label: 'Fair', upper: 15, colour: '#50ccaa' },
  { label: 'Moderate', upper: 50, colour: '#f0e641' },
  { label: 'Extremely poor', upper: null, colour: '#7d2181' },
]
const names = { Good: 'good', Fair: 'fair', Moderate: 'moderate', 'Extremely poor': 'extremely poor' }

describe('bandName', () => {
  it('names the band a value falls in', () => {
    expect(bandName(3, bands, names)).toBe('good')
    expect(bandName(20, bands, names)).toBe('moderate')
  })

  // The server treats Upper as inclusive (colour.go bandLabel).
  it('puts a value exactly on a boundary in the lower band', () => {
    expect(bandName(5, bands, names)).toBe('good')
    expect(bandName(15, bands, names)).toBe('fair')
  })

  it('names the open top band', () => {
    expect(bandName(900, bands, names)).toBe('extremely poor')
  })

  it('returns an empty string where there is nothing honest to say', () => {
    expect(bandName(null, bands, names)).toBe('')
    expect(bandName(Number.NaN, bands, names)).toBe('')
    expect(bandName(3, [], names)).toBe('')
    expect(bandName(3, bands, {})).toBe('')
  })
})

describe('readBandNames', () => {
  it('reads the six data-t-band-* attributes keyed by the scale label', () => {
    const d = {
      tBandGood: 'good', tBandFair: 'fair', tBandModerate: 'moderate',
      tBandPoor: 'poor', tBandVeryPoor: 'very poor', tBandExtremelyPoor: 'extremely poor',
    }
    expect(readBandNames(d)).toEqual({
      Good: 'good', Fair: 'fair', Moderate: 'moderate', Poor: 'poor',
      'Very poor': 'very poor', 'Extremely poor': 'extremely poor',
    })
  })

  it('leaves a missing attribute empty', () => {
    expect(readBandNames({}).Good).toBe('')
  })
})
