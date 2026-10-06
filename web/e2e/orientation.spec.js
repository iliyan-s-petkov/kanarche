import { test, expect, mapSettled } from './fixtures.js'

// The orientation popover: live sliders, reset to north, and staying on screen.
const VIEWPORTS = [
  ['1440x900', { width: 1440, height: 900 }],
  ['393x873', { width: 393, height: 873 }],
]
const SHOTS = process.env.ORIENT_SHOTS

const camera = (page) => page.evaluate(() => {
  const m = document.querySelector('[data-island="map"]').__map
  return { pitch: m.getPitch(), bearing: m.getBearing() }
})

for (const [name, size] of VIEWPORTS) {
  test(`orientation popover drives and resets the camera at ${name}`, async ({ page }) => {
    await page.setViewportSize(size)
    await page.goto('/en/')
    await mapSettled(page)
    await page.evaluate(() => document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' }))

    const button = page.locator('.map-orient__btn')
    const panel = page.locator('.map-orient__panel')
    await expect(button).toBeVisible()
    await expect(button).toHaveAttribute('aria-label', 'Orientation')
    await expect(button).toHaveAttribute('aria-expanded', 'false')
    await expect(panel).toBeHidden()

    await button.click()
    await expect(panel).toBeVisible()
    await expect(button).toHaveAttribute('aria-expanded', 'true')

    await page.locator('.map-orient__heading').fill('90')
    await expect.poll(() => camera(page)).toMatchObject({ bearing: 90 })
    await page.locator('.map-orient__tilt').fill('40')
    await expect.poll(() => camera(page)).toMatchObject({ pitch: 40, bearing: 90 })
    await expect(page.locator('.map-orient__heading')).toHaveValue('90')

    // A programmatic move (as a gesture would make) reaches the sliders and the needle.
    await page.evaluate(() => document.querySelector('[data-island="map"]').__map.jumpTo({ bearing: -45, pitch: 20 }))
    await expect(page.locator('.map-orient__heading')).toHaveValue('-45')
    await expect(page.locator('.map-orient__tilt')).toHaveValue('20')
    await expect(page.locator('[data-needle]')).toHaveAttribute('transform', 'rotate(45 8 8)')

    const box = await panel.boundingBox()
    expect(box.x).toBeGreaterThanOrEqual(0)
    expect(box.x + box.width).toBeLessThanOrEqual(size.width)
    expect(box.y).toBeGreaterThanOrEqual(0)
    expect(box.y + box.height).toBeLessThanOrEqual(size.height)

    if (SHOTS) {
      for (const theme of ['light', 'dark']) {
        await page.evaluate((t) => { document.documentElement.dataset.theme = t }, theme)
        await page.screenshot({ path: `${SHOTS}/orient-${name}-${theme}-tilted.png` })
      }
    }

    await page.locator('.map-orient__north-btn').click()
    await expect.poll(() => camera(page), { timeout: 15_000 }).toEqual({ pitch: 0, bearing: 0 })

    await page.keyboard.press('Escape')
    await expect(panel).toBeHidden()
    await expect(button).toBeFocused()
  })

  test(`orientation panel closes on outside click and the X at ${name}`, async ({ page }) => {
    await page.setViewportSize(size)
    await page.goto('/en/')
    await mapSettled(page)
    await page.evaluate(() => document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' }))
    const panel = page.locator('.map-orient__panel')
    await page.locator('.map-orient__btn').click()
    await expect(panel).toBeVisible()
    await page.locator('.map-orient__close').click()
    await expect(panel).toBeHidden()
    await expect(page.locator('.map-orient__btn')).toBeFocused()
    await page.locator('.map-orient__btn').click()
    await expect(panel).toBeVisible()
    await page.mouse.click(8, size.height / 2)
    await expect(panel).toBeHidden()
  })
}

test('orientation gestures are enabled and strings follow the language', async ({ page }) => {
  await page.goto('/en/')
  await mapSettled(page)
  const gestures = await page.evaluate(() => {
    const m = document.querySelector('[data-island="map"]').__map
    return {
      dragRotate: m.dragRotate.isEnabled(),
      touchPitch: m.touchPitch.isEnabled(),
      touchZoomRotate: m.touchZoomRotate.isEnabled(),
      maxPitch: m.getMaxPitch(),
    }
  })
  expect(gestures).toEqual({ dragRotate: true, touchPitch: true, touchZoomRotate: true, maxPitch: 60 })
  await expect(page.locator('.map-orient__tip')).toHaveText('Right-drag or Ctrl+drag to tilt and rotate. On touch, use two fingers.')
  await expect(page.locator('.map-orient__north-btn')).toHaveText('Reset to north')

  await page.goto('/')
  await mapSettled(page)
  await expect(page.locator('.map-orient__btn')).toHaveAttribute('aria-label', 'Ориентация')
  await expect(page.locator('.map-orient__north-btn')).toHaveText('Към север')
})

test('orientation control stays visible in fullscreen', async ({ page }) => {
  await page.goto('/en/')
  await mapSettled(page)
  await page.evaluate(() => document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' }))
  await page.locator('.map__full').click()
  await expect(page.locator('.map-orient__btn')).toBeVisible()
  if (SHOTS) {
    await page.locator('.map-orient__btn').click()
    await page.screenshot({ path: `${SHOTS}/orient-fullscreen.png` })
  }
})
