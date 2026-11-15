import { test, expect } from './fixtures.js'

// From 1024px the home page hides the card under the map only while the map's dock
// is mounted (html.has-map-dock). Without the dock the card stays visible, so a dock
// that fails to mount can never leave the reader with no sensor panel at all.
test('home 1440: the card under the map hides only while the dock is mounted', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/en/#sensor=101')
  const dock = page.locator('.map-dock')
  const card = page.locator('[data-island="panel"].place-host--docked .sensor-panel')
  await expect(dock).toBeVisible({ timeout: 20000 })
  await expect(page.locator('html')).toHaveClass(/\bhas-map-dock\b/)
  await expect(card).toBeHidden()

  // The dock gone, the same card shows in place.
  await page.evaluate(() => {
    document.querySelector('.map-dock').remove()
    document.documentElement.classList.remove('has-map-dock')
  })
  await expect(card).toBeVisible()
  await page.close()
})
