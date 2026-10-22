// Animated wind streaks as a 2D canvas over the map; the default wind rendering (#576). ?windfx=arrows opts out.
import { arrowBearing } from '../islands/wind.js'
import contract from './contract.json'

const GRID_PX = 20
// The served vectors sit on the model's 0.25 degree lattice (about 20-28 km apart). Within FULL_KM of one the streaks are full strength; they fade out by REACH_KM.
const FULL_KM = 30
const REACH_KM = 80
const NEAREST = 4
// Per-frame chance a faded particle is culled, so the fringe thins without flickering.
const FRINGE_CULL = 0.06
const KM_PER_DEG = (Math.PI * contract.hex.earth_radius_km) / 180
// Tuning. Speed: screen px per ms per m/s of wind, 5 m/s is about 0.75 px per 60 fps frame (was 0.032, 3.5x faster).
const SPEED_PX_PER_MS = 0.009
// Trail fade per frame; lower than the old 0.1 (2.5x) so the slower streaks keep a visible tail without smearing.
const FADE = 0.04
const LINE_ALPHA = 0.65
const LINE_WIDTH = 1.4
// One particle per this many px2 of viewport (was 1800), so phones get fewer than desktops.
const AREA_PER_PARTICLE = 4500
const MAX_PARTICLES = 500
const MAX_AGE = 90
// Blue ramp by wind speed (m/s, colour), light to deep. Hue stays 213-226: clear of the teal clean-air end and the purple end of the PM scale.
const SPEED_MAX_MS = 12
const RAMP = [
  [0, 'rgb(110,170,240)'],
  [4, 'rgb(60,128,222)'],
  [8, 'rgb(32,90,200)'],
  [12, 'rgb(24,58,170)'],
]
const BUCKETS = 6

export const WIND_RAMP = RAMP
export const WIND_FULL_KM = FULL_KM
export const WIND_REACH_KM = REACH_KM
export const WIND_SPEED_MAX_MS = SPEED_MAX_MS
export const WIND_LINE_ALPHA = LINE_ALPHA
export const WIND_BUCKETS = BUCKETS
export const WIND_AREA_PER_PARTICLE = AREA_PER_PARTICLE
export const WIND_MAX_PARTICLES = MAX_PARTICLES

// windColour interpolates the ramp at a speed; speeds outside 0..SPEED_MAX_MS clamp to the ends.
export function windColour(speedMs) {
  const s = Math.min(Math.max(speedMs, 0), SPEED_MAX_MS)
  let i = 1
  while (i < RAMP.length - 1 && s > RAMP[i][0]) i++
  const [s0, c0] = RAMP[i - 1]
  const [s1, c1] = RAMP[i]
  const t = (s - s0) / (s1 - s0)
  const a = c0.match(/\d+/g).map(Number)
  const b = c1.match(/\d+/g).map(Number)
  return `rgb(${a.map((v, k) => Math.round(v + (b[k] - v) * t)).join(',')})`
}

// speedBucket groups speeds so a frame draws one path per colour, not one per particle.
export function speedBucket(speedMs) {
  const t = Math.min(Math.max(speedMs, 0), SPEED_MAX_MS) / SPEED_MAX_MS
  return Math.min(BUCKETS - 1, Math.floor(t * BUCKETS))
}

// bucketColour is the colour at the middle of a bucket's speed range.
const bucketColour = (i) => windColour(((i + 0.5) / BUCKETS) * SPEED_MAX_MS)

// particleCount scales with viewport area, so a phone is not denser than a desktop.
export function particleCount(width, height) {
  return Math.min(MAX_PARTICLES, Math.max(0, Math.round((width * height) / AREA_PER_PARTICLE)))
}

// stepPx is how far a particle moves in one frame of dtMs at the given wind speed.
export function stepPx(speedMs, dtMs) {
  return speedMs * SPEED_PX_PER_MS * dtMs
}

// windfxMode picks the wind rendering: streaks unless motion is reduced or ?windfx=arrows.
export function windfxMode(search, reducedMotion) {
  const optedOut = new URLSearchParams(search).get('windfx') === 'arrows'
  return optedOut || reducedMotion ? 'arrows' : 'streaks'
}

// toUV turns served vectors into east/north components of where the air goes.
export function toUV(body) {
  if (!body || body.forecast !== true || !Array.isArray(body.vectors)) return []
  return body.vectors.map((v) => {
    const a = (arrowBearing(v.direction_deg) * Math.PI) / 180
    return { lon: v.lon, lat: v.lat, u: v.speed_ms * Math.sin(a), v: v.speed_ms * Math.cos(a) }
  })
}

