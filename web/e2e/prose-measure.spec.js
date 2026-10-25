import { test, expect } from './fixtures.js'

// Prose measure follows the viewport: wider than 65ch on wide
// screens, never past the 100ch ceiling, no horizontal overflow on a phone.
const chWidth = (page, ch) => page.evaluate((n) => {
  const probe = document.createElement('div')
  probe.style.font = getComputedStyle(document.querySelector('.faq__answer')).font
  probe.style.inlineSize = `${n}ch`
  document.body.append(probe)
  const w = probe.getBoundingClientRect().width
  probe.remove()
  return w
}, ch)

test('area FAQ prose is wider than 65ch and within 100ch at 1920', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1920, height: 900 })
  await page.goto('/en/area/sofia')
  await page.locator('.faq__item > summary').first().click()
  const w100 = await chWidth(page, 100)
  const widths = await page.locator('.faq__answer, .faq__more').evaluateAll(
    (els) => els.map((e) => e.getBoundingClientRect().width))
  const faq = await page.locator('.faq').evaluate((e) => e.getBoundingClientRect().width)
  const column = await page.locator('.faq').evaluate((e) => {
    const cs = getComputedStyle(e.parentElement)
    return e.parentElement.getBoundingClientRect().width - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight)
  })
  const w65 = await chWidth(page, 65)
  expect(widths.length).toBeGreaterThan(0)
  for (const w of widths) {
    expect(w).toBeGreaterThan(w65 + 1)
    expect(w).toBeLessThanOrEqual(w100 + 1)
  }
  // The block spans the content column; only the text is capped.
  expect(Math.abs(faq - column)).toBeLessThanOrEqual(1)
  await page.close()
})

// Screenshots for review, written only when KANARCHE_MEASURE_SHOTS names a directory.
const SHOTS = process.env.KANARCHE_MEASURE_SHOTS
test('measure screenshots', async ({ ctx }) => {
  test.skip(!SHOTS, 'set KANARCHE_MEASURE_SHOTS to write screenshots')
  test.setTimeout(240_000)
  for (const path of ['/en/area/sofia', '/area/sofia', '/en/area/sofia-oblast', '/area/sofia-oblast', '/en/area/mladost', '/area/mladost', '/en/about', '/en/about-the-data', '/en/air-pollution', '/en/pm25', '/en/privacy', '/en/']) {
    for (const w of [390, 1440, 1920]) {
      const page = await ctx.newPage()
      await page.setViewportSize({ width: w, height: 900 })
      await page.goto(path)
      await page.waitForTimeout(500)
      const over = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
      expect(over, `${path} at ${w}`).toBeLessThanOrEqual(0)
      await page.screenshot({ path: `${SHOTS}/v2-after-${path.replace(/\//g, '_')}-${w}.png`, fullPage: true })
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
