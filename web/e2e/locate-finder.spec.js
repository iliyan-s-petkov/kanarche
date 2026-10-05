import { test, expect, mapSettled } from './fixtures.js'

// Find me jumps straight to the sensor zoom, past the city tier, so the finder
// used to keep the country list. It must offer the city groups, districts first.
test('the finder lists the city groups after find me', async ({ ctx }) => {
  await ctx.grantPermissions(['geolocation'])
  await ctx.setGeolocation({ longitude: 23.33, latitude: 42.70 })
  const page = await ctx.newPage()
  const gridLoaded = page.waitForResponse(/\/api\/v1\/hexes/)
  await page.goto('/en/')
  await mapSettled(page)
  await gridLoaded
  const sensors = page.waitForRequest(/\/api\/v1\/area\/sofia[^/]*\/sensors/)
  await page.getByRole('button', { name: 'Find me' }).click()
  await sensors
  await mapSettled(page)

  const input = page.locator('[data-island="finder"] input')
  const labels = page.locator('[data-island="finder"] .combobox__group-label')
  await expect(async () => {
    await input.click()
    await expect(labels).toHaveCount(2, { timeout: 1000 })
  }).toPass({ timeout: 15000 })
  await expect(labels.nth(0)).toHaveText('City districts')
  await expect(labels.nth(1)).toHaveText('Cities and provinces')
  await ctx.clearPermissions()
  await page.close()
})
