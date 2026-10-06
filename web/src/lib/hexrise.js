// Rise on tilt: the hex cells stay flat top-down and grow into value-height columns as the map pitches.
// The columns live on their own source, filled only while tilted, and the ramp runs on feature-state:
// a data-driven paint change would relayout every tile per frame, and state on the shared hex source
// would rewrite the fill, outline, point and label buckets too.

import { rampPosition } from './ramp.js'
import { HEX_SOURCE_ID, HEX_COLUMN_SOURCE_ID, HEX_EXTRUSION_LAYER_ID } from './mapids.js'

// Pitch in degrees at which the columns reach full height.
export const HEX_RISE_FULL_PITCH = 35
// Ramp quantisation: at most this many feature-state passes over a whole tilt.
export const HEX_RISE_STEPS = 24
// A column at the scale's top stands this many cell widths tall, at every tier.
export const HEX_HEIGHT_PER_CELL = 1.5
// Opaque enough for columns to occlude each other, translucent enough to keep the ground legible.
export const HEX_EXTRUDED_OPACITY = 0.85

// Metres, on the colour ramp's own axis, so the cap is the scale's top value.
export function columnHeight(value, bands, cellKM) {
  const pos = rampPosition(value, bands)
  if (pos === null || !(cellKM > 0)) return 0
  return (pos / 100) * HEX_HEIGHT_PER_CELL * cellKM * 1000
}

// 0 at pitch 0, smoothstep to 1 at HEX_RISE_FULL_PITCH; a plain step under reduced motion.
export function riseFactor(pitch, reducedMotion = false) {
  if (!(pitch > 0)) return 0
  if (reducedMotion) return pitch >= HEX_RISE_FULL_PITCH / 2 ? 1 : 0
  const t = Math.min(1, pitch / HEX_RISE_FULL_PITCH)
  const s = t * t * (3 - 2 * t)
  // Never rounded down to 0 while tilted: 0 is the flat state.
  return Math.max(1 / HEX_RISE_STEPS, Math.round(s * HEX_RISE_STEPS) / HEX_RISE_STEPS)
}

export function hexExtrusionPaint() {
  return {
    'fill-extrusion-color': ['get', 'colour'],
    'fill-extrusion-height': ['*', ['coalesce', ['get', 'height'], 0], ['coalesce', ['feature-state', 'rise'], 0]],
    'fill-extrusion-opacity': 0,
    'fill-extrusion-vertical-gradient': true,
  }
}

const prefersReducedMotion = () =>
  globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false

const collection = (features) => ({ type: 'FeatureCollection', features })

// Mirrors the painted hex cells onto the column source while tilted and drives `rise` from the pitch,
// at most once per animation frame.
export function installHexRise(map, { raf = globalThis.requestAnimationFrame, reducedMotion = prefersReducedMotion } = {}) {
  // The last painted features, unfiltered: playback paints a frame per tick, and the polygon filter is
  // only worth running when the columns are actually up.
  let painted = []
  let cells = []
  let factor = 0
  let queued = false

  const setRise = () => {
    for (const f of cells) map.setFeatureState({ source: HEX_COLUMN_SOURCE_ID, id: f.id }, { rise: factor })
  }
  const fill = (features) => map.getSource(HEX_COLUMN_SOURCE_ID)?.setData(collection(features))

  const syncCells = () => {
    cells = painted.filter((f) => f.geometry?.type === 'Polygon' && f.id != null)
  }

  const apply = () => {
    queued = false
    const next = riseFactor(map.getPitch?.() ?? 0, reducedMotion())
    if (next === factor) return
    const wasFlat = factor === 0
    factor = next
    // Rising: load the columns at the current factor. Flattening: hide them and drop the data.
    if (wasFlat) {
      syncCells()
      fill(cells)
    }
    setRise()
    if (wasFlat || factor === 0) {
      map.setPaintProperty(HEX_EXTRUSION_LAYER_ID, 'fill-extrusion-opacity', factor > 0 ? HEX_EXTRUDED_OPACITY : 0)
    }
    if (factor === 0) fill([])
  }

  map.on('pitch', () => {
    if (queued) return
    queued = true
    raf(apply)
  })

  map.getContainer?.()?.addEventListener?.('airbg:paint', (e) => {
    if (e.detail?.source !== HEX_SOURCE_ID) return
    painted = e.detail.features ?? []
    if (factor === 0) return
    syncCells()
    fill(cells)
    setRise()
  })

  apply()
  return { apply }
}
