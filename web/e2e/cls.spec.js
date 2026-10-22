import { test, expect } from './fixtures.js'

// OpenProject #609: production CLS on /area/varna measured 0.185, sourced to
// the switcher and sensorbar islands hydrating from zero height and pushing
// the fixed-height map down (see app.css's min-block-size reservations on
// [data-island="switcher"] / [data-island="sensorbar"]). Sofia is this
// suite's equivalent covered area; 393x873 matches the Lighthouse mobile
// viewport the baseline was measured at.
test('CLS on /area/sofia stays under 0.1 at the Lighthouse mobile viewport', async ({ ctx }) => {
  const page = await ctx.newPage()
  // Registered before navigation so entries from the very first paint are
  // captured, not just ones after the script tag most viewport waits for.
  await page.addInitScript(() => {
    window.__cls = 0
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (!entry.hadRecentInput) window.__cls += entry.value
      }
    }).observe({ type: 'layout-shift', buffered: true })
  })
  await page.setViewportSize({ width: 393, height: 873 })
  await page.goto('/area/sofia')
  // Past hydration: islands mount asynchronously and this is what the shift
  // reservation is meant to have already priced in by then.
  await page.waitForTimeout(3000)
  const cls = await page.evaluate(() => window.__cls)
  expect(cls).toBeLessThan(0.1)
  await page.close()
})

// OpenProject #609: production CLS on /areas measured 0.315, mostly on
// #below-map — the table island's controls and pager hydrate with no
// reserved height. Same reservation pattern as the switcher/sensorbar above.
test('CLS on /areas stays under 0.1 at the Lighthouse mobile viewport', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.addInitScript(() => {
    window.__cls = 0
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (!entry.hadRecentInput) window.__cls += entry.value
      }
    }).observe({ type: 'layout-shift', buffered: true })
  })
  await page.setViewportSize({ width: 393, height: 873 })
  await page.goto('/areas')
  await page.waitForTimeout(3000)
  const cls = await page.evaluate(() => window.__cls)
  expect(cls).toBeLessThan(0.1)
  await page.close()
})

// OpenProject #609: the layout-shift metric above stays quiet even with the
// reservations deleted, because this fixture's content isn't big enough to
// cross the 0.1 CLS threshold. This test doesn't rely on that threshold: it
// reads the live CSS reservation itself (getComputedStyle, so it reflects
// whatever's in app.css right now) and checks it against what actually
// renders — must not undershoot (no room to shift into) or overshoot by
// more than 8px (a permanent blank gap instead).
const widths = [393, 768, 1280]

for (const width of widths) {
  test(`/areas table island reservation matches rendered height at ${width}px`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.setViewportSize({ width, height: 900 })
    await page.goto('/areas')
    await page.waitForTimeout(1500)
    const actual = await page.evaluate(() => {
      // .pager-slot no longer exists post-mount (table.js replaces it), so
      // its reserved size is read off a detached probe carrying the same
      // class — the CSS selector, not the live node, is what's under test.
      const probe = document.createElement('div')
      probe.className = 'pager-slot'
      probe.style.position = 'absolute'
      probe.style.visibility = 'hidden'
      document.body.appendChild(probe)
      const reservedPager = parseFloat(getComputedStyle(probe).minBlockSize)
      probe.remove()
      const island = document.querySelector('[data-island="table"]')
      return {
        reservedTable: island ? parseFloat(getComputedStyle(island).minBlockSize) : null,
        reservedPager,
        table: document.querySelector('.table-controls')?.getBoundingClientRect().height,
        pager: document.querySelector('.pager')?.getBoundingClientRect().height,
      }
    })
    expect(actual.table, 'table controls height').toBeLessThanOrEqual(actual.reservedTable + 1)
    expect(actual.reservedTable - actual.table, 'table over-reservation').toBeLessThan(8)
    expect(actual.pager, 'pager height').toBeLessThanOrEqual(actual.reservedPager + 1)
    expect(actual.reservedPager - actual.pager, 'pager over-reservation').toBeLessThan(8)
    await page.close()
  })
}

// On touch phones .frame is a grid (hover: none in app.css), a grid item does
// not collapse its children's margins, so the sensorbar island is 36px taller
// there than on desktop. Its reservation was the desktop height and the map
// and summary below it shifted on hydrate (prod /area/sofia CLS 0.21). A
// touch context is needed to reproduce it; 390x844 is the Lighthouse mobile size.
const phone = { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 }

test('area page layout shift stays under 0.05 at 390x844', async ({ browser }) => {
  const context = await browser.newContext(phone)
  const page = await context.newPage()
  await page.addInitScript(() => {
    window.__cls = 0
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (!entry.hadRecentInput) window.__cls += entry.value
      }
    }).observe({ type: 'layout-shift', buffered: true })
  })
  await page.goto('/area/sofia')
  await page.waitForTimeout(3000)
  const cls = await page.evaluate(() => window.__cls)
  expect(cls).toBeLessThan(0.05)
  await context.close()
})

for (const width of [360, 390, 412]) {
  test(`area sensorbar reservation covers its rendered height at ${width}px touch`, async ({ browser }) => {
    const context = await browser.newContext({ ...phone, viewport: { width, height: 844 } })
    const page = await context.newPage()
    await page.goto('/area/sofia')
    await expect(page.locator('[data-island="sensorbar"] .sensor-bar')).toBeVisible()
    const m = await page.evaluate(() => {
      const bar = document.querySelector('[data-island="sensorbar"]')
      return {
        reserved: parseFloat(getComputedStyle(bar).minBlockSize),
        actual: bar.getBoundingClientRect().height,
      }
    })
    // Rendered above the reservation means the island grows on hydrate.
    expect(m.actual, 'sensorbar height').toBeLessThanOrEqual(m.reserved + 1)
    expect(m.reserved - m.actual, 'sensorbar over-reservation').toBeLessThan(8)
    await context.close()
  })
}
