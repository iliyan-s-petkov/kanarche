import { describe, it, expect, vi } from 'vitest'
import { searchAddress, GeocodeError } from '../geocode.js'

const ok = (body) => ({ ok: true, status: 200, json: async () => body })
const status = (code) => ({ ok: false, status: code, json: async () => ({}) })

describe('searchAddress', () => {
  it('asks the same-origin proxy with the query and the page language', async () => {
    const fetchFn = vi.fn().mockResolvedValue(ok([]))
    await searchAddress('ул. Витоша 1', 'bg', fetchFn)
    expect(fetchFn).toHaveBeenCalledTimes(1)
    const [url] = fetchFn.mock.calls[0]
    expect(url.startsWith('/api/v1/geocode?')).toBe(true)
    const p = new URL(url, 'http://x').searchParams
    expect(p.get('q')).toBe('ул. Витоша 1')
    expect(p.get('lang')).toBe('bg')
  })

  it('returns the rows as given', async () => {
    const rows = [{ label: 'a', lat: 1, lon: 2, bbox: [0, 0, 0, 0] }]
    expect(await searchAddress('abc', 'en', vi.fn().mockResolvedValue(ok(rows)))).toEqual(rows)
  })

  it('calls a 503 and a 429 busy', async () => {
    for (const code of [503, 429]) {
      const err = await searchAddress('abc', 'en', vi.fn().mockResolvedValue(status(code))).catch((e) => e)
      expect(err).toBeInstanceOf(GeocodeError)
      expect(err.kind).toBe('busy')
    }
  })

  it('calls any other failure, and a network error, failed', async () => {
    for (const f of [vi.fn().mockResolvedValue(status(502)), vi.fn().mockRejectedValue(new TypeError('net'))]) {
      const err = await searchAddress('abc', 'en', f).catch((e) => e)
      expect(err).toBeInstanceOf(GeocodeError)
      expect(err.kind).toBe('failed')
    }
  })

  it('treats a body that is not an array as failed', async () => {
    const err = await searchAddress('abc', 'en', vi.fn().mockResolvedValue(ok({ nope: 1 }))).catch((e) => e)
    expect(err.kind).toBe('failed')
  })
})
