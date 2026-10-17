import { test, expect } from './fixtures.js'

// Screen readers hear the unit in words while the symbol stays on screen, and the
// home page carries a hidden one-line summary of the country's air.
test('the home readouts and summary are spoken in words', async ({ ctx }) => {
  const page = await ctx.newPage()
  try {
    await page.goto('/en/')
    const meta = page.locator('meta[name="spoken-units"]')
    await expect(meta).toHaveAttribute('content', /micrograms per cubic metre/)

    const unit = page.locator('[data-island="readouts"] .readout__unit, .readout__unit').first()
    await expect(unit).toHaveAttribute('aria-hidden', 'true')
    await expect(unit).toHaveText('µg/m³')
    await expect(page.locator('.readout .visually-hidden, .readout .sr-only').first())
      .toHaveText(/micrograms per cubic metre/)

    const summary = page.locator('#home-summary')
    await expect(summary).toHaveText(/\S/)
    await expect(summary).toContainText('micrograms per cubic metre')
  } finally {
    await page.close()
  }
})
