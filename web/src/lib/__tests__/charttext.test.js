import { describe, it, expect } from 'vitest'
import { chartText } from '../charttext.js'
import { formatDay, formatStamp } from '../charttooltip.js'

const TEXT = {
  summary: '{metric}, {period}: latest {latest}, lowest {min}, highest {max}.',
  none: '{metric}, {period}: no readings.',
  captionHourly: 'Hourly averages for {metric}, {period}',
  captionDaily: 'Daily averages for {metric}, {period}',
}
const P2 = { name: 'PM2.5', unit: 'µg/m³' }
const TEMP = { name: 'Temperature', unit: '°C' }
const T0 = 1786665600
const base = { lang: 'en', timeZone: 'UTC', periodLabel: '24 hours', text: TEXT, timeLabel: 'Time' }

// n readings from start (ISO), every stepMin minutes, value f(i).
const series = (start, n, stepMin, f = (i) => i) => {
  const t0 = Date.parse(start) / 1000
  const times = Array.from({ length: n }, (_, i) => t0 + i * stepMin * 60)
  return [times, times.map((_, i) => f(i))]
}

describe('chartText spoken units', () => {
  const speak = (u) => ({ 'µg/m³': 'micrograms per cubic metre' }[u] ?? u)

  it('says the unit in words in the summary a screen reader gets', () => {
    const t = chartText({ ...base, speak, lines: [P2], data: [[T0, T0 + 3600], [12.3, 30]] })
    expect(t.summary).toBe('PM2.5, 24 hours: latest 30 micrograms per cubic metre, lowest 12.3 micrograms per cubic metre, highest 30 micrograms per cubic metre.')
  })

  it('keeps the symbol in the column head, which is read as text on screen', () => {
    const t = chartText({ ...base, speak, lines: [P2], data: [[T0, T0 + 3600], [12.3, 30]] })
    expect(t.head).toEqual(['Time', 'PM2.5 (µg/m³)'])
  })
})

describe('chartText summary', () => {
  it('names the metric, period, latest, lowest and highest with units', () => {
    const t = chartText({ ...base, lines: [P2], data: [[T0, T0 + 3600, T0 + 7200], [12.3, 30, 20.5]] })
    expect(t.summary).toBe('PM2.5, 24 hours: latest 20.5 µg/m³, lowest 12.3 µg/m³, highest 30 µg/m³.')
  })

  it('takes the latest reading that exists when the newest bucket is a gap', () => {
    const t = chartText({ ...base, lines: [P2], data: [[T0, T0 + 3600, T0 + 7200], [5, 9, null]] })
    expect(t.summary).toContain('latest 9 µg/m³')
    expect(t.summary).toContain('lowest 5 µg/m³')
  })

  it('summarises each line in turn', () => {
    const t = chartText({ ...base, lines: [P2, TEMP], data: [[T0, T0 + 3600], [1, 2], [21, 22]] })
    expect(t.summary).toBe(
      'PM2.5, 24 hours: latest 2 µg/m³, lowest 1 µg/m³, highest 2 µg/m³. ' +
      'Temperature, 24 hours: latest 22 °C, lowest 21 °C, highest 22 °C.')
    expect(t.caption).toBe('Hourly averages for PM2.5, Temperature, 24 hours')
  })

  it('gives a no-readings summary and no rows for an empty series, never NaN', () => {
    const t = chartText({ ...base, lines: [P2], data: [[], []] })
    expect(t.summary).toBe('PM2.5, 24 hours: no readings.')
    expect(t.summary).not.toMatch(/NaN|undefined/)
    expect(t.rows).toEqual([])
  })

  it('treats a line of only gaps as having no readings', () => {
    const t = chartText({ ...base, lines: [P2], data: [[T0, T0 + 3600], [null, null]] })
    expect(t.summary).toBe('PM2.5, 24 hours: no readings.')
  })

  it('leaves the summary on the raw readings of a long series', () => {
    const t = chartText({ ...base, lines: [P2], data: series('2026-08-01T00:00:00Z', 200, 60) })
    expect(t.summary).toBe('PM2.5, 24 hours: latest 199 µg/m³, lowest 0 µg/m³, highest 199 µg/m³.')
  })
})

