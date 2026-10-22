import { describe, it, expect } from 'vitest'
import {
  windfxMode, toUV, idw, sample, WIND_REACH_KM, WIND_FULL_KM, windColour, speedBucket, particleCount, stepPx, WIND_RAMP, WIND_SPEED_MAX_MS,
  WIND_LINE_ALPHA, WIND_BUCKETS, WIND_AREA_PER_PARTICLE, WIND_MAX_PARTICLES,
} from '../windstreaks.js'

describe('windfxMode', () => {
  it('is streaks by default', () => expect(windfxMode('', false)).toBe('streaks'))
  it('is streaks with the streaks param', () => expect(windfxMode('?windfx=streaks', false)).toBe('streaks'))
  it('is arrows with the arrows param', () => expect(windfxMode('?windfx=arrows', false)).toBe('arrows'))
  it('falls back to arrows under reduced motion', () => expect(windfxMode('?windfx=streaks', true)).toBe('arrows'))
})

describe('reduced motion', () => {
  it('keeps the arrows whatever the flag says', () => {
    expect(windfxMode('?windfx=streaks&x=1', true)).toBe('arrows')
    expect(windfxMode('?windfx=arrows', true)).toBe('arrows')
    expect(windfxMode('', true)).toBe('arrows')
    expect(windfxMode('?windfx=other', false)).toBe('streaks')
  })
})

describe('field', () => {
  const body = { forecast: true, vectors: [{ lon: 25, lat: 42, speed_ms: 4, direction_deg: 270 }] }
  it('turns a westerly into an eastward u', () => {
    const [v] = toUV(body)
    expect(v.u).toBeCloseTo(4)
    expect(v.v).toBeCloseTo(0)
  })
  it('refuses a body not marked as forecast', () => expect(toUV({ vectors: body.vectors })).toEqual([]))
  it('has no value far from every vector', () => expect(idw(toUV(body), 27, 42)).toBeNull())
  it('reproduces the vector on top of it', () => expect(idw(toUV(body), 25, 42)[0]).toBeCloseTo(4))
})

describe('sparse field reach', () => {
  // 0.45 deg of longitude at 42 N is about 37 km per 0.5 deg; two vectors ~50 km apart.
  const at = (lon) => ({ forecast: true, vectors: [
    { lon: 25, lat: 42, speed_ms: 2, direction_deg: 270 },
    { lon, lat: 42, speed_ms: 6, direction_deg: 270 },
  ] })
  const kmPerLon = 111.195 * Math.cos((42 * Math.PI) / 180)
  const lon50 = 25 + 50 / kmPerLon
  const vs = toUV(at(lon50))
  it('interpolates midway between vectors 50 km apart', () => {
    const r = idw(vs, 25 + 25 / kmPerLon, 42)
    expect(r).not.toBeNull()
    expect(r[0]).toBeGreaterThan(2)
    expect(r[0]).toBeLessThan(6)
  })
  it('has no value 200 km from every vector', () => {
    expect(idw(vs, 25 - 200 / kmPerLon, 42)).toBeNull()
    expect(idw(vs, lon50 + 200 / kmPerLon, 42)).toBeNull()
  })
  it('fades out with distance instead of cutting off', () => {
    const near = idw(vs, 25 + 5 / kmPerLon, 42)[2]
    const mid = idw(vs, 25 - ((WIND_FULL_KM + WIND_REACH_KM) / 2) / kmPerLon, 42)[2]
    const far = idw(vs, 25 - (WIND_REACH_KM + 5) / kmPerLon, 42)
    expect(near).toBe(1)
    expect(mid).toBeGreaterThan(0)
    expect(mid).toBeLessThan(1)
    expect(far).toBeNull()
  })
  it('sample keeps a point inside the field and drops one outside', () => {
    const g = { cols: 3, rows: 3, vx: new Float32Array(9).fill(1), vy: new Float32Array(9), ok: new Uint8Array(9).fill(1), fade: new Float32Array(9).fill(1) }
    expect(sample(g, 20, 20)[2]).toBeCloseTo(1)
    g.ok.fill(0)
    expect(sample(g, 20, 20)).toBeNull()
  })
})

const parse = (c) => c.match(/\d+/g).map(Number)
const lum = ([r, g, b]) => 0.2126 * r + 0.7152 * g + 0.0722 * b
const hue = ([r, g, b]) => {
  const mx = Math.max(r, g, b)
  const d = mx - Math.min(r, g, b)
  if (d === 0) return 0
  if (mx === b) return 60 * ((r - g) / d) + 240
  if (mx === g) return 60 * ((b - r) / d) + 120
  return (60 * ((g - b) / d) + 360) % 360
}

