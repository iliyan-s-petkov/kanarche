import { test as base, expect } from './fixtures.js'

// Phone 390x844 touch: the foot bar must leave the attribution (i) reachable
// and keep the folded legend name on one line.
const test = base.extend({
  phoneCtx: [async ({ browser }, use) => {
    const context = await browser.newContext({
      isMobile: true, hasTouch: true, deviceScaleFactor: 2, viewport: { width: 390, height: 844 },
    })
    await use(context)
    await context.close()
  }, { scope: 'worker' }],
})

test.afterEach(async ({ phoneCtx }) => {
  for (const p of phoneCtx.pages()) {
    if (!p.isClosed()) await p.evaluate(() => localStorage.clear()).catch(() => {})
    if (!p.isClosed()) await p.close().catch(() => {})
  }
})

const mapSettled = (page) => page.waitForFunction(() => {
  const map = document.querySelector('[data-island="map"]')?.__map
  return !!map?.isStyleLoaded?.() && !map.isMoving()
})

for (const path of ['/en/area/sofia', '/en']) {
  test.describe(`phone foot bar on ${path}`, () => {
    for (const width of [390, 360]) {
    test(`attribution toggle at ${width}px is visible, inside the map, 44px, and on top`, async ({ phoneCtx }) => {
      const page = await phoneCtx.newPage()
      await page.setViewportSize({ width, height: 844 })
      await page.goto(path)
      await mapSettled(page)
      const btn = page.locator('.maplibregl-ctrl-attrib-button')
      await expect(btn).toBeVisible()
      const r = await page.evaluate(() => {
        const b = document.querySelector('.maplibregl-ctrl-attrib-button')
        b.scrollIntoView({ block: 'center' })
        const box = b.getBoundingClientRect()
        const map = document.querySelector('[data-island="map"]').getBoundingClientRect()
        // The bar is pointer-events:none, so a plain hit test sees through it even when
        // it paints over the button. Force hit testing on so it follows paint order
        // (CSSOM, since the CSP blocks an injected <style>).
        const shell = document.querySelector('.map-shell')
        const forced = [shell, ...shell.querySelectorAll('*')]
        for (const e of forced) e.style.setProperty('pointer-events', 'auto', 'important')
        const hit = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2)
        for (const e of forced) e.style.removeProperty('pointer-events')
        return {
          box: { x: box.x, y: box.y, w: box.width, h: box.height },
          map: { x: map.x, y: map.y, r: map.right, b: map.bottom },
          onTop: !!hit && (hit === b || b.contains(hit)),
          hit: hit ? `${hit.tagName}.${hit.className}` : 'none',
        }
      })
      expect(r.box.w, 'target width').toBeGreaterThanOrEqual(44)
      expect(r.box.h, 'target height').toBeGreaterThanOrEqual(44)
      expect(r.box.x, 'left edge in map').toBeGreaterThanOrEqual(r.map.x)
      expect(r.box.y, 'top edge in map').toBeGreaterThanOrEqual(r.map.y)
      expect(r.box.x + r.box.w, 'right edge in map').toBeLessThanOrEqual(r.map.r + 0.5)
      expect(r.box.y + r.box.h, 'bottom edge in map').toBeLessThanOrEqual(r.map.b + 0.5)
      expect(r.onTop, `element at centre: ${r.hit}`).toBe(true)
      // Not under the play button either.
      const play = await page.locator('.map-bar .map-play').first().boundingBox()
      if (play) {
        const apart = play.x + play.width <= r.box.x + 0.5 || r.box.x + r.box.w <= play.x + 0.5 ||
          play.y + play.height <= r.box.y + 0.5 || r.box.y + r.box.h <= play.y + 0.5
        expect(apart, 'play button clear of the attribution toggle').toBe(true)
      }
      await page.close()
    })
    }

    for (const width of [390, 360]) {
      test(`folded legend name stays on one line at ${width}px without crowding the bar`, async ({ phoneCtx }) => {
        const page = await phoneCtx.newPage()
        await page.setViewportSize({ width, height: 844 })
        await page.goto(path)
        await mapSettled(page)
        const label = page.locator('.map-bar .scale--onmap .scale__toggle-label')
        await expect(label).toBeVisible()
        const m = await label.evaluate((el) => {
          const cs = getComputedStyle(el)
          const lh = parseFloat(cs.lineHeight) || parseFloat(cs.fontSize) * 1.5
          const pill = el.closest('.scale--onmap').getBoundingClientRect()
          const time = document.querySelector('.map-bar .map-freshness').getBoundingClientRect()
          return { h: el.getBoundingClientRect().height, lh, rects: el.getClientRects().length, pillEnd: pill.right, timeStart: time.left }
        })
        expect(m.h, `label ${m.h}px tall at line-height ${m.lh}px`).toBeLessThan(m.lh * 1.5)
        expect(m.rects, 'label line boxes').toBe(1)
        expect(m.pillEnd, 'legend pill ends before the time controls').toBeLessThanOrEqual(m.timeStart + 0.5)
        expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width)
        await page.close()
      })
    }
  })
}
