import { test, expect } from './fixtures.js'

// The layers button that opens the wind note must be a clear target, and the
// open note must sit unobscured inside the viewport.

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

// Centre plus four corners stepped in past the border radius; each must hit el itself.
const unobscured = (sel) => {
  const t = document.querySelector(sel)
  const r = t.getBoundingClientRect()
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
  return { w: r.width, h: r.height, bottom: r.bottom, right: r.right, vh: innerHeight, vw: innerWidth, hits }
}

const sizes = [{ width: 1350, height: 940 }, { width: 412, height: 823 }, { width: 393, height: 873 }]

for (const size of sizes) {
  test(`the layers button and the open wind note are unobscured at ${size.width}x${size.height}`, async ({ ctx }) => {
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
    const btn = await page.evaluate(unobscured, '.map__layers .colmenu__btn')
    expect(btn.w).toBeGreaterThanOrEqual(24)
    expect(btn.h).toBeGreaterThanOrEqual(24)
    expect(btn.hits.filter((h) => !h.ok), JSON.stringify(btn)).toEqual([])

    await page.locator('.map__layers .colmenu__btn').click()
    await expect(page.locator('.map-layers__wind')).toBeVisible()
    const note = await page.evaluate(unobscured, '.map-layers__wind')
    expect(note.bottom).toBeLessThanOrEqual(note.vh)
    expect(note.right).toBeLessThanOrEqual(note.vw)
    expect(note.hits.filter((h) => !h.ok), JSON.stringify(note)).toEqual([])
    await page.close()
  })
}
