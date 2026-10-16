import { describe, it, expect } from 'vitest'
import { sensorRows, filterSensorRows, pageView, statusText, summaryText, PER_PAGE_OPTIONS, DEFAULT_PER_PAGE } from '../areasensors.js'

const bands = [
  { label: 'Good', upper: 10 },
  { label: 'Fair', upper: 20 },
  { label: 'Moderate', upper: null },
]
const names = { Good: 'good', Fair: 'fair', Moderate: 'moderate' }

// Station 11 is two boxes (11 particulate, 12 climate); 5 and 30 stand alone.
const body = {
  sensors: {
    id: [30, 11, 12, 5],
    station: [30, 11, 11, 5],
    P2: [4, 15, null, null],
    P1: [25, 8, null, 3],
    last_seen: ['2026-10-16T10:00:00Z', '2026-10-16T09:00:00Z', '2026-10-16T09:30:00Z', '2026-10-16T08:00:00Z'],
  },
}
const opts = { districts: { 12: 'Mladost', 30: 'Lozenets' }, bands: { P2: bands, P1: bands }, names }

describe('sensorRows', () => {
  it('lists each station once, by number, with its readings', () => {
    const rows = sensorRows(body, opts)
    expect(rows.map((r) => r.id)).toEqual([5, 11, 30])
    expect(rows[1]).toMatchObject({ id: 11, members: [11, 12], P2: 15, P1: 8 })
    expect(rows[0]).toMatchObject({ P2: null, P1: 3 })
  })

  it('names the district through any member of the station', () => {
    const rows = sensorRows(body, opts)
    expect(rows.map((r) => r.district)).toEqual(['', 'Mladost', 'Lozenets'])
  })

  it('takes the newest reading time across the station', () => {
    expect(sensorRows(body, opts)[1].updated).toBe('2026-10-16T09:30:00Z')
  })

  // EAQI rule: the overall band is the worst of the pollutants.
  it('rates a station by its worse particulate band', () => {
    expect(sensorRows(body, opts).map((r) => r.band)).toEqual(['good', 'fair', 'moderate'])
  })

  it('is empty before the data arrives', () => {
    expect(sensorRows(null, opts)).toEqual([])
  })
})

describe('filterSensorRows', () => {
  const rows = sensorRows(body, opts)
  it('matches a typed number against every box at the station', () => {
    expect(filterSensorRows(rows, '12').map((r) => r.id)).toEqual([11])
    expect(filterSensorRows(rows, ' 1 ').map((r) => r.id)).toEqual([11])
  })
  it('keeps everything for an empty query', () => {
    expect(filterSensorRows(rows, '')).toHaveLength(3)
  })
})

describe('pageView', () => {
  const many = Array.from({ length: 60 }, (_, i) => ({ id: i + 1 }))
  it('windows the rows and says which ranks are on screen', () => {
    expect(pageView(many, 25, 3)).toMatchObject({ page: 3, pages: 3, from: 51, to: 60, total: 60 })
    expect(pageView(many, 25, 3).rows).toHaveLength(10)
  })
  it('clamps a page past the end, as a filter shrinks the set', () => {
    expect(pageView(many.slice(0, 5), 25, 3)).toMatchObject({ page: 1, from: 1, to: 5 })
  })
  it('shows all rows on one page for "all"', () => {
    expect(pageView(many, 'all', 1)).toMatchObject({ pages: 1, from: 1, to: 60 })
  })
  it('reports zero ranks for an empty result', () => {
    expect(pageView([], 25, 1)).toMatchObject({ from: 0, to: 0, total: 0 })
  })
  it('defaults to 25 a page', () => {
    expect(DEFAULT_PER_PAGE).toBe(25)
    expect(PER_PAGE_OPTIONS).toContain(25)
  })
})

describe('texts', () => {
  const t = {
    status: 'Showing {from} to {to} of {total} sensors', rangeOne: 'Showing 1 sensor', empty: 'No sensor matches this filter.',
    none: 'No live sensors in this area.', summary: 'All sensors in this area ({n})', summaryOne: 'The sensor in this area',
  }
  it('fills the status from the view', () => {
    expect(statusText(t, { from: 1, to: 25, total: 264 }, 264)).toBe('Showing 1 to 25 of 264 sensors')
  })
  it('names a single sensor in the singular', () => {
    expect(statusText(t, { from: 1, to: 1, total: 1 }, 30)).toBe('Showing 1 sensor')
  })
  it('says nothing matches when a filter emptied the result', () => {
    expect(statusText(t, { from: 0, to: 0, total: 0 }, 30)).toBe('No sensor matches this filter.')
  })
  it('says the area has no live sensors when there is nothing to filter', () => {
    expect(statusText(t, { from: 0, to: 0, total: 0 }, 0)).toBe('No live sensors in this area.')
  })
  it('uses the singular summary for one sensor', () => {
    expect(summaryText(t, 1)).toBe('The sensor in this area')
    expect(summaryText(t, 264)).toBe('All sensors in this area (264)')
  })
})
