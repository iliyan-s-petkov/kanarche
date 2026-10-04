import { test, expect } from './fixtures.js'

// OpenProject #697: the desktop bottom panel resizes from its top edge, and the right-hand
// map controls never run into each other or the panel, whatever its height.
const WIDE = { width: 1440, height: 900 }
const PANEL = '.map-dock'
const GRIP = '.map-dock__grip'
const REM = 16

async function openDocked(page, path = '/en/') {
  await page.goto(`${path}#sensor=101`)
  await expect(page.locator(`${PANEL} .gauges`)).toBeVisible({ timeout: 20000 })
  await page.evaluate(() => document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' }))
}

const box = async (locator) => {
  const b = await locator.boundingBox()
  expect(b).not.toBeNull()
  return b
}
const overlaps = (a, b) => a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height
const panelHeight = async (page) => Math.round((await box(page.locator(PANEL))).height)
const mapHeight = (page) => page.evaluate(() => document.querySelector('.map-dock').parentElement.clientHeight)
// The cap the handle reports: 8rem of map left above, or less when the left control stack needs more room.
const cap = async (page) => {
  const max = Number(await page.locator(GRIP).getAttribute('aria-valuemax'))
  expect(max).toBeGreaterThanOrEqual(14 * REM)
  expect(max).toBeLessThanOrEqual((await mapHeight(page)) - 8 * REM)
  return max
}
const covered = (page) => page.evaluate(() => {
  const shell = document.querySelector('.map-shell')
  const map = document.querySelector('[data-island="map"]').__map
  return { css: parseFloat(shell.style.getPropertyValue('--map-panel-h')), padding: map.getPadding().bottom }
})

async function drag(page, dy) {
  const g = await box(page.locator(GRIP))
  const x = g.x + g.width / 2
  const y = g.y + g.height / 2
  await page.mouse.move(x, y)
  await page.mouse.down()
  await page.mouse.move(x, y + dy, { steps: 8 })
  await page.mouse.up()
}

test('1440: dragging the handle resizes the panel live, moves the controls and padding, and survives a reload', async ({ browser }, testInfo) => {
  testInfo.setTimeout(90000)
  const context = await browser.newContext({ viewport: WIDE })
  const page = await context.newPage()
  await openDocked(page)
  const grip = page.locator(GRIP)
  await expect(grip).toBeVisible()
  expect(await grip.evaluate((el) => getComputedStyle(el).cursor)).toBe('ns-resize')
  const before = await panelHeight(page)
  const locateBefore = (await box(page.locator('.map-locate'))).y

  await drag(page, -150)
  await expect.poll(() => panelHeight(page)).toBe(before + 150)
  await expect.poll(async () => (await covered(page)).padding).toBe((await covered(page)).css)
  expect((await covered(page)).css).toBeGreaterThanOrEqual(before + 150)
  expect((await box(page.locator('.map-locate'))).y).toBeLessThan(locateBefore - 140)
  expect(await page.evaluate(() => localStorage.getItem('kanarche:panel-height'))).toBe(String(before + 150))
  // The chart follows the panel's height.
  const plot = await box(page.locator(`${PANEL} .chart-frame`))
  expect(plot.height).toBeGreaterThan(200)

  await page.reload()
  await expect(page.locator(`${PANEL} .gauges`)).toBeVisible({ timeout: 20000 })
  await expect.poll(() => panelHeight(page)).toBe(before + 150)

  // Clamped at both ends: 14rem, and 8rem of map left above.
  await drag(page, 2000)
  await expect.poll(() => panelHeight(page)).toBe(14 * REM)
  await drag(page, -2000)
  const max = await cap(page)
  await expect.poll(() => panelHeight(page)).toBe(max)

  // A double-click goes back to the default and forgets the choice.
  await grip.dblclick()
  await expect.poll(() => panelHeight(page)).toBe(before)
  expect(await page.evaluate(() => localStorage.getItem('kanarche:panel-height'))).toBeNull()
  await context.close()
})

test('1440: the handle drags by touch too', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext({ viewport: WIDE, hasTouch: true })
  const page = await context.newPage()
  await openDocked(page)
  const before = await panelHeight(page)
  const g = await box(page.locator(GRIP))
  const x = Math.round(g.x + g.width / 2)
  const y = Math.round(g.y + g.height / 2)
  const cdp = await context.newCDPSession(page)
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y }] })
  for (let i = 1; i <= 8; i++) await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x, y: y - i * 15 }] })
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
  await expect.poll(() => panelHeight(page)).toBe(before + 120)
  await context.close()
})

