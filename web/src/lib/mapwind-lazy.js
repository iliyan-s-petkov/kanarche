// Stands in for mapwind.js so the forecast field and its labelling load on the first
// wind toggle, not with the map. Same call shapes as the real exports.
import { lazy } from './lazy.js'

const loadWind = lazy(() => import('./mapwind.js'))
// Set once setWind has loaded the module; paintWind needs it synchronously on moveend.
let loaded = null

export async function setWind(map, cfg, chrome, state, on) {
  loaded = await loadWind()
  return loaded.setWind(map, cfg, chrome, state, on)
}

// No held forecast means nothing to refresh, so the module is not fetched for it.
export async function refreshWind(map, cfg, chrome, state) {
  if (!state.body) return false
  loaded = await loadWind()
  return loaded.refreshWind(map, cfg, chrome, state)
}

// Only called while the layer is on, which setWind has already loaded the module for.
export function paintWind(map, state) {
  loaded?.paintWind(map, state)
}
