import { readFileSync } from 'node:fs'
import { test, expect, mapSettled } from './fixtures.js'

// Every real area h1, rendered by the Go test TestRealAreaH1Texts from data/boundaries.
const TEXTS = JSON.parse(readFileSync(new URL('./area-h1-texts.json', import.meta.url), 'utf8'))
const PORTRAIT = [[360, 640], [375, 667], [390, 844]]
// The portrait layout budgets one or two h1 lines above the map and tab.
const MAX_LINES = 2

// Puts each text into the real h1 and returns the ones that wrap to more than max lines.
const overBudget = ({ rows, max }) => {
  const h1 = document.querySelector('.page-head h1')
  const lh = parseFloat(getComputedStyle(h1).lineHeight)
  const bad = []
  for (const r of rows) {
    h1.textContent = r.h1
    const lines = Math.round(h1.getBoundingClientRect().height / lh)
    if (lines > max) bad.push(`${r.slug} (${lines} lines): ${r.h1}`)
  }
  return { bad, lh, width: h1.getBoundingClientRect().width }
}

for (const lang of ['bg', 'en']) {
  for (const [w, h] of PORTRAIT) {
    test(`${lang} ${w}x${h}: no real area h1 needs more than ${MAX_LINES} lines`, async ({ browser }) => {
      const ctx = await browser.newContext({ viewport: { width: w, height: h }, isMobile: true, hasTouch: true })
      const page = await ctx.newPage()
      await page.goto(lang === 'en' ? '/en/area/sofia' : '/area/sofia')
      await mapSettled(page)
      await page.evaluate(() => document.fonts.ready)
      const rows = TEXTS.filter((t) => t.lang === lang)
      expect(rows.length).toBeGreaterThan(70)
      const res = await page.evaluate(overBudget, { rows, max: MAX_LINES })
      // A line height of 0 would make every h1 look like one line.
      expect(res.lh).toBeGreaterThan(10)
      expect(res.bad, `${lang} ${w}x${h}, h1 width ${res.width}px`).toEqual([])
      await ctx.close()
    })
  }
}
