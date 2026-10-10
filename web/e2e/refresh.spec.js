import { test, expect, mapSettled } from './fixtures.js'

// OpenProject #660: one refresh control, a popover on the freshness pill.
const DESKTOP = { width: 1280, height: 800 }
const PHONE = { width: 393, height: 873 }

async function visit(ctx, vp, path = '/en/') {
  const page = await ctx.newPage()
  await page.setViewportSize(vp)
  await page.goto(path)
  await mapSettled(page)
  return page
}

// On a phone the pill lives in the window panel's footer, so that opens first.
async function openPopover(page, phone) {
  if (phone) await page.locator('.map-window__btn').click()
  const trigger = page.locator('.data-refresh__btn--icon')
  await trigger.click()
  await expect(page.locator('.data-refresh__panel')).toBeVisible()
  return trigger
}

const hexRequests = (page) => {
  const seen = []
  page.on('request', (r) => { if (r.url().includes('/api/v1/hexes')) seen.push(r.url()) })
  return seen
}

for (const [name, vp, phone] of [['desktop', DESKTOP, false], ['phone', PHONE, true]]) {
  test.describe(`refresh popover on ${name}`, () => {
    test('there is exactly one refresh control and it opens the popover', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      await expect(page.locator('.data-refresh__btn')).toHaveCount(1)
      await expect(page.locator('[data-island="refresh"], .toolbar__refresh, .data-refresh__auto')).toHaveCount(0)
      const trigger = await openPopover(page, phone)
      await expect(trigger).toHaveAttribute('aria-expanded', 'true')
      await expect(page.getByRole('button', { name: 'Refresh now' })).toBeVisible()
      const group = page.getByRole('radiogroup', { name: 'Refresh automatically' })
      await expect(group.getByRole('radio')).toHaveCount(4)
      await expect(group.getByRole('radio', { name: 'Every 5 min' })).toBeChecked()
      await page.close()
    })

    test('Refresh now refetches the hex grid and closes the popover', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      const hexes = hexRequests(page)
      await openPopover(page, phone)
      const before = hexes.length
      await page.getByRole('button', { name: 'Refresh now' }).click()
      await expect.poll(() => hexes.length).toBeGreaterThan(before)
      await expect(page.locator('.data-refresh__panel')).toBeHidden()
      await page.close()
    })

    test('the interval persists across a reload', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      await openPopover(page, phone)
      await page.getByRole('radio', { name: 'Every 15 min' }).check()
      expect(await page.evaluate(() => localStorage.getItem('kanarche:auto-refresh'))).toBe('15')
      await page.reload()
      await mapSettled(page)
      await openPopover(page, phone)
      await expect(page.getByRole('radio', { name: 'Every 15 min' })).toBeChecked()
      await page.getByRole('radio', { name: 'Off' }).check()
      expect(await page.evaluate(() => localStorage.getItem('kanarche:auto-refresh'))).toBe('0')
      await page.close()
    })

    test('Escape closes it and focus returns to the icon', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      const trigger = await openPopover(page, phone)
      await page.getByRole('radio', { name: 'Every 5 min' }).focus()
      await page.keyboard.press('Escape')
      await expect(page.locator('.data-refresh__panel')).toBeHidden()
      await expect(trigger).toBeFocused()
      await page.close()
    })

    test('a click outside closes it', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      await openPopover(page, phone)
      await page.locator('.masthead').click({ position: { x: 5, y: 5 } })
      await expect(page.locator('.data-refresh__panel')).toBeHidden()
      await page.close()
    })

    test('the popover fits the viewport and sits above the dock and sheet', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      await openPopover(page, phone)
      const box = await page.locator('.data-refresh__panel').boundingBox()
      expect(box.x).toBeGreaterThanOrEqual(0)
      expect(box.y).toBeGreaterThanOrEqual(0)
      expect(box.x + box.width).toBeLessThanOrEqual(vp.width)
      expect(box.y + box.height).toBeLessThanOrEqual(vp.height)
      // The element under the panel's centre is the panel, not the sheet or the dock.
      const top = await page.evaluate(({ x, y }) => !!document.elementFromPoint(x, y)?.closest('.data-refresh__panel'), {
        x: box.x + box.width / 2, y: box.y + box.height / 2,
      })
      expect(top).toBe(true)
      await page.close()
    })
  })
}

test('an old boolean setting is read as 5 minutes or off', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize(DESKTOP)
  await page.addInitScript(() => localStorage.setItem('kanarche:auto-refresh', 'false'))
  await page.goto('/en/')
  await mapSettled(page)
  await openPopover(page, false)
  await expect(page.getByRole('radio', { name: 'Off' })).toBeChecked()
  await page.close()
})
