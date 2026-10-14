import { mkdir } from 'node:fs/promises'
import { test, expect, mapSettled } from './fixtures.js'

// The number in each hexagon: 14px on a desktop map, 12px on a phone, and a
// cell wide enough to hold it. Needs a build made with VITE_E2E_MAP_HANDLE=1.
// KANARCHE_HEXLABEL_SHOTS=<dir> also writes the review screenshots.
const LAYER = 'kanarche-hex-labels'
const SOFIA = [23.32, 42.69]
const DESKTOP = { width: 1280, height: 800 }
const PHONE = { width: 390, height: 844 }
const SHOTS = process.env.KANARCHE_HEXLABEL_SHOTS

const openAt = async (page, size, zoom) => {
  await page.setViewportSize(size)
  await page.goto('/en/')
  await mapSettled(page)
  await page.evaluate(([center, zoom]) => {
    document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
    document.querySelector('[data-island="map"]').__map.jumpTo({ center, zoom })
  }, [SOFIA, zoom])
  await mapSettled(page)
  // Labels exist only once the grid has been fetched and the symbols placed.
  await expect.poll(() => rendered(page).then((r) => r.length), { timeout: 30_000 }).toBeGreaterThan(0)
  // Placement keeps thinning for a moment after the first label lands.
  await page.waitForTimeout(2000)
}

// Each rendered label with its cell's on-screen width, taken from the cell polygon.
const rendered = (page) => page.evaluate((layer) => {
  const map = document.querySelector('[data-island="map"]').__map
  const cells = map.queryRenderedFeatures({ layers: ['kanarche-hex-fill'] })
  const width = (f) => {
    const xs = f.geometry.coordinates[0].map((c) => map.project(c).x)
    return Math.max(...xs) - Math.min(...xs)
  }
  const cellWidths = cells.filter((f) => f.geometry.type === 'Polygon').map(width)
  const labels = map.queryRenderedFeatures({ layers: [layer] })
  return labels.map((f) => ({
    value: f.properties.value,
    size: map.getLayoutProperty(layer, 'text-size'),
    cell: cellWidths.length ? Math.min(...cellWidths) : 0,
  }))
}, LAYER)

const textSize = (page) => page.evaluate((layer) =>
  document.querySelector('[data-island="map"]').__map.getLayoutProperty(layer, 'text-size'), LAYER)

test.describe('hexagon number size', () => {
  test('is 14px on a 1280 wide map', async ({ ctx }) => {
    const page = await ctx.newPage()
    try {
      await openAt(page, DESKTOP, 10)
      expect(await textSize(page)).toBe(14)
    } finally {
      await page.close()
    }
  })

  test('is 12px on a 390 wide map', async ({ ctx }) => {
    const page = await ctx.newPage()
    try {
      await openAt(page, PHONE, 10)
      expect(await textSize(page)).toBe(12)
    } finally {
      await page.close()
    }
  })

  test('follows the viewport when it crosses the phone width', async ({ ctx }) => {
    const page = await ctx.newPage()
    try {
      await openAt(page, DESKTOP, 10)
      await page.setViewportSize(PHONE)
      await expect.poll(() => textSize(page)).toBe(12)
      await page.setViewportSize(DESKTOP)
      await expect.poll(() => textSize(page)).toBe(14)
    } finally {
      await page.close()
    }
  })

  test('probe: label and cell widths', async ({ ctx }) => {
    test.skip(!process.env.KANARCHE_HEXLABEL_PROBE, 'diagnostic')
    const page = await ctx.newPage()
    try {
      for (const size of [DESKTOP, PHONE]) {
        for (const zoom of [8, 9, 10, 11, 12, 13, 14]) {
          await openAt(page, size, zoom)
          const all = await rendered(page)
          console.log('PROBE', size.width, zoom, all.length, all[0]?.cell)
        }
      }
    } finally {
      await page.close()
    }
  })
})

test.describe('hexagon number screenshots', () => {
  test.skip(!SHOTS, 'set KANARCHE_HEXLABEL_SHOTS to write screenshots')

  for (const [name, size] of [['desk', DESKTOP], ['phone', PHONE]]) {
    for (const zoom of [7, 10, 13]) {
      test(`${name} z${zoom}`, async ({ ctx }) => {
        await mkdir(SHOTS, { recursive: true })
        const page = await ctx.newPage()
        try {
          await openAt(page, size, zoom)
          const count = (await rendered(page)).length
          console.log(`LABELS ${name} z${zoom} ${count}`)
          await page.locator('#map').screenshot({ path: `${SHOTS}/hexlabel-new-${name}-z${zoom}.png` })
        } finally {
          await page.close()
        }
      })
    }
  }

  for (const scheme of ['light', 'dark']) {
    test(`desk z10 ${scheme}`, async ({ browser }) => {
      const context = await browser.newContext({ colorScheme: scheme })
      try {
        const page = await context.newPage()
        await openAt(page, DESKTOP, 10)
        await page.locator('#map').screenshot({ path: `${SHOTS}/hexlabel-new-desk-z10-${scheme}.png` })
      } finally {
        await context.close()
      }
    })
  }
})
