import { test, expect, mapSettled } from './fixtures.js'

// The "Sensors in view" list and the band names in text. internal/e2e/e2e_test.go seeds
// sensors 101, 102 and 103 around sofia (lon 23.32, lat 42.69).
const CENTRE = [23.32, 42.69]
const PLACE = /^(At the map centre|\d+(\.\d)? (m|km) (north|south)?(east|west)? ?of the map centre)$/
const BAND = /\b(good|fair|moderate|poor|very poor|extremely poor)\b/

async function prepareMap(page, path) {
  // The first hexes response marks the opening camera as placed; jumping earlier gets overridden.
  const placed = page.waitForResponse(/\/api\/v1\/hexes/)
  await page.goto(path)
  await placed
  await mapSettled(page)
  // Past the point tier, where a cell names one sensor.
  await page.evaluate((c) => {
    document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
    document.querySelector('[data-island="map"]').__map.jumpTo({ center: c, zoom: 13 })
  }, CENTRE)
}

const nearby = (page) => page.locator('details.nearby')
const items = (page) => nearby(page).locator('button.nearby__item')
const summary = (page) => nearby(page).locator('summary')

async function openList(page) {
  await summary(page).focus()
  await page.keyboard.press('Enter')
  await expect(items(page).first()).toBeVisible({ timeout: 20000 })
}

test('Tab reaches the nearby list, which names value and band in text', async ({ ctx }) => {
  const page = await ctx.newPage()
  try {
    await prepareMap(page, '/en/')
    await expect(summary(page)).toContainText('Sensors in view')
    await openList(page)
    const first = items(page).first()
    await expect(first).toContainText('µg/m³')
    await expect(first.locator('.nearby__reading')).toHaveText(BAND)
    await expect(first.locator('.nearby__name')).toHaveText(/\S/)
    // Where it is, in words: the data holds no street or district, so it is the offset from the map centre.
    await expect(first.locator('.nearby__place')).toHaveText(PLACE)

    // Tab from the summary lands on the first item.
    await summary(page).focus()
    await page.keyboard.press('Tab')
    await expect(first).toBeFocused()
  } finally {
    await page.close()
  }
})

test('Enter on a list item opens that sensor, whose panel names the band', async ({ ctx }) => {
  const page = await ctx.newPage()
  try {
    await prepareMap(page, '/en/')
    await openList(page)
    const names = await items(page).locator('.nearby__name').allTextContents()
    const index = Math.max(0, names.findIndex((n) => /101\s*$/.test(n)))
    const id = names[index].match(/(\d+)\s*$/)?.[1]
    expect(id).toBeTruthy()
    await items(page).nth(index).focus()
    await page.keyboard.press('Enter')

    // The dock names the sensor in its heading, so the id must match the item that was opened.
    await expect(page.getByRole('heading', { level: 2, name: new RegExp(`${id}$`) })).toBeVisible({ timeout: 10000 })
    await expect(page).toHaveURL(new RegExp(`#sensor=${id}$`))
    await expect(page.locator('.gauge__name').first()).toHaveText(BAND)
  } finally {
    await page.close()
  }
})

test('keyboard focus on a list item shows the card and says nothing extra', async ({ ctx }) => {
  const page = await ctx.newPage()
  try {
    await prepareMap(page, '/en/')
    await openList(page)
    const tip = page.locator('.hex-tip')
    await summary(page).focus()
    await page.keyboard.press('Tab')
    await expect(tip).toBeVisible()
    await expect(tip).toHaveText(BAND)
    // The focused button already reads its own text, so no live region may repeat it.
    await expect(page.locator('[data-island="map"] .visually-hidden[role="status"]')).toHaveCount(0)

    await page.keyboard.press('Shift+Tab')
    await expect(tip).toBeHidden()
  } finally {
    await page.close()
  }
})
