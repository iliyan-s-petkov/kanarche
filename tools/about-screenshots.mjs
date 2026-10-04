#!/usr/bin/env node
// Regenerates the Getting started screenshots on /about from https://kanarche.eu.
// Read-only browsing of production. Needs `npm ci` in web/ and its Chromium.
//
//   node tools/about-screenshots.mjs [--base https://kanarche.eu] [--only layers,areas]
//
// Output: internal/web/static/about/start-<step>-<lang>-<theme>.webp, 560x460.
// Annotations are injected DOM (ring plus numbered badge), so a rerun redraws them.
import { mkdir, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const { chromium } = await import(pathToFileURL(resolve(here, '../web/node_modules/@playwright/test/index.mjs')).href)

const args = process.argv.slice(2)
const opt = (name, dflt) => (args.includes(name) ? args[args.indexOf(name) + 1] : dflt)
const BASE = opt('--base', 'https://kanarche.eu')
const ONLY = opt('--only', '')
const OUT = resolve(here, '../internal/web/static/about')
const W = 560
const H = 460
const MAX_BYTES = 60 * 1024

const LANGS = { bg: { prefix: '', locale: 'bg-BG' }, en: { prefix: '/en', locale: 'en-GB' } }
const THEMES = ['light', 'dark']

// Draws a ring around the box and a numbered badge on its top-left corner.
// Inline style is set through CSSOM: the site CSP has no 'unsafe-inline' for style attributes.
async function annotate(page, boxes) {
  await page.evaluate((list) => {
    list.forEach((b, i) => {
      const ring = document.createElement('div')
      Object.assign(ring.style, {
        position: 'fixed', left: b.x - 4 + 'px', top: b.y - 4 + 'px',
        width: b.w + 8 + 'px', height: b.h + 8 + 'px', boxSizing: 'border-box',
        border: '3px solid #ff5a1f', borderRadius: '10px', zIndex: '2147483647',
        boxShadow: '0 0 0 2px #fff', pointerEvents: 'none',
      })
      const badge = document.createElement('div')
      badge.textContent = String(b.n ?? i + 1)
      Object.assign(badge.style, {
        position: 'absolute', left: '-14px', top: '-14px', width: '24px', height: '24px',
        borderRadius: '50%', background: '#ff5a1f', color: '#fff', font: '700 14px/24px system-ui, sans-serif',
        textAlign: 'center', boxShadow: '0 0 0 2px #fff',
      })
      ring.append(badge)
      document.body.append(ring)
    })
  }, boxes)
}

const box = async (page, sel) => {
  const r = await page.locator(sel).first().boundingBox()
  return { x: r.x, y: r.y, w: r.width, h: r.height }
}

async function settleMap(page) {
  await page.waitForLoadState('networkidle')
  await page.locator('canvas.maplibregl-canvas').first().waitFor()
  // Tiles and hexes keep painting after the network goes quiet.
  await page.waitForTimeout(3000)
}

const STEPS = {
  async layers(page, { prefix }) {
    await page.setViewportSize({ width: 1280, height: 800 })
    await page.goto(`${BASE}${prefix}/`)
    await settleMap(page)
    await page.locator('.map__layers .colmenu__btn').click()
    await page.locator('.map__layers .colmenu__panel').waitFor()
    await page.waitForTimeout(500)
    const map = await box(page, '#map')
    const panel = await box(page, '.map__layers .colmenu__panel')
    const btn = await box(page, '.map__layers .colmenu__btn')
    await annotate(page, [{ ...btn, n: 1 }, { ...panel, n: 2 }])
    return { x: map.x - 16, y: map.y - 8, width: W, height: H }
  },
  async window(page, { prefix }) {
    await page.setViewportSize({ width: 1280, height: 800 })
    await page.goto(`${BASE}${prefix}/`)
    await settleMap(page)
    await page.locator('.map-window__btn').click()
    await page.locator('.map-window__panel').waitFor()
    await page.waitForTimeout(500)
    const map = await box(page, '#map')
    const panel = await box(page, '.map-window__panel')
    const btn = await box(page, '.map-window__btn')
    await annotate(page, [{ ...btn, n: 1 }, { ...panel, n: 2 }])
    return { x: map.x - 16, y: map.y + map.h - H + 8, width: W, height: H }
  },
  async areas(page, { prefix }) {
    await page.setViewportSize({ width: W, height: 900 })
    await page.goto(`${BASE}${prefix}/area/plovdiv`)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1500)
    const sum = await box(page, '.area-summary')
    await annotate(page, [{ ...sum, n: 1 }])
    return { x: 0, y: sum.y - 150, width: W, height: H }
  },
  async embed(page, { prefix }) {
    await page.setViewportSize({ width: W, height: H })
    await page.goto(`${BASE}${prefix}/embed?area=plovdiv-oblast`)
    await settleMap(page)
    await annotate(page, [{ x: 4, y: 4, w: W - 8, h: H - 8, n: 1 }])
    return { x: 0, y: 0, width: W, height: H }
  },
}

// Chromium encodes the WebP itself, so no image dependency is needed.
async function toWebp(page, png) {
  for (const q of [0.82, 0.74, 0.66, 0.58, 0.5]) {
    const b64 = await page.evaluate(async ({ data, quality }) => {
      const bmp = await createImageBitmap(await (await fetch(`data:image/png;base64,${data}`)).blob())
      const c = new OffscreenCanvas(bmp.width, bmp.height)
      c.getContext('2d').drawImage(bmp, 0, 0)
      const blob = await c.convertToBlob({ type: 'image/webp', quality })
      const buf = new Uint8Array(await blob.arrayBuffer())
      let s = ''
      for (const v of buf) s += String.fromCharCode(v)
      return btoa(s)
    }, { data: png.toString('base64'), quality: q })
    const out = Buffer.from(b64, 'base64')
    if (out.length <= MAX_BYTES) return out
  }
  throw new Error('image stays above the size budget at the lowest quality')
}

await mkdir(OUT, { recursive: true })
const browser = await chromium.launch()
const encoder = await (await browser.newContext()).newPage()
await encoder.goto('about:blank')
const only = ONLY ? ONLY.split(',') : Object.keys(STEPS)

for (const [lang, cfg] of Object.entries(LANGS)) {
  for (const theme of THEMES) {
    for (const step of only) {
      const ctx = await browser.newContext({ locale: cfg.locale, colorScheme: theme, viewport: { width: 1280, height: 800 } })
      const page = await ctx.newPage()
      const clip = await STEPS[step](page, cfg)
      const png = await page.screenshot({ clip })
      const webp = await toWebp(encoder, png)
      const file = `start-${step}-${lang}-${theme}.webp`
      await writeFile(resolve(OUT, file), webp)
      console.log(`${file}\t${(webp.length / 1024).toFixed(1)} KB`)
      await ctx.close()
    }
  }
}
await browser.close()
