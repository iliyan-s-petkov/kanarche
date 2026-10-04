import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { test, expect, mapSettled, userMove } from './fixtures.js'

// OpenProject #584: after exercising the map's real controls, every key the
// browser actually holds in localStorage must be in the published allow-list.
// A static source scan (web/src/__tests__/storage-allowlist.test.js) covers
// the code; this covers what a real page load, in a real browser, writes.
const allowListPath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '../../internal/web/static/storage-keys.json',
)
const allowList = JSON.parse(fs.readFileSync(allowListPath, 'utf8'))
const allowed = new Set([...(allowList.localStorage ?? []), ...(allowList.sessionStorage ?? [])])

test('localStorage after exercising the map holds only allow-listed keys', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.goto('/en/area/sofia')

  // Toggle a layer, which writes kanarche:map-layers; the page load itself may
  // have already read/written kanarche:theme and kanarche:legend-open.
  await page.locator('.map__layers .colmenu__btn').click()
  await page.locator('.map__layers .colmenu__panel input[type="checkbox"]').first().click()

  const keys = await page.evaluate(() => Object.keys(localStorage))
  const unexpected = keys.filter((k) => !allowed.has(k))
  expect(unexpected, `unlisted localStorage keys: ${unexpected.join(', ')}`).toEqual([])

  await page.close()
})

test('localStorage after moving the home map holds only allow-listed keys', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.goto('/en/')
  await mapSettled(page)
  await userMove(page)

  const keys = await page.evaluate(() => Object.keys(localStorage))
  const unexpected = keys.filter((k) => !allowed.has(k))
  expect(unexpected, `unlisted localStorage keys: ${unexpected.join(', ')}`).toEqual([])

  await page.close()
})

test('localStorage after the first-visit locate tip holds only allow-listed keys', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.goto('/en/')
  await expect.poll(() => page.evaluate(() => localStorage.getItem('kanarche:locate-hint-seen'))).toBe('1')

  const keys = await page.evaluate(() => Object.keys(localStorage))
  const unexpected = keys.filter((k) => !allowed.has(k))
  expect(unexpected, `unlisted localStorage keys: ${unexpected.join(', ')}`).toEqual([])

  await page.close()
})

test('localStorage after starring a sensor holds only allow-listed keys', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.goto('/en/#sensor=101')
  await page.locator('.panel-star:visible').click({ timeout: 20000 })

  const keys = await page.evaluate(() => Object.keys(localStorage))
  expect(keys).toContain('kanarche:favourite-sensor')
  const unexpected = keys.filter((k) => !allowed.has(k))
  expect(unexpected, `unlisted localStorage keys: ${unexpected.join(', ')}`).toEqual([])

  await page.close()
})

test('localStorage after folding the sensor panel holds only allow-listed keys', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/en/#sensor=101')
  await page.locator('.map-dock').getByRole('button', { name: 'Fold' }).click({ timeout: 20000 })

  const keys = await page.evaluate(() => Object.keys(localStorage))
  expect(keys).toContain('kanarche:panel-folded')
  const unexpected = keys.filter((k) => !allowed.has(k))
  expect(unexpected, `unlisted localStorage keys: ${unexpected.join(', ')}`).toEqual([])

  await page.close()
})

test('localStorage after resizing the sensor panel holds only allow-listed keys', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/en/#sensor=101')
  await page.locator('.map-dock__grip').focus({ timeout: 20000 })
  await page.keyboard.press('End')

  const keys = await page.evaluate(() => Object.keys(localStorage))
  expect(keys).toContain('kanarche:panel-height')
  const unexpected = keys.filter((k) => !allowed.has(k))
  expect(unexpected, `unlisted localStorage keys: ${unexpected.join(', ')}`).toEqual([])

  await page.close()
})