test('1440: the handle is a keyboard separator; folded it is gone and the height comes back on expand', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext({ viewport: WIDE })
  const page = await context.newPage()
  await openDocked(page)
  const grip = page.getByRole('separator', { name: 'Resize panel' })
  await expect(grip).toHaveAttribute('aria-orientation', 'horizontal')
  const max = await cap(page)
  await expect(grip).toHaveAttribute('aria-valuemin', String(14 * REM))
  const before = await panelHeight(page)
  await expect(grip).toHaveAttribute('aria-valuenow', String(before))

  await grip.focus()
  await page.keyboard.press('ArrowUp')
  await expect.poll(() => panelHeight(page)).toBe(before + 16)
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('ArrowDown')
  await expect.poll(() => panelHeight(page)).toBe(before - 16)
  await page.keyboard.press('Home')
  await expect.poll(() => panelHeight(page)).toBe(14 * REM)
  await expect(grip).toHaveAttribute('aria-valuenow', String(14 * REM))
  await page.keyboard.press('End')
  await expect.poll(() => panelHeight(page)).toBe(max)
  await expect(grip).toHaveAttribute('aria-valuenow', String(max))

  await page.locator(PANEL).getByRole('button', { name: 'Fold' }).click()
  await expect(page.locator(GRIP)).toBeHidden()
  await expect.poll(() => panelHeight(page)).toBeLessThan(200)
  await page.locator(PANEL).getByRole('button', { name: 'Expand' }).click()
  await expect(page.locator(GRIP)).toBeVisible()
  await expect.poll(() => panelHeight(page)).toBe(max)
  await context.close()
})

test('1440: the handle is named in Bulgarian on the Bulgarian page', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext({ viewport: WIDE })
  const page = await context.newPage()
  await openDocked(page, '/')
  await expect(page.getByRole('separator', { name: 'Преоразмери панела' })).toBeVisible()
  await context.close()
})

test('below 1024 there is no panel and no handle', async ({ browser }, testInfo) => {
  testInfo.setTimeout(60000)
  const context = await browser.newContext({ viewport: { width: 900, height: 700 } })
  const page = await context.newPage()
  await page.goto('/en/#sensor=101')
  await expect(page.locator('[data-island="panel"] .gauges')).toBeVisible({ timeout: 20000 })
  await expect(page.locator(GRIP)).toHaveCount(0)
  await context.close()
})

// Every visible right-hand control, the attribution's (i) and the panel: no two may overlap.
const RIGHT = ['.map__full', '.map-zoom', '.map-orient__btn', '.map-locate', '.maplibregl-ctrl-attrib-button']

async function assertClear(page, when) {
  await page.waitForTimeout(400)
  const boxes = []
  for (const sel of RIGHT) {
    const c = page.locator(sel).first()
    if (await c.count() === 0 || !(await c.isVisible())) continue
    boxes.push([sel, await box(c)])
  }
  expect(boxes.map(([s]) => s), `controls missing (${when})`).toEqual(expect.arrayContaining(['.map__full', '.map-zoom', '.map-locate']))
  const panel = await box(page.locator(PANEL))
  const map = await box(page.locator(PANEL).locator('xpath=..'))
  for (const [sel, b] of boxes) {
    expect(overlaps(panel, b), `${sel} sits under the panel (${when})`).toBe(false)
    expect(b.y >= map.y - 1 && b.y + b.height <= map.y + map.height + 1, `${sel} leaves the map (${when})`).toBe(true)
  }
  for (let i = 0; i < boxes.length; i++) {
    for (let j = i + 1; j < boxes.length; j++) {
      expect(overlaps(boxes[i][1], boxes[j][1]), `${boxes[i][0]} overlaps ${boxes[j][0]} (${when})`).toBe(false)
    }
  }
}

test('the right-hand map controls never overlap each other or the panel (1024, 1280, 1440; home and area; open, folded, max)', async ({ browser }, testInfo) => {
  testInfo.setTimeout(300000)
  for (const size of [{ width: 1024, height: 768 }, { width: 1280, height: 800 }, WIDE]) {
    const context = await browser.newContext({ viewport: size })
    const page = await context.newPage()
    for (const path of ['/en/', '/en/area/sofia']) {
      const at = `${size.width}x${size.height} ${path}`
      await openDocked(page, path)
      await assertClear(page, `${at} open`)
      await page.locator(GRIP).focus()
      await page.keyboard.press('End')
      await assertClear(page, `${at} max`)
      await page.keyboard.press('Home')
      await assertClear(page, `${at} min`)
      await page.locator(PANEL).getByRole('button', { name: 'Fold' }).click()
      await expect(page.locator(PANEL).getByRole('button', { name: 'Expand' })).toBeVisible()
      await assertClear(page, `${at} folded`)
      await page.evaluate(() => localStorage.clear())
    }
    await context.close()
  }
})

