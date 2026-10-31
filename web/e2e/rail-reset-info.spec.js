import { test, expect, mapSettled } from './fixtures.js'

// The map rail ends in reset view then locate, with no table link; the (i) credit
// button sits in the bottom-right corner of the map, clear of the foot bar.

const DESKTOP = [[1440, 900], [1280, 800]]
const TOUCH = [[390, 844], [844, 390], [360, 640]]
const CORNER = 16

const shot = (page, name) => process.env.KANARCHE_SHOT_DIR
  ? page.screenshot({ path: `${process.env.KANARCHE_SHOT_DIR}/${process.env.KANARCHE_SHOT_TAG}-${name}.png` })
  : null

const open = async (browser, [w, h], touch, path) => {
  const ctx = await browser.newContext({ viewport: { width: w, height: h }, ...(touch ? { isMobile: true, hasTouch: true } : {}) })
  const page = await ctx.newPage()
  await page.goto(path)
  await mapSettled(page)
  await page.evaluate(() => document.fonts.ready)
  return { ctx, page }
}

const cases = []
for (const path of ['/', '/en']) {
  for (const vp of DESKTOP) cases.push({ path, vp, touch: false })
  for (const vp of TOUCH) cases.push({ path, vp, touch: true })
}
const label = ({ path, vp, touch }) => `${vp[0]}x${vp[1]} ${touch ? 'touch' : 'desktop'} ${path}`
const tag = ({ path, vp }) => `${vp[0]}x${vp[1]}${path.replace(/\//g, '_')}`

for (const c of cases) {
  test(`${label(c)}: rail ends reset view then locate, no table link`, async ({ browser }) => {
    const { ctx, page } = await open(browser, c.vp, c.touch, c.path)
    await shot(page, `rail-${tag(c)}`)
    // .map-zoom is display: contents on phones, so list its buttons rather than the wrapper.
    const kinds = await page.evaluate(() => [...document.querySelectorAll('.map-rail > :not(.map-locate-hint)')]
      .flatMap((e) => (e.classList.contains('map-zoom') ? [...e.children] : [e]))
      .filter((e) => e.checkVisibility()).map((e) => e.className.split(' ')[0]))
    expect(kinds).toEqual(['map-zoom__btn', 'map-zoom__btn', 'map-orient', 'map__full', 'map-reset', 'map-locate'])
    await expect(page.locator('.map-table')).toHaveCount(0)
    await expect(page.locator('.map-rail a')).toHaveCount(0)
    await expect(page.locator('.map-rail .map-reset')).toHaveAccessibleName(/./)
    await expect(page.locator('.map-orient__panel [data-act="reset"]')).toHaveCount(0)
    await ctx.close()
  })

  test(`${label(c)}: the (i) sits in the map's bottom-right corner, clear of every other control`, async ({ browser }) => {
    const { ctx, page } = await open(browser, c.vp, c.touch, c.path)
    await page.evaluate(() => document.querySelector('.map-shell').scrollIntoView({ block: 'end', behavior: 'instant' }))
    await shot(page, `info-${tag(c)}`)
    const res = await page.evaluate(() => {
      const box = (e) => { const r = e.getBoundingClientRect(); return { l: r.left, t: r.top, r: r.right, b: r.bottom } }
      const shell = box(document.querySelector('.map-shell'))
      const btn = document.querySelector('.maplibregl-ctrl-attrib-button')
      const i = box(btn)
      const hit = (x, y) => { const h = document.elementFromPoint(x, y); return !!h && (h === btn || btn.contains(h)) }
      const others = [...document.querySelectorAll('.map-shell button, .map-shell summary, .map-shell a, .map-shell input, .map-shell select, .map-freshness > *')]
        .filter((e) => e !== btn && !btn.contains(e) && e.checkVisibility() && e.getBoundingClientRect().width > 0)
        .map((e) => ({ name: `${e.tagName.toLowerCase()}.${String(e.className).split(' ')[0]}`, ...box(e) }))
      const over = others.filter((o) => o.l < i.r && o.r > i.l && o.t < i.b && o.b > i.t).map((o) => o.name)
      return {
        right: shell.r - i.r, bottom: shell.b - i.b, w: i.r - i.l, h: i.b - i.t,
        centreHit: hit((i.l + i.r) / 2, (i.t + i.b) / 2),
        // Four points a quarter of the way in from the centre, clear of the rounded corners.
        cornersHit: [[-1, -1], [1, -1], [-1, 1], [1, 1]].map(([dx, dy]) => hit((i.l + i.r) / 2 + dx * (i.r - i.l) / 4, (i.t + i.b) / 2 + dy * (i.b - i.t) / 4)),
        over,
      }
    })
    expect(res.right, 'gap to the right edge').toBeGreaterThanOrEqual(0)
    expect(res.right, 'gap to the right edge').toBeLessThanOrEqual(CORNER)
    expect(res.bottom, 'gap to the bottom edge').toBeGreaterThanOrEqual(0)
    expect(res.bottom, 'gap to the bottom edge').toBeLessThanOrEqual(CORNER)
    expect(res.over).toEqual([])
    expect(res.centreHit).toBe(true)
    if (c.touch) {
      expect(res.cornersHit).toEqual([true, true, true, true])
      expect(res.w).toBeGreaterThanOrEqual(44 - 0.5)
      expect(res.h).toBeGreaterThanOrEqual(44 - 0.5)
    }
    await ctx.close()
  })

  test(`${label(c)}: the opened credit card is fully on screen and covers no control`, async ({ browser }) => {
    const { ctx, page } = await open(browser, c.vp, c.touch, c.path)
    await page.evaluate(() => document.querySelector('.map-shell').scrollIntoView({ block: 'end', behavior: 'instant' }))
    await page.locator('.maplibregl-ctrl-attrib-button').click()
    await expect(page.locator('.maplibregl-ctrl-attrib')).toHaveClass(/maplibregl-compact-show/)
    await shot(page, `info-open-${tag(c)}`)
    const res = await page.evaluate(() => {
      const r = document.querySelector('.maplibregl-ctrl-attrib-inner').getBoundingClientRect()
      const shell = document.querySelector('.map-shell').getBoundingClientRect()
      const btn = document.querySelector('.maplibregl-ctrl-attrib-button')
      const under = [...document.querySelectorAll('.map-shell button, .map-shell summary, .map-shell a, .map-shell input, .map-shell select')]
        .filter((e) => e !== btn && !e.closest('.maplibregl-ctrl-attrib') && e.checkVisibility() && e.getBoundingClientRect().width > 0)
        .filter((e) => { const b = e.getBoundingClientRect(); return b.left < r.right && b.right > r.left && b.top < r.bottom && b.bottom > r.top })
        .map((e) => `${e.tagName.toLowerCase()}.${String(e.className).split(' ')[0]}`)
      return { l: r.left, t: r.top, r: r.right, b: r.bottom, vw: innerWidth, vh: innerHeight, shell: { l: shell.left, t: shell.top, r: shell.right, b: shell.bottom }, under }
    })
    expect(res.l).toBeGreaterThanOrEqual(Math.max(0, res.shell.l))
    expect(res.t).toBeGreaterThanOrEqual(Math.max(0, res.shell.t))
    expect(res.r).toBeLessThanOrEqual(Math.min(res.vw, res.shell.r) + 0.5)
    expect(res.b).toBeLessThanOrEqual(Math.min(res.vh, res.shell.b) + 0.5)
    expect(res.under, 'controls under the open card').toEqual([])
    await ctx.close()
  })
}

