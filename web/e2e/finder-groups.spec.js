import { test, expect } from './fixtures.js'

// Zoomed over Sofia the map loads the city tier: cities plus the district
// (neighbourhood) Mladost. The finder lists districts first under their own
// heading, then the cities.
async function openOverSofia(ctx, path, vp) {
  const page = await ctx.newPage()
  await page.setViewportSize(vp)
  await page.goto(path)
  await page.waitForFunction(() => document.querySelector('[data-island="map"]')?.__map?.isStyleLoaded?.())
  await page.evaluate(() => document.querySelector('[data-island="map"]').__map.jumpTo({ center: [23.32, 42.69], zoom: 11 }))
  const input = page.locator('[data-island="finder"] input')
  await expect(async () => {
    await input.click()
    await expect(page.locator('[data-island="finder"] .combobox__group-label')).toHaveCount(2, { timeout: 1000 })
  }).toPass({ timeout: 15000 })
  return { page, input }
}

test('districts sit above cities under labelled groups; arrows skip headings', async ({ ctx }) => {
  const { page, input } = await openOverSofia(ctx, '/en/', { width: 1440, height: 900 })
  const labels = page.locator('[data-island="finder"] .combobox__group-label')
  await expect(labels.nth(0)).toHaveText('City districts')
  await expect(labels.nth(1)).toHaveText('Cities and provinces')
  const groups = page.locator('[data-island="finder"] [role="group"]')
  await expect(groups.nth(0).locator('[role="option"]')).toHaveText(['Mladost'])
  await expect(groups.nth(1).locator('[role="option"]').first()).not.toHaveText('Mladost')
  await input.press('ArrowDown')
  await expect(page.locator('[data-island="finder"] [role="option"][aria-selected="true"]')).toHaveText('Mladost')
  await input.press('ArrowDown')
  const active = await input.getAttribute('aria-activedescendant')
  await expect(page.locator(`#${active}`)).toHaveAttribute('role', 'option')
  await page.screenshot({ path: '/tmp/airbg-verify/findgroups-1440-en.png' })
  await input.fill('mlad')
  await expect(labels).toHaveCount(0)
  await page.close()
})

test('Bulgarian headings on a phone', async ({ ctx }) => {
  const { page } = await openOverSofia(ctx, '/', { width: 390, height: 844 })
  await expect(page.locator('[data-island="finder"] .combobox__group-label').nth(0)).toHaveText('Градски райони')
  await expect(page.locator('[data-island="finder"] .combobox__group-label').nth(1)).toHaveText('Градове и области')
  await page.screenshot({ path: '/tmp/airbg-verify/findgroups-390-bg.png' })
  await page.close()
})
