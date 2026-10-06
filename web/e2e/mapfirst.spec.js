import { test, expect, mapSettled } from './fixtures.js'

// Map-first home: no hero or toolbar above the map; its controls ride on it.
const DESKTOP = { width: 1440, height: 900 }
const PHONE = { width: 393, height: 873 }

// Every spec measures what the map island builds or shifts (pills, finder, cue),
// so each waits for the island and its opening camera before reading geometry.
async function visit(ctx, vp, path = '/en/') {
  const page = await ctx.newPage()
  await page.setViewportSize(vp)
  await page.goto(path)
  await mapSettled(page)
  return page
}

const mastheadBottom = (page) => page.locator('.masthead').evaluate((el) => el.getBoundingClientRect().bottom)

test('desktop 1440x900: the map starts under the masthead and fills the viewport', async ({ ctx }) => {
  const page = await visit(ctx, DESKTOP, '/en/')
  await expect.poll(async () => {
    const map = await page.locator('#map').boundingBox()
    return Math.abs(map.y - await mastheadBottom(page))
  }).toBeLessThanOrEqual(8)
  await expect.poll(async () => {
    const map = await page.locator('#map').boundingBox()
    return Math.abs(map.y + map.height - DESKTOP.height)
  }).toBeLessThanOrEqual(2)
  await page.close()
})

for (const [name, vp] of [['1440', DESKTOP], ['390', { width: 390, height: 844 }]]) {
  test(`${name}px: brand reads "kanarche" and keeps "Kanarche" as its accessible name`, async ({ ctx }) => {
    const page = await visit(ctx, vp, '/en/')
    const brand = page.locator('header.masthead .masthead__brand')
    await expect(brand).toHaveText('kanarche', { useInnerText: true })
    await expect(brand).toHaveAttribute('aria-label', 'Kanarche')
    await page.close()
  })
}

// A masthead wider than the screen makes a phone zoom the whole page out.
for (const path of ['/', '/en/']) {
  test(`360px ${path}: the masthead fits the screen`, async ({ ctx }) => {
    const page = await visit(ctx, { width: 360, height: 740 }, path)
    await expect(page.locator('[data-island="theme"]')).toBeVisible()
    await expect.poll(() => page.evaluate(() => [document.querySelector('header.masthead').scrollWidth, document.documentElement.scrollWidth])).toEqual([360, 360])
    await page.close()
  })
}

for (const [name, vp] of [['desktop', DESKTOP], ['phone', PHONE]]) {
  test(`${name}: no .toolbar above the map, one hidden h1, no visible hero`, async ({ ctx }) => {
    const page = await visit(ctx, vp, '/en/')
    await expect(page.locator('.toolbar')).toHaveCount(0)
    await expect(page.locator('.page-head')).toHaveCount(0)
    const h1 = page.locator('h1')
    await expect(h1).toHaveCount(1)
    await expect(h1).toHaveText('Kanarche, Bulgaria air quality map: PM2.5 and PM10 now')
    await expect.poll(async () => {
      const box = await h1.boundingBox()
      return box.width * box.height
    }).toBeLessThanOrEqual(1)
    await page.close()
  })

  test(`${name}: metric pill is exactly the metric label, on the map top-left half`, async ({ ctx }) => {
    const page = await visit(ctx, vp, '/en/')
    const pill = page.locator('#metric-menu')
    await expect(pill).toBeVisible()
    const metric = await page.locator('#map').getAttribute('data-metric')
    const metrics = (await page.locator('#map').getAttribute('data-metrics')).split(',').map((s) => s.trim())
    const labels = (await page.locator('#map').getAttribute('data-metric-labels')).split(',').map((s) => s.trim())
    const label = labels[metrics.indexOf(metric)]
    await expect(pill).toHaveText(label)
    await expect(pill).toHaveAttribute('aria-label', new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
    // One poll for the whole geometry: a reflow between separate reads would mix two layouts.
    await expect.poll(async () => {
      const map = await page.locator('#map').boundingBox()
      const box = await pill.boundingBox()
      return box.x >= map.x && box.y >= map.y && box.y + box.height <= map.y + 64 &&
        box.x + box.width < map.x + map.width / 2 && (vp !== PHONE || box.height >= 44)
    }).toBe(true)
    await page.close()
  })

  test(`${name}: finder is top-right on the map and clear of the other controls`, async ({ ctx }) => {
    const page = await visit(ctx, vp, '/en/')
    const input = page.locator('[data-island="finder"] input')
    await expect(input).toBeVisible()
    const hit = (a, b) => a.x < b.x + b.width && a.x + a.width > b.x && a.y < b.y + b.height && a.y + a.height > b.y
    // One poll for the whole geometry: a reflow between separate reads would mix two layouts.
    await expect.poll(async () => {
      const map = await page.locator('#map').boundingBox()
      const f = await input.boundingBox()
      if (!(f.y >= map.y && f.y + f.height <= map.y + 64 && f.x + f.width <= map.x + map.width)) return 'outside'
      if (vp === PHONE && f.height < 44) return 'short'
      for (const sel of ['.map__full', '.map__layers', '#metric-menu']) {
        const o = await page.locator(sel).first().boundingBox()
        if (hit(f, o)) return sel
      }
      return 'ok'
    }).toBe('ok')
    await page.close()
  })
}

test('the manual refresh sits in the freshness line beside the timer and still reloads', async ({ ctx }) => {
  const page = await visit(ctx, DESKTOP, '/en/')
  const line = page.locator('.map-freshness')
  const refresh = line.locator('.data-refresh__btn--icon')
  await expect(refresh).toBeVisible()
  await expect(line.locator('.data-refresh__auto')).toBeVisible()
  const reload = page.waitForResponse((r) => r.url().includes('/api/v1/') && r.request().method() === 'GET')
  await refresh.click()
  await reload
  await page.close()
})

test('desktop: a scroll cue sits inside the map bottom edge and reaches the content below', async ({ ctx }) => {
  const page = await visit(ctx, DESKTOP, '/en/')
  const cue = page.locator('a.scroll-cue')
  await expect(cue).toBeVisible()
  await expect.poll(async () => {
    const map = await page.locator('#map').boundingBox()
    const box = await cue.boundingBox()
    return box.y >= map.y && box.y + box.height <= map.y + map.height + 1 && box.y + box.height <= DESKTOP.height
  }).toBe(true)
  await cue.click()
  await expect.poll(() => page.evaluate(() => Math.abs(document.getElementById('below-map').getBoundingClientRect().top) <= 80 || scrollY > 0)).toBe(true)
  await page.close()
})
