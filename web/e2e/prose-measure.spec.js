import { test, expect } from './fixtures.js'

// Prose measure follows the viewport: wider than the old 70ch cap on wide
// screens, never past the 90ch ceiling, no horizontal overflow on a phone.
const chWidth = (page, ch) => page.evaluate((n) => {
  const probe = document.createElement('div')
  probe.style.font = getComputedStyle(document.querySelector('.faq__answer')).font
  probe.style.inlineSize = `${n}ch`
  document.body.append(probe)
  const w = probe.getBoundingClientRect().width
  probe.remove()
  return w
}, ch)

test('area FAQ prose is wider than 70ch and within 90ch at 1920', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1920, height: 900 })
  await page.goto('/en/area/sofia')
  await page.locator('.faq__item > summary').first().click()
  const w70 = await chWidth(page, 70)
  const w90 = await chWidth(page, 90)
  const widths = await page.locator('.faq__answer, .faq__more').evaluateAll(
    (els) => els.map((e) => e.getBoundingClientRect().width))
  const faq = await page.locator('.faq').evaluate((e) => e.getBoundingClientRect().width)
  expect(widths.length).toBeGreaterThan(0)
  for (const w of widths) {
    expect(w).toBeGreaterThan(w70 + 1)
    expect(w).toBeLessThanOrEqual(w90 + 1)
  }
  expect(faq).toBeLessThanOrEqual(w90 + 1)
  await page.close()
})

// Screenshots for review, written only when KANARCHE_MEASURE_SHOTS names a directory.
const SHOTS = process.env.KANARCHE_MEASURE_SHOTS
test('measure screenshots', async ({ ctx }) => {
  test.skip(!SHOTS, 'set KANARCHE_MEASURE_SHOTS to write screenshots')
  for (const path of ['/en/area/sofia', '/area/sofia', '/en/about', '/en/about-the-data', '/en/air-pollution', '/en/pm25', '/en/privacy', '/en/']) {
    for (const w of [390, 768, 1280, 1440, 1920, 2560]) {
      const page = await ctx.newPage()
      await page.setViewportSize({ width: w, height: 900 })
      await page.goto(path)
      await page.waitForTimeout(500)
      const over = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
      expect(over, `${path} at ${w}`).toBeLessThanOrEqual(0)
      await page.screenshot({ path: `${SHOTS}/after-${path.replace(/\//g, '_')}-${w}.png`, fullPage: true })
      await page.close()
    }
  }
})

test('area page does not overflow horizontally at 390', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 390, height: 800 })
  await page.goto('/en/area/sofia')
  const over = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  expect(over).toBeLessThanOrEqual(0)
  await page.close()
})
