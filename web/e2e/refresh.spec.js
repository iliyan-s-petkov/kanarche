import { test, expect, mapSettled } from './fixtures.js'

// OpenProject #660: one refresh control, a popover on the freshness pill.
const DESKTOP = { width: 1280, height: 800 }
const PHONE = { width: 393, height: 873 }

async function visit(ctx, vp, path = '/en/') {
  const page = await ctx.newPage()
  await page.setViewportSize(vp)
  await page.goto(path)
  await mapSettled(page)
  return page
}

// On a phone the pill lives in the window panel's footer, so that opens first.
async function openPopover(page, phone) {
  if (phone) await page.locator('.map-window__btn').click()
  const trigger = page.locator('.data-refresh__btn--icon')
  await trigger.click()
  await expect(page.locator('.data-refresh__panel')).toBeVisible()
  return trigger
}

const hexRequests = (page) => {
  const seen = []
  page.on('request', (r) => { if (r.url().includes('/api/v1/hexes')) seen.push(r.url()) })
  return seen
}

for (const [name, vp, phone] of [['desktop', DESKTOP, false], ['phone', PHONE, true]]) {
  test.describe(`refresh popover on ${name}`, () => {
    test('there is exactly one refresh control and it opens the popover', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      await expect(page.locator('.data-refresh__btn')).toHaveCount(1)
      await expect(page.locator('[data-island="refresh"], .toolbar__refresh, .data-refresh__auto')).toHaveCount(0)
      const trigger = await openPopover(page, phone)
      await expect(trigger).toHaveAttribute('aria-expanded', 'true')
      await expect(page.getByRole('button', { name: 'Refresh now' })).toBeVisible()
      const group = page.getByRole('radiogroup', { name: 'Refresh automatically' })
      await expect(group.getByRole('radio')).toHaveCount(4)
      await expect(group.getByRole('radio', { name: 'Every 5 min' })).toBeChecked()
      await page.close()
    })

    test('Refresh now refetches the hex grid and closes the popover', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      const hexes = hexRequests(page)
      await openPopover(page, phone)
      const before = hexes.length
      await page.getByRole('button', { name: 'Refresh now' }).click()
      await expect.poll(() => hexes.length).toBeGreaterThan(before)
      await expect(page.locator('.data-refresh__panel')).toBeHidden()
      await page.close()
    })

    test('the interval persists across a reload', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      await openPopover(page, phone)
      await page.getByRole('radio', { name: 'Every 15 min' }).check()
      expect(await page.evaluate(() => localStorage.getItem('kanarche:auto-refresh'))).toBe('15')
      await page.reload()
      await mapSettled(page)
      await openPopover(page, phone)
      await expect(page.getByRole('radio', { name: 'Every 15 min' })).toBeChecked()
      await page.getByRole('radio', { name: 'Off' }).check()
      expect(await page.evaluate(() => localStorage.getItem('kanarche:auto-refresh'))).toBe('0')
      await page.close()
    })

    test('Escape closes it and focus returns to the icon', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      const trigger = await openPopover(page, phone)
      await page.getByRole('radio', { name: 'Every 5 min' }).focus()
      await page.keyboard.press('Escape')
      await expect(page.locator('.data-refresh__panel')).toBeHidden()
      await expect(trigger).toBeFocused()
      await page.close()
    })

    test('a click outside closes it', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      await openPopover(page, phone)
      await page.locator('.masthead').click({ position: { x: 5, y: 5 } })
      await expect(page.locator('.data-refresh__panel')).toBeHidden()
      await page.close()
    })

    test('the popover fits the viewport and sits above the dock and sheet', async ({ ctx }) => {
      const page = await visit(ctx, vp)
      await openPopover(page, phone)
      const box = await page.locator('.data-refresh__panel').boundingBox()
      expect(box.x).toBeGreaterThanOrEqual(0)
      expect(box.y).toBeGreaterThanOrEqual(0)
      expect(box.x + box.width).toBeLessThanOrEqual(vp.width)
      expect(box.y + box.height).toBeLessThanOrEqual(vp.height)
      // The element under the panel's centre is the panel, not the sheet or the dock.
      const top = await page.evaluate(({ x, y }) => !!document.elementFromPoint(x, y)?.closest('.data-refresh__panel'), {
        x: box.x + box.width / 2, y: box.y + box.height / 2,
      })
      expect(top).toBe(true)
      await page.close()
    })
  })
}

test('an old boolean setting is read as 5 minutes or off', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize(DESKTOP)
  await page.addInitScript(() => localStorage.setItem('kanarche:auto-refresh', 'false'))
  await page.goto('/en/')
  await mapSettled(page)
  await openPopover(page, false)
  await expect(page.getByRole('radio', { name: 'Off' })).toBeChecked()
  await page.close()
})

const isApi = (r, name) => new URL(r.url()).pathname === `/api/v1/${name}`

