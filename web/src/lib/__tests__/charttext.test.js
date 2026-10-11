import { describe, it, expect } from 'vitest'
import { chartText } from '../charttext.js'
import { formatStamp } from '../charttooltip.js'

const TEXT = {
  summary: '{metric}, {period}: latest {latest}, lowest {min}, highest {max}.',
  none: '{metric}, {period}: no readings.',
  caption: 'Readings for {metric}, {period}',
}
const P2 = { name: 'PM2.5', unit: 'µg/m³' }
const TEMP = { name: 'Temperature', unit: '°C' }
const T0 = 1786665600
const base = { lang: 'en', timeZone: 'UTC', periodLabel: '24 hours', text: TEXT, timeLabel: 'Time' }

describe('chartText', () => {
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
    expect(t.caption).toBe('Readings for PM2.5, Temperature, 24 hours')
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

  it('heads the table with the time label and each line with its unit, and rows carry the chart stamp', () => {
    const t = chartText({ ...base, lines: [P2, TEMP], data: [[T0, T0 + 3600], [12.34, null], [21, 22]] })
    expect(t.head).toEqual(['Time', 'PM2.5 (µg/m³)', 'Temperature (°C)'])
    expect(t.rows).toHaveLength(2)
    expect(t.rows[0][0]).toBe(formatStamp(T0, 'en', 'UTC'))
    expect(t.rows[0].slice(1)).toEqual(['12.3', '21'])
    expect(t.rows[1].slice(1)[1]).toBe('22')
  })
})
