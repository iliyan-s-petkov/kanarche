import { test, expect, mapSettled } from './fixtures.js'

// OpenProject #683: star a sensor so the home map opens on it.
// The helpers below are the ones sidedock.spec.js uses.
const KEY = 'kanarche:favourite-sensor'
const WIDE = { width: 1440, height: 900 }
const PHONE = { width: 390, height: 844, isMobile: true, hasTouch: true }
const shot = (page, name) => process.env.E2E_SHOT_DIR && page.screenshot({ path: `${process.env.E2E_SHOT_DIR}/${name}.png` })

async function prepareMap(page, path = '/en/') {
  // The opening camera (favourite, saved view, geoip) is placed before the first
  // hex paint, and the style can report loaded and idle before that jump runs.
  // The first hexes response marks the camera as placed; an area page has its own.
  const placed = path.includes('/area/') ? null : page.waitForResponse(/\/api\/v1\/hexes/)
  await page.goto(path)
  await placed
  await mapSettled(page)
  await page.evaluate(() => {
    document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
    document.querySelector('[data-island="map"]').__map.jumpTo({ center: [23.32, 42.69], zoom: 11 })
  })
  await mapSettled(page)
}

// Client points of hex cells naming one station, clear of the map's edges and
// of any overlay, skipping sensor ids in `skip`.
const hexPoints = (page, skip) => page.evaluate((skip) => {
  const map = document.querySelector('[data-island="map"]').__map
  if (!map?.getLayer?.('airbg-hex-fill')) return []
  const box = map.getCanvas().getBoundingClientRect()
  const out = []
  for (const f of map.queryRenderedFeatures({ layers: ['airbg-hex-fill'] })) {
    const id = f.properties?.sensorId
    if (f.geometry.type !== 'Polygon' || id == null || skip.includes(Number(id))) continue
    const ring = f.geometry.coordinates[0].slice(0, -1)
    const c = [0, 1].map((i) => ring.reduce((a, p) => a + p[i], 0) / ring.length)
    const p = map.project(c)
    const y = box.top + p.y
    if (p.x < 30 || p.x > box.width - 30 || y < 80 || p.y > box.height - 30) continue
    const x = box.left + p.x
    if (document.elementFromPoint(x, y) !== map.getCanvas()) continue
    if (out.some((o) => o.id === Number(id))) continue
    out.push({ x, y, id: Number(id) })
  }
  return out
}, skip)

// Points are re-read for every click: a fullscreen resize or a late tile can
// move the hex between the read and the click, and a stale point misses it.
async function tapHex(page, skip = []) {
  let id = null
  await expect(async () => {
    const pts = await hexPoints(page, skip)
    expect(pts.length).toBeGreaterThan(0)
    await page.mouse.click(pts[0].x, pts[0].y)
    await expect(page).toHaveURL(/#.*sensor=\d+/, { timeout: 2000 })
    id = pts[0].id
  }).toPass({ timeout: 20000 })
  return id
}


const box = async (locator) => {
  const b = await locator.boundingBox()
  expect(b).not.toBeNull()
  return b
}
const stored = (page) => page.evaluate((k) => localStorage.getItem(k), KEY)
const visibleStars = (page) => page.locator('.panel-star:visible')

async function open(browser, opts, path = '/en/') {
  const context = await browser.newContext(opts)
  const page = await context.newPage()
  await prepareMap(page, path)
  const id = await tapHex(page)
  return { context, page, id }
}

test('below the map: the star toggles, stores only the id and keeps its state', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const { context, page, id } = await open(browser, { viewport: { width: 900, height: 700 } })
  const star = page.locator('.sensor-panel .panel-star')
  await expect(star).toHaveCount(1)
  await expect(star).toHaveAttribute('aria-pressed', 'false')
  await expect(star).toHaveAccessibleName('Save as my sensor')
  await star.click()
  await expect(star).toHaveAttribute('aria-pressed', 'true')
  await expect(star).toHaveAccessibleName('Remove my sensor')
  expect(await stored(page)).toBe(String(id))
  await star.click()
  await expect(star).toHaveAttribute('aria-pressed', 'false')
  expect(await stored(page)).toBeNull()
  await context.close()
})

