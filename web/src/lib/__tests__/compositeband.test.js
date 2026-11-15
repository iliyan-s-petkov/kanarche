import { describe, it, expect } from 'vitest'
import { compositeBand } from '../compositeband.js'

const bands = {
  P2: [
    { label: 'Good', upper: 5 },
    { label: 'Fair', upper: 15 },
    { label: 'Moderate', upper: null },
  ],
  P1: [
    { label: 'Good', upper: 15 },
    { label: 'Fair', upper: 45 },
    { label: 'Moderate', upper: null },
  ],
}
const names = { Good: 'good', Fair: 'fair', Moderate: 'moderate' }

// The EAQI rates a site by the worse of its particulate bands.
describe('compositeBand', () => {
  it('takes PM10 when it is worse', () => {
    expect(compositeBand({ P2: 4, P1: 60 }, bands, names)).toBe('moderate')
  })
  it('takes PM2.5 when it is worse', () => {
    expect(compositeBand({ P2: 30, P1: 10 }, bands, names)).toBe('moderate')
    expect(compositeBand({ P2: 10, P1: 10 }, bands, names)).toBe('fair')
  })
  it('uses the one metric that is present', () => {
    expect(compositeBand({ P2: 10 }, bands, names)).toBe('fair')
    expect(compositeBand({ P1: 50 }, bands, names)).toBe('moderate')
    expect(compositeBand({ P2: null, P1: 10 }, bands, names)).toBe('good')
  })
  it('is empty with no data', () => {
    expect(compositeBand({}, bands, names)).toBe('')
    expect(compositeBand(null, bands, names)).toBe('')
    expect(compositeBand({ P2: null, P1: null }, bands, names)).toBe('')
  })
})
