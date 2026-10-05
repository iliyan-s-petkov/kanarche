import { filterByStatus } from './sensorfilter.svelte.js'

// Whether stations flagged faulty for the current layer are drawn. Module-level
// $state with a listener set, like sourcefilter.svelte.js; hidden by default.
// The layers menu persists the checkbox and re-applies it at build time.
let show = $state(false)

const listeners = new Set()

export function getShowFaulty() {
  return show
}

export function setShowFaulty(on) {
  if (on === show) return
  show = on
  for (const fn of listeners) fn(show)
}

export function onShowFaultyChange(fn) {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

// TEST-ONLY reset seam.
export function resetFaultyFilterForTests() {
  show = false
  listeners.clear()
}

export function filterFaulty(features, showFaulty) {
  return showFaulty ? features : features.filter((f) => !f.properties.faulty)
}

// Faulty stations are shown whenever the toggle is on, whatever the with-data
// status says: the status filter does not speak for them, value or not.
export function filterSensorFeatures(features, status, showFaulty) {
  const healthy = filterByStatus(features.filter((f) => !f.properties.faulty), status)
  return showFaulty ? healthy.concat(features.filter((f) => f.properties.faulty)) : healthy
}
