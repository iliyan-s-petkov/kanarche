// The bottom panel's height when dragged from its top edge (lib/sidedock.svelte.js): bounds, keys and the saved choice.
import { safeStorage } from './storage.js'

export const HEIGHT_KEY = 'kanarche:panel-height'
// The same floor, cap and share as the .map-dock rule in app.css; ABOVE is the map always left over the panel.
const FLOOR_REM = 14
const CAP_REM = 24
const ABOVE_REM = 8
const SHARE = 0.45
const STEP = 16

export function heightBounds(mapHeight, rem) {
  const min = Math.round(FLOOR_REM * rem)
  return { min, max: Math.max(min, Math.round(mapHeight - ABOVE_REM * rem)) }
}

export function defaultHeight(mapHeight, rem) {
  return Math.round(Math.min(CAP_REM * rem, Math.max(SHARE * mapHeight, FLOOR_REM * rem)))
}

export function clampHeight(h, { min, max }) {
  return Math.min(max, Math.max(min, Math.round(h)))
}

// The height a key on the handle asks for, or null for a key the handle does not take.
export function keyHeight(key, h, bounds) {
  const next = { ArrowUp: h + STEP, ArrowDown: h - STEP, Home: bounds.min, End: bounds.max }[key]
  return next === undefined ? null : clampHeight(next, bounds)
}

export function readHeight(storage = safeStorage()) {
  try {
    const n = Number.parseInt(storage?.getItem(HEIGHT_KEY) ?? '', 10)
    return Number.isFinite(n) && n > 0 ? n : null
  } catch {
    return null
  }
}

export function writeHeight(h, storage = safeStorage()) {
  try {
    storage?.setItem(HEIGHT_KEY, String(Math.round(h)))
  } catch {
    /* private mode, or a full quota: the height still holds this visit */
  }
}

export function clearHeight(storage = safeStorage()) {
  try {
    storage?.removeItem(HEIGHT_KEY)
  } catch {
    /* nothing saved is the default anyway */
  }
}