// idw blends u/v from the NEAREST closest vectors within REACH_KM; the third value is a 0..1 fade that is 1 within FULL_KM of a vector and 0 at REACH_KM. Null past REACH_KM.
// Hot loop (one call per grid node, hundreds per rebuild): no per-call arrays, a latitude reject before the longitude math, and a running top-NEAREST instead of a sort.
export function idw(vectors, lon, lat) {
  const kmLon = KM_PER_DEG * Math.cos((lat * Math.PI) / 180)
  const reach2 = REACH_KM * REACH_KM
  // Sorted ascending by squared distance; earlier vectors win ties, as the stable sort they replace did.
  const bestD = new Float64Array(NEAREST).fill(Infinity)
  const best = new Array(NEAREST)
  let count = 0
  for (const p of vectors) {
    const dy = (p.lat - lat) * KM_PER_DEG
    if (dy > REACH_KM || dy < -REACH_KM) continue
    const dx = (p.lon - lon) * kmLon
    const d2 = dx * dx + dy * dy
    if (d2 >= reach2 || d2 >= bestD[NEAREST - 1]) continue
    let k = Math.min(count, NEAREST - 1)
    while (k > 0 && bestD[k - 1] > d2) {
      bestD[k] = bestD[k - 1]
      best[k] = best[k - 1]
      k--
    }
    bestD[k] = d2
    best[k] = p
    if (count < NEAREST) count++
  }
  if (count === 0) return null
  let su = 0
  let sv = 0
  let sw = 0
  for (let k = 0; k < count; k++) {
    // The +25 keeps a point sitting on a vector from ignoring every other one.
    const w = 1 / (bestD[k] + 25)
    su += best[k].u * w
    sv += best[k].v * w
    sw += w
  }
  const fade = Math.min(1, (REACH_KM - Math.sqrt(bestD[0])) / (REACH_KM - FULL_KM))
  return [su / sw, sv / sw, fade]
}

// buildGrid samples the field at screen nodes; vx/vy are screen-space (y down) m/s.
export function buildGrid(vectors, width, height, unproject, bearingDeg = 0) {
  const cols = Math.ceil(width / GRID_PX) + 1
  const rows = Math.ceil(height / GRID_PX) + 1
  const vx = new Float32Array(cols * rows)
  const vy = new Float32Array(cols * rows)
  const ok = new Uint8Array(cols * rows)
  const fade = new Float32Array(cols * rows)
  const rot = (bearingDeg * Math.PI) / 180
  const cos = Math.cos(rot)
  const sin = Math.sin(rot)
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      const ll = unproject(c * GRID_PX, r * GRID_PX)
      const uv = idw(vectors, ll[0], ll[1])
      if (!uv) continue
      const i = r * cols + c
      // Rotate east/north into screen axes for a rotated map.
      vx[i] = uv[0] * cos - uv[1] * sin
      vy[i] = -(uv[0] * sin + uv[1] * cos)
      fade[i] = uv[2]
      ok[i] = 1
    }
  }
  return { cols, rows, vx, vy, ok, fade }
}

// sample bilinear-blends the four surrounding nodes into [vx, vy, fade]; null off the field.
export function sample(g, x, y) {
  const fx = x / GRID_PX
  const fy = y / GRID_PX
  const c = Math.floor(fx)
  const r = Math.floor(fy)
  if (c < 0 || r < 0 || c + 1 >= g.cols || r + 1 >= g.rows) return null
  const tx = fx - c
  const ty = fy - r
  let ax = 0
  let ay = 0
  let aw = 0
  let af = 0
  for (let j = 0; j < 4; j++) {
    const i = (r + (j >> 1)) * g.cols + c + (j & 1)
    if (!g.ok[i]) continue
    const w = ((j & 1) ? tx : 1 - tx) * ((j >> 1) ? ty : 1 - ty)
    ax += g.vx[i] * w
    ay += g.vy[i] * w
    af += g.fade[i] * w
    aw += w
  }
  return aw > 0.35 ? [ax / aw, ay / aw, af] : null
}

const running = new WeakMap()

