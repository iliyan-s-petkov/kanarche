// A cursor over the server's frames and a way to turn one back into the hex body
// the map already draws. Nothing here fetches or paints; the island owns both.
import contract from './contract.json'
import { SPEEDS } from './timelapse.js'

// Sourced from contract.json, generated from snapshot.FrameSpecs.
export const SPANS = contract.spans.map((s) => s.span)

// A day passes in about eight seconds at full speed.
export const FRAME_MS = 320

// Slower is a LONGER gap between frames. Dividing the other way round would
// make the slow setting the fast one.
export function frameDelay(speed) {
  return SPEEDS.includes(speed) ? FRAME_MS / speed : FRAME_MS
}

export function knownSpan(name) {
  return SPANS.includes(name)
}

// spanFor follows the map's window, falling back to the day for the windows no
// animation is published for — a 400 on the first press of play is not a feature.
export function spanFor(window) {
  return knownSpan(window) ? window : SPANS[0]
}

// resolutionKm is the wanted cell size; the server snaps it onto a published
// replay tier, which is a subset of the hex tiers (see hexes.js). Omitted, a
// caller gets today's URL unchanged, so existing callers and cached URLs are
// unaffected.
export function timelapseURL(metric, span, resolutionKm) {
  const params = new URLSearchParams({ metric, span: spanFor(span) })
  if (typeof resolutionKm === 'number' && Number.isFinite(resolutionKm)) {
    params.set('resolution_km', String(round(resolutionKm, 4)))
  }
  return `/api/v1/timelapse?${params}`
}

function round(v, places) {
  const f = 10 ** places
  return Math.round(v * f) / f
}

// The one place the cell list and a frame's numbers are paired. No `n`: a frame
// carries no sensor count, and a made-up one would be a popup saying so.
export function frameBody(body, i) {
  const frame = body?.frames?.[i]
  if (!frame) return { resolution_km: body?.resolution_km, hexes: [] }
  const cells = body.cells ?? []
  return {
    resolution_km: body.resolution_km,
    hexes: cells.map(([lon, lat], j) => ({
      lon,
      lat,
      values: { [body.metric]: frame.v?.[j] ?? null },
      carried: frame.c?.[j] === true,
    })),
  }
}

export function frameCount(body) {
  return body?.frames?.length ?? 0
}

// Null rather than an invalid Date: this labels the scrubber, and "Invalid Date"
// over the map is worse than no label.
export function frameTime(body, i) {
  const t = body?.frames?.[i]?.t
  if (!t) return null
  const d = new Date(t)
  return Number.isNaN(d.getTime()) ? null : d
}

// The playhead. No timer of its own — the island owns the interval — and it
// wraps, because an animation is watched round.
export function cursor(count) {
  return { i: 0, count, playing: false }
}

export function step(c) {
  if (c.count <= 0) return 0
  c.i = (c.i + 1) % c.count
  return c.i
}

export function seek(c, i) {
  if (c.count <= 0) return 0
  c.i = Math.min(Math.max(0, Math.trunc(i) || 0), c.count - 1)
  return c.i
}

// A cell that goes silent for an hour keeps its no-data colour and drops its
// digit, so an animation of a network with ~6% intermittent cells reads as
// numbers blinking on and off at random rather than as gaps. fillForward holds
// each cell at its last reading instead, and flags the held hours in a parallel
// `c` array so they can be drawn muted — a held number is not a measured one.
//
// Forward only: a reading can be held over, but an hour before the cell first
// reported is an hour nobody measured, and backfilling it would invent one.
export function fillForward(body) {
  const frames = body?.frames ?? []
  const last = []
  return {
    ...body,
    frames: frames.map((f) => {
      const vs = f?.v ?? []
      const v = []
      const c = []
      vs.forEach((value, j) => {
        const held = value === null || value === undefined
        if (!held) last[j] = value
        const carried = held && last[j] !== undefined
        v.push(carried ? last[j] : (held ? null : value))
        c.push(carried)
      })
      return { ...f, v, c }
    }),
  }
}

// A frame is thin below a quarter of the best hour in the same body. Relative,
// not absolute: the networks differ by two orders of magnitude in size, so an
// absolute floor would caption every hour of a small-but-complete network.
export const THIN_COVERAGE = 0.25

// Cells per frame that actually carry a reading. Not a sensor count — the
// payload carries none — so this measures breadth of the map, not depth.
export function frameCoverage(body) {
  return (body?.frames ?? []).map(
    (f) => (f?.v ?? []).reduce((n, v) => n + (v === null || v === undefined ? 0 : 1), 0),
  )
}

// Whether this body is worth animating at all. A metric with no history plays
// as a blank country under a running clock, which reads as clean air.
export function hasHistory(body) {
  return frameCoverage(body).some((n) => n > 0)
}

// The frames to caption as partial, judged against the body's own best hour.
// A body with no history yields nothing here — its best hour is 0, so no frame
// is under a quarter of it — and hasHistory speaks for that body instead.
export function thinFrames(body) {
  const cov = frameCoverage(body)
  const floor = Math.max(0, ...cov) * THIN_COVERAGE
  const thin = new Set()
  cov.forEach((n, i) => {
    if (n < floor) thin.add(i)
  })
  return thin
}
