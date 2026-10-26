import { test, expect, mapSettled } from './fixtures.js'

// The reset button returns zoom, tilt and rotation: north-up and flat.
const VIEWPORTS = [
  ['1440x900', { width: 1440, height: 900 }],
  ['393x873', { width: 393, height: 873 }],
]

for (const [name, size] of VIEWPORTS) {
  test(`reset-view clears pitch and bearing at ${name}`, async ({ page }) => {
    await page.setViewportSize(size)
    await page.goto('/en/')
    await mapSettled(page)
    await page.evaluate(() => {
      document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
      document.querySelector('[data-island="map"]').__map.jumpTo({ pitch: 45, bearing: 30 })
    })
    const camera = () => page.evaluate(() => {
      const m = document.querySelector('[data-island="map"]').__map
      return { pitch: m.getPitch(), bearing: m.getBearing() }
    })
    expect(await camera()).toEqual({ pitch: 45, bearing: 30 })
    // Reset view lives in the compass panel.
    await page.locator('.map-orient__btn').click()
    await page.locator('.map-orient__panel [data-act="reset"]').click()
    await expect.poll(camera, { timeout: 15_000 }).toEqual({ pitch: 0, bearing: 0 })
  })
}
