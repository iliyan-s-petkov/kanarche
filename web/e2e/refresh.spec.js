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

// The label's clock as the page writes it: 24-hour HH:MM in the page language.
const clock = (page, ms) => page.evaluate((t) => new Date(t)
  .toLocaleTimeString(document.documentElement.lang || 'en', { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }), ms)

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
  // The request event does not carry the fetch cache mode, so the page's own fetch is watched.
  await page.evaluate(() => {
    window.__sent = []
    const real = window.fetch
    window.fetch = (input, init) => {
      const path = new URL(typeof input === 'string' ? input : input.url, location.href).pathname
      window.__sent.push({ path, cache: init?.cache ?? input?.cache })
      return real(input, init)
    }
  })
  const wire = []
  page.on('request', (r) => { if (isApi(r, 'hexes') || isApi(r, 'overview')) wire.push(new URL(r.url()).pathname) })
  await openPopover(page, false)
  await page.getByRole('button', { name: 'Refresh now' }).click()
  await expect.poll(() => new Set(wire).size).toBe(2)
  const sent = await page.evaluate(() => window.__sent)
  for (const path of ['/api/v1/hexes', '/api/v1/overview']) {
    const calls = sent.filter((s) => s.path === path)
    expect(calls.length, path).toBeGreaterThan(0)
    expect(calls[0].cache, path).toBe('no-cache')
  }

  const want = await clock(page, snapshot)
  await expect(page.locator('.data-refresh__btn--icon')).toHaveAttribute('title', new RegExp(`${want}$`))
  await expect(page.locator('.data-refresh__btn--icon')).toHaveAttribute('aria-label', `Refresh ${want}`)
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

  // Counts every paint of the hex source from here on. The listener is on the document's capture phase because the event does not bubble.
  await page.evaluate(() => {
    window.__hexPaints = 0
    document.addEventListener('kanarche:paint', (e) => {
      if (e.detail?.source === 'kanarche-hexes') window.__hexPaints += 1
    }, true)
  })
  const reloaded = page.waitForResponse((r) => isApi(r.request(), 'hexes'))
  await openPopover(page, false)
  await page.getByRole('button', { name: 'Refresh now' }).click()
  // The live grid is reloaded underneath, so the request still goes out.
  await reloaded
  // The refresh is over when the button stops being busy, and nothing was painted by it.
  await expect(page.locator('.data-refresh__btn--icon')).toHaveAttribute('aria-busy', 'false')
  expect(await page.evaluate(() => window.__hexPaints)).toBe(0)
  expect(await hexData(page)).toEqual(paused)
  expect(await page.locator('.map-play__clock').textContent()).toBe(clock)

  await page.locator('.map-play__exit').click()
  await expect.poll(() => hexData(page)).toEqual(live)
  await page.close()
})

// The time beside the refresh icon: the snapshot the server built, in view and
// inside its box at both widths. Set E2E_SHOTS=1 to keep a screenshot of each in test-results.

for (const [name, vp, phone] of [['1440', { width: 1440, height: 900 }, false], ['390', { width: 390, height: 844 }, true]]) {
  for (const scheme of ['light', 'dark']) {
    test(`the refresh icon shows the snapshot time at ${name} in ${scheme}`, async ({ ctx }, testInfo) => {
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
      const want = await clock(page, Date.parse(generated))
      await expect(time).toHaveText(want)
      // --stale resolves against the theme's own foreground, so the dark theme gets the dark one.
      const token = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--stale'))
      const fg = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--fg').trim())
      expect(token.toLowerCase()).toContain(fg.toLowerCase())

      // One line, inside the button, inside the viewport.
      const t = await time.boundingBox()
      const b = await btn.boundingBox()
      expect(t.height).toBeLessThan(24)
      expect(t.x).toBeGreaterThanOrEqual(b.x)
      expect(t.x + t.width).toBeLessThanOrEqual(b.x + b.width + 0.5)
      expect(b.x + b.width).toBeLessThanOrEqual(vp.width)
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)

      if (process.env.E2E_SHOTS) await page.screenshot({ path: testInfo.outputPath(`time-${name}-${scheme}.png`) })
      await page.close()
    })
  }
}

// Where the label's time comes from, end to end: the hex body's generated_at, kept
// when a later refresh brings none, and never the browser's own clock.
test.describe('the refresh time follows the server snapshot', () => {
  const T1 = '2026-01-15T03:07:00Z'
  const T2 = '2026-01-15T03:22:00Z'

  // mode.at is the generated_at to serve, undefined to strip it, and mode.fail drops the request.
  async function withHexTime(ctx, mode) {
    const page = await ctx.newPage()
    await page.setViewportSize(DESKTOP)
    await page.route('**/api/v1/hexes*', async (route) => {
      if (mode.fail) return route.abort()
      const res = await route.fetch()
      const body = await res.json()
      if (mode.at === undefined) delete body.generated_at
      else body.generated_at = mode.at
      return route.fulfill({ response: res, json: body })
    })
    return page
  }

  async function refreshOnce(page) {
    const hexes = hexRequests(page)
    await openPopover(page, false)
    await page.getByRole('button', { name: 'Refresh now' }).click()
    await expect.poll(() => hexes.length).toBeGreaterThan(0)
    await expect(page.locator('.data-refresh__btn--icon')).toHaveAttribute('aria-busy', 'false')
  }

  test('takes the hex time, keeps it through a refresh that brings none, and moves to a newer one', async ({ ctx }) => {
    const mode = { at: T1 }
    const page = await withHexTime(ctx, mode)
    await page.goto('/en/')
    await mapSettled(page)
    const time = page.locator('.data-refresh__time')
    await expect(time).toHaveText(await clock(page, Date.parse(T1)))

    mode.fail = true
    await refreshOnce(page)
    await expect(time).toHaveText(await clock(page, Date.parse(T1)))

    mode.fail = false
    mode.at = undefined
    await refreshOnce(page)
    await expect(time).toHaveText(await clock(page, Date.parse(T1)))

    mode.at = T2
    await refreshOnce(page)
    await expect(time).toHaveText(await clock(page, Date.parse(T2)))
    await page.close()
  })

  test('shows no time at all when the server never gave one', async ({ ctx }) => {
    const page = await withHexTime(ctx, { at: undefined })
    await page.goto('/en/')
    await mapSettled(page)
    await expect(page.locator('.data-refresh__btn--icon')).toBeVisible()
    await expect(page.locator('.data-refresh__time')).toHaveCount(0)
    await expect(page.locator('.data-refresh__btn--icon')).toHaveAttribute('aria-label', 'Refresh')
    await page.close()
  })
})
