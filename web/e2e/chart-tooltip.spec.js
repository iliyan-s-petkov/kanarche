import { test, expect } from './fixtures.js'

// The reading card at the hovered point of the map dock's chart and the area page's history chart.
// Sensor 101 carries a day of P2 history (internal/e2e seedFixtures).
const WIDE = { width: 1440, height: 900 }
const SHOTS = '/tmp/airbg-verify'
const DOCK = '.map-dock'
const HISTORY = '[data-island="panel"] .panel-chart'

async function prepareMap(page, path) {
  await page.goto(path)
  await page.waitForFunction(() => document.querySelector('[data-island="map"]')?.__map?.isStyleLoaded?.())
  await page.waitForTimeout(1000)
  await expect.poll(() => page.evaluate(() => document.querySelector('[data-island="map"]').__map.isMoving())).toBe(false)
  await page.evaluate(() => {
    document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
    document.querySelector('[data-island="map"]').__map.jumpTo({ center: [23.32, 42.69], zoom: 11 })
  })
}

// Client points of hex cells naming sensor 101, clear of the map's edges and overlays.
const hexPoint = (page) => page.evaluate(() => {
  const map = document.querySelector('[data-island="map"]').__map
  if (!map?.getLayer?.('airbg-hex-fill')) return null
  const box = map.getCanvas().getBoundingClientRect()
  for (const f of map.queryRenderedFeatures({ layers: ['airbg-hex-fill'] })) {
    if (f.geometry.type !== 'Polygon' || Number(f.properties?.sensorId) !== 101) continue
    const ring = f.geometry.coordinates[0].slice(0, -1)
    const c = [0, 1].map((i) => ring.reduce((a, p) => a + p[i], 0) / ring.length)
    const p = map.project(c)
    const x = box.left + p.x
    const y = box.top + p.y
    if (p.x < 30 || p.x > box.width - 30 || y < 80 || p.y > box.height - 30) continue
    if (document.elementFromPoint(x, y) !== map.getCanvas()) continue
    return { x, y }
  }
  return null
})

