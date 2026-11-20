import { describe, it, expect } from 'vitest'
import { chartData, dayLabel, partialLabel, sliceRange, sofiaClock, trailingAverage, visitorDays } from '../visitors.js'

const day = (d, uniques, extra = {}) => ({ day: d, uniques, requests: 9999, page_views: 8888, partial: false, ...extra })
const ymd = (n) => new Date(Date.UTC(2026, 8, 1 + n)).toISOString().slice(0, 10)
// n consecutive closed days from 2026-09-01 with uniques 10, 20, 30, ...
const run = (n) => Array.from({ length: n }, (_, i) => day(ymd(i), (i + 1) * 10))
const sec = (d) => Date.parse(`${d}T00:00:00Z`) / 1000

describe('visitorDays', () => {
  it('keeps uniques only, on epoch-second UTC midnights', () => {
    const days = visitorDays({ days: [day('2026-09-26', 10), day('2026-09-27', 25)] })
    expect(days.map((d) => d.y)).toEqual([10, 25])
    expect(days.map((d) => d.x)).toEqual([sec('2026-09-26'), sec('2026-09-27')])
    expect(JSON.stringify(days)).not.toContain('9999')
    expect(JSON.stringify(days)).not.toContain('8888')
  })

  it('orders days ascending even when the server does not', () => {
    const days = visitorDays({ days: [day('2026-09-28', 3), day('2026-09-26', 1), day('2026-09-27', 2)] })
    expect(days.map((d) => d.y)).toEqual([1, 2, 3])
  })

  it('drops rows without a usable day or count instead of inventing a point', () => {
    const body = { days: [day('2026-09-26', 4), { day: 'nope', uniques: 5 }, { day: '2026-09-27', uniques: null }, day('2026-09-28', 6)] }
    expect(visitorDays(body).map((d) => d.y)).toEqual([4, 6])
  })

  it('carries the partial flag and fetched_at, and treats anything but true as closed', () => {
    const days = visitorDays({
      days: [day('2026-09-27', 1, { partial: 'yes' }), day('2026-09-28', 2, { partial: true, fetched_at: '2026-09-28T10:00:00Z' })],
    })
    expect(days.map((d) => d.partial)).toEqual([false, true])
    expect(days[1].fetchedAt).toBe('2026-09-28T10:00:00Z')
    expect(days[0].fetchedAt).toBeNull()
  })

  it('returns an empty list for a missing or malformed body', () => {
    expect(visitorDays(null)).toEqual([])
    expect(visitorDays({})).toEqual([])
    expect(visitorDays({ days: 'x' })).toEqual([])
  })
})

describe('trailingAverage', () => {
  it('is null until seven days exist, then the mean of the last seven', () => {
    const avg = trailingAverage(visitorDays({ days: run(9) }))
    expect(avg.slice(0, 6)).toEqual([null, null, null, null, null, null])
    // days 1..7 are 10..70, mean 40; days 2..8 are 20..80, mean 50
    expect(avg[6]).toBe(40)
    expect(avg[7]).toBe(50)
    expect(avg[8]).toBe(60)
  })

  it('leaves a gap where a day inside the window is missing', () => {
    const rows = run(10).filter((_, i) => i !== 4)
    const avg = trailingAverage(visitorDays({ days: rows }))
    // The missing day sits in every seven-day window up to index 9 of the calendar.
    expect(avg.every((v) => v === null)).toBe(true)
    const longer = trailingAverage(visitorDays({ days: run(14).filter((_, i) => i !== 4) }))
    expect(longer[longer.length - 1]).not.toBeNull()
  })

  it('never includes the partial day and gives it no value', () => {
    const rows = run(8)
    rows[7] = day(ymd(7), 100000, { partial: true, fetched_at: '2026-09-08T10:00:00Z' })
    const avg = trailingAverage(visitorDays({ days: rows }))
    expect(avg[7]).toBeNull()
    expect(avg[6]).toBe(40)
  })

  it('does not treat the partial day as a way to fill a window', () => {
    const rows = run(7)
    rows[6] = { ...rows[6], partial: true, fetched_at: '2026-09-07T10:00:00Z' }
    const avg = trailingAverage(visitorDays({ days: rows }))
    expect(avg.every((v) => v === null)).toBe(true)
  })
})

describe('sliceRange', () => {
  const days = visitorDays({ days: run(100) })

  it('keeps the last N calendar days ending at the newest day', () => {
    expect(sliceRange(days, 30)).toHaveLength(30)
    expect(sliceRange(days, 90)).toHaveLength(90)
    expect(sliceRange(days, 30)[29]).toBe(days[99])
  })

  it('keeps everything when fewer days exist than the range asks for', () => {
    expect(sliceRange(days, 365)).toHaveLength(100)
    expect(sliceRange(visitorDays({ days: run(12) }), 30)).toHaveLength(12)
  })

  it('counts calendar days, so a gap shortens the slice', () => {
    const gappy = visitorDays({ days: run(40).filter((_, i) => i < 5 || i > 20) })
    expect(sliceRange(gappy, 30).every((d) => d.x >= gappy[gappy.length - 1].x - 29 * 86400)).toBe(true)
  })
})

