import { test, expect, mapSettled } from './fixtures.js'

// Home chrome on touch phones: the finder clears the map rail, the masthead
// items keep a gap, and the focused skip link is whole and on top.

const TOUCH = [[360, 640], [375, 667], [390, 844], [412, 915], [667, 375], [844, 390], [915, 412]]
const MASTHEAD_W = [360, 375, 390, 412]
const GAP = 8

const touchPage = async (browser, w, h) => {
  const ctx = await browser.newContext({ viewport: { width: w, height: h }, isMobile: true, hasTouch: true })
  const page = await ctx.newPage()
  return { ctx, page }
}

const shot = (page, name, clip) => process.env.KANARCHE_SHOT_DIR
  ? page.screenshot({ path: `${process.env.KANARCHE_SHOT_DIR}/${process.env.KANARCHE_SHOT_TAG}-${name}.png`, ...(clip ? { clip } : {}) })
  : null

// Gap between two boxes, negative when they overlap.
const gapOf = (a, b) => Math.max(a.left - b.right, b.left - a.right, a.top - b.bottom, b.top - a.bottom)

for (const path of ['/', '/en']) {
  for (const [w, h] of TOUCH) {
    test(`${w}x${h} touch ${path}: the finder keeps ${GAP}px from every rail button`, async ({ browser }) => {
      const { ctx, page } = await touchPage(browser, w, h)
      await page.goto(path)
      await mapSettled(page)
      await page.evaluate(() => document.fonts.ready)
      await shot(page, `finder-${w}x${h}${path.replace(/\//g, '_')}`)
      const res = await page.evaluate(() => {
        const box = (e) => { const r = e.getBoundingClientRect(); return { left: r.left, right: r.right, top: r.top, bottom: r.bottom, width: r.width } }
        const find = document.querySelector('.map-controls__find .input')
        const rail = [...document.querySelectorAll('.map-rail button, .map-rail a')].filter((e) => e.checkVisibility())
        return { find: box(find), rail: rail.map((e) => ({ name: e.className, ...box(e) })) }
      })
      expect(res.rail.length).toBeGreaterThan(0)
      const close = res.rail.filter((r) => gapOf(res.find, r) < GAP).map((r) => `${r.name} gap ${Math.round(gapOf(res.find, r))}`)
      expect(close).toEqual([])
      // Still a field a person can type in: wide enough for a few characters, the placeholder ends in an ellipsis.
      expect(res.find.width).toBeGreaterThanOrEqual(88)
      await ctx.close()
    })
  }
}

for (const path of ['/', '/en']) {
  for (const w of MASTHEAD_W) {
    test(`${w} touch ${path}: masthead items keep ${GAP}px apart and no hit boxes overlap`, async ({ browser }) => {
      const { ctx, page } = await touchPage(browser, w, 844)
      await page.goto(path)
      await mapSettled(page)
      await page.evaluate(() => document.fonts.ready)
      await shot(page, `masthead-${w}${path.replace(/\//g, '_')}`, { x: 0, y: 0, width: w, height: 64 })
      const res = await page.evaluate(() => {
        const items = [...document.querySelectorAll('.masthead__brand, .masthead__link, .masthead .langpick > summary, .masthead .langpick__btn')]
          .filter((e) => e.checkVisibility())
        const name = (e) => `${e.className.trim().split(/\s+/)[0]} "${e.textContent.trim().replace(/\s+/g, ' ').slice(0, 14)}"`
        const rects = items.map((e) => ({ e, r: e.getBoundingClientRect() }))
        // The hit box is the box grown to at least 44px each way, about its centre.
        const hit = ({ r }) => {
          const cx = (r.left + r.right) / 2, cy = (r.top + r.bottom) / 2
          const hw = Math.max(r.width, 44) / 2, hh = Math.max(r.height, 44) / 2
          return { left: cx - hw, right: cx + hw, top: cy - hh, bottom: cy + hh }
        }
        const out = { gaps: [], overlaps: [], count: items.length, over: document.documentElement.scrollWidth - innerWidth }
        for (let i = 0; i < rects.length; i++) {
          for (let j = i + 1; j < rects.length; j++) {
            const a = rects[i], b = rects[j]
            if (a.e.contains(b.e) || b.e.contains(a.e)) continue
            const dx = Math.max(a.r.left - b.r.right, b.r.left - a.r.right)
            const dy = Math.max(a.r.top - b.r.bottom, b.r.top - a.r.bottom)
            const g = Math.max(dx, dy)
            if (g < 8) out.gaps.push(`${name(a.e)} / ${name(b.e)} gap ${Math.round(g)}`)
            const ha = hit(a), hb = hit(b)
            if (Math.min(ha.right, hb.right) - Math.max(ha.left, hb.left) > 1 && Math.min(ha.bottom, hb.bottom) - Math.max(ha.top, hb.top) > 1) out.overlaps.push(`${name(a.e)} / ${name(b.e)}`)
          }
        }
        return out
      })
      expect(res.count).toBeGreaterThan(2)
      expect(res.over).toBeLessThanOrEqual(0)
      expect(res.gaps).toEqual([])
      expect(res.overlaps).toEqual([])
      await ctx.close()
    })
  }
}

// The skip link, once focused by keyboard, is whole inside the viewport and nothing is drawn over it.
const SKIP_VIEWPORTS = [[1280, 800, false], [360, 640, true], [390, 844, true], [844, 390, true]]
for (const path of ['/', '/en']) {
  for (const [w, h, touch] of SKIP_VIEWPORTS) {
    test(`${w}x${h}${touch ? ' touch' : ''} ${path}: the focused skip link is fully visible on top`, async ({ browser }) => {
      const ctx = await browser.newContext({ viewport: { width: w, height: h }, ...(touch ? { isMobile: true, hasTouch: true } : {}) })
      const page = await ctx.newPage()
      await page.goto(path)
      await page.evaluate(() => document.fonts.ready)
      await page.keyboard.press('Tab')
      await shot(page, `skip-${w}x${h}${path.replace(/\//g, '_')}`, { x: 0, y: 0, width: w, height: 90 })
      const res = await page.evaluate(() => {
        const a = document.activeElement
        if (!a || !a.classList.contains('skip')) return { focused: false }
        const r = a.getBoundingClientRect()
        const covered = []
        // Inset sample points, so a rounded corner is not a miss.
        for (const [fx, fy] of [[0.1, 0.2], [0.5, 0.5], [0.9, 0.2], [0.1, 0.8], [0.9, 0.8]]) {
          const hit = document.elementFromPoint(r.left + r.width * fx, r.top + r.height * fy)
          if (!(hit && (hit === a || a.contains(hit)))) covered.push(hit ? `${hit.tagName.toLowerCase()}.${hit.className}` : 'none')
        }
        return { focused: true, left: r.left, top: r.top, right: r.right, vw: innerWidth, covered }
      })
      expect(res.focused).toBe(true)
      expect(res.left).toBeGreaterThanOrEqual(0)
      expect(res.top).toBeGreaterThanOrEqual(0)
      expect(res.right).toBeLessThanOrEqual(res.vw)
      expect(res.covered).toEqual([])
      await ctx.close()
    })
  }
}
