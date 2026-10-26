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

// Time controls sit at the bottom-left, right after the legend. Boxes are measured
// relative to the map shell so scroll position does not matter.
const rowProbe = () => {
  const shell = document.querySelector('.map-shell').getBoundingClientRect()
  const rel = (el) => {
    if (!el) return null
    const r = el.getBoundingClientRect()
    if (r.width === 0) return null
    return { x: Math.round(r.left - shell.left), y: Math.round(r.top - shell.top), r: Math.round(r.right - shell.left), b: Math.round(r.bottom - shell.top) }
  }
  const q = (s) => document.querySelector(s)
  return {
    vw: document.documentElement.clientWidth, sw: document.documentElement.scrollWidth,
    legend: rel(q('.map-shell .scale--onmap')),
    group: rel(q('.map-freshness')),
    refresh: rel(q('.map-freshness .data-refresh')),
    window: rel(q('.map-window__btn')),
    play: rel(q('.map-play')),
    attrib: rel(q('.maplibregl-ctrl-attrib')),
    attribBtn: rel(q('.maplibregl-ctrl-attrib-button')),
  }
}

const overlaps = (a, b) => a && b && a.x < b.r && b.x < a.r && a.y < b.b && b.y < a.b

for (const path of ['/', '/area/sofia', '/en', '/en/area/sofia']) {
  for (const [w, h] of [[1280, 800], [1440, 900]]) {
    test(`desktop ${w} ${path}: time controls follow the legend`, async ({ ctx }) => {
      const page = await ctx.newPage()
      await page.setViewportSize({ width: w, height: h })
      await page.goto(path)
      await mapSettled(page)
      await expect(page.locator('.map-window__btn')).toBeVisible()
      const m = await page.evaluate(rowProbe)
      const label = `${w} ${path}`
      const first = m.refresh || m.window
      const gap = first.x - m.legend.r
      expect(gap, `${label} gap legend to first time pill`).toBeGreaterThanOrEqual(8)
      expect(gap, `${label} gap legend to first time pill`).toBeLessThanOrEqual(16)
      const mid = (b) => (b.y + b.b) / 2
      for (const k of ['refresh', 'window', 'play']) {
        if (m[k]) expect(Math.abs(mid(m[k]) - mid(m.legend)), `${label} ${k} centre vs legend`).toBeLessThanOrEqual(2)
      }
      for (const k of ['group', 'refresh', 'window', 'play']) {
        expect(overlaps(m[k], m.attrib), `${label} ${k} overlaps attribution`).toBeFalsy()
        expect(overlaps(m[k], m.attribBtn), `${label} ${k} overlaps info button`).toBeFalsy()
      }
      expect(m.sw, `${label} horizontal overflow`).toBeLessThanOrEqual(m.vw)
      await page.close()
    })
  }
}

// Phone positions as measured on master 6ff505a, pinned so the desktop rules cannot move them.
const phonePins = {
  "390x844 /": {"legend":{"x":8,"y":688,"r":192,"b":732},"group":{"x":214,"y":688,"r":330,"b":732},"refresh":null,"window":{"x":214,"y":688,"r":274,"b":732},"play":{"x":278,"y":688,"r":330,"b":732},"attrib":{"x":333,"y":687,"r":377,"b":731},"attribBtn":{"x":333,"y":687,"r":377,"b":731}},
  "360x640 /": {"legend":{"x":8,"y":484,"r":176,"b":528},"group":{"x":184,"y":484,"r":300,"b":528},"refresh":null,"window":{"x":184,"y":484,"r":244,"b":528},"play":{"x":248,"y":484,"r":300,"b":528},"attrib":{"x":303,"y":483,"r":347,"b":527},"attribBtn":{"x":303,"y":483,"r":347,"b":527}},
  "390x844 /area/sofia": {"legend":{"x":8,"y":464,"r":174,"b":508},"group":{"x":182,"y":464,"r":298,"b":508},"refresh":null,"window":{"x":182,"y":464,"r":242,"b":508},"play":{"x":246,"y":464,"r":298,"b":508},"attrib":{"x":301,"y":463,"r":345,"b":507},"attribBtn":{"x":301,"y":463,"r":345,"b":507}},
  "360x640 /area/sofia": {"legend":{"x":8,"y":260,"r":144,"b":304},"group":{"x":152,"y":260,"r":268,"b":304},"refresh":null,"window":{"x":152,"y":260,"r":212,"b":304},"play":{"x":216,"y":260,"r":268,"b":304},"attrib":{"x":271,"y":259,"r":315,"b":303},"attribBtn":{"x":271,"y":259,"r":315,"b":303}},
}
for (const path of ['/', '/area/sofia']) {
  for (const [w, h] of [[390, 844], [360, 640]]) {
    test(`phone ${w}x${h} ${path}: bottom row unchanged`, async ({ phoneCtx }) => {
      const page = await phoneCtx.newPage()
      await page.setViewportSize({ width: w, height: h })
      await page.goto(path)
      await mapSettled(page)
      await expect(page.locator('.map-window__btn')).toBeVisible()
      const { vw, sw, ...boxes } = await page.evaluate(rowProbe)
      if (!phonePins) { console.log('PIN ' + JSON.stringify({ key: `${w}x${h} ${path}`, boxes })); return }
      expect(boxes).toEqual(phonePins[`${w}x${h} ${path}`])
      expect(sw).toBeLessThanOrEqual(vw)
      await page.close()
    })
  }
}

// Expanded legend: it grows up from the collapsed pill's bottom edge, with no gap below it,
// the time controls stay bottom-aligned beside it, and it clears the map top and the rail.
const mockWindOpen = (page) => page.route('**/api/v1/wind', (route) => route.fulfill({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify({
    generated_at: new Date().toISOString(),
    valid_at: new Date().toISOString(),
    model: 'Test Model',
    model_resolution_deg: 0.25,
    resolution_km: 25,
    forecast: true,
    vectors: [{ lon: 23.3, lat: 42.68, speed_ms: 3.2, direction_deg: 180 }],
  }),
}))

