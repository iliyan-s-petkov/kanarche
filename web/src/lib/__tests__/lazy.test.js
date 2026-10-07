import { describe, it, expect, vi } from 'vitest'
import { lazy } from '../lazy.js'

describe('lazy', () => {
  it('does not call the loader until first use', () => {
    const load = vi.fn(async () => ({ ok: 1 }))
    lazy(load)
    expect(load).not.toHaveBeenCalled()
  })

  it('loads once and hands every caller the same promise', async () => {
    const load = vi.fn(async () => ({ ok: 1 }))
    const get = lazy(load)
    const a = get()
    const b = get()
    expect(a).toBe(b)
    expect(await a).toEqual({ ok: 1 })
    expect(load).toHaveBeenCalledTimes(1)
  })

  it('forgets a rejected load so the next call retries', async () => {
    const load = vi.fn()
      .mockRejectedValueOnce(new Error('chunk 404'))
      .mockResolvedValueOnce({ ok: 2 })
    const get = lazy(load)
    await expect(get()).rejects.toThrow('chunk 404')
    expect(await get()).toEqual({ ok: 2 })
    expect(load).toHaveBeenCalledTimes(2)
  })

  it('does not cache a loader that throws synchronously', async () => {
    const load = vi.fn()
      .mockImplementationOnce(() => { throw new Error('sync') })
      .mockResolvedValueOnce({ ok: 3 })
    const get = lazy(load)
    await expect(get()).rejects.toThrow('sync')
    expect(await get()).toEqual({ ok: 3 })
  })
})