// Refresh now must go back to the server, and the label must name the snapshot
// the server built, not the moment the browser asked.
test('Refresh now revalidates /hexes and /overview and the label shows the snapshot time', async ({ ctx }) => {
  const page = await visit(ctx, DESKTOP)
  const snapshot = Date.parse('2026-01-15T03:07:00Z')
  // Pins the snapshot to a time that is nowhere near the client clock.
  await page.route('**/api/v1/hexes*', async (route) => {
    const res = await route.fetch()
    const body = await res.json()
    body.generated_at = new Date(snapshot).toISOString()
    await route.fulfill({ response: res, json: body })
  })
  const sent = []
  page.on('request', async (r) => {
    if (isApi(r, 'hexes') || isApi(r, 'overview')) sent.push({ name: new URL(r.url()).pathname, headers: await r.allHeaders() })
  })
  await openPopover(page, false)
  await page.getByRole('button', { name: 'Refresh now' }).click()
  await expect.poll(() => new Set(sent.map((s) => s.name)).size).toBe(2)
  for (const s of sent) expect(s.headers['cache-control'], s.name).toMatch(/no-cache|max-age=0/)

  const want = await page.evaluate((ms) => new Date(ms)
    .toLocaleTimeString(document.documentElement.lang || 'en', { hour: '2-digit', minute: '2-digit' }), snapshot)
  await expect(page.locator('.data-refresh__btn--icon')).toHaveAttribute('title', new RegExp(`${want}$`))
  await page.close()
})

const hexData = (page) => page.evaluate(() => {
  const map = document.querySelector('[data-island="map"]').__map
  return map.getSource('kanarche-hexes').serialize().data.features
    .filter((f) => f.properties.value !== null && f.properties.value !== undefined)
    .map((f) => ({ v: f.properties.value, carried: f.properties.carried === true }))
})

test('a refresh while the replay is paused leaves the frame and the clock alone', async ({ ctx }) => {
  const page = await visit(ctx, DESKTOP)
  await expect.poll(async () => (await hexData(page)).length).toBeGreaterThan(0)
  const live = await hexData(page)

  const play = page.locator('.map-play__btn[aria-pressed]')
  await play.evaluate((el) => el.scrollIntoView({ block: 'center' }))
  await play.click()
  await expect(page.locator('.map-play--open')).toHaveCount(1)
  await play.click()
  await expect(play).toHaveAttribute('aria-pressed', 'false')
  const paused = await hexData(page)
  const clock = await page.locator('.map-play__clock').textContent()
  expect(paused).not.toEqual(live)

  const hexes = hexRequests(page)
  await openPopover(page, false)
  await page.getByRole('button', { name: 'Refresh now' }).click()
  // The live grid is reloaded underneath, so the request still goes out.
  await expect.poll(() => hexes.length).toBeGreaterThan(0)
  await expect(page.locator('.data-refresh__btn--icon')).toHaveAttribute('aria-busy', 'false')
  await page.waitForTimeout(1000)
  expect(await hexData(page)).toEqual(paused)
  expect(await page.locator('.map-play__clock').textContent()).toBe(clock)

  await page.locator('.map-play__exit').click()
  await expect.poll(() => hexData(page)).toEqual(live)
  await page.close()
})

// The time beside the refresh icon: the snapshot the server built, in view and
// inside its box at both widths. Shots go to the verification cache for review.
const SHOTS = '/Users/iliyan/.cache/kanarche-verify/refresh-shots'

for (const [name, vp, phone] of [['1440', { width: 1440, height: 900 }, false], ['390', { width: 390, height: 844 }, true]]) {
  for (const scheme of ['light', 'dark']) {
    test(`the refresh icon shows the snapshot time at ${name} in ${scheme}`, async ({ ctx }) => {
      const page = await ctx.newPage()
      await page.emulateMedia({ colorScheme: scheme })
      await page.setViewportSize(vp)
      const hexes = page.waitForResponse((r) => isApi(r.request(), 'hexes'))
      await page.goto('/en/')
      await mapSettled(page)
      const generated = (await (await hexes).json()).generated_at
      if (phone) await page.locator('.map-window__btn').click()
      const btn = page.locator('.data-refresh__btn--icon')
      const time = btn.locator('.data-refresh__time')
      await expect(time).toBeVisible()
      const want = await page.evaluate((iso) => new Date(iso)
        .toLocaleTimeString(document.documentElement.lang || 'en', { hour: '2-digit', minute: '2-digit' }), generated)
      await expect(time).toHaveText(want)

      // One line, inside the button, inside the viewport.
      const t = await time.boundingBox()
      const b = await btn.boundingBox()
      expect(t.height).toBeLessThan(24)
      expect(t.x).toBeGreaterThanOrEqual(b.x)
      expect(t.x + t.width).toBeLessThanOrEqual(b.x + b.width + 0.5)
      expect(b.x + b.width).toBeLessThanOrEqual(vp.width)
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)

      await page.screenshot({ path: `${SHOTS}/time-${name}-${scheme}.png` })
      await page.close()
    })
  }
}
