import { describe, it, expect, vi } from 'vitest'
import { deferFirstOn } from '../deferfirst.js'

function gate() {
  let open
  const ready = vi.fn(() => new Promise((r) => { open = r }))
  return { ready, open: () => open() }
}

describe('deferFirstOn', () => {
  it('does not run the first on until ready resolves', async () => {
    const g = gate()
    const run = vi.fn(async () => true)
    const apply = deferFirstOn(g.ready, run)
    expect(apply(true)).toBe(true)
    await Promise.resolve()
    expect(run).not.toHaveBeenCalled()
    g.open()
    await new Promise((r) => setTimeout(r, 0))
    expect(run).toHaveBeenCalledExactlyOnceWith(true)
  })

  it('applies the last wanted state when ready resolves', async () => {
    const g = gate()
    const run = vi.fn(async () => true)
    const apply = deferFirstOn(g.ready, run)
    apply(true)
    apply(false)
    g.open()
    await new Promise((r) => setTimeout(r, 0))
    expect(run).toHaveBeenCalledExactlyOnceWith(false)
  })

  it('runs an off first call and later calls immediately', () => {
    const g = gate()
    const run = vi.fn(() => false)
    const apply = deferFirstOn(g.ready, run)
    apply(false)
    apply(true)
    expect(run).toHaveBeenCalledTimes(2)
    expect(g.ready).not.toHaveBeenCalled()
  })

  it('warns instead of rejecting when the deferred run throws', async () => {
    const warn = vi.fn()
    const apply = deferFirstOn(() => Promise.resolve(), () => { throw new Error('x') }, warn)
    apply(true)
    await new Promise((r) => setTimeout(r, 0))
    expect(warn).toHaveBeenCalled()
  })
})