describe('chartData', () => {
  const withToday = (n) => {
    const rows = run(n)
    rows[n - 1] = day(ymd(n - 1), 5, { partial: true, fetched_at: '2026-10-01T09:05:00Z' })
    return { days: rows }
  }

  it('splits closed days from the partial pair', () => {
    const d = chartData(withToday(10), 30)
    expect(d.partial).toBe(9)
    expect(d.complete[9]).toBeNull()
    expect(d.complete.slice(0, 9)).toEqual([10, 20, 30, 40, 50, 60, 70, 80, 90])
    // dashed pair: yesterday (closed value) and today
    expect(d.today.slice(0, 8).every((v) => v === null)).toBe(true)
    expect(d.today[8]).toBe(90)
    expect(d.today[9]).toBe(5)
    expect(d.fetchedAt).toBe('2026-10-01T09:05:00Z')
  })

  it('has no partial pair when the newest day is closed', () => {
    const d = chartData({ days: run(10) }, 30)
    expect(d.partial).toBe(-1)
    expect(d.today.every((v) => v === null)).toBe(true)
    expect(d.fetchedAt).toBeNull()
    expect(d.complete[9]).toBe(100)
  })

  it('starts the dashed segment only from an adjacent yesterday', () => {
    const rows = run(10)
    rows[9] = day(ymd(9), 5, { partial: true, fetched_at: 'x' })
    const d = chartData({ days: rows.filter((_, i) => i !== 8) }, 30)
    expect(d.today.filter((v) => v !== null)).toEqual([5])
  })

  it('averages over all stored days so a short range has values at its left edge', () => {
    const d = chartData({ days: run(60) }, 30)
    expect(d.xs).toHaveLength(30)
    expect(d.average[0]).not.toBeNull()
    expect(d.average[0]).toBe(trailingAverage(visitorDays({ days: run(60) }))[30])
  })

  it('slices to 30, 90 and 365 days and falls back to all stored days', () => {
    const body = { days: run(100) }
    expect(chartData(body, 30).xs).toHaveLength(30)
    expect(chartData(body, 90).xs).toHaveLength(90)
    expect(chartData(body, 365).xs).toHaveLength(100)
  })

  it('is null when fewer than two days are in range', () => {
    expect(chartData({ days: [] })).toBeNull()
    expect(chartData(null)).toBeNull()
    expect(chartData({ days: [day('2026-09-28', 5)] })).toBeNull()
  })
})

describe('sofiaClock', () => {
  it('formats HH:MM in Europe/Sofia, 24 hour', () => {
    expect(sofiaClock('2026-07-10T09:05:00Z')).toBe('12:05')
    expect(sofiaClock('2026-01-10T09:05:00Z')).toBe('11:05')
    expect(sofiaClock('2026-01-10T21:59:00Z')).toBe('23:59')
    expect(sofiaClock('2026-01-10T22:00:00Z')).toBe('00:00')
  })

  it('follows the clock change in spring (2026-03-29 01:00 UTC)', () => {
    expect(sofiaClock('2026-03-29T00:30:00Z')).toBe('02:30')
    expect(sofiaClock('2026-03-29T01:30:00Z')).toBe('04:30')
  })

  it('follows the clock change in autumn (2026-10-25 01:00 UTC)', () => {
    expect(sofiaClock('2026-10-25T00:30:00Z')).toBe('03:30')
    expect(sofiaClock('2026-10-25T01:30:00Z')).toBe('03:30')
    expect(sofiaClock('2026-10-25T02:30:00Z')).toBe('04:30')
  })

  it('is empty for a missing or unreadable stamp', () => {
    expect(sofiaClock(null)).toBe('')
    expect(sofiaClock('soon')).toBe('')
  })
})

describe('partialLabel', () => {
  it('fills {time} in the page-language template', () => {
    expect(partialLabel('днес до {time} ч.', '2026-07-10T09:05:00Z')).toBe('днес до 12:05 ч.')
    expect(partialLabel('today until {time}', '2026-07-10T09:05:00Z')).toBe('today until 12:05')
  })

  it('is empty without a stamp, so no half-filled label is shown', () => {
    expect(partialLabel('today until {time}', null)).toBe('')
  })
})

describe('dayLabel', () => {
  const midnight = Date.UTC(2026, 10, 8) / 1000

  it('prints the UTC day in the page language, with no time', () => {
    expect(dayLabel(midnight, 'bg')).toBe('08.11.2026')
    expect(dayLabel(midnight, 'en')).toBe('8 Nov 2026')
  })

  it('does not slip back a day for a reader behind UTC', () => {
    const was = process.env.TZ
    process.env.TZ = 'America/Los_Angeles'
    try {
      // Local time there is 2026-11-07 16:00, so a local formatter would say the 7th.
      expect(new Date(midnight * 1000).getDate()).toBe(7)
      expect(dayLabel(midnight, 'bg')).toBe('08.11.2026')
      expect(dayLabel(midnight, 'en')).toBe('8 Nov 2026')
    } finally {
      if (was === undefined) delete process.env.TZ
      else process.env.TZ = was
    }
  })

  it('is empty for an unusable value', () => {
    expect(dayLabel(NaN)).toBe('')
  })
})
