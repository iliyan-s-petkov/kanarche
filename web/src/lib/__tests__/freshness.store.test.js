import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createFreshness, getFreshness, resetFreshnessForTests } from '../freshness.svelte.js'
import { AUTO_KEY, intervalMs } from '../freshness.js'

// A document whose visibility the test flips by hand.
function fakeDoc() {
  const listeners = new Set()
  return {
    visibilityState: 'visible',
    listeners,
    addEventListener(type, fn) { if (type === 'visibilitychange') listeners.add(fn) },
    removeEventListener(type, fn) { if (type === 'visibilitychange') listeners.delete(fn) },
    set(state) { this.visibilityState = state; for (const fn of [...listeners]) fn() },
  }
}

// A fake window whose interval never actually fires: the tests drive it by
// hand, so a five-minute cadence costs nothing and nothing leaks between them.
function fakeWin() {
  const timers = new Map()
  let next = 1
  return {
    timers,
    setInterval(fn, ms) { timers.set(next, { fn, ms }); return next++ },
    clearInterval(id) { timers.delete(id) },
    fire(id) { timers.get(id).fn() },
  }
}

function store(initial = {}) {
  const map = new Map(Object.entries(initial))
  return {
    map,
    getItem(k) { return map.has(k) ? map.get(k) : null },
    setItem(k, v) { map.set(k, v) },
  }
}

let errs
beforeEach(() => { errs = vi.spyOn(console, 'error').mockImplementation(() => {}) })
afterEach(() => { errs.mockRestore(); resetFreshnessForTests() })

describe('createFreshness', () => {
  it('starts with a time, because the server-rendered page was fresh', () => {
    const f = createFreshness({ win: fakeWin(), storage: store(), now: () => 1000 })
    expect(f.at).toBe(1000)
    expect(f.busy).toBe(false)
    expect(f.failed).toBe(false)
  })

  // A map going quietly stale while someone watches it is the one failure this
  // page cannot report — the numbers still look like numbers.
  it('auto-refreshes every 5 minutes unless the reader chose otherwise', () => {
    expect(createFreshness({ win: fakeWin(), storage: store() }).minutes).toBe(5)
    expect(createFreshness({ win: fakeWin(), storage: store({ [AUTO_KEY]: '0' }) }).minutes).toBe(0)
    expect(createFreshness({ win: fakeWin(), storage: store({ [AUTO_KEY]: '30' }) }).minutes).toBe(30)
  })

  // The key used to hold a boolean, and a returning visitor still has one.
  it('migrates the old boolean: true becomes 5 minutes, false becomes off', () => {
    expect(createFreshness({ win: fakeWin(), storage: store({ [AUTO_KEY]: 'true' }) }).minutes).toBe(5)
    expect(createFreshness({ win: fakeWin(), storage: store({ [AUTO_KEY]: 'false' }) }).minutes).toBe(0)
  })

  it('runs every registered provider and stamps the time on success', async () => {
    const calls = []
    let clock = 100
    const f = createFreshness({ win: fakeWin(), storage: store(), now: () => clock })
    f.provide(async () => { calls.push('a') })
    f.provide(async () => { calls.push('b') })
    clock = 200
    await f.request()
    expect(calls.sort()).toEqual(['a', 'b'])
    expect(f.at).toBe(200)
    expect(f.failed).toBe(false)
  })

  // The status line is the only place a reader can find out that the numbers
  // they are looking at did not move, so a failure must not leave a fresh
  // timestamp behind it.
  it('keeps the old time and reports a failure when a provider throws', async () => {
    let clock = 100
    const f = createFreshness({ win: fakeWin(), storage: store(), now: () => clock })
    f.provide(async () => { throw new Error('offline') })
    clock = 200
    await f.request()
    expect(f.at).toBe(100)
    expect(f.failed).toBe(true)
    expect(f.busy).toBe(false)
    expect(errs).toHaveBeenCalled()
  })

  it('clears a previous failure once a request succeeds', async () => {
    const f = createFreshness({ win: fakeWin(), storage: store() })
    let ok = false
    f.provide(async () => { if (!ok) throw new Error('offline') })
    await f.request()
    expect(f.failed).toBe(true)
    ok = true
    await f.request()
    expect(f.failed).toBe(false)
  })

  // The button is deliberately not disabled while in flight (a disabled button
  // drops keyboard focus), so the guard has to live here instead.
  it('ignores a second request while one is running', async () => {
    let running
    let started = 0
    const f = createFreshness({ win: fakeWin(), storage: store() })
    f.provide(() => { started += 1; return new Promise((resolve) => { running = resolve }) })
    const first = f.request()
    expect(f.busy).toBe(true)
    await f.request()
    expect(started).toBe(1)
    running()
    await first
    expect(f.busy).toBe(false)
  })

  it('unregisters a provider through the handle provide returns', async () => {
    let called = 0
    const f = createFreshness({ win: fakeWin(), storage: store() })
    const off = f.provide(async () => { called += 1 })
    await f.request()
    off()
    await f.request()
    expect(called).toBe(1)
  })

  it('works with no provider at all', async () => {
    const f = createFreshness({ win: fakeWin(), storage: store(), now: () => 7 })
    await expect(f.request()).resolves.toBeUndefined()
    expect(f.at).toBe(7)
  })
})

