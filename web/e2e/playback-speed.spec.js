import { test, expect } from './fixtures.js'

// The replay speed is a small menu on the play bar: choose a speed, it labels
// the button, and the choice survives a reload.

const VIEWPORTS = [
  { name: 'phone 393x873', viewport: { width: 393, height: 873 }, isMobile: true, hasTouch: true },
  { name: 'desktop 1280x800', viewport: { width: 1280, height: 800 } },
]

const startReplay = async (page) => {
  await page.goto('/en')
  const play = page.locator('.map-play__btn[aria-pressed]')
  await play.evaluate((el) => el.scrollIntoView({ block: 'center' }))
  await play.click()
  await expect(page.locator('.map-play--open')).toHaveCount(1)
}

for (const vp of VIEWPORTS) {
  test.describe(vp.name, () => {
    test('choosing a speed labels the button and survives a reload', async ({ browser }) => {
      const { name, ...opts } = vp
      const context = await browser.newContext(opts)
      const page = await context.newPage()
      await startReplay(page)

      const button = page.locator('.map-play__speed')
      await expect(button).toHaveText('0.5×')
      await button.click()
      const panel = page.locator('.map-play__speedpanel')
      await expect(panel).toBeVisible()
      await expect(panel.getByRole('menuitemradio')).toHaveCount(4)
      await expect(panel.getByRole('menuitemradio', { name: '0.5×' })).toHaveAttribute('aria-checked', 'true')

      // The menu opens upward and stays inside the map frame.
      const map = await page.locator('#map').boundingBox()
      const box = await panel.boundingBox()
      expect(box.y).toBeGreaterThanOrEqual(map.y)
      expect(box.x).toBeGreaterThanOrEqual(map.x)
      expect(box.x + box.width).toBeLessThanOrEqual(map.x + map.width)

      await panel.getByRole('menuitemradio', { name: '2×' }).click()
      await expect(panel).toBeHidden()
      await expect(button).toHaveText('2×')

      await page.reload()
      await startReplay(page)
      await expect(page.locator('.map-play__speed')).toHaveText('2×')
      await context.close()
    })

    test('Escape closes the menu and returns focus to the button', async ({ browser }) => {
      const { name, ...opts } = vp
      const context = await browser.newContext(opts)
      const page = await context.newPage()
      await startReplay(page)

      const button = page.locator('.map-play__speed')
      await button.focus()
      await button.press('Enter')
      await expect(page.locator('.map-play__speedpanel')).toBeVisible()
      await page.keyboard.press('ArrowDown')
      await page.keyboard.press('Escape')
      await expect(page.locator('.map-play__speedpanel')).toBeHidden()
      await expect(button).toBeFocused()
      await context.close()
    })
  })
}
