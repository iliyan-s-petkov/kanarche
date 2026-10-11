import { test, expect } from './fixtures.js'

// A11Y-04 (epic #662): the uPlot canvas has a text alternative, a one-line summary on the plot
// and the readings in a visually hidden table beside it.
const CHART = '.chart-frame'

test.describe('chart text alternative', () => {
  test('the area chart is an image with a summary and a hidden readings table', async ({ page }) => {
    await page.goto('/en/area/sofia')
    const plot = page.locator(`${CHART} [role="img"]`)
    await expect(plot).toBeVisible()
    await expect(plot).toHaveAttribute('aria-label', /^PM2\.5, 24 hours: latest [\d.]+ µg\/m³, lowest [\d.]+ µg\/m³, highest [\d.]+ µg\/m³\.$/)

    const table = page.locator(`${CHART} .sr-only table`)
    await expect(table.locator('caption')).toHaveText(/^Readings for PM2\.5, 24 hours$/)
    await expect(table.locator('thead th[scope="col"]')).toHaveText(['Time', 'PM2.5 (µg/m³)'])
    expect(await table.locator('tbody tr').count()).toBeGreaterThan(1)
    await expect(table.locator('tbody tr').first().locator('th[scope="row"]')).toHaveText(/\d{1,2} \p{L}+ \d{2}:\d{2}/u)
    // Visually hidden: it takes no room on the page.
    const box = await page.locator(`${CHART} .sr-only`).boundingBox()
    expect(box === null || (box.width <= 2 && box.height <= 2)).toBe(true)
    // The table is outside the image, whose children assistive tech ignores.
    expect(await table.evaluate((t) => t.closest('[role="img"]') === null)).toBe(true)
  })

  test('the summary and table follow the period', async ({ page }) => {
    await page.goto('/en/area/sofia')
    const plot = page.locator(`${CHART} [role="img"]`)
    await expect(plot).toHaveAttribute('aria-label', /24 hours/)
    await page.locator('#area-period-select').selectOption('7d')
    await expect(plot).toHaveAttribute('aria-label', /^PM2\.5, 7 days: /)
    await expect(page.locator(`${CHART} table caption`)).toHaveText('Readings for PM2.5, 7 days')
  })

  test('the Bulgarian chart is described in Bulgarian', async ({ page }) => {
    await page.goto('/area/sofia')
    const plot = page.locator(`${CHART} [role="img"]`)
    await expect(plot).toHaveAttribute('aria-label', /^ФПЧ2\.5, 24 часа: последно [\d,]+ µg\/m³, най-ниско [\d,]+ µg\/m³, най-високо [\d,]+ µg\/m³\.$/)
  })

  // The sensor panel is a second island with its own attribute plumbing (sensor 101 has a day of P2).
  test('the sensor panel chart carries the same text alternative', async ({ page }) => {
    await page.goto('/en/#sensor=101')
    const plot = page.locator('.panel-chart [role="img"]')
    await expect(plot).toHaveAttribute('aria-label', /^PM2\.5, 24 hours: latest [\d.]+ µg\/m³, lowest [\d.]+ µg\/m³, highest [\d.]+ µg\/m³\.$/)
    await expect(page.locator('.panel-chart .sr-only table caption')).toHaveText('Readings for PM2.5, 24 hours')
  })
})
