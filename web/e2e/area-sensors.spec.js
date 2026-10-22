import { createRequire } from 'node:module'
import { test, expect, mapSettled } from './fixtures.js'

// The area page's sensor table: the map's text alternative and the keyboard route to each sensor.
// internal/e2e/e2e_test.go seeds sensors 101, 102 and 103 inside the sofia area.
const axeSource = createRequire(import.meta.url).resolve('axe-core/axe.min.js')

async function axeViolations(page, selector) {
  await page.addScriptTag({ path: axeSource })
  return page.evaluate(async (sel) => {
    const r = await window.axe.run(sel)
    return r.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)
  }, selector)
}

const VIEWPORTS = { desktop: { width: 1280, height: 800 }, phone: { width: 390, height: 844 } }
const THEMES = ['light', 'dark']

const table = (page) => page.locator('details.area-sensors:not([data-area-sensors-static])')
const summary = (page) => table(page).locator('summary')
const rowButton = (page, id) => table(page).getByRole('button', { name: `Show sensor ${id} on the map` })

async function openTable(page) {
  await summary(page).focus()
  await page.keyboard.press('Enter')
  await expect(rowButton(page, 101)).toBeVisible({ timeout: 20000 })
}

for (const [label, viewport] of Object.entries(VIEWPORTS)) {
  test(`keyboard opens a sensor from the table and Esc returns to its row (${label})`, async ({ browser }, testInfo) => {
    testInfo.setTimeout(90000)
    const context = await browser.newContext({ viewport })
    const page = await context.newPage()
    try {
      await page.goto('/en/area/sofia')
      await expect(page.locator('#area-map-alt')).toContainText('table of sensors')
      await expect(summary(page)).toHaveText(/All sensors in this area \(\d+\)/, { timeout: 20000 })
      await openTable(page)

      await table(page).locator('#area-sensors-filter').fill('102')
      await expect(table(page).locator('tbody tr')).toHaveCount(1)
      await expect(table(page).locator('[role="status"]')).toHaveText(/\S/)
      await table(page).locator('#area-sensors-filter').fill('')
      await expect(rowButton(page, 101)).toBeVisible()

      await rowButton(page, 101).focus()
      await page.keyboard.press('Enter')
      await expect(page.locator('#sensor-panel-title')).toHaveText(/101$/, { timeout: 10000 })
      await expect(page).toHaveURL(/#sensor=101$/)

      await page.keyboard.press('Escape')
      await expect(page).not.toHaveURL(/#sensor=/, { timeout: 10000 })
      await expect(rowButton(page, 101)).toBeFocused()
    } finally {
      await context.close()
    }
  })

  for (const theme of THEMES) {
    test(`the open table has no axe violations and fits the page (${label}, ${theme})`, async ({ browser }, testInfo) => {
      testInfo.setTimeout(90000)
      const context = await browser.newContext({ viewport, bypassCSP: true })
      const page = await context.newPage()
      try {
        await page.addInitScript((v) => localStorage.setItem('kanarche:theme', v), theme)
        await page.goto('/en/area/sofia')
        await openTable(page)
        expect(await axeViolations(page, 'details.area-sensors')).toEqual([])
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
        expect(overflow).toBeLessThanOrEqual(0)
      } finally {
        await context.close()
      }
    })
  }
}

test('only a refresh that moves the area into another band is spoken, and only once', async ({ browser }, testInfo) => {
  testInfo.setTimeout(90000)
  const context = await browser.newContext({ viewport: VIEWPORTS.desktop })
  const page = await context.newPage()
  try {
    let worse = false
    // Readings fifty times higher after the switch, so the median lands in another band.
    await page.route(/\/api\/v1\/area\/[^/]+\/sensors/, async (route) => {
      const res = await route.fetch()
      const body = await res.json()
      if (worse && Array.isArray(body?.sensors?.P2)) body.sensors.P2 = body.sensors.P2.map((v) => (typeof v === 'number' ? v * 50 + 100 : v))
      await route.fulfill({ response: res, json: body })
    })
    await page.goto('/en/area/sofia')
    await mapSettled(page)
    await expect(summary(page)).toHaveText(/All sensors in this area \([1-9]\d*\)/, { timeout: 20000 })
    const live = page.locator('.data-refresh [role="status"]')
    const trigger = page.locator('.data-refresh__btn--icon')
    await expect(live).toHaveText('')
    await expect(trigger).toHaveAttribute('title', /Data from /)

    const refreshNow = async () => {
      await trigger.click()
      await page.getByRole('button', { name: 'Refresh now' }).click()
    }
    // A value-only refresh is never spoken: the time moves in the title, the live region stays empty.
    await refreshNow()
    await expect(trigger).not.toHaveAttribute('aria-busy', 'true', { timeout: 15000 })
    await expect(trigger).toHaveAttribute('title', /Data from /)
    await expect(live).toHaveText('')

    worse = true
    await refreshNow()
    await expect(live).toHaveText(/^.+: air quality is now [a-z ]+, was [a-z ]+\.$/, { timeout: 15000 })
    expect(((await live.textContent()) ?? '').length).toBeLessThan(80)

    await refreshNow()
    await expect(trigger).not.toHaveAttribute('aria-busy', 'true', { timeout: 15000 })
    await expect(live).toHaveText('', { timeout: 15000 })
  } finally {
    await context.close()
  }
})

test('the home map keeps its canvas as a Tab stop and has no area sensor table', async ({ ctx }) => {
  const page = await ctx.newPage()
  try {
    await page.goto('/en/')
    const canvas = page.locator('[data-island="map"] .maplibregl-canvas')
    await expect(canvas).toHaveAttribute('tabindex', '0', { timeout: 20000 })
    await canvas.focus()
    await expect(canvas).toBeFocused()
    await expect(page.locator('[data-area-sensors]')).toHaveCount(0)
  } finally {
    await page.close()
  }
})
