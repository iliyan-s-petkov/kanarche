import { describe, it, expect } from 'vitest'
import { offsetFrom } from '../geometry.js'

const C = [23.32, 42.69]

describe('offsetFrom', () => {
  it('is zero metres at the centre itself', () => {
    expect(offsetFrom(C, C).metres).toBe(0)
  })

  it('measures about 111 km per degree of latitude', () => {
    const { metres } = offsetFrom(C, [C[0], C[1] + 1])
    expect(metres).toBeGreaterThan(110000)
    expect(metres).toBeLessThan(112000)
  })

  it('shrinks a degree of longitude with latitude', () => {
    const { metres } = offsetFrom(C, [C[0] + 1, C[1]])
    expect(metres).toBeGreaterThan(80000)
    expect(metres).toBeLessThan(83000)
  })

  it('names the eight compass octants clockwise from north', () => {
    const d = 0.01
    const at = (dx, dy) => offsetFrom(C, [C[0] + dx, C[1] + dy]).octant
    expect([at(0, d), at(d, d), at(d, 0), at(d, -d), at(0, -d), at(-d, -d), at(-d, 0), at(-d, d)])
      .toEqual([0, 1, 2, 3, 4, 5, 6, 7])
  })
})
