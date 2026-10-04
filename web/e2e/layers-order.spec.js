import { test, expect, mapSettled } from './fixtures.js'

// The layers menu lists the toggles in a fixed order, with Inactive stations
// directly above Faulty stations and their labels starting at the same x.
test('the layers menu lists Inactive stations directly above Faulty stations', async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } })
  const page = await context.newPage()
  await page.goto('/en')
  await mapSettled(page)
  await page.locator('.map__layers .colmenu__btn').click()
  const panel = page.locator('.map__layers .colmenu__panel')
  await expect(panel).toBeVisible()

  const labels = await panel.locator('.colmenu__opt--view').allInnerTexts()
  // A row can carry a suffix such as "— does not measure this"; compare the name only.
  const names = labels.map((s) => s.split(' — ')[0].trim())
  const expected = [
    'Scale', 'Cell values', 'OpenStreetMap', 'Citizen sensors', 'Official stations',
    'Inactive stations', 'Faulty stations', 'Wind', 'Bathing water', 'Province outlines',
  ]
  // Wind appears only when a forecast is loaded; compare the rows that are there.
  expect(names).toEqual(expected.filter((n) => names.includes(n)))
  expect(names).toContain('Inactive stations')
  const inactive = names.indexOf('Inactive stations')
  expect(inactive).toBeGreaterThan(-1)
  expect(names[inactive + 1]).toBe('Faulty stations')
  expect(names[inactive - 1]).toBe('Official stations')

  const textX = async (name) => (await panel.locator('label', { hasText: new RegExp(`^\\s*${name}\\s*$`) })
    .locator('span:not(.colmenu__mark)').boundingBox()).x
  expect(await textX('Inactive stations')).toBe(await textX('Faulty stations'))

  if (process.env.AIRBG_SHOT_DIR) await page.screenshot({ path: `${process.env.AIRBG_SHOT_DIR}/inactive-menu.png` })
  await context.close()
})