describe('chartText rows', () => {
  it('heads the table with the time label and each line with its unit', () => {
    const t = chartText({ ...base, lines: [P2, TEMP], data: [[T0, T0 + 3600], [12.34, null], [21, 22]] })
    expect(t.head).toEqual(['Time', 'PM2.5 (µg/m³)', 'Temperature (°C)'])
    expect(t.rows).toHaveLength(2)
    expect(t.rows[0][0]).toBe(formatStamp(T0, 'en', 'UTC'))
    expect(t.rows[0].slice(1)).toEqual(['12.3', '21'])
    expect(t.rows[1].slice(1)[1]).toBe('22')
  })

  it('averages 24 hours of 5-minute readings into one row per clock hour', () => {
    // 288 readings from 00:05 UTC: 12 per hour, 23:55 the last. Value is the hour number.
    const t = chartText({ ...base, lines: [P2], data: series('2026-08-01T00:05:00Z', 288, 5, (i) => Math.floor((i + 1) / 12)) })
    expect(t.caption).toBe('Hourly averages for PM2.5, 24 hours')
    expect(t.rows.length).toBeGreaterThanOrEqual(24)
    expect(t.rows.length).toBeLessThanOrEqual(25)
    // The first bucket is 00:00-00:55 and its label is the hour, not the first reading.
    expect(t.rows[0][0]).toBe(formatStamp(Date.parse('2026-08-01T00:00:00Z') / 1000, 'en', 'UTC'))
  })

  it('averages 7 days of hourly readings into daily rows', () => {
    const t = chartText({ ...base, periodLabel: '7 days', lines: [P2], data: series('2026-08-01T00:00:00Z', 168, 60, () => 3) })
    expect(t.caption).toBe('Daily averages for PM2.5, 7 days')
    expect(t.rows).toHaveLength(7)
    expect(t.rows[0][0]).toBe(formatDay(Date.parse('2026-08-01T00:00:00Z') / 1000, 'en', 'UTC'))
    expect(t.rows[0][0]).toMatch(/2026/)
    expect(t.rows[0][1]).toBe('3')
  })

  it('shrinks a year of hourly readings to about 365 daily rows', () => {
    const t = chartText({ ...base, lines: [P2], data: series('2025-10-11T00:00:00Z', 8760, 60) })
    expect(t.rows.length).toBeGreaterThanOrEqual(365)
    expect(t.rows.length).toBeLessThanOrEqual(366)
  })

  it('stays hourly at a span of exactly 48 hours and goes daily just past it', () => {
    // 49 readings an hour apart span exactly 48 hours.
    const at48 = chartText({ ...base, lines: [P2], data: series('2026-08-01T00:00:00Z', 49, 60) })
    expect(at48.caption).toMatch(/^Hourly/)
    expect(at48.rows).toHaveLength(49)
    const past = chartText({ ...base, lines: [P2], data: series('2026-08-01T00:00:00Z', 50, 60) })
    expect(past.caption).toMatch(/^Daily/)
  })

  it('splits days at local midnight across the autumn DST change in Sofia', () => {
    // Starts 23:00 on 24 Oct in Sofia; the 25th has 25 hours (clocks go back at 04:00).
    const t = chartText({ ...base, timeZone: 'Europe/Sofia', lines: [P2], data: series('2026-10-24T20:00:00Z', 200, 60) })
    expect(t.rows[0].slice(1)).toEqual(['0'])
    expect(t.rows[1].slice(1)).toEqual(['13'])
    expect(t.rows[2].slice(1)).toEqual(['37.5'])
    expect(t.rows[1][0]).toMatch(/25/)
    expect(t.rows[1][0]).not.toMatch(/\d:\d/)
  })

  it('splits hours at the local clock in a zone with a half-hour offset', () => {
    // 00:20 and 00:40 UTC are 05:50 and 06:10 in Kolkata, so they fall in different hours.
    const t = chartText({ ...base, timeZone: 'Asia/Kolkata', lines: [P2], data: [[Date.parse('2026-08-01T00:20:00Z') / 1000, Date.parse('2026-08-01T00:40:00Z') / 1000], [1, 3]] })
    expect(t.rows).toHaveLength(2)
  })

  it('averages without the gaps and shows the gap mark for a day with none', () => {
    const data = series('2026-08-01T00:00:00Z', 200, 60, (i) => (i < 24 ? null : i < 48 ? (i % 2 ? 10 : null) : 4))
    const t = chartText({ ...base, lines: [P2], data })
    expect(t.rows[0].slice(1)).toEqual(['—'])
    expect(t.rows[1].slice(1)).toEqual(['10'])
    expect(t.rows[2].slice(1)).toEqual(['4'])
  })
})