async function openDock(browser, size, path, ctx = {}) {
  const context = await browser.newContext({ viewport: size, ...ctx })
  const page = await context.newPage()
  await prepareMap(page, path)
  let pick = null
  await expect.poll(async () => { pick = await hexPoint(page); return !!pick }, { timeout: 20000 }).toBe(true)
  await page.mouse.click(pick.x, pick.y)
  await expect(page).toHaveURL(/#.*sensor=101/)
  await expect(page.locator(`${DOCK} .uplot`).first()).toBeVisible()
  return { context, page }
}

// The chart's plot box and one data point, in client pixels. which: 'max' | 'last' | 'mid'.
const pointOf = (page, scope, which = 'mid', series = 1) => page.locator(`${scope} .uplot`).first().evaluate((root, args) => {
  const [which, series] = args
  const u = root.__uplot
  const ys = u.data[series]
  let idx = Math.floor(ys.length / 2)
  if (which === 'last') idx = ys.length - 1
  if (which === 'max') ys.forEach((v, i) => { if (v != null && v > ys[idx]) idx = i })
  const o = u.over.getBoundingClientRect()
  return {
    // A point on the plot's very edge is one pixel outside the element; hover just inside it.
    x: Math.min(Math.max(o.left + u.valToPos(u.data[0][idx], 'x'), o.left + 1), o.right - 2),
    y: o.top + u.valToPos(ys[idx], u.series[series].scale),
    over: { left: o.left, right: o.right, top: o.top, bottom: o.bottom },
    value: ys[idx],
  }
}, [which, series])

const tipBox = async (page, scope) => {
  const tip = page.locator(`${scope} .chart-tip`)
  await expect(tip).toBeVisible()
  return { tip, box: await tip.boundingBox() }
}

async function hoverAt(page, scope, which, series) {
  const p = await pointOf(page, scope, which, series)
  // The cursor's own y does not matter for the anchor; x picks the point.
  await page.mouse.move(p.x, p.y)
  return p
}

test.describe('chart reading tooltip', () => {
  test('1440 dock: the card sits just above the hovered point with value and unit', async ({ browser }, testInfo) => {
    testInfo.setTimeout(90000)
    const { context, page } = await openDock(browser, WIDE, '/en/')
    const p = await pointOf(page, DOCK, 'mid')
    // A mid-height point of a short plot may not have room above, so take the lowest reading.
    const low = await page.locator(`${DOCK} .uplot`).first().evaluate((root) => {
      const u = root.__uplot
      let idx = 1
      u.data[1].forEach((v, i) => { if (v != null && v < u.data[1][idx]) idx = i })
      const o = u.over.getBoundingClientRect()
      return { x: o.left + u.valToPos(u.data[0][idx], 'x'), y: o.top + u.valToPos(u.data[1][idx], 'y') }
    })
    await page.mouse.move(low.x, low.y)
    const { tip, box } = await tipBox(page, DOCK)
    await expect(tip).toHaveAttribute('data-placement', 'above')
    const gap = low.y - (box.y + box.height)
    expect(gap, `card bottom is ${gap}px above the point`).toBeGreaterThanOrEqual(4)
    expect(gap).toBeLessThanOrEqual(20)
    await expect(tip).toContainText(/\d[\d.]*\s*µg\/m³/)
    await expect(tip).toContainText(/PM2\.5 · \d{1,2} \p{L}+ \d{2}:\d{2}/u)
    await page.screenshot({ path: `${SHOTS}/tooltip-dock-1440-en.png` })
    expect(p.value).toBeGreaterThan(0)
    await context.close()
  })

  test('1440 dock: near the top of the plot the card flips below the point', async ({ browser }, testInfo) => {
    testInfo.setTimeout(90000)
    const { context, page } = await openDock(browser, WIDE, '/en/')
    const p = await hoverAt(page, DOCK, 'max')
    const { tip, box } = await tipBox(page, DOCK)
    await expect(tip).toHaveAttribute('data-placement', 'below')
    expect(box.y).toBeGreaterThan(p.y)
    expect(box.y - p.y).toBeLessThanOrEqual(20)
    expect(box.y + box.height).toBeLessThanOrEqual(p.over.bottom + 1)
    await page.screenshot({ path: `${SHOTS}/tooltip-dock-1440-top-edge.png` })
    await context.close()
  })

  test('1440 dock: near the right edge the card stays inside the plot box', async ({ browser }, testInfo) => {
    testInfo.setTimeout(90000)
    const { context, page } = await openDock(browser, WIDE, '/en/')
    const p = await hoverAt(page, DOCK, 'last')
    const { box } = await tipBox(page, DOCK)
    expect(box.x + box.width, 'the card runs past the right edge').toBeLessThanOrEqual(p.over.right + 1)
    expect(box.x).toBeGreaterThanOrEqual(p.over.left - 1)
    await page.screenshot({ path: `${SHOTS}/tooltip-dock-1440-right-edge.png` })
    await context.close()
  })

  test('1440 dock: the card never leaves the plot box at the left edge either', async ({ browser }, testInfo) => {
    testInfo.setTimeout(90000)
    const { context, page } = await openDock(browser, WIDE, '/en/')
    await page.locator(`${DOCK} .uplot`).first().evaluate(() => {})
    const first = await page.locator(`${DOCK} .uplot`).first().evaluate((root) => {
      const u = root.__uplot
      const o = u.over.getBoundingClientRect()
      return { x: o.left + u.valToPos(u.data[0][0], 'x'), y: o.top + u.valToPos(u.data[1][0], 'y'), left: o.left }
    })
    await page.mouse.move(first.x, first.y)
    const { box } = await tipBox(page, DOCK)
    expect(box.x).toBeGreaterThanOrEqual(first.left - 1)
    await context.close()
  })

  test('1440 dock: the legend readout is gone and a single line has no key', async ({ browser }, testInfo) => {
    testInfo.setTimeout(90000)
    const { context, page } = await openDock(browser, WIDE, '/en/')
    await hoverAt(page, DOCK, 'mid')
    await tipBox(page, DOCK)
    await expect(page.locator(`${DOCK} .u-legend`)).toHaveCount(0)
    await expect(page.locator(`${DOCK} .u-value:visible`)).toHaveCount(0)
    await context.close()
  })

  test('1440 dock: the reading is also in a polite live region, not on every move', async ({ browser }, testInfo) => {
    testInfo.setTimeout(90000)
    const { context, page } = await openDock(browser, WIDE, '/en/')
    const live = page.locator(`${DOCK} .uplot [aria-live="polite"]`)
    await expect(live).toHaveCount(1)
    await page.locator(`${DOCK} .uplot`).first().evaluate((root) => {
      window.__live = []
      new MutationObserver(() => window.__live.push(root.querySelector('[aria-live]').textContent))
        .observe(root.querySelector('[aria-live]'), { childList: true, characterData: true, subtree: true })
    })
    const p = await pointOf(page, DOCK, 'mid')
    // Twenty one-pixel steps inside one point's reach make at most a couple of announcements.
    for (let i = 0; i < 20; i++) await page.mouse.move(p.x + i * 0.2, p.y)
    await expect(live).toContainText(/PM2\.5 [\d.]+ µg\/m³, \d{1,2} \p{L}+ \d{2}:\d{2}/u)
    expect(await page.evaluate(() => window.__live.length)).toBeLessThanOrEqual(2)
    await expect(page.locator(`${DOCK} .chart-tip`)).toHaveAttribute('aria-hidden', 'true')
    await context.close()
  })

  test('1440 dock, dark theme: the card reads on the dark surface', async ({ browser }, testInfo) => {
    testInfo.setTimeout(90000)
    const { context, page } = await openDock(browser, WIDE, '/en/', { colorScheme: 'dark' })
    await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'dark'))
    await hoverAt(page, DOCK, 'mid')
    const { tip } = await tipBox(page, DOCK)
    const colours = await tip.evaluate((el) => ({ bg: getComputedStyle(el).backgroundColor, fg: getComputedStyle(el).color }))
    expect(colours.bg).not.toBe('rgb(255, 255, 255)')
    expect(colours.fg).not.toBe(colours.bg)
    await page.screenshot({ path: `${SHOTS}/tooltip-dock-1440-dark.png` })
    await context.close()
  })

  test('1024 BG: the card carries a Bulgarian time and decimal comma', async ({ browser }, testInfo) => {
    testInfo.setTimeout(90000)
    const { context, page } = await openDock(browser, { width: 1024, height: 768 }, '/')
    await hoverAt(page, DOCK, 'mid')
    const { tip } = await tipBox(page, DOCK)
    const text = await tip.innerText()
    expect(text).toMatch(/\d{1,2} (януари|февруари|март|април|май|юни|юли|август|септември|октомври|ноември|декември) \d{2}:\d{2}/)
    expect(text).not.toMatch(/\b(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\b/)
    await page.screenshot({ path: `${SHOTS}/tooltip-dock-1024-bg.png` })
    await context.close()
  })

  test('1440 area page: the history chart under the map shows the same card', async ({ browser }, testInfo) => {
    testInfo.setTimeout(90000)
    const context = await browser.newContext({ viewport: WIDE })
    const page = await context.newPage()
    await page.goto('/en/area/sofia#sensor=101')
    await expect(page.locator(`${HISTORY} .uplot`).first()).toBeVisible({ timeout: 20000 })
    await page.locator(`${HISTORY} .uplot`).first().scrollIntoViewIfNeeded()
    const low = await page.locator(`${HISTORY} .uplot`).first().evaluate((root) => {
      const u = root.__uplot
      let idx = 1
      u.data[1].forEach((v, i) => { if (v != null && v < u.data[1][idx]) idx = i })
      const o = u.over.getBoundingClientRect()
      return { x: o.left + u.valToPos(u.data[0][idx], 'x'), y: o.top + u.valToPos(u.data[1][idx], 'y') }
    })
    await page.mouse.move(low.x, low.y)
    const { tip, box } = await tipBox(page, HISTORY)
    const gap = low.y - (box.y + box.height)
    expect(gap).toBeGreaterThanOrEqual(4)
    expect(gap).toBeLessThanOrEqual(20)
    await expect(tip).toContainText(/µg\/m³/)
    await expect(page.locator(`${HISTORY} .u-legend`)).toHaveCount(0)
    const p = await hoverAt(page, HISTORY, 'last')
    const edge = await tipBox(page, HISTORY)
    expect(edge.box.x + edge.box.width).toBeLessThanOrEqual(p.over.right + 1)
    const top = await hoverAt(page, HISTORY, 'max')
    const flipped = await tipBox(page, HISTORY)
    // A tall plot may leave room above even its highest point; then it must stay above and inside.
    const placement = await flipped.tip.getAttribute('data-placement')
    if (placement === 'below') expect(flipped.box.y).toBeGreaterThan(top.y)
    else expect(flipped.box.y + flipped.box.height).toBeLessThanOrEqual(top.y)
    await page.screenshot({ path: `${SHOTS}/tooltip-history-1440.png` })
    await context.close()
  })

  test('1440 dock with several metrics: one card lists every line, the hovered one first; the key has no values', async ({ browser }, testInfo) => {
    testInfo.setTimeout(90000)
    const { context, page } = await openDock(browser, WIDE, '/en/')
    await page.locator(`${DOCK} #dock-metric-menu`).click()
    const options = page.locator('#dock-metric-menu-panel .colmenu__opt')
    const n = await options.count()
    for (let i = 0; i < n; i++) {
      const opt = options.nth(i)
      const box = opt.locator('input')
      if (!(await box.isChecked())) await opt.click()
    }
    await page.keyboard.press('Escape')
    await expect.poll(() => page.locator(`${DOCK} .uplot`).first().evaluate((r) => r.__uplot.series.length)).toBeGreaterThan(2)
    await expect(page.locator(`${DOCK} .u-legend .u-marker`).first()).toBeVisible()
    await expect(page.locator(`${DOCK} .u-legend .u-value`)).toHaveCount(0)
    const p = await hoverAt(page, DOCK, 'mid', 1)
    const { tip } = await tipBox(page, DOCK)
    await expect(tip.locator('.chart-tip__row')).not.toHaveCount(0)
    expect(await tip.locator('.chart-tip__row').count()).toBeGreaterThan(1)
    await expect(tip.locator('.chart-tip__row').first()).toHaveClass(/chart-tip__row--focus/)
    await expect(tip.locator('.chart-tip__swatch').first()).toBeVisible()
    await page.screenshot({ path: `${SHOTS}/tooltip-dock-1440-multi.png` })
    expect(p.value).not.toBeNull()
    await context.close()
  })
})

