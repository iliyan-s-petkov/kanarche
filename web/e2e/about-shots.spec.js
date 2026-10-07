import { readFileSync } from 'node:fs'
import { mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { test, expect } from './fixtures.js'

// Regenerates the About "Getting started" screenshots from the harness data.
// Skipped unless AIRBG_SHOTS_DIR names the output directory (internal/web/static/about).
// Needs a build made with VITE_E2E_MAP_HANDLE=1. Output: start-<scene>-<lang>-<theme>.webp, 560x460.
// AIRBG_SHOTS_ONLY=tilt,legend limits the run to those scenes.
const OUT = process.env.AIRBG_SHOTS_DIR
const ONLY = (process.env.AIRBG_SHOTS_ONLY ?? '').split(',').filter(Boolean)
const W = 560
const H = 460
const MAX_BYTES = 60 * 1024
const SOFIA = [23.32, 42.69]

// Layer labels come from the translation files, so the bg run finds the same controls.
const STRINGS = Object.fromEntries(['en', 'bg'].map((l) => [l, JSON.parse(readFileSync(new URL(`../../internal/i18n/${l}.json`, import.meta.url), 'utf8'))]))

test.skip(!OUT, 'set AIRBG_SHOTS_DIR to regenerate the About screenshots')

// Chromium encodes the WebP itself, so no image dependency is needed. With fit
// 'contain' the PNG keeps its proportions and sits centred on a flat backdrop.
async function toWebp(page, png, { fit = 'stretch', backdrop = '#ffffff' } = {}) {
  for (const q of [0.82, 0.74, 0.66, 0.58, 0.5, 0.42]) {
    const b64 = await page.evaluate(async ({ data, quality, w, h, fit, backdrop }) => {
      const bmp = await createImageBitmap(await (await fetch(`data:image/png;base64,${data}`)).blob())
      const c = new OffscreenCanvas(w, h)
      const g = c.getContext('2d')
      if (fit === 'contain') {
        g.fillStyle = backdrop
        g.fillRect(0, 0, w, h)
        const s = Math.min(w / bmp.width, h / bmp.height)
        g.drawImage(bmp, (w - bmp.width * s) / 2, (h - bmp.height * s) / 2, bmp.width * s, bmp.height * s)
      } else {
        g.drawImage(bmp, 0, 0, w, h)
      }
      const blob = await c.convertToBlob({ type: 'image/webp', quality })
      const buf = new Uint8Array(await blob.arrayBuffer())
      let s = ''
      for (const v of buf) s += String.fromCharCode(v)
      return btoa(s)
    }, { data: png.toString('base64'), quality: q, w: W, h: H, fit, backdrop })
    const out = Buffer.from(b64, 'base64')
    if (out.length <= MAX_BYTES) return out
  }
  throw new Error('image stays above the size budget at the lowest quality')
}

async function mapReady(page) {
  await page.waitForFunction(() => document.querySelector('[data-island="map"]')?.__map?.isStyleLoaded?.(), null, { timeout: 45_000 })
  await page.waitForTimeout(1000)
  await expect.poll(() => page.evaluate(() => document.querySelector('[data-island="map"]').__map.isMoving())).toBe(false)
}

// Waits until the map has drawn something other than the base style.
const mapIdle = (page) => expect.poll(() => page.evaluate(() => {
  const map = document.querySelector('[data-island="map"]').__map
  return map.loaded() && !map.isMoving()
}), { timeout: 30_000, intervals: [500] }).toBe(true)

const jump = (page, camera) => page.evaluate((c) => document.querySelector('[data-island="map"]').__map.jumpTo(c), camera)

// Page rectangles of every visible element matching one of the selectors.
const rects = (page, selectors) => selectors.length === 0 ? [] : page.evaluate((sels) => {
  const out = []
  for (const el of document.querySelectorAll(sels.join(','))) {
    const r = el.getBoundingClientRect()
    const cs = getComputedStyle(el)
    if (r.width > 0 && r.height > 0 && cs.visibility !== 'hidden' && cs.display !== 'none') out.push({ x: r.x, y: r.y, w: r.width, h: r.height })
  }
  return out
}, selectors)

// A 560:460 window around the union of the rectangles, never smaller than 560x460
// so nothing is upscaled, and kept inside the viewport.
async function clipAround(page, selectors, { pad = 24, shift = { x: 0, y: 0 }, within = null, extra = [], corner = false } = {}) {
  const list = [...(await rects(page, selectors)), ...extra]
  if (!list.length) throw new Error(`nothing visible for ${selectors.join(', ')}`)
  const x0 = Math.min(...list.map((r) => r.x)) - pad
  const y0 = Math.min(...list.map((r) => r.y)) - pad
  const x1 = Math.max(...list.map((r) => r.x + r.w)) + pad
  const y1 = Math.max(...list.map((r) => r.y + r.h)) + pad
  if (corner) {
    // A fixed window from the top left of the union: for features wider than the window, where the left part carries the point.
    const vp = page.viewportSize()
    return { x: Math.max(0, x0), y: Math.max(0, Math.min(y0, vp.height - H)), width: W, height: H }
  }
  let width = Math.max(W, x1 - x0)
  let height = (width * H) / W
  if (height < y1 - y0) {
    height = y1 - y0
    width = (height * W) / H
  }
  const vp = page.viewportSize()
  // Optionally keep the window inside one element, so the page around the map stays out of the shot.
  const [bound] = within ? await rects(page, [within]) : [{ x: 0, y: 0, w: vp.width, h: vp.height }]
  let x = (x0 + x1) / 2 - width / 2 + shift.x
  let y = (y0 + y1) / 2 - height / 2 + shift.y
  if (bound.w >= width) x = Math.max(bound.x, Math.min(x, bound.x + bound.w - width))
  if (bound.h >= height) y = Math.max(bound.y, Math.min(y, bound.y + bound.h - height))
  x = Math.max(0, Math.min(x, vp.width - width))
  y = Math.max(0, Math.min(y, vp.height - height))
  return { x, y, width: Math.min(width, vp.width), height: Math.min(height, vp.height) }
}

// Page position of a lon/lat on the map, as a small rectangle clipAround can include.
const lonLatRect = (page, lonLat) => page.evaluate((c) => {
  const map = document.querySelector('[data-island="map"]').__map
  const box = map.getCanvas().getBoundingClientRect()
  const p = map.project(c)
  return { x: box.left + p.x - 8, y: box.top + p.y - 8, w: 16, h: 16 }
}, lonLat)

// A ring round the feature, drawn as injected DOM like tools/about-screenshots.mjs.
// Inline style goes through CSSOM because the site CSP has no 'unsafe-inline' for style attributes.
async function ring(page, selector, rect = null) {
  const [b] = rect ? [rect] : await rects(page, [selector])
  if (!b) return
  await page.evaluate((b) => {
    const el = document.createElement('div')
    Object.assign(el.style, {
      position: 'fixed', left: b.x - 4 + 'px', top: b.y - 4 + 'px',
      width: b.w + 8 + 'px', height: b.h + 8 + 'px', boxSizing: 'border-box',
      border: '3px solid #ff5a1f', borderRadius: '10px', zIndex: '2147483647',
      boxShadow: '0 0 0 2px #fff', pointerEvents: 'none',
    })
    document.body.append(el)
  }, b)
}

const layersBox = (page, label) => page.locator('.map__layers').getByLabel(label, { exact: true })
async function openLayers(page) {
  if (!(await page.locator('.map__layers .colmenu__panel').isVisible())) await page.locator('.map__layers .colmenu__btn').click()
  await expect(page.locator('.map__layers .colmenu__panel')).toBeVisible()
}
async function layerOn(page, label) {
  await openLayers(page)
  await layersBox(page, label).check()
  await page.keyboard.press('Escape')
}

// Over Sofia at the sensor tier, so individual sensors are drawn.
async function overSofia(page, prefix, zoom = 12.5) {
  await page.goto(`${prefix}/`)
  await mapReady(page)
  await jump(page, { center: SOFIA, zoom })
  await expect.poll(() => page.evaluate(() => document.querySelector('[data-island="map"]').__map.queryRenderedFeatures({ layers: ['airbg-hex-fill'] }).length), { timeout: 30_000, intervals: [500] }).toBeGreaterThan(0)
  await mapIdle(page)
  await page.waitForTimeout(800)
}

// Opens the sensor panel on sensor 101 (desktop dock or phone sheet).
async function withSensor(page, prefix, panel) {
  await page.goto(`${prefix}/#sensor=101`)
  await mapReady(page)
  await expect(page.locator(`${panel} .chart-frame`)).toBeVisible({ timeout: 20_000 })
  await page.waitForTimeout(1500)
}

const windGrid = () => {
  const vectors = []
  for (let i = 0; i < 14; i++) {
    for (let j = 0; j < 10; j++) {
      vectors.push({ lon: 22.9 + i * 0.07, lat: 42.35 + j * 0.07, speed_ms: 3 + ((i + j) % 5), direction_deg: 215 + ((i * 3 + j * 5) % 30) })
    }
  }
  return vectors
}

// Each scene returns { clip } or { png, fit }, and runs on a page already at the desktop viewport unless it asks for another.
const SCENES = {
  async metrics(page, { prefix }) {
    await overSofia(page, prefix)
    await page.locator('#metric-menu').evaluate((el) => window.scrollBy(0, el.getBoundingClientRect().top - 16))
    await page.locator('#metric-menu').click()
    await page.waitForTimeout(400)
    await ring(page, '#metric-menu')
    return { clip: await clipAround(page, ['#metric-menu', '#metric-menu ~ *', '[role="radiogroup"]'], { within: '#map' }) }
  },

  async legend(page, { prefix }) {
    await overSofia(page, prefix)
    const legend = page.locator('.scale--onmap')
    if ((await legend.getAttribute('open')) === null) await legend.locator('> .scale__toggle').click()
    await page.waitForTimeout(500)
    await ring(page, '.scale--onmap')
    return { clip: await clipAround(page, ['.scale--onmap'], { pad: 90, within: '#map' }) }
  },

  async inactive(page, { prefix, lang }) {
    await overSofia(page, prefix)
    await openLayers(page)
    await layersBox(page, STRINGS[lang]['map.view.inactive_stations']).check()
    await layersBox(page, STRINGS[lang]['map.view.faulty_stations']).check()
    await page.waitForTimeout(800)
    return { clip: await clipAround(page, ['.map__layers .colmenu__panel'], { within: '#map', pad: 60 }) }
  },

  async search(page, { prefix, lang }) {
    await overSofia(page, prefix, 11)
    const input = page.locator('[data-island="finder"] input')
    await input.click()
    await input.fill(lang === 'bg' ? 'Мла' : 'Mla')
    await expect(page.locator('[data-island="finder"] [role="option"]').first()).toBeVisible()
    await page.waitForTimeout(400)
    await ring(page, '[data-island="finder"] input')
    return { clip: await clipAround(page, ['[data-island="finder"] input', '[data-island="finder"] [role="listbox"]'], { within: '#map' }) }
  },

  async below(page, { prefix }) {
    await page.goto(`${prefix}/`)
    await mapReady(page)
    await page.waitForTimeout(1500)
    // The arrow strip hangs off the bottom edge of the map; the page below it is what it points at.
    await page.evaluate(() => window.scrollBy(0, 240))
    await page.waitForTimeout(800)
    await ring(page, '.scroll-cue')
    return { clip: await clipAround(page, ['.scroll-cue'], { pad: 200 }) }
  },

  async pollen(page, { prefix, lang }) {
    await page.goto(`${prefix}/`)
    await mapReady(page)
    // The fixture forecast is one grid cell; the camera puts it beside the legend in the bottom left.
    await jump(page, { center: [SOFIA[0] + 0.6, SOFIA[1] + 0.45], zoom: 8.6 })
    await layerOn(page, STRINGS[lang]['pollen.species'])
    await mapIdle(page)
    const legend = page.locator('.scale--onmap')
    if ((await legend.getAttribute('open')) === null) await legend.locator('> .scale__toggle').click()
    await page.waitForTimeout(2500)
    await ring(page, '.scale--onmap')
    return { clip: await clipAround(page, ['.scale--onmap'], { pad: 150, within: '#map' }) }
  },

  async sea(page, { prefix, lang }) {
    await page.goto(`${prefix}/`)
    await mapReady(page)
    await jump(page, { center: [23.45, 42.75], zoom: 10 })
    await layerOn(page, STRINGS[lang]['sea.toggle'])
    let pt = null
    await expect.poll(async () => {
      pt = await page.evaluate((c) => {
        const map = document.querySelector('[data-island="map"]').__map
        if (!map.queryRenderedFeatures({ layers: ['sea-sites'] }).length) return null
        const box = map.getCanvas().getBoundingClientRect()
        const p = map.project(c)
        return { x: box.left + p.x, y: box.top + p.y }
      }, [23.45, 42.75])
      return pt
    }, { timeout: 20_000 }).not.toBeNull()
    await page.mouse.click(pt.x, pt.y)
    await expect(page.locator('.map-sea .sea-panel')).toBeVisible()
    await page.waitForTimeout(1000)
    return { clip: await clipAround(page, ['.map-sea .sea-panel'], { pad: 16, within: '#map', extra: [await lonLatRect(page, [23.45, 42.75])] }) }
  },

  async wind(page, { prefix, lang }) {
    await page.route('**/api/v1/wind', (route) => route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        generated_at: new Date().toISOString(),
        valid_at: new Date().toISOString(),
        model: 'Test Model',
        model_resolution_deg: 0.25,
        resolution_km: 25,
        forecast: true,
        vectors: windGrid(),
      }),
    }))
    await overSofia(page, prefix, 10)
    await layerOn(page, STRINGS[lang]['wind.toggle'])
    await mapIdle(page)
    const legend = page.locator('.scale--onmap')
    if ((await legend.getAttribute('open')) === null) await legend.locator('> .scale__toggle').click()
    await page.waitForTimeout(3500)
    await ring(page, '.scale--onmap .scale__wind')
    return { clip: await clipAround(page, ['.scale--onmap'], { pad: 200, within: '#map' }) }
  },

  async official(page, { prefix, lang }) {
    await overSofia(page, prefix, 13)
    // The harness station reports PM10 only, so the map shows PM10 for it to appear.
    await page.locator('#metric-menu').click()
    await page.getByRole('radio', { name: STRINGS[lang]['metric.P1'], exact: true }).check()
    await page.keyboard.press('Escape')
    await openLayers(page)
    const box = page.locator('.map__layers').getByLabel(STRINGS[lang]['map.view.official_stations'])
    if (!(await box.first().isChecked())) await box.first().check()
    // Slide the map so the station sits right of the open menu, and ring it.
    await page.evaluate(() => document.querySelector('[data-island="map"]').__map.panBy([260, 0], { animate: false }))
    await mapIdle(page)
    await page.waitForTimeout(2000)
    const at = await lonLatRect(page, SOFIA)
    // The station is drawn as the cell that holds it; ring that cell's outline.
    const cell = await page.evaluate((c) => {
      const map = document.querySelector('[data-island="map"]').__map
      const p = map.project(c)
      const hit = map.queryRenderedFeatures([p.x, p.y], { layers: ['airbg-hex-fill'] })[0]
      if (!hit) return null
      const box = map.getCanvas().getBoundingClientRect()
      const pts = hit.geometry.coordinates[0].map((q) => map.project(q))
      const xs = pts.map((q) => q.x)
      const ys = pts.map((q) => q.y)
      return { x: box.left + Math.min(...xs), y: box.top + Math.min(...ys), w: Math.max(...xs) - Math.min(...xs), h: Math.max(...ys) - Math.min(...ys) }
    }, SOFIA)
    if (!cell) throw new Error('no cell under the station')
    await ring(page, null, cell)
    return { clip: await clipAround(page, ['.map__layers .colmenu__panel'], { pad: 20, within: '#map', extra: [at] }) }
  },

  async replay(page, { prefix }) {
    await page.goto(`${prefix}/`)
    await mapReady(page)
    const play = page.locator('.map-play__btn[aria-pressed]')
    await play.evaluate((el) => el.scrollIntoView({ block: 'center' }))
    const loaded = page.waitForResponse((r) => r.url().includes('timelapse'))
    await play.click()
    await loaded
    await expect(page.locator('.map-play--open')).toHaveCount(1)
    await page.waitForTimeout(1500)
    await ring(page, '.map-play')
    return { clip: await clipAround(page, ['.map-play', '.scale--onmap', '.map-window__btn'], { pad: 20, within: '#map' }) }
  },

  async charts(page, { prefix }) {
    await withSensor(page, prefix, '.map-dock')
    return { clip: await clipAround(page, ['.map-dock'], { pad: 12, corner: true }) }
  },

  async favourite(page, { prefix }) {
    await withSensor(page, prefix, '.map-dock')
    await ring(page, '.map-dock .panel-star')
    return { clip: await clipAround(page, ['.map-dock .panel-star', '.map-dock h2', '.map-dock .gauges'], { pad: 40 }) }
  },

  async share(page, { prefix }) {
    await withSensor(page, prefix, '.map-dock')
    await page.locator('.map-dock .panel-more').click()
    await expect(page.locator('.map-dock .panel-menu')).toBeVisible()
    await page.waitForTimeout(400)
    await ring(page, '.map-dock .panel-menu')
    return { clip: await clipAround(page, ['.map-dock .panel-more', '.map-dock .panel-menu'], { pad: 40 }) }
  },

  // A 560 px wide page, so the province table shows its reading columns instead of cutting them off.
  table: {
    context: { viewport: { width: W, height: 900 } },
    async run(page, { prefix }) {
      await page.goto(`${prefix}/areas`)
      await page.waitForSelector('.table-controls')
      await page.waitForTimeout(1500)
      return { clip: await clipAround(page, ['.table-controls'], { pad: 16, corner: true }) }
    },
  },

  async locate(page, { prefix }) {
    await page.goto(`${prefix}/`)
    await mapReady(page)
    await expect(page.locator('.map-locate-hint')).toBeVisible()
    await ring(page, '.map-locate')
    return { clip: await clipAround(page, ['.map-locate', '.map-locate-hint'], { pad: 120 }) }
  },

  async tilt(page, { prefix }) {
    await page.goto(`${prefix}/`)
    await mapReady(page)
    await page.evaluate(() => document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' }))
    await jump(page, { center: SOFIA, zoom: 12.9 })
    // Flat first: the fine tier of cells loads for the top-down view, then the map tilts onto it.
    await expect.poll(() => page.evaluate(() => {
      const map = document.querySelector('[data-island="map"]').__map
      if (!map.loaded() || map.isMoving()) return 0
      return new Set(map.queryRenderedFeatures({ layers: ['airbg-hex-fill'] }).map((f) => f.properties.value)).size
    }), { timeout: 30_000, intervals: [500] }).toBeGreaterThan(1)
    await jump(page, { pitch: 55, bearing: 25 })
    await expect.poll(() => page.evaluate(() => {
      const map = document.querySelector('[data-island="map"]').__map
      if (!map.loaded() || map.isMoving()) return 0
      return new Set(map.queryRenderedFeatures({ layers: ['airbg-hex-extrusion'] }).map((f) => f.properties.value)).size
    }), { timeout: 30_000, intervals: [500] }).toBeGreaterThan(1)
    // The popup reads the camera, so it shows the tilt and heading just set.
    await page.locator('.map-orient__btn').click()
    await expect(page.locator('.map-orient__panel')).toBeVisible()
    await page.waitForTimeout(2500)
    await ring(page, '.map-orient__panel')
    return { clip: await clipAround(page, ['.map-orient__btn', '.map-orient__panel'], { pad: 140, within: '#map' }) }
  },

  // The phone viewport keeps its own proportions and sits centred on a backdrop.
  phone: {
    context: { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 },
    async run(page, { prefix, theme }) {
      await page.goto(`${prefix}/area/sofia`)
      await mapReady(page)
      await page.locator('.map__full').click()
      await expect(page.locator('.map__full')).toHaveAttribute('aria-pressed', 'true')
      await mapReady(page)
      // Tap the cell of sensor 101; the point is read again on every try because the resize can move it.
      await expect(async () => {
        const pt = await page.evaluate(() => {
          const map = document.querySelector('[data-island="map"]').__map
          const f = map.queryRenderedFeatures({ layers: ['airbg-hex-fill'] }).find((q) => Number(q.properties?.sensorId) === 101)
          if (!f) return null
          const ring = f.geometry.coordinates[0].slice(0, -1)
          const c = [0, 1].map((i) => ring.reduce((a, q) => a + q[i], 0) / ring.length)
          const box = map.getCanvas().getBoundingClientRect()
          const q = map.project(c)
          return { x: box.left + q.x, y: box.top + q.y }
        })
        expect(pt).not.toBeNull()
        await page.touchscreen.tap(pt.x, pt.y)
        await expect(page.locator('.map-sensor-sheet')).toBeVisible({ timeout: 2000 })
      }).toPass({ timeout: 20_000 })
      await page.waitForTimeout(2000)
      return { fit: 'contain', backdrop: theme === 'dark' ? '#14181c' : '#eef1f4' }
    },
  },
}

for (const [lang, prefix, locale] of [['en', '/en', 'en-GB'], ['bg', '', 'bg-BG']]) {
  for (const theme of ['light', 'dark']) {
    for (const [name, scene] of Object.entries(SCENES)) {
      if (ONLY.length && !ONLY.includes(name)) continue
      test(`${name} screenshot ${lang} ${theme}`, async ({ browser }) => {
        const run = typeof scene === 'function' ? scene : scene.run
        const context = await browser.newContext({
          locale,
          colorScheme: theme,
          viewport: { width: 1280, height: 800 },
          ...(scene.context ?? {}),
        })
        const page = await context.newPage()
        const encoder = await context.newPage()
        const shot = await run(page, { prefix, lang, theme })
        const png = await page.screenshot(shot.clip ? { clip: shot.clip } : {})
        const webp = await toWebp(encoder, png, shot)
        await mkdir(OUT, { recursive: true })
        await writeFile(resolve(OUT, `start-${name}-${lang}-${theme}.webp`), webp)
        await context.close()
      })
    }
  }
}
