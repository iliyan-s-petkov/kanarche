import { test, expect, mapSettled } from './fixtures.js'

// Sensor 104 (e2e_test.go's seedFixtures) has a good P1 and a 'stuck' P2, so it
// is faulty on the PM2.5 layer only. EN routes; sofia opens at the sensor tier.
const sensorIds = (page) => page.evaluate(async () => {
  const map = document.querySelector('[data-island="map"]').__map
  const feats = (await map.getSource('airbg-data').getData()).features ?? []
  return feats.filter((f) => f.properties.id != null)
    .map((f) => ({ id: f.properties.id, faulty: f.properties.faulty }))
})

// The layers button toggles, and the menu may be open from an earlier test.
const openLayers = async (page) => {
  const box = page.getByRole('checkbox', { name: 'Faulty stations', exact: true })
  if (!(await box.isVisible())) await page.getByRole('button', { name: 'Layers' }).click()
}

test.describe.serial('the faulty stations toggle', () => {
  let page

  test.beforeAll(async ({ ctx }) => {
    page = await ctx.newPage()
    await page.goto('/en/area/sofia')
    await mapSettled(page)
    await expect.poll(async () => (await sensorIds(page)).length).toBeGreaterThan(0)
  })

  test.afterAll(async () => { await page.close() })

  test('faulty stations are hidden by default, and not counted', async () => {
    expect((await sensorIds(page)).map((s) => s.id)).not.toContain(104)
    await expect(page.locator('.sensor-bar + .meta')).toHaveText('Showing 3 of 4 sensors, 1 with no recent readings')
    await page.getByRole('button', { name: 'Layers' }).click()
    await expect(page.getByRole('checkbox', { name: 'Faulty stations', exact: true })).not.toBeChecked()
  })

  test('the toggle draws them as a hollow ring and counts them as silent', async () => {
    await page.getByRole('checkbox', { name: 'Faulty stations', exact: true }).check()
    await expect.poll(async () => (await sensorIds(page)).find((s) => s.id === 104)?.faulty).toBe(true)
    await expect(page.locator('.sensor-bar + .meta')).toHaveText('Showing 4 of 5 sensors, 2 with no recent readings')
    const ring = await page.evaluate(() => {
      const map = document.querySelector('[data-island="map"]').__map
      return map.getLayer('airbg-markers-faulty').type
    })
    expect(ring).toBe('circle')
  })

  test('the choice survives a reload', async () => {
    // The fixture clears storage after every test, so this one ticks the box itself.
    await openLayers(page)
    // Still ticked from the last test, so a bare check() would write nothing: untick first.
    const box = page.getByRole('checkbox', { name: 'Faulty stations', exact: true })
    await box.setChecked(false)
    await box.setChecked(true)
    await expect.poll(async () => (await sensorIds(page)).find((s) => s.id === 104)?.faulty).toBe(true)
    await page.reload()
    await mapSettled(page)
    await expect.poll(async () => (await sensorIds(page)).find((s) => s.id === 104)?.faulty).toBe(true)
    await openLayers(page)
    await expect(page.getByRole('checkbox', { name: 'Faulty stations', exact: true })).toBeChecked()
    await page.getByRole('checkbox', { name: 'Faulty stations', exact: true }).uncheck()
    await expect.poll(async () => (await sensorIds(page)).map((s) => s.id)).not.toContain(104)
  })

  test('a station is faulty per layer: 104 is a normal PM10 station', async () => {
    await page.getByRole('button', { name: /^Metric:/ }).and(page.locator('#metric-menu')).click()
    await page.getByRole('radio', { name: 'PM10' }).check()
    await expect.poll(async () => (await sensorIds(page)).find((s) => s.id === 104)?.faulty).toBe(false)
  })
})

test('the panel names the failed metric', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.goto('/en/area/sofia#sensor=104')
  await expect(page.getByText('PM2.5: This reading has not changed in a while.')).toBeVisible({ timeout: 10000 })
  await page.close()
})
