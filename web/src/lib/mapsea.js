// The map-driving seam for the bathing-water layer; sea.js holds the pure builders.
import { getJSON } from './api.js'
import { MARKER_PIXEL_RATIO } from './mappaint.js'
import { SQUARE_HALF_PX } from './markericon.js'
import { SEA_IMAGE_ID, SEA_LAYER_ID, SEA_SOURCE_ID, seaFeatures } from './sea.js'

// Same on-screen size as the station diamonds: 5px at zoom 5 to 9px at zoom 12.
export function seaLayout() {
  const unit = SQUARE_HALF_PX / MARKER_PIXEL_RATIO
  return {
    'icon-image': SEA_IMAGE_ID,
    'icon-size': ['interpolate', ['linear'], ['zoom'], 5, 5 / unit, 12, 9 / unit],
    'icon-allow-overlap': true,
    'icon-ignore-placement': true,
  }
}

export function seaPaint(cfg) {
  return {
    'icon-color': ['get', 'colour'],
    'icon-halo-color': cfg.markerStrokeColour,
    'icon-halo-width': 1,
  }
}

// Same shape as setWind and setBoundaries: fetch once, show or hide, return the state reached.
export async function setSea(map, cfg, chrome, st, on, fetchJSON = getJSON) {
  if (!on) {
    st.on = false
    setVisibility(map, 'none')
    chrome.showSea(false)
    st.closePanel?.()
    return false
  }
  if (st.loading) return st.on
  if (!st.body) {
    st.loading = true
    try {
      st.body = await fetchJSON('/api/v1/sea/sites')
    } catch {
      setVisibility(map, 'none')
      chrome.showSea(false)
      return false
    } finally {
      st.loading = false
    }
  }
  // The legend's dataset link follows the supplement the API sent, if any.
  chrome.setSeaSupplement?.(st.body?.supplement?.url ?? '')
  map.getSource?.(SEA_SOURCE_ID)?.setData(seaFeatures(st.body, cfg.seaColours))
  setVisibility(map, 'visible')
  st.on = true
  chrome.showSea(true)
  return true
}

function setVisibility(map, visibility) {
  if (map.getLayer?.(SEA_LAYER_ID)) map.setLayoutProperty(SEA_LAYER_ID, 'visibility', visibility)
}
