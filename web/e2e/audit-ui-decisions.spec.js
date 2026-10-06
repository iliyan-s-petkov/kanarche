import { test as base, expect } from './fixtures.js'

// Decisions from the 2026-10-01 audit: S03, U08, U09, A04 and the map attribution glyph.

const test = base.extend({
  phoneCtx: [async ({ browser }, use) => {
    const context = await browser.newContext({ isMobile: true, hasTouch: true, deviceScaleFactor: 2 })
    await use(context)
    await context.close()
  }, { scope: 'worker' }],
})

test.afterEach(async ({ ctx, phoneCtx }) => {
  for (const c of [ctx, phoneCtx]) {
    for (const p of c.pages()) if (!p.isClosed()) await p.close().catch(() => {})
  }
})

test.describe('S03: home heading', () => {
  test('one hidden H1 and no visible hero on desktop and phone', async ({ ctx, phoneCtx }) => {
    for (const [context, size] of [[ctx, { width: 1280, height: 800 }], [phoneCtx, { width: 390, height: 844 }]]) {
      const page = await context.newPage()
      await page.setViewportSize(size)
      await page.goto('/en')
      await expect(page.locator('h1')).toHaveCount(1)
      await expect(page.locator('h1')).toHaveText('Kanarche, Bulgaria air quality map: PM2.5 and PM10 now')
      await expect(page.locator('h1')).toHaveClass('visually-hidden')
      await expect(page.locator('.page-head')).toHaveCount(0)
      await page.close()
    }
  })
})

test.describe('U08: About and GitHub on a phone', () => {
  test('the header drops them and the language menu carries them', async ({ phoneCtx }) => {
    const page = await phoneCtx.newPage()
    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/en')
    await expect(page.locator('.masthead__link--about')).toBeHidden()
    await expect(page.locator('.masthead__link--source')).toBeHidden()
    await page.locator('.langpick__btn').first().click()
    const about = page.locator('.langpick__extra a[href$="/about"]')
    const source = page.locator('.langpick__extra a[href^="https://github.com/"]')
    await expect(about).toBeVisible()
    await expect(source).toBeVisible()
    await expect(source).toHaveAttribute('rel', 'noopener noreferrer')
  })

  test('the desktop menu lists languages only', async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.setViewportSize({ width: 1280, height: 800 })
    await page.goto('/en')
    await expect(page.locator('.masthead__link--about')).toBeVisible()
    await page.locator('.langpick__btn').first().click()
    await expect(page.locator('.langpick__extra a').first()).toBeHidden()
  })
})

test.describe('A04: phone play bar tap targets', () => {
  test('play and window buttons are at least 44x44', async ({ phoneCtx }) => {
    const page = await phoneCtx.newPage()
    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/en')
    const buttons = [page.getByRole('button', { name: 'Play the animation' }), page.locator('.map-window__btn')]
    for (const b of buttons) {
      await expect(b).toBeVisible()
      await expect.poll(async () => (await b.boundingBox())?.width ?? 0).toBeGreaterThanOrEqual(44)
      await expect.poll(async () => (await b.boundingBox())?.height ?? 0).toBeGreaterThanOrEqual(44)
    }
    // The control row still sits inside the map and clear of the legend.
    const map = await page.locator('.map').first().boundingBox()
    const play = await buttons[0].boundingBox()
    expect(play.y + play.height).toBeLessThanOrEqual(map.y + map.height)
  })
})

test.describe('attribution toggle glyph', () => {
  test('shows a copyright mark, keeps its accessible name', async ({ phoneCtx }) => {
    const page = await phoneCtx.newPage()
    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/en')
    const btn = page.locator('.maplibregl-ctrl-attrib-button')
    await expect(btn).toBeVisible()
    const glyph = await btn.evaluate((el) => getComputedStyle(el, '::before').content)
    expect(glyph).toBe('"©"')
    expect((await btn.getAttribute('aria-label')) || '').not.toBe('')
  })
})

test.describe('footer snapshot time in local time', () => {
  test('is rewritten from UTC to the browser zone', async ({ browser }) => {
    const context = await browser.newContext({ timezoneId: 'Asia/Tokyo', locale: 'en-GB' })
    const page = await context.newPage()
    await page.goto('/en')
    const el = page.locator('footer time[data-local-time]')
    const iso = await el.getAttribute('datetime')
    await expect(el).not.toContainText('UTC')
    const fmt = new Intl.DateTimeFormat('en', { hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZone: 'Asia/Tokyo' })
    await expect(el).toContainText(fmt.format(new Date(iso)))
    await context.close()
  })
})