export function startStreaks(map, body, { doc = document, win = window } = {}) {
  // One overlay per map, whoever asks.
  running.get(map)?.stop()
  const vectors = toUV(body)
  const host = map.getCanvasContainer()
  const canvas = doc.createElement('canvas')
  canvas.className = 'map-wind-streaks'
  canvas.setAttribute('aria-hidden', 'true')
  canvas.style.cssText = 'position:absolute;inset:0;pointer-events:none;'
  host.appendChild(canvas)
  const ctx = canvas.getContext('2d')

  let w = 0
  let h = 0
  let grid = null
  let particles = []
  let raf = 0
  let last = 0
  let moving = false

  // The basemap stays light in both themes, so one blue ramp serves both.
  const bucketColours = Array.from({ length: BUCKETS }, (_, i) => bucketColour(i))

  const seed = (p) => {
    // Accept a spot with probability equal to its fade, so streaks thin out away from the vectors.
    for (let n = 0; n < 8; n++) {
      p.x = Math.random() * w
      p.y = Math.random() * h
      const v = sample(grid, p.x, p.y)
      if (v && Math.random() < v[2]) break
    }
    p.age = Math.floor(Math.random() * MAX_AGE)
  }

  const rebuild = () => {
    const gl = map.getCanvas()
    w = gl.clientWidth
    h = gl.clientHeight
    const dpr = Math.min(win.devicePixelRatio || 1, 1.5)
    canvas.width = Math.round(w * dpr)
    canvas.height = Math.round(h * dpr)
    canvas.style.width = `${w}px`
    canvas.style.height = `${h}px`
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    grid = buildGrid(vectors, w, h, (x, y) => {
      const ll = map.unproject([x, y])
      return [ll.lng, ll.lat]
    }, map.getBearing?.() ?? 0)
    const want = particleCount(w, h)
    particles = Array.from({ length: want }, () => {
      const p = { x: 0, y: 0, age: 0 }
      seed(p)
      return p
    })
    ctx.clearRect(0, 0, w, h)
  }

  const frame = (now) => {
    raf = win.requestAnimationFrame(frame)
    const dt = Math.min(now - last, 50)
    last = now
    if (moving || !grid) return
    // Fade old trails toward transparent so the hexes underneath never get covered.
    ctx.globalCompositeOperation = 'destination-out'
    ctx.fillStyle = `rgba(0,0,0,${FADE})`
    ctx.fillRect(0, 0, w, h)
    ctx.globalCompositeOperation = 'source-over'
    ctx.globalAlpha = LINE_ALPHA
    ctx.lineWidth = LINE_WIDTH
    ctx.lineCap = 'round'
    // One path per speed bucket, each stroked in its own blue.
    const paths = bucketColours.map(() => new Path2D())
    for (const p of particles) {
      const v = sample(grid, p.x, p.y)
      if (!v || Math.random() < (1 - v[2]) * FRINGE_CULL || ++p.age > MAX_AGE) {
        seed(p)
        p.age = 0
        continue
      }
      const k = stepPx(1, dt)
      const nx = p.x + v[0] * k
      const ny = p.y + v[1] * k
      const path = paths[speedBucket(Math.hypot(v[0], v[1]))]
      path.moveTo(p.x, p.y)
      path.lineTo(nx, ny)
      p.x = nx
      p.y = ny
    }
    paths.forEach((path, i) => {
      ctx.strokeStyle = bucketColours[i]
      ctx.stroke(path)
    })
    ctx.globalAlpha = 1
  }

  const run = () => {
    if (raf || doc.hidden) return
    last = win.performance.now()
    raf = win.requestAnimationFrame(frame)
  }
  const halt = () => {
    win.cancelAnimationFrame(raf)
    raf = 0
  }
  const onVisibility = () => (doc.hidden ? halt() : run())
  // Streaks live in screen space, so a move invalidates them: clear, then reseed on moveend.
  const onMoveStart = () => { moving = true; ctx.clearRect(0, 0, w, h) }
  const onMoveEnd = () => { rebuild(); moving = false }

  doc.addEventListener('visibilitychange', onVisibility)
  map.on('movestart', onMoveStart)
  map.on('moveend', onMoveEnd)
  map.on('resize', onMoveEnd)
  rebuild()
  run()

  const handle = {
    stop() {
      running.delete(map)
      halt()
      doc.removeEventListener('visibilitychange', onVisibility)
      map.off('movestart', onMoveStart)
      map.off('moveend', onMoveEnd)
      map.off('resize', onMoveEnd)
      canvas.remove()
    },
  }
  running.set(map, handle)
  return handle
}
