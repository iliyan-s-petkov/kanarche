import { test, expect, mapSettled } from './fixtures.js'

// A11Y-07, 10, 11, 13 and 17 (epic #662, PR 1): names and announcements that
// were wrong or missing. Desktop and English, so the strings are readable.

async function visit(ctx, path = '/en/') {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1280, height: 800 })
  await page.goto(path)
  await mapSettled(page)
  return page
}

test('the scale dialog has a metric title, a unit line and two distinct close names', async ({ ctx }) => {
  const page = await visit(ctx)
  const legend = page.locator('.scale--onmap')
  if ((await legend.getAttribute('open')) === null) await legend.locator('> .scale__toggle').click()
  await legend.locator('> .scale__label .scale__info').click()
  const dialog = page.locator('dialog.scaleinfo[open]')
  await expect(dialog).toBeVisible()
  await expect(page.getByRole('dialog', { name: 'PM2.5 colour scale' })).toBeVisible()
  const title = await dialog.locator('h2').textContent()
  expect(title).toMatch(/colour scale$/)
  expect(title).not.toContain('µg')
  await expect(dialog.locator('.scaleinfo__units')).toContainText('Values are in')
  await expect(dialog.getByRole('button', { name: 'Close colour scale' })).toHaveCount(1)
  await expect(dialog.getByRole('button', { name: 'Close', exact: true })).toHaveCount(1)
  const ranges = await dialog.locator('.scaleinfo__band-range').allTextContents()
  for (const r of ranges) expect(r).not.toMatch(/[–—<>]/)
})

// The time is visible beside the icon, so WCAG 2.5.3 puts it in the name after the word.
test('the refresh button name is the word followed by the visible time', async ({ ctx }) => {
  const page = await visit(ctx)
  const trigger = page.locator('.data-refresh__btn--icon')
  const time = (await trigger.locator('.data-refresh__time').textContent()).trim()
  await expect(trigger).toHaveAttribute('aria-label', `Refresh ${time}`)
})

test('the legend band list is named once with its unit and each row speaks its range', async ({ ctx }) => {
  const page = await visit(ctx)
  const list = page.locator('.scale--onmap .scale__bands')
  await expect(list).toHaveAttribute('aria-label', /bands, values in /)
  const first = page.locator('.scale--onmap .scale__band').first()
  await expect(first.locator('.sr-only')).toContainText('above')
  const last = page.locator('.scale--onmap .scale__band').last()
  await expect(last.locator('.sr-only')).toContainText('up to')
})

test('the finder announces the result count as the reader types', async ({ ctx }) => {
  const page = await visit(ctx)
  const input = page.getByRole('combobox', { name: 'Find an area' })
  await input.fill('So')
  const status = page.locator('.toolbar__find [role="status"]')
  await expect(status).toHaveText(/areas? found|No area by that name/)
})

test('the Data table link is visible, and takes a visible focus', async ({ ctx }) => {
  const page = await visit(ctx)
  const link = page.getByRole('link', { name: 'Data table', exact: true })
  await expect(link).toBeVisible()
  const box = await link.boundingBox()
  expect(box.width).toBeGreaterThan(8)
  expect(box.height).toBeGreaterThanOrEqual(24)
  await link.focus()
  const ring = await link.evaluate((a) => {
    const s = getComputedStyle(a)
    return (s.outlineStyle !== 'none' && parseFloat(s.outlineWidth) > 0) || s.boxShadow !== 'none'
  })
  expect(ring).toBe(true)
  await link.click()
  await expect(page).toHaveURL(/\/en\/areas$/)
})
