import { createRequire } from 'node:module'
import { test as base, expect, mapSettled } from './fixtures.js'

// A11Y-15 (prefers-contrast), A11Y-16 (24px targets, scroll cue under the sheet),
// desktop Esc focus return, and the D5 ruling: the map canvas stays a Tab stop
// because MapLibre's keyboard handler pans and zooms from it.
const axeSource = createRequire(import.meta.url).resolve('axe-core/axe.min.js')
const CENTRE = [23.32, 42.69]

const test = base.extend({
  deskCtx: [async ({ browser }, use) => {
    const context = await browser.newContext({ bypassCSP: true, viewport: { width: 1280, height: 800 } })
    await use(context)
    await context.close()
  }, { scope: 'worker' }],
  phoneCtx: [async ({ browser }, use) => {
    const context = await browser.newContext({
      bypassCSP: true, isMobile: true, hasTouch: true, deviceScaleFactor: 2, viewport: { width: 390, height: 844 },
    })
    await use(context)
    await context.close()
  }, { scope: 'worker' }],
})

test.afterEach(async ({ deskCtx, phoneCtx }) => {
  for (const c of [deskCtx, phoneCtx]) {
    for (const p of c.pages()) if (!p.isClosed()) await p.close().catch(() => {})
  }
})

// Visible buttons smaller than 24x24 CSS px, by class and size.
const smallButtons = (page) => page.evaluate(() => [...document.querySelectorAll('button, [role="button"], summary')]
  .filter((b) => {
    const r = b.getBoundingClientRect()
    const s = getComputedStyle(b)
    return r.width > 0 && r.height > 0 && s.visibility !== 'hidden' && !b.closest('[hidden]') && !b.closest('.visually-hidden')
  })
  .map((b) => ({ cls: `${b.tagName}.${b.className} ${(b.textContent || '').trim().slice(0, 30)}`, w: Math.round(b.getBoundingClientRect().width), h: Math.round(b.getBoundingClientRect().height) }))
  .filter((b) => b.w < 24 || b.h < 24))

async function openSensor(page, path) {
  await page.goto(path)
  await mapSettled(page)
  await expect(page.locator('.sensor-panel')).toBeVisible({ timeout: 15000 })
  await expect(page.locator('.sensor-panel .chart-frame')).toBeVisible({ timeout: 15000 })
}

test('axe target-size: phone 390x844 dark with the sensor sheet open', async ({ phoneCtx }) => {
  const page = await phoneCtx.newPage()
  await page.addInitScript(() => localStorage.setItem('kanarche:theme', 'dark'))
  await page.goto('/en/')
  await mapSettled(page)
  const cue = page.locator('.scroll-cue')
  await expect(cue).toBeVisible()
  await page.goto('/en/#sensor=101')
  await page.reload()
  await mapSettled(page)
  await expect(page.locator('.sensor-panel')).toBeVisible({ timeout: 15000 })
  // What a tap does: the card scrolls up under the masthead, which then covers the pull tab.
  await page.evaluate(() => document.querySelector('.sensor-panel').scrollIntoView({ block: 'start', behavior: 'instant' }))
  await expect(cue).toBeHidden()
  await page.addScriptTag({ path: axeSource })
  const found = await page.evaluate(async () => {
    const r = await window.axe.run(document, { runOnly: { type: 'rule', values: ['target-size'] } })
    return r.violations.flatMap((v) => v.nodes.map((n) => n.target.join(' ')))
  })
  expect(found).toEqual([])
})

for (const [name, ctxName] of [['desktop', 'deskCtx'], ['phone', 'phoneCtx']]) {
  test(`every visible button is at least 24x24 on ${name} with a sensor open`, async ({ deskCtx, phoneCtx }) => {
    const page = await (ctxName === 'phoneCtx' ? phoneCtx : deskCtx).newPage()
    await openSensor(page, '/en/area/sofia#sensor=101')
    expect(await smallButtons(page)).toEqual([])
  })
}

test('desktop: Escape closes the sensor panel and focus does not fall to body', async ({ deskCtx }) => {
  const page = await deskCtx.newPage()
  const placed = page.waitForResponse(/\/api\/v1\/hexes/)
  await page.goto('/en/')
  await placed
  await mapSettled(page)
  await page.evaluate((c) => {
    document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
    document.querySelector('[data-island="map"]').__map.jumpTo({ center: c, zoom: 13 })
  }, CENTRE)
  await page.locator('details.nearby summary').focus()
  await page.keyboard.press('Enter')
  const first = page.locator('details.nearby button.nearby__item').first()
  await expect(first).toBeVisible({ timeout: 20000 })
  await first.focus()
  await page.keyboard.press('Enter')
  await expect(page.locator('.map-dock')).toBeVisible({ timeout: 10000 })
  // The list re-renders when the camera moves, so the opener may be gone by the time the panel closes.
  await page.evaluate(() => document.querySelector('details.nearby button.nearby__item')?.remove())
  await page.locator('.map-dock').focus()
  await page.keyboard.press('Escape')
  await expect(page.locator('.map-dock')).toHaveCount(0)
  const where = await page.evaluate(() => {
    const a = document.activeElement
    return a === document.body ? 'body' : (a.matches('.maplibregl-canvas') ? 'canvas' : String(a.className))
  })
  expect(where).toBe('canvas')
})

test('prefers-contrast: more darkens muted text and the focus ring', async ({ deskCtx }) => {
  const page = await deskCtx.newPage()
  await page.goto('/en/about')
  const read = () => page.evaluate(() => {
    const probe = document.createElement('span')
    probe.style.color = 'var(--muted)'
    document.body.appendChild(probe)
    const muted = getComputedStyle(probe).color
    probe.remove()
    return { muted, ring: getComputedStyle(document.documentElement).getPropertyValue('--focus-ring').trim() }
  })
  const before = await read()
  await page.emulateMedia({ contrast: 'more' })
  const after = await read()
  expect(after.muted).not.toBe(before.muted)
  expect(after.ring).not.toBe(before.ring)
})

test('the map canvas is a Tab stop because the arrow keys pan it', async ({ deskCtx }) => {
  const page = await deskCtx.newPage()
  await page.goto('/en/')
  await mapSettled(page)
  const centre = () => page.evaluate(() => Object.values(document.querySelector('[data-island="map"]').__map.getCenter()))
  const before = await centre()
  await page.locator('.maplibregl-canvas').focus()
  for (let i = 0; i < 6; i++) await page.keyboard.press('ArrowRight')
  await expect.poll(async () => (await centre())[0]).not.toBe(before[0])
})