test('1440: exactly one star, in the dock, and it works there', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const { context, page, id } = await open(browser, WIDE)
  await expect(page.locator('.map-dock')).toBeVisible()
  await expect(visibleStars(page)).toHaveCount(1)
  await expect(page.locator('.map-dock .panel-star')).toBeVisible()
  await page.locator('.map-dock .panel-star').click()
  await expect(page.locator('.map-dock .panel-star')).toHaveAttribute('aria-pressed', 'true')
  await expect.poll(() => stored(page)).toBe(String(id))
  await shot(page, 'favourite-desktop')
  // Back under the map the same state shows on the one star there.
  await page.setViewportSize({ width: 900, height: 700 })
  await expect(page.locator('.map-dock')).toHaveCount(0)
  await expect(visibleStars(page)).toHaveCount(1)
  await expect(page.locator('.sensor-panel .panel-star')).toHaveAttribute('aria-pressed', 'true')
  await context.close()
})

test('phone fullscreen: the sheet shows one star and it works', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext(PHONE)
  const page = await context.newPage()
  await prepareMap(page, '/en/area/sofia')
  await page.locator('.map__full').click()
  await expect.poll(() => page.evaluate(() => !!document.querySelector('.map:fullscreen, .map--faux-full'))).toBe(true)
  await expect(page.locator('.map__full')).toHaveAttribute('aria-pressed', 'true')
  await mapSettled(page)
  const id = await tapHex(page)
  await expect(page.locator('.map-sensor-sheet')).toBeVisible()
  await expect(visibleStars(page)).toHaveCount(1)
  await expect(page.locator('.map-sensor-sheet .panel-star')).toBeVisible()
  await page.locator('.map-sensor-sheet .panel-star').click()
  await expect(page.locator('.map-sensor-sheet .panel-star')).toHaveAttribute('aria-pressed', 'true')
  await expect.poll(() => stored(page)).toBe(String(id))
  await shot(page, 'favourite-phone-sheet')
  await context.close()
})

test('a reload of the home map opens on the favourite; an explicit URL beats it', async ({ browser }, testInfo) => {
  testInfo.setTimeout(90000)
  const { context, page, id } = await open(browser, WIDE)
  await page.locator('.map-dock .panel-star').click()
  expect(await stored(page)).toBe(String(id))

  await page.goto('/en/')
  await expect(page.locator('.map-dock .gauges')).toBeVisible({ timeout: 20000 })
  await expect(page).toHaveURL(new RegExp(`sensor=${id}\\b`))
  await expect(page.locator('.map-dock .panel-star')).toHaveAttribute('aria-pressed', 'true')

  const other = id === 101 ? 102 : 101
  await page.goto(`/en/#sensor=${other}`)
  await page.reload()
  await expect(page.locator('.map-dock .gauges')).toBeVisible({ timeout: 20000 })
  await expect(page).toHaveURL(new RegExp(`sensor=${other}\\b`))
  await context.close()
})

test('a favourite that is no longer in the data is ignored and the key stays', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext(WIDE)
  const page = await context.newPage()
  await page.addInitScript((k) => localStorage.setItem(k, '999999999'), KEY)
  // The favourite is resolved before the first hex paint, so that response means it was ignored.
  const placed = page.waitForResponse(/\/api\/v1\/hexes/)
  await page.goto('/en/')
  await placed
  await mapSettled(page)
  await expect(page.locator('.sensor-panel')).toHaveCount(0)
  expect(page.url()).not.toContain('sensor=')
  expect(await stored(page)).toBe('999999999')
  await context.close()
})

test('touch: the star is at least 44px square', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext(PHONE)
  const page = await context.newPage()
  await page.goto('/en/#sensor=101')
  const star = page.locator('.panel-star:visible')
  await expect(star).toBeVisible({ timeout: 20000 })
  const b = await box(star)
  expect(b.width).toBeGreaterThanOrEqual(44)
  expect(b.height).toBeGreaterThanOrEqual(44)
  await shot(page, 'favourite-phone')
  await context.close()
})

test('the embed has no star', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext(WIDE)
  const page = await context.newPage()
  await page.goto('/embed#sensor=101')
  await mapSettled(page)
  await expect(page.locator('.panel-star')).toHaveCount(0)
  await context.close()
})
