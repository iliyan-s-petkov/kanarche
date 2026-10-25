import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { statusText, snapshotTime, reloadedSnapshot, isStale, STALE_MS, formatTime, AUTO_KEY, AUTO_CHOICES, DEFAULT_MINUTES, intervalMs, minutesFromStored } from '../freshness.js'

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

  it('states the snapshot time of the last success', () => {
    expect(statusText({ busy: false, at: AT + 99, dataAt: AT, failed: false }, T)).toBe(`${T.updated} ${formatTime(AT)}`)
  })

  // Absence stated plainly (DESIGN.md §2.3): a page that has never loaded says
  // nothing at all rather than announcing that it has no time to show.
  it('says nothing when there has never been a reading', () => {
    expect(statusText({ busy: false, dataAt: null, failed: false }, T)).toBe('')
    expect(statusText({ busy: false, dataAt: undefined, failed: false }, T)).toBe('')
  })

  // 0 is a legitimate epoch instant, and `at == null` is what distinguishes
  // "never" from it. A truthiness check here would silence the line.
  it('treats epoch zero as a time, not as an absence', () => {
    expect(statusText({ busy: false, dataAt: 0, failed: false }, T)).not.toBe('')
  })
})

describe('statusText with a snapshot time', () => {
  const SNAP = Date.parse('2026-10-25T14:20:30Z')

  it('shows the snapshot time, not the client fetch time', () => {
    expect(statusText({ busy: false, at: AT, dataAt: SNAP, failed: false }, T)).toBe(`${T.updated} ${formatTime(SNAP)}`)
  })

  // The client clock is not the server's, so it is never written as "Data from".
  it('says nothing when only the client fetch time is known', () => {
    expect(statusText({ busy: false, at: AT, dataAt: null, failed: false }, T)).toBe('')
    expect(statusText({ busy: false, at: AT, failed: false }, T)).toBe('')
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

describe('reloadedSnapshot', () => {
  const OLD = { generated_at: '2026-10-25T14:00:00Z' }
  const NEW = { generated_at: '2026-10-25T14:10:00Z' }
  const SENSOR = { generated_at: '2026-10-25T14:09:00Z' }

  it('takes the time of a body that was replaced', () => {
    expect(reloadedSnapshot([OLD, null], [NEW, null])).toBe(Date.parse(NEW.generated_at))
  })

  // A body the refresh did not reload says nothing about this refresh.
  it('ignores a body that is the same object as before', () => {
    expect(reloadedSnapshot([OLD, SENSOR], [OLD, SENSOR])).toBeNull()
    expect(reloadedSnapshot([OLD, SENSOR], [OLD, { ...SENSOR, generated_at: '2026-10-25T14:12:00Z' }]))
      .toBe(Date.parse('2026-10-25T14:12:00Z'))
  })

  it('does not let an unreloaded first-load sensor body supply the time', () => {
    expect(reloadedSnapshot([OLD, SENSOR], [{}, SENSOR])).toBeNull()
  })

  it('prefers the first reloaded body in order, and ignores a missing one', () => {
    expect(reloadedSnapshot([OLD, null], [NEW, SENSOR])).toBe(Date.parse(NEW.generated_at))
    expect(reloadedSnapshot([null, null], [null, SENSOR])).toBe(Date.parse(SENSOR.generated_at))
    expect(reloadedSnapshot([OLD, null], [null, null])).toBeNull()
  })
})

describe('formatTime', () => {
  // Zone pinned so the literals are the same on every runner.
  const zone = process.env.TZ
  beforeAll(() => { process.env.TZ = 'Europe/Sofia' })
  afterAll(() => { if (zone === undefined) delete process.env.TZ; else process.env.TZ = zone })

  it('writes 24-hour HH:MM in both page languages', () => {
    // 14:07 UTC is 16:07 in Sofia in March. The afternoon is what a 12-hour clock would show as PM.
    expect(formatTime(AT, 'bg')).toBe('16:07')
    expect(formatTime(AT, 'en')).toBe('16:07')
    expect(formatTime(AT, 'en-US')).toBe('16:07')
  })

  it('writes midnight as 00:xx, not 24:xx', () => {
    expect(formatTime(Date.UTC(2026, 2, 1, 22, 5), 'en')).toBe('00:05')
    expect(formatTime(Date.UTC(2026, 2, 1, 22, 5), 'bg')).toBe('00:05')
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
