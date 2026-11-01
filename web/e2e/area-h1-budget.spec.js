import { readFileSync } from 'node:fs'
import { test, expect, mapSettled } from './fixtures.js'

// Every real area h1, rendered by the Go test TestRealAreaH1Texts from data/boundaries.
const TEXTS = JSON.parse(readFileSync(new URL('./area-h1-texts.json', import.meta.url), 'utf8'))
const PORTRAIT = [[360, 640], [375, 667], [390, 844]]
// The portrait layout budgets one or two h1 lines above the map and tab.
const MAX_LINES = 2

// Puts each text into the real h1 and returns the rows that break the budget, with the reason.
// A word wider than the h1 does not wrap, so it counts as one line while overflowing;
// scrollWidth > clientWidth is what catches that case.
const overBudget = ({ rows, max }) => {
  const h1 = document.querySelector('.page-head h1')
  const lh = parseFloat(getComputedStyle(h1).lineHeight)
  const bad = []
  for (const r of rows) {
    h1.textContent = r.h1
    const lines = Math.round(h1.getBoundingClientRect().height / lh)
    const reasons = []
    if (lines > max) reasons.push(`${lines} lines`)
    if (h1.scrollWidth > h1.clientWidth) reasons.push(`overflow ${h1.scrollWidth}>${h1.clientWidth}px`)
    if (reasons.length) bad.push(`${r.slug} (${reasons.join(', ')}): ${r.h1}`)
  }
  return { bad, lh, width: h1.getBoundingClientRect().width }
}

for (const lang of ['bg', 'en']) {
  for (const [w, h] of PORTRAIT) {
    test(`${lang} ${w}x${h}: no real area h1 needs more than ${MAX_LINES} lines`, async ({ browser }) => {
      const ctx = await browser.newContext({ viewport: { width: w, height: h }, isMobile: true, hasTouch: true })
      // The context closes even when an expect fails, so a failing run leaves no Chrome behind.
      try {
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
      } finally {
        await ctx.close()
      }
    })
  }
}

// Phone landscape: the h1 sits in a narrow column of the header row, which must grow for a 3-line name.
const LANDSCAPE = [[844, 390], [667, 375]]
// h1 top measured on master with the fixed 48px row (row at 40..88, h1 centred): the row must not move these.
const PINNED_TOP = { sofia: 54, 'varna-oblast': 44 }
const TOL = 0.5

// Puts each text into the real h1 and returns the names whose h1 box leaves the first grid row
// or reaches into the masthead, plus the h1 top of the pinned names.
const landscapeFit = ({ rows, pins, tol }) => {
  const h1 = document.querySelector('.page-head h1')
  const frame = h1.closest('.frame')
  const cs = getComputedStyle(frame)
  const rowH = parseFloat(cs.gridTemplateRows)
  const rowTop = frame.getBoundingClientRect().top + parseFloat(cs.paddingTop) + parseFloat(cs.borderTopWidth)
  const mast = document.querySelector('.masthead').getBoundingClientRect().bottom
  const bad = []
  const tops = {}
  for (const r of rows) {
    h1.textContent = r.h1
    // The row is re-read per text: it grows with the name.
    const rh = parseFloat(getComputedStyle(frame).gridTemplateRows)
    const b = h1.getBoundingClientRect()
    const why = []
    if (b.top < rowTop - tol) why.push(`top ${b.top} above row top ${rowTop}`)
    if (b.bottom > rowTop + rh + tol) why.push(`bottom ${b.bottom} below row bottom ${rowTop + rh}`)
    if (b.top < mast - tol) why.push(`top ${b.top} overlaps masthead bottom ${mast}`)
    if (why.length) bad.push(`${r.slug} (${why.join(', ')})`)
    if (r.slug in pins) tops[r.slug] = b.top
  }
  return { bad, tops, rowH }
}

for (const lang of ['bg', 'en']) {
  for (const [w, h] of LANDSCAPE) {
    test(`${lang} ${w}x${h} landscape: every real area h1 stays inside its grid row`, async ({ browser }) => {
      const ctx = await browser.newContext({ viewport: { width: w, height: h }, isMobile: true, hasTouch: true })
      try {
        const page = await ctx.newPage()
        await page.goto(lang === 'en' ? '/en/area/sofia' : '/area/sofia')
        await mapSettled(page)
        await page.evaluate(() => document.fonts.ready)
        const rows = TEXTS.filter((t) => t.lang === lang)
        expect(rows.length).toBeGreaterThan(70)
        const res = await page.evaluate(landscapeFit, { rows, pins: PINNED_TOP, tol: TOL })
        expect(res.bad, `${lang} ${w}x${h}`).toEqual([])
      } finally {
        await ctx.close()
      }
    })
  }
}

for (const [w, h] of LANDSCAPE) {
  test(`en ${w}x${h} landscape: 1- and 2-line h1 keep their top`, async ({ browser }) => {
    const ctx = await browser.newContext({ viewport: { width: w, height: h }, isMobile: true, hasTouch: true })
    try {
      const page = await ctx.newPage()
      await page.goto('/en/area/sofia')
      await mapSettled(page)
      await page.evaluate(() => document.fonts.ready)
      const rows = TEXTS.filter((t) => t.lang === 'en' && t.slug in PINNED_TOP)
      expect(rows.map((r) => r.slug).sort()).toEqual(Object.keys(PINNED_TOP).sort())
      const res = await page.evaluate(landscapeFit, { rows, pins: PINNED_TOP, tol: TOL })
      for (const [slug, top] of Object.entries(PINNED_TOP)) {
        expect(Math.abs(res.tops[slug] - top), `${slug} ${w}x${h}: h1 top ${res.tops[slug]}, pinned ${top}`).toBeLessThanOrEqual(1)
      }
    } finally {
      await ctx.close()
    }
  })
}
