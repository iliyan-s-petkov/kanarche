// The map-driving seam for the pollen layer; pollen.js holds the pure builders.
import { getJSON } from './api.js'
import {
  HEX_LAYER_ID, HEX_OUTLINE_LAYER_ID, HEX_EXTRUSION_LAYER_ID, HEX_POINT_LAYER_ID, HEX_LABEL_LAYER_ID,
} from './mapids.js'
import {
  POLLEN_FILL_LAYER_ID, POLLEN_LINE_LAYER_ID, POLLEN_SOURCE_ID, pollenColours, pollenFeatures, pollenFillPaint,
} from './pollen.js'

// The grid paints air readings; under the pollen fill it would read as a second, unrelated colour key.
export const POLLEN_HIDES = [HEX_LAYER_ID, HEX_OUTLINE_LAYER_ID, HEX_EXTRUSION_LAYER_ID, HEX_POINT_LAYER_ID, HEX_LABEL_LAYER_ID]
const POLLEN_LAYERS = [POLLEN_FILL_LAYER_ID, POLLEN_LINE_LAYER_ID]

// The theme's computed custom property, so the map follows the area page and the dark theme.
export function readThemeVar(name) {
  return getComputedStyle(document.documentElement).getPropertyValue(name)
}

// Same shape as setSea: fetch once, show or hide, return the state reached.
export async function setPollen(map, cfg, chrome, st, on, fetchJSON = getJSON, read = readThemeVar) {
  if (!on) {
    st.on = false
    show(map, false)
    chrome.showPollen(false)
    return false
  }
  if (st.loading) return st.on
  if (!st.body) {
    st.loading = true
    try {
      const [boundaries, pollen] = await Promise.all([fetchJSON('/api/v1/boundaries'), fetchJSON('/api/v1/pollen')])
      st.body = pollenFeatures(boundaries, pollen)
    } catch {
      show(map, false)
      chrome.showPollen(false)
      return false
    } finally {
      st.loading = false
    }
  }
  map.getSource?.(POLLEN_SOURCE_ID)?.setData(st.body)
  // Read at each toggle: the theme can change between one and the next.
  if (map.getLayer?.(POLLEN_FILL_LAYER_ID)) {
    const paint = pollenFillPaint(pollenColours(read, cfg.noDataColour), cfg.noDataColour)
    for (const [k, v] of Object.entries(paint)) map.setPaintProperty(POLLEN_FILL_LAYER_ID, k, v)
  }
  show(map, true)
  st.on = true
  chrome.showPollen(true)
  return true
}

// Pollen on hides the grid and shows the provinces; off restores the grid.
function show(map, on) {
  for (const id of POLLEN_LAYERS) setVisibility(map, id, on ? 'visible' : 'none')
  for (const id of POLLEN_HIDES) setVisibility(map, id, on ? 'none' : 'visible')
}

function setVisibility(map, id, visibility) {
  if (map.getLayer?.(id)) map.setLayoutProperty(id, 'visibility', visibility)
}
