// The keyboard list of sensors in the current map view. Pure: no DOM, no map.
import { featureCentre } from './geometry.js'
import { number } from './areacards.js'
import { bandName } from './bandname.js'

// readingText: "12.3 µg/m³ · good". The band name is dropped for a metric with no scale.
export function readingText({ value, unit, band, lang }) {
  const figure = `${number(value, lang)} ${unit}`.trim()
  return band ? `${figure} · ${band}` : figure
}

// describeReading: the reading text for one value of the metric on the map. Shared by
// the list, the hex readout and (through bandName) the panel, so the three agree.
export function describeReading(value, { bands, names, unit, lang }) {
  return readingText({ value, unit, band: bandName(value, bands, names), lang })
}

// buildNearby(features, opts) -> { items, total, rest }
// features: the hex layer's painted features, so the list agrees with what the map shows.
// Only a cell that names one sensor and carries a reading can be listed; a bin of
// several has no single sensor to open. Nearest to the view centre first.
export function buildNearby(features, { centre, bounds, limit, nameOf, readingOf }) {
  const [west, south, east, north] = bounds
  // Longitude degrees shrink with latitude (same equirectangular rule as nearest.js).
  const k = Math.cos((centre[1] * Math.PI) / 180)
  const byId = new Map()
  for (const f of features ?? []) {
    const id = f.properties?.sensorId
    const value = f.properties?.value
    if (id === undefined || id === null || typeof value !== 'number') continue
    const at = featureCentre(f.geometry)
    if (!at) continue
    const [lon, lat] = at
    if (lon < west || lon > east || lat < south || lat > north) continue
    const dx = (lon - centre[0]) * k
    const dy = lat - centre[1]
    const distance = dx * dx + dy * dy
    const seen = byId.get(id)
    if (seen && seen.distance <= distance) continue
    byId.set(id, { id, featureId: f.id, value, lon, lat, distance })
  }
  const sorted = [...byId.values()].sort((a, b) => a.distance - b.distance || a.id - b.id)
  const items = sorted.slice(0, limit).map((s) => ({
    id: s.id, featureId: s.featureId, lon: s.lon, lat: s.lat,
    name: nameOf(s.id), reading: readingOf(s.value),
  }))
  return { items, total: sorted.length, rest: sorted.length - items.length }
}
