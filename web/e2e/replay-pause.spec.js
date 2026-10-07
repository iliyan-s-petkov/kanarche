import { test, expect } from './fixtures.js'

// Pausing a replay leaves the paused hour on the map. Exit puts the live grid back.

const hexData = (page) => page.evaluate(() => {
  const map = document.querySelector('[data-island="map"]').__map
  const data = map.getSource('airbg-hexes').serialize().data
  return data.features
    .filter((f) => f.properties.value !== null && f.properties.value !== undefined)
    .map((f) => ({ v: f.properties.value, carried: f.properties.carried === true }))
})

test('pause keeps the paused frame on the map, exit restores live', async ({ page }) => {
  await page.goto('/en')
  await page.waitForFunction(() => document.querySelector('[data-island="map"]').__map?.getSource('airbg-hexes'))
  // The grid arrives after the source exists; a snapshot before it would be empty.
  await expect.poll(async () => (await hexData(page)).length).toBeGreaterThan(0)
  const live = await hexData(page)

  const play = page.locator('.map-play__btn[aria-pressed]')
  await play.evaluate((el) => el.scrollIntoView({ block: 'center' }))
  const replay = page.waitForResponse((r) => r.url().includes('/api/') && r.url().includes('timelapse'))
  await play.click()
  const body = await (await replay).json()
  await expect(page.locator('.map-play--open')).toHaveCount(1)

  // Pause on whichever frame the clock has reached.
  await play.click()
  await expect(play).toHaveAttribute('aria-pressed', 'false')
  const i = Number(await page.locator('.map-play__scrub').inputValue())
  const clock = await page.locator('.map-play__clock').textContent()
  const frame = body.frames[i].v
  const paused = await hexData(page)
  expect(paused.length).toBeGreaterThan(0)
  for (const c of paused.filter((c) => !c.carried)) expect(frame).toContain(c.v)

  // Longer than a frame at the slowest speed: a live repaint or a running clock would show.
  await page.waitForTimeout(2500)
  expect(await hexData(page)).toEqual(paused)
  expect(await page.locator('.map-play__clock').textContent()).toBe(clock)

  await page.locator('.map-play__exit').click()
  await expect.poll(() => hexData(page)).toEqual(live)
})
