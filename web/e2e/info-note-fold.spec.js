import { test, expect } from './fixtures.js'

// The wind note lives at the foot of the layers panel: off the map until the
// layers button opens it, then shown as the panel's last item.

const mockWind = (page) => page.route('**/api/v1/wind', (route) => route.fulfill({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify({
    generated_at: new Date().toISOString(),
    valid_at: new Date().toISOString(),
    model: 'Test Model',
    model_resolution_deg: 0.25,
    resolution_km: 25,
    forecast: true,
    vectors: [{ lon: 23.3, lat: 42.68, speed_ms: 3.2, direction_deg: 180 }],
  }),
}))

const sizes = [{ width: 1440, height: 900 }, { width: 393, height: 873 }]

for (const size of sizes) {
  test.describe(`wind note in the layers panel at ${size.width}x${size.height}`, () => {
    const open = async (ctx) => {
      const page = await ctx.newPage()
      await mockWind(page)
      await page.setViewportSize(size)
      await page.goto('/en')
      await page.waitForSelector('.map-layers__wind', { state: 'attached' })
      await expect.poll(async () => page.locator('.map-layers__wind').evaluate((el) => el.hidden)).toBe(false)
      await page.waitForFunction(() => {
        const map = document.querySelector('[data-island="map"]')?.__map
        return !!map?.isStyleLoaded?.() && !map.isMoving()
      })
      return page
    }

    test('the note is not on the map while the panel is closed', async ({ ctx }) => {
      const page = await open(ctx)
      await expect(page.locator('.map-layers__wind')).toBeHidden()
      await expect(page.locator('.map-wind-label')).toHaveCount(0)
      await page.close()
    })

    test('opening the layers button shows the note as the panel foot', async ({ ctx }) => {
      const page = await open(ctx)
      await page.locator('.map__layers .colmenu__btn').click()
      const note = page.locator('.map__layers .colmenu__panel > .map-layers__wind')
      await expect(note).toBeVisible()
      expect((await note.textContent()).trim().length).toBeGreaterThan(0)
      const last = await page.locator('.map__layers .colmenu__panel').evaluate((p) => p.lastElementChild?.className)
      expect(last).toBe('map-layers__wind')
      await page.close()
    })

    test('the layers button closes the panel and the note with it', async ({ ctx }) => {
      const page = await open(ctx)
      const btn = page.locator('.map__layers .colmenu__btn')
      await btn.click()
      await expect(page.locator('.map-layers__wind')).toBeVisible()
      await btn.click()
      await expect(page.locator('.map-layers__wind')).toBeHidden()
      await page.close()
    })
  })
}
