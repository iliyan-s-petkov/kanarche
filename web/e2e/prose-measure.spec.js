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

test('area page does not overflow horizontally at 390', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 390, height: 800 })
  await page.goto('/en/area/sofia')
  const over = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  expect(over).toBeLessThanOrEqual(0)
  await page.close()
})
