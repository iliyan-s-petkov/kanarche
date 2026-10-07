import { test, expect } from './fixtures.js'

// Wide screens carry the About tab in the masthead. At 480px and below it is
// hidden (the masthead has no room) and the footer link is the way in.
for (const lang of ['/en', '']) {
  const label = lang || '/bg'

  test(`about tab is in the masthead at 1280 (${label})`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.setViewportSize({ width: 1280, height: 800 })
    await page.goto(`${lang}/about`)
    const tab = page.locator(`.masthead__link--about[href="${lang}/about"]`)
    await expect(tab).toBeVisible()
    await expect(tab).toHaveAttribute('aria-current', 'page')
    const m = await page.evaluate(() => ({
      over: document.documentElement.scrollWidth - document.documentElement.clientWidth,
      tabHeight: document.querySelector('.masthead a[aria-current="page"]').getBoundingClientRect().height,
    }))
    expect(m.over).toBeLessThanOrEqual(0)
    // One line: a wrapped label would be taller than the 48px control.
    expect(m.tabHeight).toBeLessThanOrEqual(48)
    await page.close()
  })

  test(`at 390 the tab is hidden, the footer link works and nothing scrolls sideways (${label})`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto(`${lang}/areas`)
    await expect(page.locator(`.masthead__link--about[href="${lang}/about"]`)).toBeHidden()
    const footer = page.locator(`.footer a[href="${lang}/about"]`)
    await expect(footer).toBeVisible()
    await footer.click()
    await expect(page).toHaveURL(new RegExp(`${lang}/about$`))
    await expect(page.locator('h1')).toBeVisible()
    const over = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(over).toBeLessThanOrEqual(0)
    await page.close()
  })
}

// The images are lazy; scrolling each into view is what makes them load.
for (const [w, h] of [[390, 844], [1280, 800]]) {
  test(`getting started screenshots load and nothing scrolls sideways at ${w}`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.setViewportSize({ width: w, height: h })
    await page.goto('/en/about')
    const shots = page.locator('#start .about-shot__img--light')
    await expect(shots).toHaveCount(5)
    for (let i = 0; i < 5; i++) {
      await shots.nth(i).scrollIntoViewIfNeeded()
      await expect.poll(() => shots.nth(i).evaluate((img) => img.naturalWidth)).toBeGreaterThan(0)
    }
    const over = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(over).toBeLessThanOrEqual(0)
    await page.close()
  })
}

test('the dark screenshot replaces the light one under an explicit dark theme', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/en/about')
  await page.evaluate(() => { document.documentElement.dataset.theme = 'dark' })
  await expect(page.locator('#start .about-shot__img--dark').first()).toBeVisible()
  await expect(page.locator('#start .about-shot__img--light').first()).toBeHidden()
  await page.close()
})

test('the visitors chart draws from the seeded days', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/en/about')
  await page.locator('#visitors').scrollIntoViewIfNeeded()
  await expect(page.locator('.visitors-chart canvas')).toBeVisible()
  await expect(page.locator('.about-stats__chart .chart-message')).toHaveCount(0)
  await page.close()
})

test('the about page has no sideways scroll at 390, 1280, 1920 and 2560', async ({ ctx }) => {
  const page = await ctx.newPage()
  for (const [width, height] of [[390, 844], [1280, 800], [1920, 1080], [2560, 1440]]) {
    await page.setViewportSize({ width, height })
    await page.goto('/en/about')
    const over = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(over).toBeLessThanOrEqual(0)
  }
  await page.close()
})

const box = (loc) => loc.evaluate((el) => {
  const r = el.getBoundingClientRect()
  return { x: r.x, y: r.y, w: r.width, h: r.height, cx: r.x + r.width / 2, cy: r.y + r.height / 2 }
})

test('getting started alternates: step 1 image right of text, step 2 image left', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1280, height: 800 })
  await page.goto('/en/about')
  const steps = page.locator('#start .about-steps > li')
  for (const [i, imageRight] of [[0, true], [1, false], [2, true], [3, false]]) {
    const text = await box(steps.nth(i).locator('> .about-step__text'))
    const shot = await box(steps.nth(i).locator('> .about-shot'))
    if (imageRight) expect(shot.x).toBeGreaterThan(text.x + text.w - 1)
    else expect(shot.x + shot.w).toBeLessThan(text.x + 1)
    // Vertically centred against the image, not hung from the top.
    expect(Math.abs(text.cy - shot.cy)).toBeLessThan(4)
  }
  await page.close()
})

test('getting started steps stack text over image at 390', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/en/about')
  const step = page.locator('#start .about-steps > li').first()
  const text = await box(step.locator('> .about-step__text'))
  const shot = await box(step.locator('> .about-shot'))
  expect(shot.y).toBeGreaterThanOrEqual(text.y + text.h - 1)
  await page.close()
})

