import { test, expect } from './fixtures.js'

// The folded wind toggle must be a clear 24x24 target: nothing may sit over any part of it.

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

const sizes = [{ width: 1350, height: 940 }, { width: 412, height: 823 }, { width: 393, height: 873 }]

for (const size of sizes) {
  test(`the folded wind toggle is unobscured at ${size.width}x${size.height}`, async ({ ctx }) => {
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
    await expect(page.locator('.map-wind-label')).not.toHaveAttribute('open', '')
    const res = await page.evaluate(() => {
      const t = document.querySelector('.map-wind-label__toggle')
      const r = t.getBoundingClientRect()
      // A rounded corner is not hit-testable at its very edge: step in past the curve.
      const radius = parseFloat(getComputedStyle(t).borderTopLeftRadius) || 0
      const d = Math.max(2, Math.ceil(radius * 0.3) + 1)
      const pts = [
        [r.left + r.width / 2, r.top + r.height / 2],
        [r.left + d, r.top + d], [r.right - d, r.top + d],
        [r.left + d, r.bottom - d], [r.right - d, r.bottom - d],
      ]
      const hits = pts.map(([x, y]) => {
        const top = document.elementsFromPoint(x, y)[0]
        return { x, y, ok: !!top && t.contains(top), top: top ? `${top.tagName}.${top.className}#${top.id}` : null }
      })
      return { w: r.width, h: r.height, hits }
    })
    expect(res.w).toBeGreaterThanOrEqual(24)
    expect(res.h).toBeGreaterThanOrEqual(24)
    expect(res.hits.filter((h) => !h.ok), JSON.stringify(res)).toEqual([])
    await page.close()
  })
}
