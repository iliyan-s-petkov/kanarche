import { test, expect } from './fixtures.js'

// A wide readout ("1013,0 hPa") must wrap or shrink inside its card: the page
// may not scroll sideways on a phone. Seeded on sensor 101 in e2e_test.go.
for (const width of [320, 360, 390]) {
  for (const path of ['/area/mladost', '/en/area/mladost', '/en/area/sofia']) {
    test(`readouts do not overflow the page: ${path} @${width}`, async ({ ctx }) => {
      const page = await ctx.newPage()
      await page.setViewportSize({ width, height: 844 })
      await page.goto(path)
      await page.waitForTimeout(800)
      const m = await page.evaluate(() => ({
        scroll: document.documentElement.scrollWidth,
        inner: window.innerWidth,
        hasWide: [...document.querySelectorAll('.readout__value')].some((v) => /1013/.test(v.textContent)),
      }))
      expect(m.hasWide, 'fixture pressure card rendered').toBe(true)
      expect(m.scroll, 'document scrollWidth').toBeLessThanOrEqual(m.inner)
      await page.close()
    })
  }
}

// Desktop: every figure and unit lies inside its card. Measured by rect, not scrollWidth.
const geometry = () => {
  const inside = (r, b, tol = 0.5) => r.left >= b.left - tol && r.right <= b.right + tol && r.top >= b.top - tol && r.bottom <= b.bottom + tol
  const out = { cards: [], seen: '' }
  for (const card of document.querySelectorAll('.readouts .readout.card')) {
    if (card.offsetParent === null) continue
    const cb = card.getBoundingClientRect()
    const v = card.querySelector('.readout__value')
    const u = card.querySelector('.readout__unit')
    const rg = document.createRange()
    // The figure's own text node: the spoken unit beside it is visually hidden.
    rg.selectNodeContents(v.firstChild.nodeType === 3 ? v.firstChild : v)
    out.seen += v.textContent.trim() + '|'
    const clipped = v.scrollWidth > v.clientWidth + 1
    out.cards.push({ text: v.textContent.trim(), ok: inside(rg.getBoundingClientRect(), cb) && (!u || inside(u.getBoundingClientRect(), cb)) && !clipped })
  }
  return out
}
for (const width of [1280, 1440, 2000]) {
  for (const path of ['/area/mladost', '/en/area/sofia']) {
    test(`readout figures fit their cards: ${path} @${width}`, async ({ ctx }) => {
      const page = await ctx.newPage()
      await page.setViewportSize({ width, height: 900 })
      await page.goto(path)
      await page.waitForTimeout(800)
      const g = await page.evaluate(geometry)
      expect(g.seen, 'pressure card rendered').toContain('1013')
      expect(g.seen, 'CO card rendered').toContain('2190')
      for (const c of g.cards) expect(c.ok, `${c.text} inside its card`).toBe(true)
      await page.close()
    })
  }
}