for (const [path, vp, touch] of [['/en', [1440, 900], false], ['/', [390, 844], true]]) {
  test(`${vp[0]}x${vp[1]} ${path}: the rail's reset view returns centre, zoom, bearing and pitch to the opening view`, async ({ browser }) => {
    const { ctx, page } = await open(browser, vp, touch, path)
    await page.emulateMedia({ reducedMotion: 'reduce' })
    const home = await page.evaluate(() => {
      const el = document.querySelector('[data-island="map"]')
      return { lon: Number(el.dataset.lon), lat: Number(el.dataset.lat), zoom: Number(el.dataset.zoom) }
    })
    await page.evaluate(() => document.querySelector('[data-island="map"]').__map.jumpTo({ center: [23.3, 42.1], zoom: 9.5, pitch: 40, bearing: 25 }))
    const cam = () => page.evaluate(() => {
      const m = document.querySelector('[data-island="map"]').__map
      const c = m.getCenter()
      return { lon: c.lng, lat: c.lat, zoom: m.getZoom(), pitch: m.getPitch(), bearing: m.getBearing() }
    })
    expect((await cam()).pitch).toBe(40)
    await page.locator('.map-rail .map-reset').click()
    await expect.poll(async () => {
      const c = await cam()
      return {
        lon: Math.abs(c.lon - home.lon) < 0.01, lat: Math.abs(c.lat - home.lat) < 0.01,
        zoom: Math.abs(c.zoom - home.zoom) < 0.05, pitch: c.pitch, bearing: c.bearing,
      }
    }, { timeout: 15_000 }).toEqual({ lon: true, lat: true, zoom: true, pitch: 0, bearing: 0 })
    await ctx.close()
  })
}
