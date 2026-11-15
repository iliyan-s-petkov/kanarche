import { test, expect } from './fixtures.js'

// Zoomed over Sofia the map loads the city tier: cities plus the district
// (neighbourhood) Mladost. The finder lists districts first under their own
// heading, then the cities.
async function openOverSofia(ctx, path, vp) {
  const page = await ctx.newPage()
  await page.setViewportSize(vp)
  await page.goto(path)
  await page.waitForFunction(() => document.querySelector('[data-island="map"]')?.__map?.isStyleLoaded?.())
  await page.waitForTimeout(1000)
  await expect.poll(() => page.evaluate(() => document.querySelector('[data-island="map"]').__map.isMoving())).toBe(false)
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
  await expect(groups.nth(0).locator('[role="option"]')).toHaveText(['Mladost', 'Mladost'])
  await expect(groups.nth(1).locator('[role="option"]').first()).not.toHaveText('Mladost')
  await input.press('ArrowDown')
  await expect(page.locator('[data-island="finder"] [role="option"][aria-selected="true"]')).toHaveText('Mladost')
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

// Sofia and Varna each have a district called Младост. Both must list, and the
// fixture fails the test on any uncaught page error (each_key_duplicate used to
// abort the flush), so the dock still has to open on a sensor afterwards.
test('two districts with the same name both list and the dock still opens', async ({ ctx }) => {
  const { page, input } = await openOverSofia(ctx, '/', { width: 1440, height: 900 })
  await input.fill('Млад')
  await expect(page.locator('[data-island="finder"] [role="option"]')).toHaveCount(2)
  await expect(page.locator('[data-island="finder"] [role="option"]')).toHaveText(['Младост', 'Младост'])
  await page.goto('/#sensor=101')
  await expect(page.locator('.map-dock')).toBeVisible({ timeout: 20000 })
  await page.close()
})
