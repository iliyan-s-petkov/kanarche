import { describe, it, expect } from 'vitest'
import { matchAreas, areaGroup } from '../find.js'

const E = [
  { name: 'Varna', area: { kind: 'city' } },
  { name: 'Lozenets', area: { kind: 'neighbourhood' } },
  { name: 'Sofia', area: { kind: 'city' } },
  { name: 'Vitosha', area: { kind: 'neighbourhood' } },
  { name: 'Burgas', area: { kind: 'oblast' } },
]

describe('grouped areas', () => {
  it('puts districts first, then the rest, each A-Z', () => {
    expect(matchAreas(E, '', 'en').map((m) => m.name))
      .toEqual(['Lozenets', 'Vitosha', 'Burgas', 'Sofia', 'Varna'])
  })

  it('keeps the grouping while a query filters', () => {
    expect(matchAreas(E, 'v', 'en').map((m) => m.name))
      .toEqual(['Vitosha', 'Varna'])
  })

  it('names the group of each match', () => {
    expect(matchAreas(E, '', 'en').map(areaGroup))
      .toEqual(['district', 'district', 'place', 'place', 'place'])
  })

  it('treats entries with no kind as places', () => {
    expect(areaGroup({ name: 'Варна', href: '/a' })).toBe('place')
  })
})
