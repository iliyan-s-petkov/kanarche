import { test as base, expect } from './fixtures.js'

// EN routes; sensor 101 on the Sofia area page has P1 and P2 readings, 24h of
// history, and an area (so the nearby-sensors item is enabled).
const test = base.extend({
  phone: [async ({ browser }, use) => {
    const context = await browser.newContext({
      viewport: { width: 390, height: 844 },
      isMobile: true,
      hasTouch: true,
      deviceScaleFactor: 2,
      permissions: ['clipboard-read', 'clipboard-write'],
    })
    await use(context)
    await context.close()
  }, { scope: 'worker' }],
})

test.afterEach(async ({ phone }) => {
  for (const p of phone.pages()) if (!p.isClosed()) await p.close().catch(() => {})
})

async function open(phone) {
  const page = await phone.newPage()
  await page.goto('/en/area/sofia#sensor=101')
  await expect(page.locator('.sensor-panel')).toBeVisible({ timeout: 15000 })
  await expect(page.locator('.sensor-panel .chart-frame')).toBeVisible({ timeout: 15000 })
  return page
}

const ITEMS = [/Nearby sensors/, /Custom range/, /Reset chart/, /About this station/, /Share/, /Embed/]

test.describe('phone sensor sheet', () => {
  test('the chart starts high in the sheet', async ({ phone }) => {
    const page = await open(phone)
    const top = await page.evaluate(() =>
      document.querySelector('.sensor-panel .chart-frame').getBoundingClientRect().top -
      document.querySelector('.sensor-panel').getBoundingClientRect().top)
    console.log(`chart top within panel: ${Math.round(top)}px`)
    expect(top).toBeLessThan(300)
  })

  test('header carries network and age, an info button and a close button', async ({ phone }) => {
    const page = await open(phone)
    const panel = page.locator('.sensor-panel')
    await expect(panel.locator('.panel-sub')).toContainText(/sensor\.community|eea/i)
    await expect(panel.getByRole('button', { name: 'About this station' }).first()).toBeVisible()
    await expect(panel.getByRole('button', { name: 'Close' })).toBeVisible()
  })

  test('sticky header edges align with the card', async ({ phone }) => {
    const page = await open(phone)
    const r = await page.evaluate(() => {
      const box = (s) => document.querySelector(s).getBoundingClientRect()
      const c = box('.sensor-panel'), h = box('.sensor-panel > header')
      return { dl: h.left - c.left, dr: h.right - c.right }
    })
    expect(Math.abs(r.dl)).toBeLessThanOrEqual(1)
    expect(Math.abs(r.dr)).toBeLessThanOrEqual(1)
  })

  test('BG gauge labels use a decimal comma', async ({ phone }) => {
    const page = await phone.newPage()
    await page.goto('/area/sofia#sensor=101')
    await expect(page.locator('.sensor-panel')).toBeVisible({ timeout: 15000 })
    const values = await page.locator('.sensor-panel .gauge__value').allInnerTexts()
    expect(values.some((v) => /^\d+,\d/.test(v))).toBe(true)
    expect(values.some((v) => /^\d+\.\d/.test(v))).toBe(false)
  })

  test('tapping a gauge selects the chart metric', async ({ phone }) => {
    const page = await open(phone)
    const gauges = page.locator('.sensor-panel button.gauge')
    const count = await gauges.count()
    expect(count).toBeGreaterThan(1)
    const pressedBefore = await page.locator('.sensor-panel button.gauge[aria-pressed="true"]').count()
    expect(pressedBefore).toBe(1)
    const idx = (await gauges.evaluateAll((els) => els.map((e) => e.getAttribute('aria-pressed')))).indexOf('false')
    expect(idx).toBeGreaterThanOrEqual(0)
    const other = gauges.nth(idx)
    const requested = page.waitForRequest((r) => r.url().includes('/series?metric='))
    await other.tap()
    await requested
    await expect(other).toHaveAttribute('aria-pressed', 'true')
    await expect(page.locator('.sensor-panel button.gauge[aria-pressed="true"]')).toHaveCount(1)
  })

  test('the period control is a segmented control of 24h / 7d / 30d / 1y', async ({ phone }) => {
    const page = await open(phone)
    const seg = page.locator('.sensor-panel .period-seg')
    await expect(seg.getByRole('button')).toHaveText(['24h', '7d', '30d', '1y'])
    await expect(seg.getByRole('button', { name: '24h' })).toHaveAttribute('aria-pressed', 'true')
    await seg.getByRole('button', { name: '7d' }).tap()
    await expect(seg.getByRole('button', { name: '7d' })).toHaveAttribute('aria-pressed', 'true')
    await expect(seg.getByRole('button', { name: '24h' })).toHaveAttribute('aria-pressed', 'false')
  })

  test('the more menu lists the six items', async ({ phone }) => {
    const page = await open(phone)
    const more = page.getByRole('button', { name: 'More actions' })
    await expect(more).toHaveAttribute('aria-expanded', 'false')
    await more.tap()
    await expect(more).toHaveAttribute('aria-expanded', 'true')
    const items = page.getByRole('menu').getByRole('menuitem')
    await expect(items).toHaveCount(6)
    for (const name of ITEMS) await expect(page.getByRole('menuitem', { name })).toBeVisible()
  })

  test('menu: nearby sensors offers the band lines', async ({ phone }) => {
    const page = await open(phone)
    await page.getByRole('button', { name: 'More actions' }).tap()
    await page.getByRole('menuitem', { name: /Nearby sensors/ }).tap()
    const box = page.getByRole('menu').getByRole('checkbox').first()
    await expect(box).toBeVisible()
    await box.check()
    await expect(box).toBeChecked()
  })

  test('menu: custom range shows the from/to fields', async ({ phone }) => {
    const page = await open(phone)
    await page.getByRole('button', { name: 'More actions' }).tap()
    await page.getByRole('menuitem', { name: /Custom range/ }).tap()
    await expect(page.getByRole('menu')).toBeHidden()
    await expect(page.locator('.sensor-panel .chart-range input[type="datetime-local"]')).toHaveCount(2)
    await expect(page.locator('.sensor-panel .chart-range input').first()).toBeVisible()
  })

  test('menu: reset returns the window to 24h', async ({ phone }) => {
    const page = await open(phone)
    const seg = page.locator('.sensor-panel .period-seg')
    await seg.getByRole('button', { name: '30d' }).tap()
    await expect(seg.getByRole('button', { name: '30d' })).toHaveAttribute('aria-pressed', 'true')
    await page.getByRole('button', { name: 'More actions' }).tap()
    await page.getByRole('menuitem', { name: /Reset chart/ }).tap()
    await expect(seg.getByRole('button', { name: '24h' })).toHaveAttribute('aria-pressed', 'true')
  })

  test('menu: about this station opens a bottom sheet that Escape closes', async ({ phone }) => {
    const page = await open(phone)
    await page.getByRole('button', { name: 'More actions' }).tap()
    await page.getByRole('menuitem', { name: /About this station/ }).tap()
    const sheet = page.getByRole('dialog', { name: 'About this station' })
    await expect(sheet).toBeVisible()
    await expect(sheet).toContainText('Devices')
    const box = await sheet.boundingBox()
    expect(box.y + box.height).toBeGreaterThan(844 - 2)
    await page.keyboard.press('Escape')
    await expect(sheet).toHaveCount(0)
  })

  test('header info button opens the same sheet', async ({ phone }) => {
    const page = await open(phone)
    await page.locator('.sensor-panel .panel-info').tap()
    await expect(page.getByRole('dialog', { name: 'About this station' })).toBeVisible()
    await page.getByRole('dialog').getByRole('button', { name: 'Close' }).tap()
    await expect(page.getByRole('dialog')).toHaveCount(0)
  })

  test('menu: share copies the link to this sensor', async ({ phone }) => {
    const page = await open(phone)
    await page.evaluate(() => { Object.defineProperty(navigator, 'share', { value: undefined, configurable: true }) })
    await page.getByRole('button', { name: 'More actions' }).tap()
    await page.getByRole('menuitem', { name: /Share/ }).tap()
    await expect(page.locator('.sensor-panel [role="status"]')).toContainText('Link copied')
    expect(await page.evaluate(() => navigator.clipboard.readText())).toContain('#sensor=101')
  })

  test('menu: embed copies an iframe snippet', async ({ phone }) => {
    const page = await open(phone)
    await page.getByRole('button', { name: 'More actions' }).tap()
    await page.getByRole('menuitem', { name: /Embed/ }).tap()
    await expect(page.locator('.sensor-panel [role="status"]')).toContainText('Embed code copied')
    const text = await page.evaluate(() => navigator.clipboard.readText())
    expect(text).toMatch(/^<iframe src="http[^"]+\/embed/)
  })

  test('keyboard: Enter opens the menu, arrows move, Escape closes and returns focus', async ({ phone }) => {
    const page = await open(phone)
    const more = page.getByRole('button', { name: 'More actions' })
    await more.focus()
    await page.keyboard.press('Enter')
    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()
    await expect(page.getByRole('menuitem').first()).toBeFocused()
    await page.keyboard.press('ArrowDown')
    await expect(page.getByRole('menuitem').nth(1)).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(menu).toBeHidden()
    await expect(more).toBeFocused()
    // The panel itself stays open: Escape closed only the menu.
    await expect(page.locator('.sensor-panel')).toBeVisible()
  })
})