describe('windColour', () => {
  const speeds = [0, 0.5, 1, 2, 3, 4, 6, 8, 10, 12, 20]
  const cols = speeds.map((s) => parse(windColour(s)))
  it('gets darker as the wind gets stronger', () => {
    for (let i = 1; i < cols.length; i++) expect(lum(cols[i])).toBeLessThanOrEqual(lum(cols[i - 1]))
    expect(lum(cols[cols.length - 1])).toBeLessThan(lum(cols[0]))
  })
  it('stays inside the ramp ends', () => {
    expect(windColour(0)).toBe(windColour(-3))
    expect(windColour(WIND_SPEED_MAX_MS)).toBe(windColour(99))
    const lo = parse(WIND_RAMP[0][1])
    const hi = parse(WIND_RAMP[WIND_RAMP.length - 1][1])
    for (const c of cols) {
      c.forEach((v, k) => {
        expect(v).toBeGreaterThanOrEqual(Math.min(lo[k], hi[k]))
        expect(v).toBeLessThanOrEqual(Math.max(lo[k], hi[k]))
      })
    }
  })
  it('is blue and never black, teal or purple', () => {
    for (const c of cols) {
      expect(lum(c)).toBeGreaterThan(50)
      expect(c[2]).toBeGreaterThan(c[0])
      expect(c[2]).toBeGreaterThan(c[1])
      expect(hue(c)).toBeGreaterThanOrEqual(205)
      expect(hue(c)).toBeLessThanOrEqual(235)
    }
  })
  it('has a partial line opacity', () => {
    expect(WIND_LINE_ALPHA).toBeGreaterThan(0.3)
    expect(WIND_LINE_ALPHA).toBeLessThan(1)
  })
})

describe('speedBucket', () => {
  it('is monotonic and in range', () => {
    let prev = 0
    for (let s = -1; s < 30; s += 0.25) {
      const b = speedBucket(s)
      expect(b).toBeGreaterThanOrEqual(prev)
      expect(b).toBeLessThan(WIND_BUCKETS)
      prev = b
    }
    expect(speedBucket(WIND_SPEED_MAX_MS)).toBe(WIND_BUCKETS - 1)
  })
})

describe('particleCount', () => {
  it('scales with viewport area', () => {
    expect(particleCount(800, 600)).toBeGreaterThan(particleCount(400, 300))
    expect(particleCount(400, 300)).toBe(Math.round((400 * 300) / WIND_AREA_PER_PARTICLE))
  })
  it('puts a phone below a desktop', () => {
    expect(particleCount(390, 844)).toBeLessThan(particleCount(1280, 800))
  })
  it('is at most half the old density (1 per 1800 px2, cap 1400)', () => {
    expect(particleCount(1280, 800)).toBeLessThanOrEqual(Math.round((1280 * 800) / 1800 / 2))
    expect(particleCount(390, 844)).toBeLessThanOrEqual(Math.round((390 * 844) / 1800 / 2))
  })
  it('is capped and never negative', () => {
    expect(particleCount(10000, 10000)).toBe(WIND_MAX_PARTICLES)
    expect(particleCount(0, 0)).toBe(0)
  })
})

describe('stepPx', () => {
  it('is proportional to speed and frame time', () => {
    expect(stepPx(4, 16)).toBeCloseTo(2 * stepPx(2, 16))
    expect(stepPx(4, 32)).toBeCloseTo(2 * stepPx(4, 16))
  })
  it('is at least 3x slower than the old 0.032 px/ms per m/s', () => {
    expect(stepPx(5, 16.7)).toBeLessThanOrEqual((0.032 * 5 * 16.7) / 3)
  })
  it('keeps calm and strong wind visibly apart', () => {
    expect(stepPx(10, 16)).toBeGreaterThanOrEqual(4 * stepPx(2, 16))
  })
})

// The pre-optimisation idw, kept as the oracle: filter, stable sort, first four.
function idwReference(vectors, lon, lat) {
  const kmPerDeg = (Math.PI * 6371) / 180
  const kmLon = kmPerDeg * Math.cos((lat * Math.PI) / 180)
  const near = []
  for (const p of vectors) {
    const dx = (p.lon - lon) * kmLon
    const dy = (p.lat - lat) * kmPerDeg
    const d2 = dx * dx + dy * dy
    if (d2 < WIND_REACH_KM * WIND_REACH_KM) near.push([d2, p])
  }
  if (near.length === 0) return null
  near.sort((a, b) => a[0] - b[0])
  let su = 0
  let sv = 0
  let sw = 0
  for (const [d2, p] of near.slice(0, 4)) {
    const w = 1 / (d2 + 25)
    su += p.u * w
    sv += p.v * w
    sw += w
  }
  const fade = Math.min(1, (WIND_REACH_KM - Math.sqrt(near[0][0])) / (WIND_REACH_KM - WIND_FULL_KM))
  return [su / sw, sv / sw, fade]
}

describe('idw matches the sort-based reference', () => {
  it('agrees on scattered vectors at many query points', () => {
    let seed = 7
    const rnd = () => { seed = (seed * 16807) % 2147483647; return seed / 2147483647 }
    const vectors = Array.from({ length: 120 }, () => ({ lon: 22 + rnd() * 6, lat: 41 + rnd() * 2.5, u: rnd() * 10 - 5, v: rnd() * 10 - 5 }))
    for (let i = 0; i < 300; i++) {
      const lon = 21.5 + rnd() * 7
      const lat = 40.5 + rnd() * 3.5
      const got = idw(vectors, lon, lat)
      const want = idwReference(vectors, lon, lat)
      if (want === null) expect(got).toBeNull()
      else want.forEach((x, k) => expect(got[k]).toBeCloseTo(x, 9))
    }
  })
})
