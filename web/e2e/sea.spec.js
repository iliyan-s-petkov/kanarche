import { test, expect } from './fixtures.js'

// The bathing-water layer: off by default, a square per site, a card with the class and samples.
// The site is seeded in internal/e2e/e2e_test.go at 23.45, 42.75.
const SITE = [23.45, 42.75]

async function prepareMap(page, path) {
  await page.goto(path)
  await page.waitForFunction(() => document.querySelector('[data-island="map"]')?.__map?.isStyleLoaded?.())
  await page.waitForTimeout(1000)
  await expect.poll(() => page.evaluate(() => document.querySelector('[data-island="map"]').__map.isMoving())).toBe(false)
  await page.evaluate((c) => {
    document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
    document.querySelector('[data-island="map"]').__map.jumpTo({ center: c, zoom: 10 })
  }, SITE)
}

async function toggleSea(page, label, on) {
  await page.locator('.map__layers .colmenu__btn').click()
  const box = page.locator('.map__layers').getByLabel(label, { exact: true })
  if (on) await box.check()
  else await box.uncheck()
  await page.keyboard.press('Escape')
}

// Waits for the square to render, then returns its page coordinates.
const sitePoint = (page) => page.evaluate((c) => {
  const map = document.querySelector('[data-island="map"]').__map
  if (!map.queryRenderedFeatures({ layers: ['sea-sites'] }).length) return null
  const box = map.getCanvas().getBoundingClientRect()
  const p = map.project(c)
  return { x: box.left + p.x, y: box.top + p.y }
}, SITE)

test('the bathing layer is off by default, and a site opens its card', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  const page = await context.newPage()
  await prepareMap(page, '/en/')

  const legendKey = page.locator('.scale--onmap .scale__sea')
  await expect(legendKey).toBeHidden()
  await toggleSea(page, 'Bathing water', true)
  await expect(legendKey).toBeAttached()
  await expect(legendKey).not.toHaveAttribute('hidden')

  let pt = null
  await expect.poll(async () => { pt = await sitePoint(page); return pt }, { timeout: 20000 }).not.toBeNull()
  await page.mouse.click(pt.x, pt.y)

  const card = page.locator('.map-sea .sea-panel')
  await expect(card).toBeVisible()
  await expect(card.locator('h2')).toHaveText('Plazh Test')
  await expect(card.locator('.sea-panel__class')).toContainText('Excellent')
  await expect(card.locator('.sea-panel__history')).toContainText('2023')
  await expect(card.locator('.sea-panel__limits')).toContainText('250 / 500')
  const rows = card.locator('.sea-panel__samples tbody tr')
  await expect(rows).toHaveCount(2)
  await expect(rows.nth(0)).toContainText('<15')
  await expect(rows.nth(0)).toContainText('▲')
  await expect(rows.nth(1)).toContainText('▲▲')
  await expect(card.locator(`a[href*="eea.europa.eu"]`)).toBeVisible()
  await expect(card.locator('a[href="https://example.org/profile.pdf"]')).toBeVisible()
  await expect(page.locator('.map-dock')).toBeHidden()

  await page.keyboard.press('Escape')
  await expect(page.locator('.map-sea')).toBeHidden()

  // Switching the layer off also takes the card with it.
  await page.mouse.click(pt.x, pt.y)
  await expect(card).toBeVisible()
  await toggleSea(page, 'Bathing water', false)
  await expect(page.locator('.map-sea')).toBeHidden()
  await context.close()
})

test('390: the card is a bottom sheet in Bulgarian', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true })
  const page = await context.newPage()
  await prepareMap(page, '/')
  await toggleSea(page, 'Води за къпане', true)
  let pt = null
  await expect.poll(async () => { pt = await sitePoint(page); return pt }, { timeout: 20000 }).not.toBeNull()
  await page.mouse.click(pt.x, pt.y)
  const sheet = page.locator('.map-sea')
  await expect(sheet.locator('h2')).toHaveText('Плаж Тест')
  await expect(sheet.locator('.sea-panel__class')).toContainText('Отлично')
  const box = await sheet.boundingBox()
  const frame = await page.locator('[data-island="map"]').boundingBox()
  expect(Math.round(box.width)).toBe(Math.round(frame.width))
  expect(Math.abs(box.y + box.height - (frame.y + frame.height))).toBeLessThanOrEqual(1)
  await context.close()
})