test('the step number is a filled circle on the same row as the heading', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1280, height: 800 })
  await page.goto('/en/about')
  const m = await page.locator('#start .about-steps h3').first().evaluate((h) => {
    const cs = getComputedStyle(h, '::before')
    const hr = h.getBoundingClientRect()
    const range = document.createRange()
    range.selectNodeContents(h)
    const lines = [...range.getClientRects()]
    const textRect = lines[lines.length - 1]
    return {
      content: cs.content, w: parseFloat(cs.width), h: parseFloat(cs.height),
      display: getComputedStyle(h).display, bg: cs.backgroundColor,
      hh: hr.height, hcy: hr.y + hr.height / 2, tcy: textRect.y + textRect.height / 2,
    }
  })
  expect(m.content).not.toBe('none')
  expect(m.w).toBeGreaterThanOrEqual(28)
  expect(m.h).toBeGreaterThanOrEqual(28)
  expect(m.bg).not.toBe('rgba(0, 0, 0, 0)')
  expect(m.display).toBe('flex')
  // One row: the heading is no taller than circle plus a little, and its text is centred on it.
  expect(m.hh).toBeLessThan(m.h + 8)
  expect(Math.abs(m.hcy - m.tcy)).toBeLessThan(4)
  await page.close()
})

test('the about container grows to 96rem and paragraphs stop at about 80ch', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1920, height: 1080 })
  await page.goto('/en/about')
  const c = await box(page.locator('.about-page'))
  expect(c.w).toBeGreaterThan(1240)
  expect(c.w).toBeLessThanOrEqual(1536)
  for (const sel of ['.page-head > p', '#start > p', '.about-steps p']) {
    const p = await box(page.locator(sel).first())
    expect(p.w).toBeLessThanOrEqual(900)
  }
  // Below the cap it tracks the viewport.
  await page.setViewportSize({ width: 1280, height: 800 })
  const c2 = await box(page.locator('.about-page'))
  expect(c2.w / 1280).toBeGreaterThan(0.9)
  await page.close()
})

test('station cards: three in a row at 1280, one column at 390, one link each', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1280, height: 800 })
  await page.goto('/en/about')
  const cards = page.locator('#station .about-card')
  await expect(cards).toHaveCount(3)
  const wide = [await box(cards.nth(0)), await box(cards.nth(1)), await box(cards.nth(2))]
  expect(Math.abs(wide[0].y - wide[2].y)).toBeLessThan(2)
  expect(wide[1].x).toBeGreaterThan(wide[0].x + wide[0].w - 1)
  expect(wide[2].x).toBeGreaterThan(wide[1].x + wide[1].w - 1)
  for (let i = 0; i < 3; i++) await expect(cards.nth(i).locator('a')).toHaveCount(1)
  await expect(cards.nth(0).locator('a')).toHaveAttribute('href', 'https://airbg.info/naprawi-si-stancia')
  await expect(cards.nth(1).locator('a')).toHaveAttribute('href', 'https://airbg.info/zaqwi-uchastie')
  const third = cards.nth(2).locator('a')
  await expect(third).toHaveAttribute('href', 'https://maps.sensor.community')
  await expect(third).toHaveAttribute('rel', 'noopener noreferrer')
  await expect(cards.nth(2)).toContainText('sensor.community')
  await expect(cards.nth(0).locator('svg[aria-hidden="true"]')).toHaveCount(1)
  await page.setViewportSize({ width: 390, height: 844 })
  const narrow = [await box(cards.nth(0)), await box(cards.nth(1)), await box(cards.nth(2))]
  expect(narrow[1].y).toBeGreaterThan(narrow[0].y + narrow[0].h - 1)
  expect(narrow[2].y).toBeGreaterThan(narrow[1].y + narrow[1].h - 1)
  expect(Math.abs(narrow[0].x - narrow[2].x)).toBeLessThan(2)
  await page.close()
})

test('a station card link shows a focus ring', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1280, height: 800 })
  await page.goto('/en/about')
  const link = page.locator('#station .about-card a').first()
  await link.focus()
  const ring = await link.evaluate((a) => getComputedStyle(a, '::after').boxShadow)
  expect(ring).not.toBe('none')
  // The stretched ::after covers the card, so the ring outlines the whole card.
  const card = await box(page.locator('#station .about-card').first())
  const after = await link.evaluate((a) => { const s = getComputedStyle(a, '::after'); return { w: parseFloat(s.width), h: parseFloat(s.height) } })
  expect(after.w).toBeGreaterThan(card.w - 4)
  expect(after.h).toBeGreaterThan(card.h - 4)
  await page.close()
})

for (const [path, gone] of [['/en/about', 'Their map'], ['/about', 'Тяхната карта']]) {
  test(`${path} no longer says "${gone}"`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.goto(path)
    expect(await page.locator('body').innerText()).not.toContain(gone)
    await page.close()
  })
}
