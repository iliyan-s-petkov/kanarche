import { test, expect, mapSettled } from './fixtures.js'

// A11Y-01, A11Y-02 and the skip link (epic #662). The focus ring is a
// box-shadow, which forced-colors mode strips, and the Layers button overrode
// it with a shadow of its own. zz-a11y.spec.js accepts any shadow, so it
// could not see either.

const focused = (page) => page.evaluate(() => {
  const a = document.activeElement
  if (!a || a === document.body) return null
  const s = getComputedStyle(a)
  return {
    cls: String(a.className),
    id: a.id,
    outline: s.outlineStyle !== 'none' && parseFloat(s.outlineWidth) > 0,
    shadow: s.boxShadow,
  }
})

// Tab until the layers button holds focus, so :focus-visible really applies.
async function tabToLayers(page) {
  for (let i = 0; i < 40; i++) {
    await page.keyboard.press('Tab')
    const f = await focused(page)
    if (f?.cls.includes('colmenu__btn')) return f
  }
  throw new Error('the Layers button never took focus')
}

for (const path of ['/', '/en/about', '/area/sofia']) {
  test(`${path}: the first Tab lands on a visible skip link and Enter moves focus to main`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.goto(path)
    await page.keyboard.press('Tab')
    const first = await focused(page)
    expect(first?.cls).toContain('skip')
    const box = await page.locator('.skip').boundingBox()
    expect(box.y).toBeGreaterThanOrEqual(0)
    expect(box.y + box.height).toBeGreaterThan(0)
    await expect(page.locator('.skip')).toBeInViewport({ ratio: 1 })
    await page.keyboard.press('Enter')
    expect((await focused(page))?.id).toBe('main')
    await page.close()
  })
}

test('the Layers button draws a focus ring beyond its resting shadow', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.goto('/')
  await mapSettled(page)
  const resting = await page.locator('.map__layers .colmenu__btn').evaluate((el) => getComputedStyle(el).boxShadow)
  const f = await tabToLayers(page)
  expect(f.shadow).not.toBe(resting)
  await page.close()
})

test('forced colours: the Layers button and every early Tab stop keep an outline', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.emulateMedia({ forcedColors: 'active' })
  await page.goto('/')
  await mapSettled(page)
  const f = await tabToLayers(page)
  expect(f.outline, 'Layers button has no outline in forced colours').toBe(true)

  await page.reload()
  await mapSettled(page)
  const bare = []
  for (let i = 0; i < 12; i++) {
    await page.keyboard.press('Tab')
    const s = await focused(page)
    if (s && !s.outline) bare.push(s.cls || s.id)
  }
  expect(bare, `Tab stops without an outline in forced colours: ${bare.join(', ')}`).toEqual([])
  await page.close()
})
