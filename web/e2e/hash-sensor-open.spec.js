import { test, expect, mapSettled } from './fixtures.js'

// A #sensor= that arrives on a page that has already loaded (a share link
// pasted into the open tab, Back/Forward between entries) must open the sensor
// the same way a fresh load of that URL does.
test('a #sensor= hash change on a loaded home page opens the sensor', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/')
  await mapSettled(page)
  await expect(page.locator('.map-dock')).toHaveCount(0)

  await page.goto('/#sensor=101')

  await expect(page.locator('.map-dock')).toBeVisible({ timeout: 15000 })
  await expect(page).toHaveURL(/#sensor=101$/)
})
