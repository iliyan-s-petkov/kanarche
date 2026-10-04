import { test, expect } from './fixtures.js'

const SNIPPET_RE = /^<iframe src="https?:\/\/[^"]+\/embed\?area=plovdiv-oblast"\n\s+title="Kanarche" width="100%" height="480"\n\s+loading="lazy" style="border:0"><\/iframe>$/

test('the embed block copies its exact snippet and says Copied', async ({ browser }) => {
  const context = await browser.newContext({ permissions: ['clipboard-read', 'clipboard-write'] })
  const page = await context.newPage()
  await page.goto('/en/about')
  const button = page.locator('.about-code__copy')
  await expect(button).toHaveText('Copy')
  const expected = await page.locator('.about-code code').evaluate((c) => c.textContent)
  await button.focus()
  await page.keyboard.press('Enter')
  await expect(button).toHaveText('Copied')
  await expect(page.locator('.about-code-wrap [aria-live="polite"]')).toHaveText('Copied')
  const clip = await page.evaluate(() => navigator.clipboard.readText())
  expect(clip).toBe(expected)
  expect(clip).toMatch(SNIPPET_RE)
  await expect(button).toHaveText('Copy', { timeout: 5000 })
  await context.close()
})

test('the link cards carry icons and the corner arrows', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/en/about')
  const cards = page.locator('.about-links a')
  await expect(cards).toHaveCount(3)
  await expect(page.locator('.about-links__icon')).toHaveCount(3)
  await expect(page.locator('.about-links__go')).toHaveText(['→', '↗', '↗'])
  const over = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  expect(over).toBeLessThanOrEqual(0)
  await page.close()
})
