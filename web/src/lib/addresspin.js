// The searched-address pin and the camera move that goes with it.
import { Marker } from 'maplibre-gl'
import { BOUNDARY_FIT_PADDING } from './mapboundaries.js'

const POINT_ZOOM = 16
const BBOX_MAX_ZOOM = 17

// bbox is [west, south, east, north]; the server sends zeros when upstream gave none.
export function cameraFor(row) {
  const [w, s, e, n] = row.bbox || [0, 0, 0, 0]
  if (w || s || e || n) {
    return { bounds: [[w, s], [e, n]], options: { padding: BOUNDARY_FIT_PADDING, maxZoom: BBOX_MAX_ZOOM } }
  }
  return { center: [row.lon, row.lat], zoom: POINT_ZOOM }
}

// One pin at a time; show() replaces, clear() and the returned stop remove it.
export function createAddressPin(map, label) {
  let marker = null
  const clear = () => {
    marker?.remove()
    marker = null
  }
  const show = (row) => {
    clear()
    const el = document.createElement('div')
    el.className = 'address-pin'
    el.setAttribute('role', 'img')
    el.setAttribute('aria-label', label || row.label)
    marker = new Marker({ element: el }).setLngLat([row.lon, row.lat]).addTo(map)
    const cam = cameraFor(row)
    if (cam.bounds) map.fitBounds(cam.bounds, cam.options)
    else map.easeTo({ center: cam.center, zoom: cam.zoom })
  }
  return { show, clear }
}
