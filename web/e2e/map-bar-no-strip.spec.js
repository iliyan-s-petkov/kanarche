import { test as base, expect } from './fixtures.js'

// The foot of the map is a row of separate glass pills, not one strip: the .map-bar
// container only positions them. The map between the pills must stay visible and
// clickable, and each time control must be its own pill with a gap to the next.
const test = base.extend({
  phoneCtx: [async ({ browser }, use) => {
    const context = await browser.newContext({
      isMobile: true, hasTouch: true, deviceScaleFactor: 2, viewport: { width: 390, height: 844 },
    })
    await use(context)
    await context.close()
  }, { scope: 'worker' }],
})

const mapSettled = (page) => page.waitForFunction(() => {
  const map = document.querySelector('[data-island="map"]')?.__map
  return !!map?.isStyleLoaded?.() && !map.isMoving()
})

const probe = () => {
  const onMap = (el) => !!el && !!el.closest('canvas, .maplibregl-canvas-container')
  const bar = document.querySelector('.map-bar')
  bar.scrollIntoView({ block: 'center' })
  const cs = getComputedStyle(bar)
  const legend = document.querySelector('.map-shell .scale--onmap').getBoundingClientRect()
  const fresh = document.querySelector('.map-freshness')
  const pills = {
    window: document.querySelector('.map-window__btn'),
    play: document.querySelector('.map-play'),
    refresh: document.querySelector('.map-freshness .data-refresh'),
  }
  const boxes = {}
  const backgrounds = {}
  for (const [k, el] of Object.entries(pills)) {
    if (!el || el.getBoundingClientRect().width === 0) continue
    const r = el.getBoundingClientRect()
    boxes[k] = { x: r.x, y: r.y, r: r.right, b: r.bottom }
    backgrounds[k] = getComputedStyle(el).backgroundColor
  }
  const legendBg = getComputedStyle(document.querySelector('.map-shell .scale--onmap')).backgroundColor
  const group = fresh.getBoundingClientRect()
  const y = (Math.max(legend.top, group.top) + Math.min(legend.bottom, group.bottom)) / 2
  // Near the legend rather than the midpoint: the home page's scroll cue sits mid-map.
  const x = legend.right + Math.min(24, (group.left - legend.right) / 2)
  const hit = document.elementFromPoint(x, y)
  const gaps = []
  const ordered = Object.values(boxes).sort((a, b) => a.x - b.x)
  for (let i = 1; i < ordered.length; i++) {
    const gx = (ordered[i - 1].r + ordered[i].x) / 2
    const gy = (ordered[i].y + ordered[i].b) / 2
    gaps.push({ gap: ordered[i].x - ordered[i - 1].r, onMap: onMap(document.elementFromPoint(gx, gy)), hit: String(document.elementFromPoint(gx, gy)?.className) })
  }
  return {
    bg: cs.backgroundColor, filt: cs.backdropFilter || 'none', shadow: cs.boxShadow, border: cs.borderTopWidth,
    backgrounds, legendBg, gaps, room: group.left - legend.right,
    hit: hit ? `${hit.tagName}.${hit.className}` : 'none', hitIsMap: onMap(hit),
  }
}

// Alpha of a computed colour: rgb(), rgba() or color(srgb r g b / a).
const alpha = (c) => {
  const m = c.match(/\/\s*([\d.]+)\)$/) || c.match(/^rgba\(.*,\s*([\d.]+)\)$/)
  return m ? Number(m[1]) : 1
}

const check = (r, label, minRoom = 4) => {
  expect(['rgba(0, 0, 0, 0)', 'transparent'], `${label} bar background ${r.bg}`).toContain(r.bg)
  expect(r.filt, `${label} bar backdrop-filter`).toBe('none')
  expect(r.shadow, `${label} bar box-shadow`).toBe('none')
  expect(r.border, `${label} bar border`).toBe('0px')
  expect(r.room, `${label} room between legend and time group`).toBeGreaterThanOrEqual(minRoom)
  expect(r.hitIsMap, `${label} point between legend and time group hits ${r.hit}`).toBe(true)
  expect(Object.keys(r.backgrounds), `${label} window and play pills`).toEqual(expect.arrayContaining(['window', 'play']))
  for (const [k, bg] of Object.entries(r.backgrounds)) {
    expect(bg, `${label} ${k} pill has its own background`).not.toBe('rgba(0, 0, 0, 0)')
    expect(alpha(bg), `${label} ${k} pill is as opaque as the legend pill`).toBeGreaterThanOrEqual(Math.min(alpha(r.legendBg), 0.95))
  }
  for (const g of r.gaps) {
    expect(g.gap, `${label} visible gap between pills`).toBeGreaterThanOrEqual(4)
    expect(g.onMap, `${label} gap between pills shows the map (hit ${g.hit})`).toBe(true)
  }
}

for (const path of ['/', '/area/sofia']) {
  test(`desktop 1280 ${path}: no strip, separate pills`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.setViewportSize({ width: 1280, height: 900 })
    await page.goto(path)
    await mapSettled(page)
    await expect(page.locator('.map-window__btn')).toBeVisible()
    check(await page.evaluate(probe), `desktop ${path}`)
    await page.close()
  })
  for (const width of [390, 360]) {
    test(`phone ${width} touch ${path}: no strip, separate pills`, async ({ phoneCtx }) => {
      const page = await phoneCtx.newPage()
      await page.setViewportSize({ width, height: 844 })
      await page.goto(path)
      await mapSettled(page)
      await expect(page.locator('.map-window__btn')).toBeVisible()
      check(await page.evaluate(probe), `phone ${width} ${path}`, 8)
      await page.close()
    })
  }
}
