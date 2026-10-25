import { test, expect } from './fixtures.js'

// A wide readout ("1013,0 hPa") must wrap or shrink inside its card: the page
// may not scroll sideways on a phone. Seeded on sensor 101 in e2e_test.go.
for (const width of [320, 360, 390]) {
  for (const path of ['/area/mladost', '/en/area/mladost', '/en/area/sofia']) {
    test(`readouts do not overflow the page: ${path} @${width}`, async ({ ctx }) => {
      const page = await ctx.newPage()
      await page.setViewportSize({ width, height: 844 })
      await page.goto(path)
      await page.waitForTimeout(800)
      const m = await page.evaluate(() => ({
        scroll: document.documentElement.scrollWidth,
        inner: window.innerWidth,
        hasWide: [...document.querySelectorAll('.readout__value')].some((v) => /1013/.test(v.textContent)),
      }))
      expect(m.hasWide, 'fixture pressure card rendered').toBe(true)
      expect(m.scroll, 'document scrollWidth').toBeLessThanOrEqual(m.inner)
      await page.close()
    })
  }
}
