// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { installTimelapseLazy } from '../timelapse-lazy.js'

function fakeChrome() {
  const toggles = []
  const button = document.createElement('button')
  button.addEventListener('click', () => { for (const fn of [...toggles]) fn() })
  return { player: { button, ontoggle: (fn) => toggles.push(fn) } }
}

describe('installTimelapseLazy', () => {
  it('returns null when the page has no player', () => {
    expect(installTimelapseLazy({}, {}, {}, {}, vi.fn())).toBeNull()
  })

  it('does not load the island before a press', () => {
    const load = vi.fn()
    installTimelapseLazy({}, {}, {}, fakeChrome(), load)
    expect(load).not.toHaveBeenCalled()
  })

  it('loads on the first press, installs once, and replays the press for the real handler', async () => {
    const chrome = fakeChrome()
    const real = { reset: vi.fn(async () => {}) }
    const installTimelapse = vi.fn((m, s, cfg, c) => {
      c.player.ontoggle(realPress)
      return real
    })
    const realPress = vi.fn()
    const load = vi.fn(async () => ({ installTimelapse }))
    installTimelapseLazy({}, {}, {}, chrome, load)

    chrome.player.button.click()
    chrome.player.button.click()
    await vi.waitFor(() => expect(realPress).toHaveBeenCalledTimes(1))
    expect(load).toHaveBeenCalledTimes(2)
    expect(installTimelapse).toHaveBeenCalledTimes(1)

    chrome.player.button.click()
    expect(realPress).toHaveBeenCalledTimes(2)
    expect(installTimelapse).toHaveBeenCalledTimes(1)
  })

  it('reset is a no-op before load and forwards after', async () => {
    const chrome = fakeChrome()
    const real = { reset: vi.fn(async () => {}) }
    const handle = installTimelapseLazy({}, {}, {}, chrome, async () => ({ installTimelapse: () => real }))
    await handle.reset()
    expect(real.reset).not.toHaveBeenCalled()
    chrome.player.button.click()
    await vi.waitFor(() => expect(handle.reset).toBeTypeOf('function'))
    await new Promise((r) => setTimeout(r, 0))
    await handle.reset()
    expect(real.reset).toHaveBeenCalledTimes(1)
  })

  it('survives a failed load and retries on the next press', async () => {
    const chrome = fakeChrome()
    const err = vi.spyOn(console, 'error').mockImplementation(() => {})
    const real = { reset: vi.fn() }
    const load = vi.fn()
      .mockRejectedValueOnce(new Error('404'))
      .mockResolvedValueOnce({ installTimelapse: () => real })
    installTimelapseLazy({}, {}, {}, chrome, load)
    chrome.player.button.click()
    await vi.waitFor(() => expect(err).toHaveBeenCalled())
    chrome.player.button.click()
    await vi.waitFor(() => expect(load).toHaveBeenCalledTimes(2))
    err.mockRestore()
  })
})
