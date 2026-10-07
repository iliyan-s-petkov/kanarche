import { mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { test, expect } from './fixtures.js'

// Regenerates the About "Getting started" tilt screenshots from the harness data.
// Skipped unless AIRBG_SHOTS_DIR names the output directory (internal/web/static/about).
// Needs a build made with VITE_E2E_MAP_HANDLE=1. Output: start-tilt-<lang>-<theme>.webp, 560x460.
const OUT = process.env.AIRBG_SHOTS_DIR
const W = 560
const H = 460
const MAX_BYTES = 60 * 1024

test.skip(!OUT, 'set AIRBG_SHOTS_DIR to regenerate the About screenshots')

// Chromium encodes the WebP itself, so no image dependency is needed.
async function toWebp(page, png) {
  for (const q of [0.82, 0.74, 0.66, 0.58, 0.5]) {
    const b64 = await page.evaluate(async ({ data, quality, w, h }) => {
      const bmp = await createImageBitmap(await (await fetch(`data:image/png;base64,${data}`)).blob())
      const c = new OffscreenCanvas(w, h)
      c.getContext('2d').drawImage(bmp, 0, 0, w, h)
      const blob = await c.convertToBlob({ type: 'image/webp', quality })
      const buf = new Uint8Array(await blob.arrayBuffer())
      let s = ''
      for (const v of buf) s += String.fromCharCode(v)
      return btoa(s)
    }, { data: png.toString('base64'), quality: q, w: W, h: H })
    const out = Buffer.from(b64, 'base64')
    if (out.length <= MAX_BYTES) return out
  }
  throw new Error('image stays above the size budget at the lowest quality')
}

for (const [lang, prefix, locale] of [['en', '/en', 'en-GB'], ['bg', '', 'bg-BG']]) {
  for (const theme of ['light', 'dark']) {
    test(`tilt screenshot ${lang} ${theme}`, async ({ browser }) => {
      const context = await browser.newContext({ locale, colorScheme: theme, viewport: { width: 1280, height: 800 } })
      const page = await context.newPage()
      const encoder = await context.newPage()
      await page.goto(`${prefix}/`)
      await page.waitForFunction(() => document.querySelector('[data-island="map"]')?.__map?.isStyleLoaded?.())
      await page.waitForTimeout(1000)
      await expect.poll(() => page.evaluate(() => document.querySelector('[data-island="map"]').__map.isMoving())).toBe(false)
      await page.evaluate(() => {
        document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
        document.querySelector('[data-island="map"]').__map.jumpTo({ center: [23.32, 42.69], zoom: 12.9 })
      })
      // Flat first: the fine tier of cells loads for the top-down view, then the map tilts onto it.
      await expect.poll(() => page.evaluate(() => {
        const map = document.querySelector('[data-island="map"]').__map
        if (!map.loaded() || map.isMoving()) return 0
        return new Set(map.queryRenderedFeatures({ layers: ['airbg-hex-fill'] }).map((f) => f.properties.value)).size
      }), { timeout: 30000, intervals: [500] }).toBeGreaterThan(1)
      await page.evaluate(() => {
        document.querySelector('[data-island="map"]').__map.jumpTo({ pitch: 55, bearing: 25 })
      })
      // Columns are drawn once the extrusion layer has rendered features and the map is idle.
      await expect.poll(() => page.evaluate(() => {
        const map = document.querySelector('[data-island="map"]').__map
        if (!map.loaded() || map.isMoving()) return 0
        // The fine tier paints after the coarse one; wait until columns of several values stand.
        return new Set(map.queryRenderedFeatures({ layers: ['airbg-hex-extrusion'] }).map((f) => f.properties.value)).size
      }), { timeout: 30000, intervals: [500] }).toBeGreaterThan(1)
      await page.waitForTimeout(2500)
      const r = await page.locator('#map').boundingBox()
      // A card-sized window around the middle of the map, clear of the masthead and map controls.
      const clip = { x: r.x + (r.width - W) / 2, y: r.y + (r.height - H) / 2, width: W, height: H }
      const webp = await toWebp(encoder, await page.screenshot({ clip }))
      await mkdir(OUT, { recursive: true })
      await writeFile(resolve(OUT, `start-tilt-${lang}-${theme}.webp`), webp)
      await context.close()
    })
  }
}
