import { describe, it, expect, vi } from 'vitest'
import { resolveLoader, runIsland, deferIsland } from '../main.js'

// Importing main.js at all is the first assertion: it must not throw when
// `document` does not exist (this test runs under Vitest's node environment,
// deliberately with no jsdom). That is exactly "a page renders/imports fine
// with no island container" — the loader's guard around init() is what makes
// this import even possible.

// resolveLoader is the pure lookup behind "leave the server-rendered fallback
// for anything this bundle does not recognise".
describe('resolveLoader', () => {
  it('resolves the registered "map" island to a loader function', () => {
    expect(typeof resolveLoader('map')).toBe('function')
  })
  it('resolves the registered "chart" island to a loader function', () => {
    expect(typeof resolveLoader('chart')).toBe('function')
  })
  it('resolves an unregistered island name to null, not undefined and not a throw', () => {
    expect(resolveLoader('')).toBeNull()
    expect(resolveLoader('nonsense')).toBeNull()
  })
})

// runIsland is the per-island failure boundary. A plain {dataset} object
// stands in for the DOM element — runIsland only ever reads el.dataset.island
// for its log line, so this stays pure-logic, not a DOM harness.
describe('runIsland', () => {
  it('mounts the loaded module against the element and reports success', async () => {
    const el = { dataset: { island: 'map' } }
    const mount = vi.fn()
    const load = vi.fn().mockResolvedValue({ mount })

    const ok = await runIsland(el, load)

    expect(ok).toBe(true)
    expect(mount).toHaveBeenCalledWith(el)
  })

  it('a failing loader is swallowed, logged, and reported as failure — it must not throw', async () => {
    const el = { dataset: { island: 'map' } }
    const load = vi.fn().mockRejectedValue(new Error('bundle failed to load'))
    const log = vi.fn()

    const ok = await runIsland(el, load, log)

    expect(ok).toBe(false)
    expect(log).toHaveBeenCalledWith('island failed:', 'map', expect.any(Error))
  })

  it('a failing mount() is swallowed the same way as a failing load()', async () => {
    const el = { dataset: { island: 'map' } }
    const load = vi.fn().mockResolvedValue({ mount: () => { throw new Error('mount blew up') } })
    const log = vi.fn()

    const ok = await runIsland(el, load, log)

    expect(ok).toBe(false)
    expect(log).toHaveBeenCalledOnce()
  })

  it('one island failing does not affect a second, independent call', async () => {
    const failing = runIsland({ dataset: { island: 'a' } }, vi.fn().mockRejectedValue(new Error('boom')), vi.fn())
    const succeeding = runIsland({ dataset: { island: 'b' } }, vi.fn().mockResolvedValue({ mount: vi.fn() }))

    const [a, b] = await Promise.all([failing, succeeding])

    expect(a).toBe(false)
    expect(b).toBe(true)
  })
})

// deferIsland decides which islands wait for the map to finish mounting.
describe('deferIsland', () => {
  it('keeps the map and the controls on the map at load', () => {
    for (const name of ['map', 'switcher', 'finder', 'freshness', 'refresh', 'theme', 'chart', 'sensorbar', 'panel']) {
      expect(deferIsland(name, '')).toBe(false)
    }
  })
  it('defers the islands below the map', () => {
    for (const name of ['readouts', 'table', 'visitors', 'copycode', 'clearsettings']) {
      expect(deferIsland(name, '')).toBe(true)
    }
  })
  it('mounts the readouts at load when the URL names a sensor', () => {
    expect(deferIsland('readouts', '#metric=P2&sensor=42')).toBe(false)
    expect(deferIsland('readouts', '')).toBe(true)
    expect(deferIsland('table', '#sensor=42')).toBe(true)
  })
  it('does not mistake a longer key for a sensor', () => {
    expect(deferIsland('readouts', '#nosensor=1')).toBe(true)
  })
})