const test390 = test.extend({
  phone: [async ({ browser }, use) => {
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 })
    await use(context)
    await context.close()
  }, { scope: 'worker' }],
})

test390.describe('chart reading tooltip on a phone', () => {
  test390('390: a tap shows the card and a tap outside the plot hides it', async ({ phone }) => {
    const page = await phone.newPage()
    await page.goto('/en/area/sofia#sensor=101')
    const scope = '.sensor-panel'
    await expect(page.locator(`${scope} .uplot`).first()).toBeVisible({ timeout: 20000 })
    await page.locator(`${scope} .uplot`).first().scrollIntoViewIfNeeded()
    const p = await pointOf(page, scope, 'mid')
    await page.touchscreen.tap(p.x, p.y)
    const { tip, box } = await tipBox(page, scope)
    expect(box.x).toBeGreaterThanOrEqual(p.over.left - 1)
    expect(box.x + box.width).toBeLessThanOrEqual(p.over.right + 1)
    await page.screenshot({ path: `${SHOTS}/tooltip-phone-390.png` })
    // It stays while the finger is elsewhere in the page's idle time.
    await page.waitForTimeout(800)
    await expect(tip).toBeVisible()
    await page.touchscreen.tap(20, 20)
    await expect(tip).toBeHidden()
    await page.close()
  })
})
