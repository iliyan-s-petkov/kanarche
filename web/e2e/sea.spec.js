import { createRequire } from 'node:module'
import { test, expect } from './fixtures.js'

const axeSource = createRequire(import.meta.url).resolve('axe-core/axe.min.js')

// Violations inside one subtree. bypassCSP on the context lets the inline script through.
async function axeViolations(page, selector) {
  await page.addScriptTag({ path: axeSource })
  return page.evaluate(async (sel) => {
    const r = await window.axe.run(sel)
    return r.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)
  }, selector)
}

// The bathing-water layer: off by default, a square per site, a card with the class and samples.
// The site is seeded in internal/e2e/e2e_test.go at 23.45, 42.75.
const SITE = [23.45, 42.75]
// The long-history site: nineteen seasons and a stale RZI sample.
const LONG_SITE = [23.53, 42.75]
// A site with an RZI sample that carries a remark.
const REMARK_SITE = [23.37, 42.75]

async function prepareMap(page, path) {
  await page.goto(path)
  await page.waitForFunction(() => document.querySelector('[data-island="map"]')?.__map?.isStyleLoaded?.())
  await page.waitForTimeout(1000)
  await expect.poll(() => page.evaluate(() => document.querySelector('[data-island="map"]').__map.isMoving())).toBe(false)
  await page.evaluate((c) => {
    document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
    document.querySelector('[data-island="map"]').__map.jumpTo({ center: c, zoom: 10 })
  }, SITE)
}

async function openCard(page, path, label) {
  await prepareMap(page, path)
  await toggleSea(page, label, true)
  let pt = null
  await expect.poll(async () => { pt = await sitePoint(page); return pt }, { timeout: 20000 }).not.toBeNull()
  await page.mouse.click(pt.x, pt.y)
  return pt
}

async function toggleSea(page, label, on) {
  await page.locator('.map__layers .colmenu__btn').click()
  const box = page.locator('.map__layers').getByLabel(label, { exact: true })
  if (on) await box.check()
  else await box.uncheck()
  await page.keyboard.press('Escape')
}

// Waits for the square to render, then returns its page coordinates.
const sitePoint = (page, c = SITE) => page.evaluate((c) => {
  const map = document.querySelector('[data-island="map"]').__map
  if (!map.queryRenderedFeatures({ layers: ['sea-sites'] }).length) return null
  const box = map.getCanvas().getBoundingClientRect()
  const p = map.project(c)
  return { x: box.left + p.x, y: box.top + p.y }
}, c)

test('the bathing layer is off by default, and a site opens its compact card', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  const page = await context.newPage()
  await prepareMap(page, '/en/')

  const legendKey = page.locator('.scale--onmap .scale__sea')
  await expect(legendKey).toBeHidden()
  await toggleSea(page, 'Bathing water', true)
  await expect(legendKey).toBeAttached()
  await expect(legendKey).not.toHaveAttribute('hidden')

  let pt = null
  await expect.poll(async () => { pt = await sitePoint(page); return pt }, { timeout: 20000 }).not.toBeNull()
  await page.mouse.click(pt.x, pt.y)

  const card = page.locator('.map-sea .sea-panel')
  await expect(card).toBeVisible()
  // The card contains the dialog, so selectors skip its subtree.
  await expect(card.locator('.sea-panel__title')).toHaveText('Plazh Test')
  await expect(card.locator('.sea-panel__badge strong')).toHaveText('Excellent')
  await expect(card.locator('.sea-panel__badge text')).toHaveText('E')
  const chips = card.locator('.sea-panel__chips .sea-chip')
  // Chips come from the newest season only, so the 2023 sample is not on the card.
  await expect(chips).toHaveCount(2)
  await expect(chips.nth(0).locator('.sea-chip__mark')).toHaveText('▲')
  await expect(chips.nth(1).locator('.sea-chip__mark')).toHaveText('▲▲')
  await expect(chips.nth(1)).toContainText('pre-season')
  await expect(card.locator('.sea-seasons--strip .sea-cell')).toHaveCount(10)
  await expect(card.locator('.sea-seasons--strip .sea-cell').last()).toHaveAttribute('aria-label', '2024 Excellent')
  await expect(card.locator('.sea-key__item:not(dialog *)')).toHaveCount(5)
  await expect(page.locator('.map-dock')).toBeHidden()

  await page.keyboard.press('Escape')
  await expect(page.locator('.map-sea')).toBeHidden()

  // Switching the layer off also takes the card with it.
  await page.mouse.click(pt.x, pt.y)
  await expect(card).toBeVisible()
  await toggleSea(page, 'Bathing water', false)
  await expect(page.locator('.map-sea')).toBeHidden()
  await context.close()
})

