import { test, expect, mapSettled } from './fixtures.js'

// OP #682: the first-visit tip pointing at the locate button.
const KEY = 'kanarche:locate-hint-seen'
const TEXT = 'See the air near you'
const hint = (page) => page.locator('.map-locate-hint')
const shot = (page, name) => process.env.E2E_SHOT_DIR && page.screenshot({ path: `${process.env.E2E_SHOT_DIR}/${name}.png` })

const sizes = { desktop: { width: 1440, height: 900 }, phone: { width: 390, height: 844 } }

async function visit(browser, size, path = '/en/') {
  const context = await browser.newContext({ viewport: size })
  const page = await context.newPage()
  await page.goto(path)
  await mapSettled(page)
  return { context, page }
}

const box = async (locator) => {
  const b = await locator.boundingBox()
  expect(b).not.toBeNull()
  return b
}
const overlaps = (a, b) => a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height

for (const [name, size] of Object.entries(sizes)) {
  test(`${name}: a first visit shows the tip beside the locate button, clear of the zoom buttons`, async ({ browser }) => {
    const { context, page } = await visit(browser, size)
    await expect(hint(page)).toBeVisible()
    await expect(hint(page)).toHaveText(TEXT)
    await expect(hint(page)).toHaveAttribute('role', 'status')
    expect(await page.evaluate((k) => localStorage.getItem(k), KEY)).toBe('1')

    const h = await box(hint(page))
    const map = await box(page.locator('#map'))
    expect(h.x).toBeGreaterThanOrEqual(map.x)
    expect(h.x + h.width).toBeLessThanOrEqual(map.x + map.width)
    const locate = await box(page.locator('.map-locate'))
    // Above the button, its right edge on the button's, so the arrow points down at it.
    expect(h.y + h.height).toBeLessThanOrEqual(locate.y)
    expect(locate.y - (h.y + h.height)).toBeLessThan(20)
    expect(Math.abs(h.x + h.width - (locate.x + locate.width))).toBeLessThan(2)
    for (const b of await page.locator('.map-zoom button, .map__full').all()) {
      if (await b.isVisible()) expect(overlaps(h, await box(b))).toBe(false)
    }
    await shot(page, `locatehint-${name}`)
    await context.close()
  })
}

test('1440 with the map docked: the tip still sits over the locate button', async ({ browser }) => {
  const { context, page } = await visit(browser, sizes.desktop)
  await expect(hint(page)).toBeVisible()
  // The docked class must not move the hint off the button.
  await page.evaluate(() => document.querySelector('.map-shell').classList.add('map-shell--docked'))
  // Layout follows the class on the next frame, so measure both until they agree.
  await expect.poll(async () => {
    const h = await hint(page).boundingBox()
    const locate = await page.locator('.map-locate').boundingBox()
    if (!h || !locate) return null
    return h.x >= 0 && Math.abs(h.x + h.width - (locate.x + locate.width)) < 2
  }).toBe(true)
  await shot(page, 'locatehint-docked')
  await context.close()
})

test('it is shown once: a reload brings nothing back', async ({ browser }) => {
  const { context, page } = await visit(browser, sizes.desktop)
  await expect(hint(page)).toBeVisible()
  await page.reload()
  await mapSettled(page)
  await expect(hint(page)).toHaveCount(0)
  await context.close()
})

test('a real wheel zoom on the map dismisses it', async ({ browser }) => {
  const { context, page } = await visit(browser, sizes.desktop)
  await expect(hint(page)).toBeVisible()
  const m = await box(page.locator('#map'))
  // A wheel sent while the map is still easing is swallowed, so send it until it lands.
  await expect(async () => {
    await page.mouse.move(m.x + m.width / 2, m.y + m.height / 2)
    await page.mouse.wheel(0, -400)
    await expect(hint(page)).toHaveCount(0, { timeout: 1500 })
  }).toPass({ timeout: 15_000 })
  await context.close()
})

test('a click on the locate button dismisses it', async ({ browser }) => {
  const { context, page } = await visit(browser, sizes.desktop)
  await expect(hint(page)).toBeVisible()
  await page.locator('.map-locate').click()
  await expect(hint(page)).toHaveCount(0)
  await context.close()
})

test('it goes by itself after eight seconds', async ({ browser }) => {
  // No settle wait first: the eight seconds start when the hint mounts, and a
  // slow settle on a busy runner could spend them before the first assertion.
  const context = await browser.newContext({ viewport: sizes.desktop })
  const page = await context.newPage()
  await page.goto('/en/')
  await expect(hint(page)).toBeVisible({ timeout: 30_000 })
  await expect(hint(page)).toHaveCount(0, { timeout: 12_000 })
  await context.close()
})

test('clicking the bubble itself locates the visitor', async ({ browser }) => {
  const context = await browser.newContext({ viewport: sizes.desktop, permissions: ['geolocation'], geolocation: { longitude: 23.33, latitude: 42.7 } })
  const page = await context.newPage()
  const grid = page.waitForResponse(/\/api\/v1\/hexes/)
  await page.goto('/en/')
  await mapSettled(page)
  await grid
  await expect(hint(page)).toBeVisible()
  const zoom = () => page.evaluate(() => document.querySelector('[data-island="map"]').__map.getZoom())
  const before = await zoom()
  await page.locator('.map-locate-hint button').click()
  await expect(hint(page)).toHaveCount(0)
  await expect.poll(zoom).toBeGreaterThan(before)
  await context.close()
})

test('not on an area page, with a saved view, or on a deep link', async ({ browser }) => {
  for (const [path, seed] of [
    ['/en/area/sofia', null],
    ['/en/', JSON.stringify({ lat: 43.2, lng: 27.9, zoom: 8 })],
    ['/en/#sensor=101', null],
  ]) {
    const context = await browser.newContext({ viewport: sizes.desktop })
    const page = await context.newPage()
    if (seed) await page.addInitScript(([k, v]) => localStorage.setItem(k, v), ['kanarche:map-view', seed])
    await page.goto(path)
    await mapSettled(page)
    await page.waitForTimeout(500)
    await expect(hint(page), path).toHaveCount(0)
    expect(await page.evaluate((k) => localStorage.getItem(k), KEY), path).toBeNull()
    await context.close()
  }
})

test('the embed never shows it', async ({ browser }) => {
  const { context, page } = await visit(browser, sizes.desktop, '/embed')
  await expect(hint(page)).toHaveCount(0)
  await context.close()
})
