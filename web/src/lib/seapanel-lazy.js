// The bathing-site card loads on its first open, keeping its component out of the map chunk.
// Same handle as createSeaPanel: { open, close, id }.
import { lazy } from './lazy.js'

const loadPanel = lazy(() => import('./seapanel.svelte.js'))

export function createSeaPanelLazy(frame, cfg, load = loadPanel) {
  let real = null
  // Bumped by every open and close, so an open whose load finishes after a later close is dropped.
  let request = 0

  async function open(id) {
    const mine = ++request
    let mod
    try {
      mod = await load()
    } catch (err) {
      console.error('sea panel failed to load:', err)
      return
    }
    if (mine !== request) return
    real ??= mod.createSeaPanel(frame, cfg)
    await real.open(id)
  }

  function close() {
    request++
    real?.close()
  }

  return {
    open,
    close,
    get id() { return real ? real.id : null },
  }
}