describe('the auto-refresh timer', () => {
  it('is scheduled at the chosen interval', () => {
    const win = fakeWin()
    createFreshness({ win, storage: store() })
    expect([...win.timers.values()].map((t) => t.ms)).toEqual([intervalMs(5)])
    const win15 = fakeWin()
    createFreshness({ win: win15, storage: store({ [AUTO_KEY]: '15' }) })
    expect([...win15.timers.values()].map((t) => t.ms)).toEqual([intervalMs(15)])
  })

  it('is not scheduled when the reader has turned auto off', () => {
    const win = fakeWin()
    createFreshness({ win, storage: store({ [AUTO_KEY]: 'false' }) })
    expect(win.timers.size).toBe(0)
  })

  it('fires a real request', async () => {
    const win = fakeWin()
    let called = 0
    const f = createFreshness({ win, storage: store() })
    f.provide(async () => { called += 1 })
    win.fire([...win.timers.keys()][0])
    await Promise.resolve()
    await Promise.resolve()
    expect(called).toBe(1)
  })

  it('is cancelled and rebuilt as the reader picks an interval, never doubled', () => {
    const win = fakeWin()
    const f = createFreshness({ win, storage: store() })
    f.setMinutes(0)
    expect(win.timers.size).toBe(0)
    f.setMinutes(15)
    expect([...win.timers.values()].map((t) => t.ms)).toEqual([intervalMs(15)])
    // Picking the value it already has must not stack a second timer.
    f.setMinutes(15)
    expect(win.timers.size).toBe(1)
  })

  it('ignores an interval the menu does not offer', () => {
    const f = createFreshness({ win: fakeWin(), storage: store() })
    f.setMinutes(7)
    expect(f.minutes).toBe(5)
  })

  it('remembers the choice for the next visit, in the same key', () => {
    const s = store()
    const f = createFreshness({ win: fakeWin(), storage: s })
    f.setMinutes(0)
    expect(s.map.get(AUTO_KEY)).toBe('0')
    f.setMinutes(30)
    expect(s.map.get(AUTO_KEY)).toBe('30')
  })

  it('is torn down by destroy', () => {
    const win = fakeWin()
    const f = createFreshness({ win, storage: store() })
    f.destroy()
    expect(win.timers.size).toBe(0)
  })
})

describe('a hidden tab', () => {
  function setup(initial = {}) {
    const win = fakeWin()
    const doc = fakeDoc()
    let clock = 0
    const f = createFreshness({ win, doc, storage: store(initial), now: () => clock })
    let calls = 0
    f.provide(async () => { calls += 1 })
    return { win, doc, f, calls: () => calls, advance(ms) { clock += ms } }
  }
  const settle = async () => { for (let i = 0; i < 4; i += 1) await Promise.resolve() }

  it('pauses the timer while hidden', () => {
    const { win, doc } = setup()
    expect(win.timers.size).toBe(1)
    doc.set('hidden')
    expect(win.timers.size).toBe(0)
  })

  it('does not schedule a pick made while hidden', () => {
    const { win, doc, f } = setup()
    doc.set('hidden')
    f.setMinutes(15)
    expect(win.timers.size).toBe(0)
    doc.set('visible')
    expect(win.timers.size).toBe(1)
  })

  it('refreshes once on return when the interval has elapsed, then resumes the timer', async () => {
    const { win, doc, calls, advance } = setup()
    doc.set('hidden')
    advance(intervalMs(5))
    doc.set('visible')
    await settle()
    expect(calls()).toBe(1)
    expect(win.timers.size).toBe(1)
  })

  it('resumes without refreshing when the interval has not elapsed', async () => {
    const { win, doc, calls, advance } = setup()
    doc.set('hidden')
    advance(intervalMs(5) - 1)
    doc.set('visible')
    await settle()
    expect(calls()).toBe(0)
    expect(win.timers.size).toBe(1)
  })

  it('measures the elapsed time against the chosen interval', async () => {
    const { doc, calls, advance } = setup({ [AUTO_KEY]: '15' })
    doc.set('hidden')
    advance(intervalMs(10))
    doc.set('visible')
    await settle()
    expect(calls()).toBe(0)
    doc.set('hidden')
    advance(intervalMs(5))
    doc.set('visible')
    await settle()
    expect(calls()).toBe(1)
  })

  it('never refreshes on return when auto-refresh is off', async () => {
    const { win, doc, calls, advance } = setup({ [AUTO_KEY]: '0' })
    doc.set('hidden')
    advance(intervalMs(60))
    doc.set('visible')
    await settle()
    expect(calls()).toBe(0)
    expect(win.timers.size).toBe(0)
  })

  it('stops listening once destroyed', () => {
    const { doc, f } = setup()
    f.destroy()
    expect(doc.listeners.size).toBe(0)
  })
})

describe('getFreshness', () => {
  // Three islands reporting three different last-refresh times is worse than
  // none: the button, the line and the map must all be talking about the same
  // request.
  it('hands every caller the same store', () => {
    const win = fakeWin()
    expect(getFreshness({ win, storage: store() })).toBe(getFreshness())
  })

  it('is rebuilt after the test-only reset', () => {
    const win = fakeWin()
    const first = getFreshness({ win, storage: store() })
    resetFreshnessForTests()
    expect(getFreshness({ win, storage: store() })).not.toBe(first)
  })
})
