// The refresh announcement: said only when the viewed area's or sensor's air quality band changes.
import { bandName } from './bandname.js'
import { bandsFor } from './mappaint.js'
import { stationMembers, readingAt } from './stations.js'
import { stationValues, median } from './areastats.js'
import { getSensors, getSensorArea, getScales } from './sensors.svelte.js'

// viewedCategory -> {kind, id, metric, band} for the open sensor, else the area page's median station, else null.
export function viewedCategory({ body, slug, area, sensorId, metric, scales, names }) {
  const bands = bandsFor(scales, metric)
  if (sensorId !== null && sensorId !== undefined) {
    const station = stationMembers(body, sensorId)
    if (!station) return null
    const band = bandName(readingAt(body, station.indices, metric).value, bands, names)
    return band ? { kind: 'sensor', id: Number(sensorId), metric, band } : null
  }
  if (!slug || area !== slug) return null
  const values = stationValues(body, metric).map((v) => v.value)
  if (values.length === 0) return null
  const band = bandName(median(values), bands, names)
  return band ? { kind: 'area', id: slug, metric, band } : null
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

const sameSubject = (a, b) => !!a && !!b && a.kind === b.kind && a.id === b.id && a.metric === b.metric

// categoryChange -> the after category plus `was`, only for the same subject and metric in a different band.
export function categoryChange(before, after) {
  if (!sameSubject(before, after) || before.band === after.band) return null
  return { ...after, was: before.band }
}

// The live region text: the change sentence while it still describes the view, or the failure. Never the plain time.
export function announceText(fresh, t) {
  if (fresh.busy) return ''
  if (fresh.failed) return t.failed || ''
  const change = fresh.changed
  const now = fresh.current?.() ?? null
  if (!change || !sameSubject(change, now) || now.band !== change.band) return ''
  return changeText(t, change)
}

// changeText(t, change): t.changed is the template, t.sensor the panel title word, t.subject the area.
export function changeText(t, change) {
  const subject = change.kind === 'sensor' ? `${t.sensor} ${change.id}`.trim() : t.subject
  if (!t.changed || !subject) return ''
  return t.changed.replaceAll('{subject}', subject).replaceAll('{band}', change.band).replaceAll('{was}', change.was)
}
