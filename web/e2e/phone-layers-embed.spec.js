import { test, expect } from './fixtures.js'

// #652: at 390x844 a long disabled-reason option ran past the panel edge, and /embed
// showed a squeezed map with the attribution open.
const PHONE = { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true }

const langPrefix = (lang) => (lang === 'en' ? '/en' : '')

for (const lang of ['bg', 'en']) {
  for (const theme of ['light', 'dark']) {
    test(`layers panel options stay inside panel and viewport (${lang}, ${theme})`, async ({ browser }) => {
      const context = await browser.newContext(PHONE)
      const page = await context.newPage()
      await page.addInitScript((v) => localStorage.setItem('kanarche:theme', v), theme)
      await page.goto(`${langPrefix(lang)}/`)
      await page.locator('.map__layers .colmenu__btn').click()
      const panel = page.locator('.map__layers .colmenu__panel')
      await expect(panel).toBeVisible()

      if (process.env.KANARCHE_SHOT_DIR) await page.screenshot({ path: `${process.env.KANARCHE_SHOT_DIR}/phone-${lang}-${theme}-layers.png` })
      const pb = await panel.boundingBox()
      expect(pb.x, 'panel left edge').toBeGreaterThanOrEqual(0)
      expect(pb.x + pb.width, 'panel right edge').toBeLessThanOrEqual(390)

      const opts = page.locator('.map__layers .colmenu__panel .colmenu__opt')
      const n = await opts.count()
      expect(n).toBeGreaterThan(0)
      for (let i = 0; i < n; i++) {
        const b = await opts.nth(i).boundingBox()
        // Sub-pixel slack only; a real overflow is tens of pixels.
        expect(b.x, `option ${i} left`).toBeGreaterThanOrEqual(pb.x - 1)
        expect(b.x + b.width, `option ${i} right vs panel`).toBeLessThanOrEqual(pb.x + pb.width + 1)
        expect(b.x + b.width, `option ${i} right vs viewport`).toBeLessThanOrEqual(390)
        // The label text itself, not just the row box, must fit.
        const over = await opts.nth(i).evaluate((el) => {
          const r = document.createRange()
          r.selectNodeContents(el)
          const p = document.querySelector('.map__layers .colmenu__panel').getBoundingClientRect()
          return r.getBoundingClientRect().right - p.right
        })
        expect(over, `option ${i} text overflow`).toBeLessThanOrEqual(1)
      }
      await context.close()
    })
  }
}

const overlap = (a, b) => a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height

for (const vp of [{ width: 390, height: 844, shot: 'phone-embed.png' }, { width: 320, height: 568, shot: 'phone-embed-320.png' }]) {
  test(`embed legend and footline do not overlap at ${vp.width}x${vp.height}`, async ({ browser }) => {
    const context = await browser.newContext({ ...PHONE, viewport: { width: vp.width, height: vp.height } })
    const page = await context.newPage()
    await page.goto('/embed')
    const legend = page.locator('.embed .scale--onmap')
    const foot = page.locator('.embed__footline')
    await expect(legend).toBeVisible()
    await expect(foot).toBeVisible()
    await page.waitForTimeout(500)
    if (process.env.KANARCHE_SHOT_DIR) await page.screenshot({ path: `${process.env.KANARCHE_SHOT_DIR}/${vp.shot}` })
    const l = await legend.boundingBox()
    const f = await foot.boundingBox()
    expect(overlap(l, f), `legend ${JSON.stringify(l)} vs footline ${JSON.stringify(f)}`).toBe(false)
    for (const b of [l, f]) {
      expect(b.x).toBeGreaterThanOrEqual(0)
      expect(b.y).toBeGreaterThanOrEqual(0)
      expect(b.x + b.width).toBeLessThanOrEqual(vp.width)
      expect(b.y + b.height).toBeLessThanOrEqual(vp.height)
    }
    await context.close()
  })
}

test('embed map fills the viewport and attribution starts collapsed on a phone', async ({ browser }) => {
  const context = await browser.newContext(PHONE)
  const page = await context.newPage()
  await page.goto('/embed')
  const map = page.locator('.embed .map')
  await expect(map).toBeVisible()
  await expect(page.locator('.maplibregl-ctrl-attrib')).toBeVisible()
  await expect(page.locator('.maplibregl-canvas')).toBeVisible()
  await page.waitForTimeout(500)
  const h = (await map.boundingBox()).height
  expect(h, 'embed map height').toBeGreaterThanOrEqual(844 - 1)
  await expect(page.locator('.maplibregl-ctrl-attrib')).not.toHaveClass(/maplibregl-compact-show/)
  await expect(page.locator('.maplibregl-ctrl-attrib-inner')).toBeHidden()
  await context.close()
})
