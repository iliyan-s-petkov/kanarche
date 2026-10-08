// Stands in for installTimelapse until the first press of play, so the replay engine
// stays out of the map chunk. Same shape as the real handle: { reset }.
import { lazy } from './lazy.js'

const loadIsland = lazy(() => import('./timelapse-island.js'))

export function installTimelapseLazy(map, state, cfg, chrome, load = loadIsland) {
  const ui = chrome.player
  if (!ui) return null
  let real = null

  ui.ontoggle(async () => {
    if (real) return
    let mod
    try {
      mod = await load()
    } catch (err) {
      console.error('replay failed to load:', err)
      return
    }
    // A second press during the load lands here too; only the first installs.
    if (real) return
    real = mod.installTimelapse(map, state, cfg, chrome)
    // The real handler registered after this press began, so replay the press for it.
    ui.button.click()
  })

  return {
    // Nothing is held before the first press, so there is nothing to reset.
    reset: async () => { await real?.reset() },
  }
}
