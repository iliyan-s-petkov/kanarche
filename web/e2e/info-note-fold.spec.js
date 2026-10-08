import { test, expect } from './fixtures.js'

// The wind note folds on a click on the map or on the note, and on Escape.

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
  test.describe(`wind note fold at ${size.width}x${size.height}`, () => {
    const open = async (ctx) => {
      const page = await ctx.newPage()
      await mockWind(page)
      await page.setViewportSize(size)
      await page.goto('/en')
      await page.waitForSelector('.map-wind-label', { state: 'attached' })
      await expect.poll(async () => page.locator('.map-wind-label').evaluate((el) => el.hidden)).toBe(false)
      await page.waitForFunction(() => {
        const map = document.querySelector('[data-island="map"]')?.__map
        return !!map?.isStyleLoaded?.() && !map.isMoving()
      })
      const note = page.locator('.map-wind-label')
      const toggle = page.locator('.map-wind-label__toggle')
      await toggle.click()
      await expect(note).toHaveAttribute('open', '')
      return { page, note, toggle }
    }

    // A point on the map clear of the note, the controls and the panels.
    const emptySpot = async (page) => {
      // The map can be taller than the viewport; aim inside the part on screen.
      const box = await page.locator('#map').boundingBox()
      const view = page.viewportSize()
      const top = Math.max(box.y, 0)
      const bottom = Math.min(box.y + box.height, view.height)
      const p = { x: box.x + box.width * 0.5, y: top + (bottom - top) * 0.35 }
      const hit = await page.evaluate(([x, y]) => document.elementFromPoint(x, y)?.closest('.maplibregl-canvas') !== null, [p.x, p.y])
      expect(hit, 'the click point must land on the map canvas').toBe(true)
      return p
    }

    test('the folded toggle is at least 24x24 CSS px', async ({ ctx }) => {
      const { page, note, toggle } = await open(ctx)
      await toggle.click()
      await expect(note).not.toHaveAttribute('open', '')
      const box = await toggle.boundingBox()
      expect(box.width).toBeGreaterThanOrEqual(24)
      expect(box.height).toBeGreaterThanOrEqual(24)
      await page.close()
    })

    test('a click on the note folds it', async ({ ctx }) => {
      const { page, note, toggle } = await open(ctx)
      await expect(toggle).toHaveAttribute('aria-expanded', 'true')
      await page.locator('.map-wind-label__text').click()
      await expect(note).not.toHaveAttribute('open', '')
      await expect(toggle).toHaveAttribute('aria-expanded', 'false')
      await page.close()
    })

    test('a click on the map folds it', async ({ ctx }) => {
      const { page, note, toggle } = await open(ctx)
      const p = await emptySpot(page)
      await page.mouse.click(p.x, p.y)
      await expect(note).not.toHaveAttribute('open', '')
      await expect(toggle).toHaveAttribute('aria-expanded', 'false')
      await page.close()
    })

    test('a drag on the map leaves it open', async ({ ctx }) => {
      const { page, note } = await open(ctx)
      const p = await emptySpot(page)
      await page.mouse.move(p.x, p.y)
      await page.mouse.down()
      await page.mouse.move(p.x + 60, p.y + 40, { steps: 8 })
      await page.mouse.up()
      await page.waitForTimeout(300)
      await expect(note).toHaveAttribute('open', '')
      await page.close()
    })

    test('Escape folds it and returns focus to the toggle', async ({ ctx }) => {
      const { page, note, toggle } = await open(ctx)
      await page.keyboard.press('Escape')
      await expect(note).not.toHaveAttribute('open', '')
      await expect(toggle).toBeFocused()
      await expect(toggle).toHaveAttribute('aria-expanded', 'false')
      await page.close()
    })

    test('the toggle still toggles', async ({ ctx }) => {
      const { page, note, toggle } = await open(ctx)
      await toggle.click()
      await expect(note).not.toHaveAttribute('open', '')
      await toggle.click()
      await expect(note).toHaveAttribute('open', '')
      await page.close()
    })
  })
}
