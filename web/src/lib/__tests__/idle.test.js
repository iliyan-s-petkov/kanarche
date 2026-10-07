import { describe, it, expect, vi } from 'vitest'
import { afterIdle, whenRenderSettled } from '../idle.js'

describe('afterIdle', () => {
  it('uses requestIdleCallback with a timeout when present', () => {
    const win = { requestIdleCallback: vi.fn((fn) => fn()), setTimeout: vi.fn() }
    const fn = vi.fn()
    afterIdle(fn, win)
    expect(win.requestIdleCallback).toHaveBeenCalledWith(expect.any(Function), { timeout: 2000 })
    expect(fn).toHaveBeenCalledTimes(1)
    expect(win.setTimeout).not.toHaveBeenCalled()
  })

  it('falls back to setTimeout without requestIdleCallback', () => {
    const win = { setTimeout: vi.fn((fn) => fn()) }
    const fn = vi.fn()
    afterIdle(fn, win)
    expect(fn).toHaveBeenCalledTimes(1)
  })
})

describe('whenRenderSettled', () => {
  it('waits for the map idle event, then the idle callback', async () => {
    let fire
    const map = { once: vi.fn((ev, fn) => { fire = fn }) }
    const win = { requestIdleCallback: (fn) => fn(), setTimeout: () => 1, clearTimeout: vi.fn() }
    let done = false
    const p = whenRenderSettled(map, win, 5000).then(() => { done = true })
    await Promise.resolve()
    expect(done).toBe(false)
    fire()
    await p
    expect(map.once).toHaveBeenCalledWith('idle', expect.any(Function))
    expect(done).toBe(true)
  })

  it('gives up waiting for idle after the cap', async () => {
    const map = { once: vi.fn() }
    const win = { requestIdleCallback: (fn) => fn(), setTimeout: (fn, ms) => { if (ms === 5000) fn(); return 1 }, clearTimeout: vi.fn() }
    await expect(whenRenderSettled(map, win, 5000)).resolves.toBeUndefined()
  })

  it('still resolves for a map with no event API', async () => {
    const win = { requestIdleCallback: (fn) => fn(), setTimeout: (fn) => { fn(); return 1 }, clearTimeout: vi.fn() }
    await expect(whenRenderSettled({}, win, 5000)).resolves.toBeUndefined()
  })
})