const legendBoxes = () => {
  const shell = document.querySelector('.map-shell').getBoundingClientRect()
  const rel = (el) => {
    const r = el.getBoundingClientRect()
    return { x: r.left - shell.left, y: r.top - shell.top, r: r.right - shell.left, b: r.bottom - shell.top }
  }
  const rail = [...document.querySelectorAll('.map__layers, .map-zoom, .map__full, .map-locate, .map-table, .map-orient__btn, .map-metric')]
    .filter((e) => e.getBoundingClientRect().width > 0).map(rel)
  const pills = [...document.querySelectorAll('.map-freshness .data-refresh, .map-window__btn, .map-play')]
    .filter((e) => e.getBoundingClientRect().width > 0).map(rel)
  return { legend: rel(document.querySelector('.map-shell .scale--onmap')), rail, pills }
}

for (const path of ['/en', '/en/area/sofia']) {
  for (const [w, h] of [[1280, 800], [1440, 900]]) {
    for (const layers of [[], ['Wind', 'Bathing water']]) {
      test(`desktop ${w}x${h} ${path} layers ${layers.length}: open legend grows up from the pill`, async ({ ctx }) => {
        const page = await ctx.newPage()
        await mockWindOpen(page)
        await page.addInitScript(() => localStorage.setItem('kanarche:legend-open', 'false'))
        await page.setViewportSize({ width: w, height: h })
        await page.goto(path)
        await mapSettled(page)
        const legend = page.locator('.map-shell .scale--onmap')
        if (layers.length) {
          await page.locator('.map__layers .colmenu__btn').click()
          for (const name of layers) await page.locator('.map__layers').getByLabel(name, { exact: true }).check()
          await page.keyboard.press('Escape')
        }
        await expect(legend).not.toHaveAttribute('open', '')
        const closed = await page.evaluate(legendBoxes)
        await legend.locator('.scale__toggle').click()
        await expect(legend).toHaveAttribute('open', '')
        const open = await page.evaluate(legendBoxes)
        const label = `${w} ${path} ${layers}`
        expect(Math.abs(open.legend.b - closed.legend.b), `${label} open bottom vs collapsed bottom`).toBeLessThanOrEqual(2)
        expect(open.legend.y, `${label} open legend top inside the map`).toBeGreaterThanOrEqual(0)
        for (const p of open.pills) {
          expect(p.x, `${label} time pill beside, not under, the legend`).toBeGreaterThanOrEqual(open.legend.r + 8 - 1)
          expect(Math.abs(p.b - open.legend.b), `${label} time pill bottom-aligned with the legend`).toBeLessThanOrEqual(8)
        }
        for (const r of open.rail) {
          expect(overlaps(open.legend, r), `${label} open legend overlaps the rail`).toBeFalsy()
        }
        await page.close()
      })
    }
  }
}

test('desktop 1440 open legend: the toggle shares the title row', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.addInitScript(() => localStorage.setItem('kanarche:legend-open', 'true'))
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/en')
  await mapSettled(page)
  await expect(page.locator('.map-shell .scale--onmap')).toHaveAttribute('open', '')
  const r = await page.evaluate(() => {
    const t = document.querySelector('.map-shell .scale__toggle').getBoundingClientRect()
    const l = document.querySelector('.map-shell .scale__label').getBoundingClientRect()
    const bands = document.querySelector('.map-shell .scale__bands--vertical').getBoundingClientRect()
    return { tTop: t.top, tBottom: t.bottom, lTop: l.top, lBottom: l.bottom, bands: bands.width }
  })
  expect(r.lTop, 'title starts within the toggle row').toBeLessThan(r.tBottom - 4)
  expect(r.tTop, 'toggle starts within the title row').toBeLessThan(r.lBottom - 4)
  expect(r.bands, 'the colour rows keep the full card width').toBeGreaterThanOrEqual(250)
  await page.close()
})

for (const w of [1280, 1440]) {
  test(`desktop ${w} open replay row stays clear of the scroll cue`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.setViewportSize({ width: w, height: 800 })
    await page.goto('/en')
    await mapSettled(page)
    const play = page.locator('.map-play__btn[aria-pressed]')
    await play.evaluate((el) => el.scrollIntoView({ block: 'center' }))
    const boxes = () => page.evaluate(() => {
      const box = (s) => { const e = document.querySelector(s); return e && e.getBoundingClientRect().toJSON() }
      return { refresh: box('.map-freshness .data-refresh'), win: box('.map-window__btn'), play: box('.map-play'), cue: box('.scroll-cue') }
    })
    const closed = await boxes()
    await play.click()
    await expect(page.locator('.map-play--open')).toHaveCount(1)
    const open = await boxes()
    expect(open.cue, 'the scroll cue is on the home map').not.toBeNull()
    // The pills stay where they were and the open row stands above them, same left edge.
    expect(Math.abs(open.refresh.left - closed.refresh.left), 'time group left edge').toBeLessThanOrEqual(2)
    expect(Math.abs(open.win.left - closed.win.left), 'window pill does not move').toBeLessThanOrEqual(2)
    expect(Math.abs(open.play.left - closed.refresh.left), 'open row starts at the group left edge').toBeLessThanOrEqual(2)
    expect(open.play.bottom, 'open row sits above the pills').toBeLessThanOrEqual(open.refresh.top)
    expect(open.play.right, 'open row ends 8px before the scroll cue').toBeLessThanOrEqual(open.cue.left - 8)
    await page.close()
  })
}
