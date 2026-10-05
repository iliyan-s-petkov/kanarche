import { test, expect } from './fixtures.js'

// The home map's pollen layer. internal/e2e/e2e_test.go seeds ragweed at 40 grains inside
// sofia-oblast (lon 23.32 ± 0.5, lat 42.69 ± 0.5), so that province reads "high".
const CENTRE = [23.32, 42.69]
// Inside the province, clear of the area marker and the sensors at its centre.
const CLICK = [23.0, 42.45]

async function prepareMap(page, path) {
  await page.goto(path)
  await page.waitForFunction(() => document.querySelector('[data-island="map"]')?.__map?.isStyleLoaded?.())
  await page.waitForTimeout(1000)
  await expect.poll(() => page.evaluate(() => document.querySelector('[data-island="map"]').__map.isMoving())).toBe(false)
  await page.evaluate((c) => {
    document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
    document.querySelector('[data-island="map"]').__map.jumpTo({ center: c, zoom: 8 })
  }, CENTRE)
}

async function togglePollen(page, label) {
  await page.locator('.map__layers .colmenu__btn').click()
  await page.locator('.map__layers').getByLabel(label, { exact: true }).check()
  await page.keyboard.press('Escape')
}

// The level painted under CLICK, and that point on the page; null until the fill renders.
const pollenAt = (page) => page.evaluate((c) => {
  const map = document.querySelector('[data-island="map"]').__map
  const p = map.project(c)
  const f = map.queryRenderedFeatures(p, { layers: ['pollen-fill'] })[0]
  if (!f) return null
  const box = map.getCanvas().getBoundingClientRect()
  return { slug: f.properties.slug, level: f.properties.level, x: box.left + p.x, y: box.top + p.y }
}, CLICK)

const hexVisibility = (page) => page.evaluate(() =>
  document.querySelector('[data-island="map"]').__map.getLayoutProperty('airbg-hex-fill', 'visibility'))

for (const shape of [
  { name: 'desktop', path: '/en/', label: 'Pollen', levels: ['None', 'Low', 'Moderate', 'High', 'Very high'], prefix: '/en', context: { viewport: { width: 1440, height: 900 } } },
  { name: 'mobile', path: '/', label: 'Прашец', levels: ['Няма', 'Нисък', 'Умерен', 'Висок', 'Много висок'], prefix: '', context: { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true } },
]) {
  test(`${shape.name}: the pollen layer colours the province and opens its area page`, async ({ browser }, testInfo) => {
    testInfo.setTimeout(60000)
    const context = await browser.newContext(shape.context)
    const page = await context.newPage()
    await prepareMap(page, shape.path)

    const key = page.locator('.scale--onmap .scale__pollen')
    await expect(key).toBeHidden()
    await togglePollen(page, shape.label)
    await expect(key).not.toHaveAttribute('hidden')
    await expect(key.locator('.legend__row')).toHaveText(shape.levels)

    let at = null
    await expect.poll(async () => { at = await pollenAt(page); return at && `${at.slug}:${at.level}` }, { timeout: 20000 })
      .toBe('sofia-oblast:high')
    // The fill takes the grid's place rather than sitting on it.
    expect(await hexVisibility(page)).toBe('none')
    // The neighbouring provinces have no cell within reach: outlined, unfilled, still there.
    const others = await page.evaluate(() => document.querySelector('[data-island="map"]').__map
      .querySourceFeatures('pollen-areas').filter((f) => f.properties.slug !== 'sofia-oblast' && f.properties.level).length)
    expect(others).toBe(0)

    // The key shows in the opened legend; folded again so it cannot cover the click below.
    const toggle = page.locator('.scale--onmap > .scale__toggle')
    await toggle.click()
    await expect(key).toBeVisible()
    if (process.env.AIRBG_SHOT_DIR) {
      await page.screenshot({ path: `${process.env.AIRBG_SHOT_DIR}/pollen-layer-${shape.name}.png` })
    }
    await toggle.click()
    await expect(key).toBeHidden()

    if (shape.context.hasTouch) await page.touchscreen.tap(at.x, at.y)
    else await page.mouse.click(at.x, at.y)
    await page.waitForURL(`**${shape.prefix}/area/sofia-oblast`)
    expect(new URL(page.url()).pathname).toBe(`${shape.prefix}/area/sofia-oblast`)
    await context.close()
  })
}

test('the area page map offers no pollen layer', async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  const page = await context.newPage()
  await page.goto('/en/area/sofia-oblast')
  await page.waitForFunction(() => document.querySelector('[data-island="map"]')?.__map?.isStyleLoaded?.())
  await page.locator('.map__layers .colmenu__btn').click()
  await expect(page.locator('.map__layers').getByLabel('Pollen', { exact: true })).toHaveCount(0)
  await context.close()
})