// The left stack (layers, legend, freshness card, wind note) at the tallest panel on a short map.
const LEFT = ['.map__layers', '.scale--onmap', '.map-freshness', '.map-wind-label', '.map-note']

test('at the tallest panel on a short map the left stack stays above the panel and on the map (1024x768, home and area)', async ({ browser }, testInfo) => {
  testInfo.setTimeout(120000)
  const context = await browser.newContext({ viewport: { width: 1024, height: 768 } })
  const page = await context.newPage()
  for (const path of ['/en/', '/en/area/sofia']) {
    await openDocked(page, path)
    await page.locator(GRIP).focus()
    await page.keyboard.press('End')
    await page.waitForTimeout(400)
    const panel = await box(page.locator(PANEL))
    const map = await box(page.locator(PANEL).locator('xpath=..'))
    // The key must have reached the cap, or the check below proves nothing.
    expect(Math.round(panel.height)).toBe(await cap(page))
    const seen = []
    for (const sel of LEFT) {
      const c = page.locator(sel).first()
      if (await c.count() === 0 || !(await c.isVisible())) continue
      const b = await box(c)
      seen.push(sel)
      expect(overlaps(panel, b), `${sel} sits under the panel (${path})`).toBe(false)
      expect(b.y >= map.y - 1 && b.y + b.height <= map.y + map.height + 1, `${sel} leaves the map (${path})`).toBe(true)
    }
    expect(seen.length, `no left-stack control found (${path})`).toBeGreaterThan(0)
    await page.evaluate(() => localStorage.clear())
  }
  await context.close()
})

// Pairwise: the stack must not collide with itself, whichever way the legend is folded (the panel's max rides it up).
const STACK = ['.map__layers', '.map-controls__metric', '.map-note', '.scale--onmap', '.map-freshness', '.map-wind-label']
const NAMES = { en: '/en/', bg: '/' }

test('at the tallest panel the left stack does not overlap itself (1024x768 and 1440x900; BG and EN; legend folded and open)', async ({ browser }, testInfo) => {
  testInfo.setTimeout(300000)
  for (const size of [{ width: 1024, height: 768 }, WIDE]) {
    for (const [lang, path] of Object.entries(NAMES)) {
      for (const open of [false, true]) {
        const at = `${size.width}x${size.height} ${lang} legend ${open ? 'open' : 'folded'}`
        const context = await browser.newContext({ viewport: size })
        const page = await context.newPage()
        await page.addInitScript((o) => localStorage.setItem('kanarche:legend-open', o ? 'true' : 'false'), open)
        await openDocked(page, path)
        const legend = page.locator('.scale--onmap')
        if (await legend.evaluate((d) => d.open) !== open) await legend.locator('summary').first().click()
        await page.locator(GRIP).focus()
        await page.keyboard.press('End')
        await page.waitForTimeout(500)
        const panel = await box(page.locator(PANEL))
        const map = await box(page.locator(PANEL).locator('xpath=..'))
        const grip = page.getByRole('separator')
        // aria-valuemax is the cap the key reached.
        await expect(grip).toHaveAttribute('aria-valuemax', String(Math.round(panel.height)))
        const seen = []
        for (const sel of STACK) {
          const c = page.locator(sel).first()
          if (await c.count() === 0 || !(await c.isVisible())) continue
          const b = await box(c)
          expect(overlaps(panel, b), `${sel} under the panel (${at})`).toBe(false)
          expect(b.y >= map.y - 1, `${sel} above the map (${at})`).toBe(true)
          seen.push([sel, b])
        }
        expect(seen.length, `stack not found (${at})`).toBeGreaterThan(2)
        for (let i = 0; i < seen.length; i++) {
          for (let j = i + 1; j < seen.length; j++) {
            expect(overlaps(seen[i][1], seen[j][1]), `${seen[i][0]} overlaps ${seen[j][0]} (${at})`).toBe(false)
          }
        }
        await context.close()
      }
    }
  }
})
