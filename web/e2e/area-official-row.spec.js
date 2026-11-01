import { test, expect } from './fixtures.js'

// The sofia fixture has citizen P1 and P2 plus one official station reporting
// P1, O3 and CO (e2e_test.go). So P1 gets the official row under its citizen
// figure, P2 has no official reading and gets none, and O3 and CO fall back to
// the official median and are labelled with the station name instead.
const cases = [
  { path: '/en/area/sofia', name: 'ИАОС stations', row: /^ИАОС stations: 18\.0 µg\/m³$/ },
  { path: '/area/sofia', name: 'Станции на ИАОС', row: /^Станции на ИАОС: 18,0 µg\/m³$/ },
]

for (const c of cases) {
  test(`official row and fallback label: ${c.path}`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.goto(c.path)
    const cards = page.locator('.readouts .readout.card')
    const p1 = cards.filter({ has: page.locator('.readout__official', { hasText: c.row }) })
    await expect(p1).toHaveCount(1)
    await expect(p1.locator('.readout__official')).toBeVisible()
    await expect(p1.locator('.readout__value')).not.toContainText('18')

    // Fallback cards carry the bare station name and no value row.
    const labelled = page.locator('.readout__official', { hasText: new RegExp(`^${c.name}$`) })
    expect(await labelled.count(), 'O3 and CO fall back').toBeGreaterThanOrEqual(2)

    // P2 has no official reading: its card has no second line at all.
    const withRow = await cards.locator('.readout__official').count()
    const cardCount = await cards.count()
    expect(withRow, 'not every card carries an official line').toBeLessThan(cardCount)
    await page.close()
  })
}

test('official row does not overflow a phone', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/en/area/sofia')
  await expect(page.locator('.readout__official').first()).toBeVisible()
  const m = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, inner: window.innerWidth }))
  expect(m.scroll).toBeLessThanOrEqual(m.inner)
  await page.close()
})

// An area page with no official station anywhere in it shows neither element.
test('area without an official station has no official line', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.goto('/en/area/oblast-1')
  await expect(page.locator('.readout__official')).toHaveCount(0)
  await page.close()
})
