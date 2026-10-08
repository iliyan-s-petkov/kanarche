import { describe, it, expect, vi } from 'vitest'
import { createSeaPanelLazy } from '../seapanel-lazy.js'

function fakeModule() {
  const panel = { open: vi.fn(async () => {}), close: vi.fn(), id: null }
  return { panel, mod: { createSeaPanel: vi.fn(() => panel) } }
}

describe('createSeaPanelLazy', () => {
  it('loads nothing until opened, and close before then is harmless', () => {
    const load = vi.fn()
    const p = createSeaPanelLazy({}, {}, load)
    p.close()
    expect(p.id).toBeNull()
    expect(load).not.toHaveBeenCalled()
  })

  it('creates the real panel once and opens each site on it', async () => {
    const { panel, mod } = fakeModule()
    const p = createSeaPanelLazy({}, {}, async () => mod)
    await p.open('a')
    await p.open('b')
    expect(mod.createSeaPanel).toHaveBeenCalledTimes(1)
    expect(panel.open.mock.calls).toEqual([['a'], ['b']])
  })

  it('forwards close and id to the real panel', async () => {
    const { panel, mod } = fakeModule()
    const p = createSeaPanelLazy({}, {}, async () => mod)
    await p.open('a')
    panel.id = 'a'
    expect(p.id).toBe('a')
    p.close()
    expect(panel.close).toHaveBeenCalledTimes(1)
  })

  it('drops an open that a close overtook while loading', async () => {
    const { panel, mod } = fakeModule()
    let release
    const p = createSeaPanelLazy({}, {}, () => new Promise((r) => { release = () => r(mod) }))
    const pending = p.open('a')
    p.close()
    release()
    await pending
    expect(panel.open).not.toHaveBeenCalled()
  })

  it('logs and gives up when the chunk fails to load', async () => {
    const err = vi.spyOn(console, 'error').mockImplementation(() => {})
    const p = createSeaPanelLazy({}, {}, async () => { throw new Error('404') })
    await expect(p.open('a')).resolves.toBeUndefined()
    expect(err).toHaveBeenCalled()
    err.mockRestore()
  })
})