test('390: the long-history card fits the sheet with its stale note, one strip row, Details on screen', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true })
  const page = await context.newPage()
  await prepareMap(page, '/en/')
  await toggleSea(page, 'Bathing water', true)
  let pt = null
  await expect.poll(async () => { pt = await sitePoint(page, LONG_SITE); return pt }, { timeout: 20000 }).not.toBeNull()
  await page.mouse.click(pt.x, pt.y)
  const sheet = page.locator('.map-sea')
  await expect(sheet.locator('.sea-panel__title')).toHaveText('Long History')
  await expect(sheet.locator('.sea-seasons--strip .sea-cell')).toHaveCount(10)
  await expect(sheet.locator('.sea-panel__fresh')).toBeVisible()
  const rows = await sheet.locator('.sea-seasons--strip .sea-cell').evaluateAll((els) => new Set(els.map((e) => Math.round(e.getBoundingClientRect().top))).size)
  expect(rows).toBe(1)
  const fit = await sheet.evaluate((el) => ({ scroll: el.scrollHeight, client: el.clientHeight }))
  expect(fit.scroll, JSON.stringify(fit)).toBeLessThanOrEqual(fit.client)
  const dbtn = await sheet.locator('.sea-panel__details').boundingBox()
  expect(dbtn.y + dbtn.height).toBeLessThanOrEqual(844)
  await context.close()
})

test('390: the card is a bottom sheet in Bulgarian', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true })
  const page = await context.newPage()
  await prepareMap(page, '/')
  await toggleSea(page, 'Води за къпане', true)
  let pt = null
  await expect.poll(async () => { pt = await sitePoint(page); return pt }, { timeout: 20000 }).not.toBeNull()
  await page.mouse.click(pt.x, pt.y)
  const sheet = page.locator('.map-sea')
  await expect(sheet.locator('.sea-panel__badge strong')).toHaveText('Отлично')
  const box = await sheet.boundingBox()
  const frame = await page.locator('[data-island="map"]').boundingBox()
  expect(Math.round(box.width)).toBe(Math.round(frame.width))
  expect(Math.abs(box.y + box.height - (frame.y + frame.height))).toBeLessThanOrEqual(1)
  // The strip never scrolls sideways.
  expect(await sheet.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  // The card fits the sheet without vertical scrolling, and the Details button is on screen.
  const fit = await sheet.evaluate((el) => ({ scroll: el.scrollHeight, client: el.clientHeight }))
  expect(fit.scroll, JSON.stringify(fit)).toBeLessThanOrEqual(fit.client)
  const dbtn = await sheet.locator('.sea-panel__details').boundingBox()
  expect(dbtn.y + dbtn.height).toBeLessThanOrEqual(844)
  const cell = sheet.locator('.sea-cell').first()
  expect((await cell.boundingBox()).width).toBeGreaterThanOrEqual(24)
  expect(await cell.getAttribute('tabindex')).toBe('0')
  await sheet.locator('.sea-panel__details').click()
  const dialog = page.locator('dialog.sea-details')
  await expect(dialog).toBeVisible()
  const dbox = await dialog.boundingBox()
  expect(Math.round(dbox.width)).toBe(390)
  expect(Math.round(dbox.height)).toBe(844)
  expect(await dialog.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  await expect(dialog.locator('.sea-details__back')).toBeVisible()
  await expect(dialog.locator('.sea-details__close')).toBeHidden()
  for (const id of ['seasons', 'about']) {
    await dialog.locator(`#sea-tab-${id}`).click()
    expect(await dialog.evaluate((el) => el.scrollWidth <= el.clientWidth), id).toBe(true)
  }
  await dialog.locator('.sea-details__back').click()
  await expect(dialog).toBeHidden()
  await expect(sheet.locator('.sea-panel__details')).toBeFocused()
  await context.close()
})

test('details: tabs by keyboard, metric switch, Escape returns focus to the Details button', async ({ browser }, testInfo) => {
  testInfo.setTimeout(90000)
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  const page = await context.newPage()
  await openCard(page, '/en/', 'Bathing water')
  const button = page.locator('.map-sea .sea-panel__details')
  await button.click()
  const dialog = page.locator('dialog.sea-details')
  await expect(dialog).toBeVisible()

  const samples = dialog.locator('#sea-tab-samples')
  await expect(samples).toHaveAttribute('aria-selected', 'true')
  await samples.focus()
  await page.keyboard.press('ArrowRight')
  await expect(dialog.locator('#sea-tab-seasons')).toHaveAttribute('aria-selected', 'true')
  await expect(dialog.locator('#sea-tab-seasons')).toBeFocused()
  await expect(dialog.locator('#sea-panel-seasons .sea-seasons--grid .sea-cell')).toHaveCount(11)
  await page.keyboard.press('End')
  await expect(dialog.locator('#sea-tab-about')).toHaveAttribute('aria-selected', 'true')
  await page.keyboard.press('Home')
  await expect(samples).toHaveAttribute('aria-selected', 'true')

  const metric = dialog.locator('.sea-seg__btn')
  await expect(metric.nth(0)).toHaveAttribute('aria-pressed', 'true')
  const chart = dialog.locator('svg.sea-chart__svg')
  await expect(chart).toHaveAttribute('aria-label', /2024/)
  const before = await chart.getAttribute('aria-label')
  await metric.nth(1).click()
  await expect(metric.nth(1)).toHaveAttribute('aria-pressed', 'true')
  await expect(chart).not.toHaveAttribute('aria-label', before)
  await expect(dialog.locator('.sea-details__select option')).toHaveCount(2)

  await dialog.locator('.sea-chart__toggle').click()
  await expect(dialog.locator('table.sea-chart__table tbody tr')).toHaveCount(2)
  await expect(chart).toHaveCount(0)

  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()
  await expect(button).toBeFocused()
  await expect(page.locator('.map-sea .sea-panel')).toBeVisible()
  await context.close()
})

