import { test, expect } from './fixtures.js'

// The (i) on the pollen and bathing-water legend sections opens a dialog with the sources copy.
const CENTRE = [23.32, 42.69]
const SHOTS = process.env.LEGEND_INFO_SHOTS || ''

async function prepareMap(page, path) {
  await page.goto(path)
  await page.waitForFunction(() => document.querySelector('[data-island="map"]')?.__map?.isStyleLoaded?.())
  await page.waitForTimeout(1000)
  await expect.poll(() => page.evaluate(() => document.querySelector('[data-island="map"]').__map.isMoving())).toBe(false)
  await page.evaluate((c) => {
    document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
    document.querySelector('[data-island="map"]').__map.jumpTo({ center: c, zoom: 8 })
  }, CENTRE)
}

async function toggleLayer(page, label) {
  await page.locator('.map__layers .colmenu__btn').click()
  await page.locator('.map__layers').getByLabel(label, { exact: true }).check()
  await page.keyboard.press('Escape')
}

const shapes = [
  {
    name: 'desktop', path: '/en/', context: { viewport: { width: 1440, height: 900 } },
    pollen: { layer: 'Pollen', label: 'What the pollen levels mean', title: 'Pollen forecast, grains per m³ of air', body: 'Three levels: low, moderate and high.', links: ['Source of the limits: EEA Climate-ADAPT', 'CAMS pollen forecast'] },
    sea: { layer: 'Bathing water', label: 'What the bathing water classes mean', title: 'Bathing water class, EEA', body: 'EU classification under Directive 2006/7/EC', links: ['EEA map of bathing water quality', 'EEA, bathing water'] },
  },
  {
    name: 'mobile', path: '/', context: { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true },
    pollen: { layer: 'Прашец', label: 'Какво означават нивата на прашеца', title: 'Прогноза за прашец, зърна в m³ въздух', body: 'Три нива: нисък, умерен и висок.', links: ['Източник на границите: EEA Climate-ADAPT', 'Прогноза за прашец на CAMS'] },
    sea: { layer: 'Води за къпане', label: 'Какво означават класовете на водите за къпане', title: 'Клас на водата за къпане, ЕАОС', body: 'класификацията на ЕС по Директива 2006/7/ЕО', links: ['Карта на ЕАОС за качеството на водите за къпане', 'ЕАОС, води за къпане'] },
  },
]

for (const shape of shapes) {
  for (const kind of ['pollen', 'sea']) {
    test(`${shape.name}: the ${kind} legend (i) opens its sources dialog`, async ({ browser }, testInfo) => {
      testInfo.setTimeout(60000)
      const context = await browser.newContext(shape.context)
      const page = await context.newPage()
      await prepareMap(page, shape.path)
      const c = shape[kind]
      await toggleLayer(page, c.layer)
      // The key may be folded to its pill, which hides the sections.
      const legend = page.locator('.scale--onmap')
      if ((await legend.getAttribute('open')) === null) await legend.locator('> .scale__toggle').click()

      const button = page.locator(`.scale--onmap .scale__${kind}`).getByRole('button', { name: c.label })
      await expect(button).toBeVisible()
      await button.focus()
      await page.keyboard.press('Enter')

      const dialog = page.locator('dialog.scaleinfo[open]')
      await expect(dialog).toBeVisible()
      await expect(dialog.locator('h2')).toHaveText(c.title)
      await expect(dialog).toContainText(c.body)
      // No percent-encoded copy, and nothing wider than the viewport.
      await expect(dialog).not.toContainText('%')
      const box = await dialog.boundingBox()
      expect(box.x).toBeGreaterThanOrEqual(0)
      expect(box.x + box.width).toBeLessThanOrEqual(shape.context.viewport.width)
      for (const text of c.links) {
        const link = dialog.getByRole('link', { name: text })
        await expect(link).toHaveAttribute('target', '_blank')
        await expect(link).toHaveAttribute('rel', /noopener/)
      }
      if (SHOTS) await page.screenshot({ path: `${SHOTS}/sources-popup-${kind}-${shape.name}.png` })

      await page.keyboard.press('Escape')
      await expect(dialog).toHaveCount(0)
      await expect(button).toBeFocused()
      await context.close()
    })
  }
}

// Each (i) sits at the right end of the heading row it explains, clear of the heading text.
const rowShapes = [
  { name: 'desktop', path: '/en/', context: { viewport: { width: 1280, height: 800 } }, sea: 'Bathing water' },
  { name: 'phone', path: '/', context: { viewport: { width: 393, height: 873 }, isMobile: true, hasTouch: true }, sea: 'Води за къпане' },
]

for (const shape of rowShapes) {
  test(`${shape.name}: each legend (i) is in its heading row, right-aligned, clear of the text`, async ({ browser }, testInfo) => {
    testInfo.setTimeout(60000)
    const context = await browser.newContext(shape.context)
    const page = await context.newPage()
    await prepareMap(page, shape.path)
    await toggleLayer(page, shape.sea)
    const legend = page.locator('.scale--onmap')
    if ((await legend.getAttribute('open')) === null) await legend.locator('> .scale__toggle').click()
    if (SHOTS) await page.screenshot({ path: `${SHOTS}/legend-rows-${shape.name}.png` })

    for (const head of ['> .scale__label', '.scale__sea-head']) {
      const row = legend.locator(head)
      const button = row.locator('button.scale__info')
      await expect(button).toBeVisible()
      const geo = await row.evaluate((r) => {
        const b = r.querySelector('button.scale__info').getBoundingClientRect()
        const range = document.createRange()
        range.selectNodeContents(r.firstChild)
        return { b: b.toJSON(), t: range.getBoundingClientRect().toJSON(), h: r.getBoundingClientRect().toJSON() }
      })
      const mid = geo.b.top + geo.b.height / 2
      expect(mid).toBeGreaterThanOrEqual(geo.h.top)
      expect(mid).toBeLessThanOrEqual(geo.h.bottom)
      expect(geo.h.right - geo.b.right).toBeLessThanOrEqual(2)
      expect(geo.t.right).toBeLessThanOrEqual(geo.b.left + 0.5)
      expect(geo.b.width).toBeLessThanOrEqual(26)

      await button.click()
      const dialog = page.locator('dialog.scaleinfo[open]')
      await expect(dialog).toBeVisible()
      await page.keyboard.press('Escape')
      await expect(dialog).toHaveCount(0)
    }
    await context.close()
  })
}
