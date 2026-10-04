import { test, expect } from './fixtures.js'

// The geocoder behind /api/v1/geocode is a stub (internal/e2e); nothing here
// reaches the real Nominatim.
const SOFIA = { lat: 42.6977, lon: 23.3219 }

async function open(ctx, path, vp = { width: 1440, height: 900 }) {
  const page = await ctx.newPage()
  await page.setViewportSize(vp)
  const calls = []
  page.on('request', (r) => { if (r.url().includes('/api/v1/geocode')) calls.push(r.url()) })
  await page.goto(path)
  await page.waitForFunction(() => document.querySelector('[data-island="map"]')?.__map?.isStyleLoaded?.())
  return { page, calls }
}

test('typing sends no geocode request; Enter does and lists the result', async ({ ctx }) => {
  const { page, calls } = await open(ctx, '/en/')
  const input = page.locator('[data-island="finder"] input')
  await input.click()
  await input.pressSequentially('Stub street 1', { delay: 40 })
  await expect(page.locator('.combobox__opt--address')).toHaveText('Search address: Stub street 1')
  await page.waitForTimeout(500)
  expect(calls).toHaveLength(0)
  await input.press('Enter')
  const opt = page.locator('[data-island="finder"] [role="option"]')
  await expect(opt).toHaveCount(1)
  await expect(opt).toContainText('Stub street 1, Sofia')
  await expect(page.locator('.combobox__credit a')).toHaveAttribute('href', 'https://www.openstreetmap.org/copyright')
  expect(calls).toHaveLength(1)
  expect(calls[0]).toContain('lang=en')
  await page.close()
})

test('picking a result moves the map to it and drops a pin; Escape clears the pin', async ({ ctx }) => {
  const { page } = await open(ctx, '/en/')
  const input = page.locator('[data-island="finder"] input')
  await input.fill('Stub street 1')
  await page.locator('.combobox__opt--address').click()
  await page.locator('[data-island="finder"] [role="option"]').first().click()
  await expect(page.locator('.address-pin')).toHaveCount(1)
  await expect(page.locator('.address-pin')).toHaveAttribute('aria-label', 'Searched address')
  await page.waitForFunction(({ lat, lon }) => {
    const c = document.querySelector('[data-island="map"]').__map.getCenter()
    return Math.abs(c.lat - lat) < 0.01 && Math.abs(c.lng - lon) < 0.01
  }, SOFIA)
  await input.focus()
  await input.press('Escape')
  await expect(page.locator('.address-pin')).toHaveCount(0)
  await page.close()
})

test('no address found says house numbers are patchy', async ({ ctx }) => {
  const { page } = await open(ctx, '/en/')
  const input = page.locator('[data-island="finder"] input')
  await input.fill('nowhere at all')
  await input.press('Enter')
  await expect(page.locator('.combobox__empty')).toContainText('house-number coverage in Bulgaria is patchy')
  await page.close()
})

test('a busy answer shows the busy state', async ({ ctx }) => {
  const { page } = await open(ctx, '/en/')
  await page.route('**/api/v1/geocode*', (route) => route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":{"code":"busy"}}' }))
  const input = page.locator('[data-island="finder"] input')
  await input.fill('Stub street 1')
  await input.press('Enter')
  await expect(page.locator('.combobox__empty[data-state="busy"]')).toContainText('busy')
  await page.close()
})

test('a network error shows the error state', async ({ ctx }) => {
  const { page } = await open(ctx, '/en/')
  await page.route('**/api/v1/geocode*', (route) => route.abort())
  const input = page.locator('[data-island="finder"] input')
  await input.fill('Stub street 1')
  await input.press('Enter')
  await expect(page.locator('.combobox__empty[data-state="error"]')).toContainText('failed')
  await page.close()
})

test('BG placeholder reads "Област или адрес" and the request carries lang=bg', async ({ ctx }) => {
  const { page, calls } = await open(ctx, '/', { width: 390, height: 844 })
  const input = page.locator('[data-island="finder"] input')
  await expect(input).toHaveAttribute('placeholder', 'Област или адрес')
  await input.fill('Stub street 1')
  await input.press('Enter')
  await expect(page.locator('[data-island="finder"] [role="option"]')).toHaveCount(1)
  expect(calls[0]).toContain('lang=bg')
  await page.close()
})
