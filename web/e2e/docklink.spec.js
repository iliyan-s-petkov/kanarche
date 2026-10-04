import { test, expect } from './fixtures.js'

// The bottom panel's footer line that points at the area figures under the map.
const WIDE = { width: 1440, height: 900 }
const PANEL = '.map-dock'
const LINE = '.map-dock__area'

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

const hexPoint = (page) => page.evaluate(() => {
  const map = document.querySelector('[data-island="map"]').__map
  if (!map?.getLayer?.('airbg-hex-fill')) return null
  const box = map.getCanvas().getBoundingClientRect()
  for (const f of map.queryRenderedFeatures({ layers: ['airbg-hex-fill'] })) {
    if (f.geometry.type !== 'Polygon' || f.properties?.sensorId == null) continue
    const ring = f.geometry.coordinates[0].slice(0, -1)
    const c = [0, 1].map((i) => ring.reduce((a, p) => a + p[i], 0) / ring.length)
    const p = map.project(c)
    const y = box.top + p.y
    if (p.x < 30 || p.x > box.width - 30 || y < 80 || p.y > box.height - 30) continue
    if (document.elementFromPoint(box.left + p.x, y) !== map.getCanvas()) continue
    return { x: box.left + p.x, y }
  }
  return null
})

async function openSensor(browser, path, size = WIDE) {
  const context = await browser.newContext({ viewport: size })
  const page = await context.newPage()
  await prepareMap(page, path)
  let pt = null
  await expect.poll(async () => { pt = await hexPoint(page); return pt }, { timeout: 20000 }).not.toBeNull()
  await page.mouse.click(pt.x, pt.y)
  await expect(page.locator(PANEL)).toBeVisible()
  return { context, page }
}

test('1440: the panel ends in a line naming the area, and the click brings the figures into view', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const { context, page } = await openSensor(browser, '/en/')
  const line = page.locator(LINE)
  await expect(line).toBeVisible()
  await expect(line).toContainText(/\S.* · \d+ sensors: area figures below/)
  const d = await page.locator(PANEL).boundingBox()
  const l = await line.boundingBox()
  expect(l.y).toBeGreaterThanOrEqual(d.y)
  expect(l.y + l.height).toBeLessThanOrEqual(d.y + d.height)
  expect(l.height, 'one compact row').toBeLessThanOrEqual(32)

  // Under the fold before the click, near the top after it.
  const figures = page.locator('[data-island="readouts"]')
  expect((await figures.boundingBox()).y, 'figures already at the top').toBeGreaterThan(600)
  await line.click()
  await expect.poll(async () => {
    const r = await figures.boundingBox()
    return r !== null && r.y >= 0 && r.y < 300
  }, { timeout: 10000 }).toBe(true)
  await expect(figures).toBeFocused()
  expect(page.url(), 'the hash keeps the sensor').toMatch(/sensor=\d+/)
  await context.close()
})

test('1440: the line is not in the fullscreen map', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const { context, page } = await openSensor(browser, '/en/')
  await expect(page.locator(LINE)).toBeVisible()
  await page.locator('.map__full').click()
  await expect(page.locator(PANEL)).toHaveCount(0)
  await expect(page.locator(LINE)).toHaveCount(0)
  await context.close()
})

test('1440 bg: the line reads in Bulgarian', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const { context, page } = await openSensor(browser, '/')
  await expect(page.locator(LINE)).toContainText(/показатели за района по-долу/)
  await context.close()
})

test('1440: with the line in place a five-row rail always has the height for its rows', async ({ browser }, testInfo) => {
  testInfo.setTimeout(90000)
  const { context, page } = await openSensor(browser, '/en/')
  await expect(page.locator(LINE)).toBeVisible()
  const bad = await page.evaluate(() => {
    const dock = document.querySelector('.map-dock')
    const rail = dock.querySelector('.gauges')
    const out = []
    for (let h = 300; h <= 440; h += 4) {
      dock.style.setProperty('--map-dock-h', `${h}px`)
      const rows = getComputedStyle(rail).gridTemplateRows.split(' ').length
      // Five 52px rows and four 4px gaps.
      if (rows === 5 && rail.clientHeight < 276) out.push({ h, rows, rail: rail.clientHeight })
    }
    return out
  })
  expect(bad, 'five rows in a rail too short for them').toEqual([])
  await context.close()
})

test('1440: an empty-state message under the chart stays above the line', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const { context, page } = await openSensor(browser, '/en/')
  await expect(page.locator(LINE)).toBeVisible()
  // The e2e fixture has no readings, so the chart shows its own empty-state message.
  const msg = page.locator('.map-dock .chart-message')
  await expect(msg).toBeVisible()
  // The shortest panel leaves the chart the least room.
  await page.getByRole('separator', { name: 'Resize panel' }).focus()
  await page.keyboard.press('Home')
  await page.waitForTimeout(300)
  const r = await page.evaluate(() => ({
    msgBottom: document.querySelector('.map-dock .chart-message').getBoundingClientRect().bottom,
    lineTop: document.querySelector('.map-dock__area').getBoundingClientRect().top,
  }))
  expect(r.msgBottom, 'the message runs into the line').toBeLessThanOrEqual(r.lineTop + 1)
  await context.close()
})

test('a phone keeps the card under the map and has no panel line', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext({ viewport: { width: 390, height: 800 } })
  const page = await context.newPage()
  await page.goto('/en/')
  await expect(page.locator(LINE)).toHaveCount(0)
  await context.close()
})