test('axe finds no violations in the card or the details dialog', async ({ browser }, testInfo) => {
  testInfo.setTimeout(90000)
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, bypassCSP: true })
  const page = await context.newPage()
  await openCard(page, '/en/', 'Bathing water')
  await expect(page.locator('.map-sea .sea-panel .sea-chip').first()).toBeVisible()
  expect(await axeViolations(page, '.map-sea')).toEqual([])
  await page.locator('.map-sea .sea-panel__details').click()
  const dialog = page.locator('dialog.sea-details')
  await expect(dialog).toBeVisible()
  for (const id of ['samples', 'seasons', 'about']) {
    await dialog.locator(`#sea-tab-${id}`).click()
    expect(await axeViolations(page, '.sea-details'), `tab ${id}`).toEqual([])
  }
  await context.close()
})

test('an RZI remark shows under the samples with its label and credit', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true })
  const page = await context.newPage()
  await prepareMap(page, '/en/')
  await toggleSea(page, 'Bathing water', true)
  let pt = null
  await expect.poll(async () => { pt = await sitePoint(page, REMARK_SITE); return pt }, { timeout: 20000 }).not.toBeNull()
  await page.mouse.click(pt.x, pt.y)
  const sheet = page.locator('.map-sea')
  await expect(sheet.locator('.sea-panel__title')).toHaveText('Remark Site')
  await expect(sheet.locator('.sea-remark__label:not(dialog *)')).toContainText('RHI remark')
  await expect(sheet.locator('.sea-remark__text:not(dialog *)')).toHaveText('Пробата е взета след пороен дъжд.')
  await expect(sheet.locator('.sea-remark__credit:not(dialog *)')).toHaveText('Source: RHI Varna')
  await context.close()
})

// The remark site has three remarked samples. The card shows the newest and never scrolls,
// and the Details samples tab lists all three. E2E_SHOTS_DIR saves screenshots of both.
for (const theme of ['light', 'dark']) {
  test(`390: the card shows one remark and does not scroll, Details lists all three (bg, ${theme})`, async ({ browser }, testInfo) => {
    testInfo.setTimeout(60000)
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, colorScheme: theme })
    const page = await context.newPage()
    await prepareMap(page, '/')
    await page.evaluate((th) => document.documentElement.setAttribute('data-theme', th), theme)
    await toggleSea(page, 'Води за къпане', true)
    let pt = null
    await expect.poll(async () => { pt = await sitePoint(page, REMARK_SITE); return pt }, { timeout: 20000 }).not.toBeNull()
    await page.mouse.click(pt.x, pt.y)
    const sheet = page.locator('.map-sea')
    await expect(sheet.locator('.sea-panel__title')).toHaveText('Бележка')
    const blocks = sheet.locator('.sea-remark:not(dialog *)')
    await expect(blocks).toHaveCount(1)
    await expect(blocks.locator('.sea-remark__text')).toHaveText('Пробата е взета след пороен дъжд.')
    const fit = await sheet.evaluate((el) => ({ scroll: el.scrollHeight, client: el.clientHeight }))
    expect(fit.scroll, JSON.stringify(fit)).toBeLessThanOrEqual(fit.client + 1)
    const dir = process.env.E2E_SHOTS_DIR
    if (dir) await page.screenshot({ path: `${dir}/card-390-bg-${theme}.png` })

    await sheet.locator('.sea-panel__details').click()
    const dialog = page.locator('dialog.sea-details')
    await expect(dialog).toBeVisible()
    await expect(dialog.locator('#sea-panel-samples .sea-remark')).toHaveCount(3)
    await expect(dialog.locator('#sea-panel-samples .sea-remark__text')).toHaveText([
      'Пробата е взета след пороен дъжд.',
      'Втора бележка: водата е мътна след вълнение.',
      'Трета бележка: пробата е взета при силен вятър.',
    ])
    await dialog.locator('#sea-panel-samples .sea-remarks').scrollIntoViewIfNeeded()
    if (dir) await page.screenshot({ path: `${dir}/details-390-bg-${theme}.png` })
    await context.close()
  })
}
