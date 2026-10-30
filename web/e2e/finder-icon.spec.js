import { test, expect, mapSettled } from './fixtures.js'

// On a touch screen under 400px the map finder is a 44px magnifier button that
// opens the full field over the control bar. Wider screens keep the field.

const NARROW = [[360, 640], [375, 667], [390, 844]]
const GAP = 8
const NAME = { '/': 'Търсене на район', '/en': 'Find an area' }
const PLACEHOLDER = { '/': 'Област или адрес', '/en': 'Area or address' }

const shot = (page, name) => process.env.KANARCHE_SHOT_DIR
  ? page.screenshot({ path: `${process.env.KANARCHE_SHOT_DIR}/${process.env.KANARCHE_SHOT_TAG}-${name}.png`, clip: { x: 0, y: 0, width: page.viewportSize().width, height: 130 } })
  : null

const visit = async (browser, w, h, path, touch = true) => {
  const ctx = await browser.newContext({ viewport: { width: w, height: h }, isMobile: touch, hasTouch: touch })
  const page = await ctx.newPage()
  await page.goto(path)
  await mapSettled(page)
  await page.evaluate(() => document.fonts.ready)
  return { ctx, page }
}

const toggle = (page) => page.locator('.map-controls__find .find-toggle')
const input = (page) => page.locator('.map-controls__find input')

// Gap between two boxes, negative when they overlap.
const gapOf = (a, b) => Math.max(a.left - b.right, b.left - a.right, a.top - b.bottom, b.top - a.bottom)

for (const path of ['/', '/en']) {
  for (const [w, h] of NARROW) {
    const at = `${w}x${h} touch ${path}`
    const tag = `${w}x${h}${path.replace(/\//g, '_')}`

    test(`${at}: closed, the finder is a named 44x44 button and the field is hidden`, async ({ browser }) => {
      const { ctx, page } = await visit(browser, w, h, path)
      await shot(page, `closed-${tag}`)
      const btn = page.getByRole('button', { name: NAME[path] })
      await expect(btn).toBeVisible()
      await expect(btn).toHaveAttribute('aria-expanded', 'false')
      await expect(input(page)).toBeHidden()
      const res = await page.evaluate(() => {
        const box = (e) => { const r = e.getBoundingClientRect(); return { left: r.left, right: r.right, top: r.top, bottom: r.bottom, width: r.width, height: r.height } }
        const own = document.querySelector('.map-controls__find .find-toggle')
        const others = [...document.querySelectorAll('.map__layers button, .map-controls__metric button, .map-rail button, .map-rail a')]
          .filter((e) => e.checkVisibility()).map((e) => ({ name: e.className, ...box(e) }))
        return { own: box(own), others }
      })
      expect(res.own.width).toBeGreaterThanOrEqual(43.5)
      expect(res.own.height).toBeGreaterThanOrEqual(43.5)
      // No two hit boxes meet: the button against the bar and the rail.
      expect(res.others.filter((o) => gapOf(res.own, o) < 0).map((o) => `${o.name} gap ${Math.round(gapOf(res.own, o))}`)).toEqual([])
      // And it keeps 8px from the rail buttons.
      expect(res.others.filter((o) => /rail|zoom|full|locate/.test(o.name) && gapOf(res.own, o) < GAP).map((o) => o.name)).toEqual([])
      await ctx.close()
    })

    test(`${at}: a tap opens the field, focused, placeholder whole, clear of the rail`, async ({ browser }) => {
      const { ctx, page } = await visit(browser, w, h, path)
      await toggle(page).tap()
      await expect(input(page)).toBeFocused()
      await expect(input(page)).toHaveAttribute('placeholder', PLACEHOLDER[path])
      await page.waitForTimeout(300)
      await shot(page, `open-${tag}`)
      const res = await page.evaluate(() => {
        const box = (e) => { const r = e.getBoundingClientRect(); return { left: r.left, right: r.right, top: r.top, bottom: r.bottom, width: r.width } }
        const f = document.querySelector('.map-controls__find input')
        const cs = getComputedStyle(f)
        const c = document.createElement('canvas').getContext('2d')
        c.font = `${cs.fontStyle} ${cs.fontWeight} ${cs.fontSize} ${cs.fontFamily}`
        const text = c.measureText(f.placeholder).width
        const room = f.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight)
        const rail = [...document.querySelectorAll('.map-rail button, .map-rail a')].filter((e) => e.checkVisibility())
        return { find: box(f), text, room, rail: rail.map((e) => ({ name: e.className, ...box(e) })), vw: innerWidth, over: document.documentElement.scrollWidth - innerWidth }
      })
      expect(res.text).toBeLessThanOrEqual(res.room)
      expect(res.find.left).toBeGreaterThanOrEqual(0)
      expect(res.find.right).toBeLessThanOrEqual(res.vw)
      expect(res.over).toBeLessThanOrEqual(0)
      expect(res.rail.length).toBeGreaterThan(0)
      expect(res.rail.filter((r) => gapOf(res.find, r) < GAP).map((r) => `${r.name} gap ${Math.round(gapOf(res.find, r))}`)).toEqual([])
      await expect(toggle(page)).toBeHidden()
      await ctx.close()
    })

    test(`${at}: Escape closes the field and returns focus to the button`, async ({ browser }) => {
      const { ctx, page } = await visit(browser, w, h, path)
      await toggle(page).tap()
      await expect(input(page)).toBeFocused()
      await page.keyboard.press('Escape')
      await expect(input(page)).toBeHidden()
      await expect(toggle(page)).toBeFocused()
      await expect(toggle(page)).toHaveAttribute('aria-expanded', 'false')
      await ctx.close()
    })
  }
}

test('360x640 touch: blur with an empty field closes it, a typed query keeps it open', async ({ browser }) => {
  const { ctx, page } = await visit(browser, 360, 640, '/en')
  await toggle(page).tap()
  await input(page).fill('Sof')
  await input(page).evaluate((e) => e.blur())
  await expect(input(page)).toBeVisible()
  await input(page).fill('')
  await input(page).evaluate((e) => e.blur())
  await expect(input(page)).toBeHidden()
  await expect(toggle(page)).toBeVisible()
  await ctx.close()
})

test('360x640 touch: choosing a result closes the field back to the icon', async ({ browser }) => {
  const { ctx, page } = await visit(browser, 360, 640, '/en')
  await toggle(page).tap()
  await input(page).fill('Stub street 1')
  await page.locator('.combobox__opt--address').tap()
  const row = page.locator('.map-controls__find [role="option"]').first()
  await expect(row).toContainText('Stub street 1, Sofia')
  await row.tap()
  await expect(input(page)).toBeHidden()
  await expect(toggle(page)).toBeFocused()
  await ctx.close()
})

for (const [w, h, touch] of [[412, 915, true], [1280, 800, false], [360, 640, false]]) {
  test(`${w}x${h} ${touch ? 'touch' : 'pointer'}: the field stays, no icon button`, async ({ browser }) => {
    const { ctx, page } = await visit(browser, w, h, '/en', touch)
    await expect(input(page)).toBeVisible()
    await expect(toggle(page)).toBeHidden()
    await ctx.close()
  })
}
