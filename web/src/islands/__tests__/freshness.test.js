// @vitest-environment jsdom
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { flushSync, unmount } from 'svelte'
import { mount } from '../freshness.js'
import { getFreshness, resetFreshnessForTests } from '../../lib/freshness.svelte.js'
import { getJSON, clearCache, resetServerClockForTests } from '../../lib/api.js'

// What the server's clock read when it answered, and the snapshot it built a few minutes earlier.
const SERVER = Date.parse('2026-10-25T12:00:00Z')
const MIN = 60 * 1000

let component
let el
beforeEach(() => {
  clearCache()
  resetServerClockForTests()
  resetFreshnessForTests()
  document.documentElement.setAttribute('lang', 'en')
  el = document.createElement('div')
  el.dataset.tLabel = 'Refresh'
  el.dataset.tUpdated = 'Data from'
  document.body.appendChild(el)
  vi.useFakeTimers()
})
afterEach(() => {
  if (component) unmount(component)
  component = null
  vi.useRealTimers()
  vi.restoreAllMocks()
  document.body.innerHTML = ''
})

// The client clock is set, a response teaches the page the server's, and a snapshot of the given age is shown.
async function showSnapshot(clientNow, ageMin) {
  vi.setSystemTime(clientNow)
  globalThis.fetch = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    headers: { get: (n) => (n === 'Date' ? new Date(SERVER).toUTCString() : null) },
    json: async () => ({}),
  })
  await getJSON('/api/v1/hexes?skew')
  getFreshness().noteSnapshot(SERVER - ageMin * MIN)
  component = mount(el)
  flushSync()
  return el.querySelector('.data-refresh__time')
}

describe('freshness island on a skewed client clock', () => {
  it('does not call a 10 minute old snapshot stale when the client is 20 minutes ahead', async () => {
    const time = await showSnapshot(SERVER + 20 * MIN, 10)
    expect(time.classList.contains('data-refresh__time--stale')).toBe(false)
  })

  it('calls a 20 minute old snapshot stale when the client is 20 minutes behind', async () => {
    const time = await showSnapshot(SERVER - 20 * MIN, 20)
    expect(time.classList.contains('data-refresh__time--stale')).toBe(true)
  })

  it('shows the snapshot time and never the client time', async () => {
    const time = await showSnapshot(SERVER + 20 * MIN, 10)
    const want = new Date(SERVER - 10 * MIN).toLocaleTimeString('en', { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })
    expect(time.textContent).toBe(want)
  })

  // The island mounts before the first response, so the clock offset arrives after the first read of "now".
  it('reads the server clock again when the snapshot arrives after mount', async () => {
    vi.setSystemTime(SERVER + 20 * MIN)
    component = mount(el)
    flushSync()
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: { get: (n) => (n === 'Date' ? new Date(SERVER).toUTCString() : null) },
      json: async () => ({}),
    })
    await getJSON('/api/v1/hexes?late')
    getFreshness().noteSnapshot(SERVER - 10 * MIN)
    flushSync()
    expect(el.querySelector('.data-refresh__time').classList.contains('data-refresh__time--stale')).toBe(false)
  })

  it('shows no time, and the plain name, when no snapshot time is known', () => {
    vi.setSystemTime(SERVER)
    component = mount(el)
    flushSync()
    expect(el.querySelector('.data-refresh__time')).toBe(null)
    expect(el.querySelector('.data-refresh__btn--icon').getAttribute('aria-label')).toBe('Refresh')
  })
})
