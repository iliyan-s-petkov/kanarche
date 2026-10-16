// The refresh announcement: said only when the viewed area's or sensor's air quality band changes.
import { bandName } from './bandname.js'
import { bandsFor } from './mappaint.js'
import { stationMembers, readingAt } from './stations.js'
import { stationValues, median } from './areastats.js'
import { getSensors, getSensorArea, getScales } from './sensors.svelte.js'

// viewedCategory -> {kind, id, band} for the open sensor, else the area page's median station, else null.
export function viewedCategory({ body, slug, area, sensorId, metric, scales, names }) {
  const bands = bandsFor(scales, metric)
  if (sensorId !== null && sensorId !== undefined) {
    const station = stationMembers(body, sensorId)
    if (!station) return null
    const band = bandName(readingAt(body, station.indices, metric).value, bands, names)
    return band ? { kind: 'sensor', id: Number(sensorId), band } : null
  }
  if (!slug || area !== slug) return null
  const values = stationValues(body, metric).map((v) => v.value)
  if (values.length === 0) return null
  const band = bandName(median(values), bands, names)
  return band ? { kind: 'area', id: slug, band } : null
}

// Registers the map's view with the freshness store. Returns the unregister.
export function watchViewedCategory({ fresh, vs, cfg }) {
  return fresh.watch(() => viewedCategory({
    body: getSensors(),
    slug: cfg.slug,
    area: getSensorArea(),
    sensorId: vs.sensorId,
    metric: vs.metric,
    scales: getScales(),
    names: cfg.bandNames ?? {},
  }))
}

// categoryChange ->the after category plus `was`, only for the same subject in a different band.
export function categoryChange(before, after) {
  if (!before || !after || before.kind !== after.kind || before.id !== after.id) return null
  if (before.band === after.band) return null
  return { ...after, was: before.band }
}

// The status line: the change sentence for the refresh that caused it, else the plain line.
export function refreshStatus(fresh, t, plain) {
  if (!fresh.changed || fresh.busy || fresh.failed) return plain
  return changeText(t, fresh.changed) || plain
}

// changeText(t, change): t.changed is the template, t.sensor the panel title word, t.subject the area.
export function changeText(t, change) {
  const subject = change.kind === 'sensor' ? `${t.sensor} ${change.id}`.trim() : t.subject
  if (!t.changed || !subject) return ''
  return t.changed.replaceAll('{subject}', subject).replaceAll('{band}', change.band).replaceAll('{was}', change.was)
}
