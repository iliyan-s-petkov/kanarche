import { test, expect, mapSettled } from './fixtures.js'

const phoneCtx = (browser, width, height) =>
  browser.newContext({ viewport: { width, height }, isMobile: true, hasTouch: true })

const LANDSCAPE = [[667, 375], [740, 360], [844, 390], [915, 412]]
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
  const overlap = (a, b) => a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top
  const inRow = (r) => r.top >= Math.max(rowTop, mast.bottom) - 0.5 && r.bottom <= rowTop + 48 + 0.5
  const tb = box('.frame > .toolbar')
  // Every visible toolbar child (picker, sensor bar) sits in the row, not above it or over the map.
  const tbKids = [...document.querySelectorAll('.frame > .toolbar > *')].map((e) => e.getBoundingClientRect()).filter((r) => r.width > 2 && r.height > 2)
  return {
    belowMasthead: h1.top >= mast.bottom - 0.5,
    inRow: h1.top >= rowTop - 0.5 && h1.bottom <= rowTop + 48 + 0.5,
    apart: !overlap(crumb, h1),
    toolbarInRow: tbKids.length >= 2 && tbKids.every(inRow),
    // No picker or pill is cut off inside its own box or pushed past the toolbar's end.
    toolbarFits: [...document.querySelectorAll('.frame > .toolbar > *')].filter((e) => e.getBoundingClientRect().width > 2).every((e) => e.scrollWidth <= e.clientWidth + 1 && e.getBoundingClientRect().right <= tb.right + 0.5),
    toolbarApart: !overlap(tb, h1) && !overlap(tb, crumb),
    noOverflow: document.documentElement.scrollWidth <= innerWidth,
  }
}

const wrapTitle = () => {
  const h1 = document.querySelector('.page-head h1')
  const lh = parseFloat(getComputedStyle(h1).lineHeight)
  while (h1.getBoundingClientRect().height < 1.5 * lh) h1.textContent += ' Long'
  // The toolbar's visually hidden texts (picker label, full count) grow with the data and the language.
  for (const e of document.querySelectorAll('.toolbar--map .meta__full, .toolbar--map .colmenu__legend')) e.textContent = 'Long text '.repeat(4)
}

for (const lang of ['bg', 'en']) {
  for (const area of AREAS) {
    for (const [w, h] of LANDSCAPE) {
      test(`${lang} ${w}x${h} /area/${area}: the header row items stay inside the row, clear of each other`, async ({ browser }) => {
        const ctx = await phoneCtx(browser, w, h)
        const page = await ctx.newPage()
        await page.goto(lang === 'en' ? `/en/area/${area}` : `/area/${area}`)
        await mapSettled(page)
        const ok = { belowMasthead: true, inRow: true, apart: true, toolbarInRow: true, toolbarFits: true, toolbarApart: true, noOverflow: true }
        await expect.poll(() => page.evaluate(headerBoxes)).toEqual(ok)
        // The same page with the title forced onto two lines.
        if (process.env.KANARCHE_SHOT_DIR && lang === 'bg' && area === 'sofia') await page.screenshot({ path: `${process.env.KANARCHE_SHOT_DIR}/after-land-sofia-${w}.png` })
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
      if (process.env.KANARCHE_SHOT_DIR && w < 400) await page.screenshot({ path: `${process.env.KANARCHE_SHOT_DIR}/after-port-${w}-${pollen ? 'pollen' : 'nopollen'}.png` })
      await ctx.close()
    })
  }
}

// Landscape rail: every control (zoom pair included) inside the map, 8px above the bottom pills.
const railLandscape = () => {
  const shell = document.querySelector('.map-shell').getBoundingClientRect()
  const items = [...document.querySelectorAll('.map-rail > *, .map-zoom button')]
    .map((el) => el.getBoundingClientRect())
    .filter((r) => r.width > 0 && r.height > 0)
  const zoom = [...document.querySelectorAll('.map-zoom button')].filter((el) => el.getBoundingClientRect().width > 0).length
  const pills = ['.scale--onmap > summary', '.map-window__btn', '.map-play', '.maplibregl-ctrl-attrib-button']
    .map((s) => document.querySelector(s))
    .filter((el) => el && el.getBoundingClientRect().width > 0)
    .map((el) => el.getBoundingClientRect())
  const railBottom = Math.max(...items.map((r) => r.bottom))
  const inside = items.every((r) => r.top >= shell.top && r.left >= shell.left && r.right <= shell.right)
  return { items: items.length, zoom, pills: pills.length, inside, gap: Math.min(...pills.map((r) => r.top)) - railBottom }
}

for (const lang of ['bg', 'en']) {
  for (const [w, h] of LANDSCAPE) {
    test(`${lang} ${w}x${h} /area/sofia: the rail, zoom included, sits inside the map 8px above the pills`, async ({ browser }) => {
      const ctx = await phoneCtx(browser, w, h)
      const page = await ctx.newPage()
      await page.goto(lang === 'en' ? '/en/area/sofia' : '/area/sofia')
      await mapSettled(page)
      await expect.poll(async () => {
        const g = await page.evaluate(railLandscape)
        // Guard the measure: a missing zoom pair or pills would pass vacuously.
        if (g.items < 6 || g.zoom !== 2 || g.pills < 3 || !g.inside) return -999
        return g.gap
      }).toBeGreaterThanOrEqual(8)
      await ctx.close()
    })
  }
}
