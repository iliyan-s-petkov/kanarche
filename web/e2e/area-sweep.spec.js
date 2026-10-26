import { test, expect, mapSettled } from './fixtures.js'

const phoneCtx = (browser, width, height) =>
  browser.newContext({ viewport: { width, height }, isMobile: true, hasTouch: true })

const LANDSCAPE = [[667, 375], [844, 390], [915, 412]]
const PORTRAIT = [[360, 640], [375, 667], [390, 844], [412, 915]]
// Fixture areas: sofia is a short name, mladost the longest. A wrapped title is forced below.
const AREAS = ['sofia', 'mladost']

// Header geometry in the landscape grid: the title's box against its 48px row and the breadcrumb.
const headerBoxes = () => {
  const box = (s) => document.querySelector(s).getBoundingClientRect()
  const h1 = box('.page-head h1')
  const crumb = box('.breadcrumb')
  const nav = box('.frame > nav')
  const mast = box('.masthead')
  // The nav is centred in the 48px row, so the row spans its midpoint plus or minus 24px.
  const rowTop = (nav.top + nav.bottom) / 2 - 24
  return {
    belowMasthead: h1.top >= mast.bottom - 0.5,
    inRow: h1.top >= rowTop - 0.5 && h1.bottom <= rowTop + 48 + 0.5,
    apart: !(crumb.left < h1.right && crumb.right > h1.left && crumb.top < h1.bottom && crumb.bottom > h1.top),
  }
}

const wrapTitle = () => {
  const h1 = document.querySelector('.page-head h1')
  const lh = parseFloat(getComputedStyle(h1).lineHeight)
  while (h1.getBoundingClientRect().height < 1.5 * lh) h1.textContent += ' Long'
}

for (const lang of ['bg', 'en']) {
  for (const area of AREAS) {
    for (const [w, h] of LANDSCAPE) {
      test(`${lang} ${w}x${h} /area/${area}: the h1 stays in its header row, clear of the breadcrumb`, async ({ browser }) => {
        const ctx = await phoneCtx(browser, w, h)
        const page = await ctx.newPage()
        await page.goto(lang === 'en' ? `/en/area/${area}` : `/area/${area}`)
        await mapSettled(page)
        const ok = { belowMasthead: true, inRow: true, apart: true }
        await expect.poll(() => page.evaluate(headerBoxes)).toEqual(ok)
        // The same page with the title forced onto two lines.
        await page.evaluate(wrapTitle)
        await expect.poll(() => page.evaluate(headerBoxes)).toEqual(ok)
        await ctx.close()
      })
    }
  }
}

// The rail's last button and the top of the bottom pills (legend, time window, play, refresh, attribution).
const railGap = () => {
  const rail = [...document.querySelectorAll('.map-rail > *')]
    .map((el) => el.getBoundingClientRect())
    .filter((r) => r.width > 0 && r.height > 0)
  const railBottom = Math.max(...rail.map((r) => r.bottom))
  const pills = ['.scale--onmap > summary', '.map-window__btn', '.map-play', '.map-freshness .data-refresh', '.maplibregl-ctrl-attrib-button']
    .map((s) => document.querySelector(s))
    .filter((el) => el && el.getBoundingClientRect().width > 0)
    .map((el) => el.getBoundingClientRect())
  const pillsTop = Math.min(...pills.map((r) => r.top))
  // Every rail item keeps one right edge and its 44px target.
  const edges = new Set(rail.map((r) => Math.round(r.right)))
  const small = rail.filter((r) => r.height < 43.5).length
  return { railBottom, pillsTop, buttons: rail.length, pills: pills.length, aligned: edges.size === 1, small }
}

for (const [w, h] of PORTRAIT) {
  for (const pollen of [true, false]) {
    test(`${w}x${h} /area/sofia ${pollen ? 'with' : 'without'} pollen: the rail ends 8px above the bottom pills`, async ({ browser }) => {
      const ctx = await phoneCtx(browser, w, h)
      const page = await ctx.newPage()
      await page.goto('/area/sofia')
      await mapSettled(page)
      if (pollen) await expect(page.locator('.toolbar--map .pollen-chip')).toBeVisible()
      else await page.evaluate(() => document.querySelector('.toolbar--map .pollen-chip')?.remove())
      await expect.poll(async () => {
        const g = await page.evaluate(railGap)
        // Guard the measure itself: a rail and pills that went missing would pass vacuously.
        if (g.buttons < 5 || g.pills < 3 || !g.aligned || g.small) return -999
        return g.pillsTop - g.railBottom
      }).toBeGreaterThanOrEqual(8)
      await ctx.close()
    })
  }
}
