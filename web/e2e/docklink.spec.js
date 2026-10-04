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

  await line.click()
  const figures = page.locator('[data-island="readouts"]')
  await expect.poll(async () => {
    const r = await figures.boundingBox()
    return r !== null && r.y < 900 && r.y + r.height > 0
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

test('1440: the five-row rail still shows every gauge with the line in place', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const { context, page } = await openSensor(browser, '/en/')
  await expect(page.locator(LINE)).toBeVisible()
  const gauges = page.locator(`${PANEL} .gauges`)
  const scroll = await gauges.evaluate((el) => el.scrollHeight - el.clientHeight)
  expect(scroll, 'the rail clips its rows').toBeLessThanOrEqual(1)
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
