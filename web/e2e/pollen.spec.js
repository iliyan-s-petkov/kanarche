import { test, expect } from './fixtures.js'

// The pollen fixture (internal/e2e/e2e_test.go): one cell inside "sofia",
// ragweed at 60 grains/m³ (high) and grass at 1 (low).
test('the area page shows the pollen table, its credit and the chip', async ({ page }) => {
  await page.goto('/en/area/sofia')
  const chip = page.locator('.toolbar .pollen-chip')
  await expect(chip).toHaveText('Pollen today: high · Ragweed')

  const section = page.locator('#pollen')
  await expect(section.locator('tbody tr')).toHaveCount(6)
  const ragweed = section.locator('tbody tr', { has: page.locator('th', { hasText: 'Ragweed' }) })
  await expect(ragweed.locator('td').first()).toHaveAttribute('data-level', 'high')
  const grass = section.locator('tbody tr', { has: page.locator('th', { hasText: 'Grasses' }) })
  await expect(grass.locator('td').first()).toHaveAttribute('data-level', 'low')
  await expect(section.locator('.pollen-credit a')).toHaveAttribute('href', 'https://open-meteo.com/')

  await chip.click()
  await expect(page).toHaveURL(/#pollen$/)
  await expect(section).toBeInViewport()
})

test('the pollen table renders in Bulgarian without JavaScript', async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false })
  const page = await context.newPage()
  await page.goto('/area/sofia')
  await expect(page.locator('.pollen-chip')).toHaveText('Прашец днес: висок · Амброзия')
  await expect(page.locator('#pollen .pollen-credit')).toContainText('Copernicus')
  await context.close()
})

test('the pollen API carries the attribution', async ({ request }) => {
  const res = await request.get('/api/v1/area/sofia/pollen')
  expect(res.status()).toBe(200)
  const body = await res.json()
  expect(body.attribution.url).toBe('https://open-meteo.com/')
  expect(body.summary).toMatchObject({ level: 'high', species: 'ragweed' })
})
