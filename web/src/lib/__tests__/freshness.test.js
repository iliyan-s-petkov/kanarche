import { describe, it, expect } from 'vitest'
import { statusText, snapshotTime, isStale, STALE_MS, formatTime, AUTO_KEY, AUTO_CHOICES, DEFAULT_MINUTES, intervalMs, minutesFromStored } from '../freshness.js'

const T = { updated: 'Данни от', loading: 'Обновяване…', failed: 'Обновяването не успя' }

// 2026-03-01 14:07 UTC. Asserted through formatTime rather than against a
// literal "14:07", because the runner's zone is not the reader's and this test
// is about which of the three sentences gets picked, not about the clock.
const AT = Date.UTC(2026, 2, 1, 14, 7)

describe('statusText', () => {
  it('says it is working before it says anything else', () => {
    // busy wins over both a previous time and a previous failure: the reader
    // needs to know a request is in flight, not what the last one did.
    expect(statusText({ busy: true, at: AT, failed: true }, T)).toBe(T.loading)
  })

  it('reports a failure rather than a stale time', () => {
    expect(statusText({ busy: false, at: AT, failed: true }, T)).toBe(T.failed)
  })

  it('states the time of the last success', () => {
    expect(statusText({ busy: false, at: AT, failed: false }, T)).toBe(`${T.updated} ${formatTime(AT)}`)
  })

  // Absence stated plainly (DESIGN.md §2.3): a page that has never loaded says
  // nothing at all rather than announcing that it has no time to show.
  it('says nothing when there has never been a reading', () => {
    expect(statusText({ busy: false, at: null, failed: false }, T)).toBe('')
    expect(statusText({ busy: false, at: undefined, failed: false }, T)).toBe('')
  })

  // 0 is a legitimate epoch instant, and `at == null` is what distinguishes
  // "never" from it. A truthiness check here would silence the line.
  it('treats epoch zero as a time, not as an absence', () => {
    expect(statusText({ busy: false, at: 0, failed: false }, T)).not.toBe('')
  })
})

describe('statusText with a snapshot time', () => {
  const SNAP = Date.parse('2026-10-25T14:20:30Z')

  it('shows the snapshot time, not the client fetch time', () => {
    expect(statusText({ busy: false, at: AT, dataAt: SNAP, failed: false }, T)).toBe(`${T.updated} ${formatTime(SNAP)}`)
  })

  it('falls back to the client time when the body had none', () => {
    expect(statusText({ busy: false, at: AT, dataAt: null, failed: false }, T)).toBe(`${T.updated} ${formatTime(AT)}`)
    expect(statusText({ busy: false, at: AT, failed: false }, T)).toBe(`${T.updated} ${formatTime(AT)}`)
  })
})

describe('isStale', () => {
  const NOW = Date.parse('2026-10-25T14:30:00Z')

  it('is stale only past fifteen minutes', () => {
    expect(STALE_MS).toBe(15 * 60 * 1000)
    expect(isStale(NOW - STALE_MS - 1, NOW)).toBe(true)
    expect(isStale(NOW - STALE_MS, NOW)).toBe(false)
    expect(isStale(NOW, NOW)).toBe(false)
  })

  it('has nothing to call stale without a time', () => {
    expect(isStale(null, NOW)).toBe(false)
    expect(isStale(undefined, NOW)).toBe(false)
  })
})

describe('snapshotTime', () => {
  it('reads generated_at from the first body that has a valid one', () => {
    expect(snapshotTime({ generated_at: '2026-10-25T14:20:30Z' })).toBe(Date.parse('2026-10-25T14:20:30Z'))
    expect(snapshotTime(null, {}, { generated_at: 'nope' }, { generated_at: '2026-10-25T14:25:31Z' }))
      .toBe(Date.parse('2026-10-25T14:25:31Z'))
  })

  it('is null when no body carries a usable time', () => {
    expect(snapshotTime()).toBeNull()
    expect(snapshotTime(null, undefined, {}, { generated_at: '' })).toBeNull()
  })
})

describe('formatTime', () => {
  it('writes hours and minutes in the page language', () => {
    expect(formatTime(AT, 'bg')).toMatch(/^\d{2}:\d{2}$/)
    // en-US asks for a 12-hour clock, so the two spellings must differ — this
    // is what passing `lang` buys, and a hard-coded format would lose it.
    expect(formatTime(AT, 'en-US')).toMatch(/AM|PM/)
  })
})

describe('constants', () => {
  it('keys the preference under the site namespace', () => {
    expect(AUTO_KEY).toBe('kanarche:auto-refresh')
  })

  it('offers off, 5, 15 and 30 minutes, defaulting to 5', () => {
    expect(AUTO_CHOICES).toEqual([0, 5, 15, 30])
    expect(DEFAULT_MINUTES).toBe(5)
    expect(intervalMs(5)).toBe(300000)
  })
})

describe('minutesFromStored', () => {
  it('migrates the old boolean: true is 5 minutes, false is off', () => {
    expect(minutesFromStored('true')).toBe(5)
    expect(minutesFromStored('false')).toBe(0)
  })

  it('reads a stored interval back as a number', () => {
    expect(minutesFromStored('0')).toBe(0)
    expect(minutesFromStored('15')).toBe(15)
    expect(minutesFromStored('30')).toBe(30)
  })

  it('falls back to the default for missing or unknown values', () => {
    expect(minutesFromStored(null)).toBe(5)
    expect(minutesFromStored('7')).toBe(5)
    expect(minutesFromStored('junk')).toBe(5)
  })
})
