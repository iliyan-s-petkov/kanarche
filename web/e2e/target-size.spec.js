import { createRequire } from 'node:module'
import { test as base, expect, mapSettled } from './fixtures.js'

// WCAG 2.2 SC 2.5.8 through axe's target-size rule on the public pages, at a
// desktop and a touch phone viewport, in both themes. OpenProject #723.
const axeSource = createRequire(import.meta.url).resolve('axe-core/axe.min.js')

// bypassCSP: the site's script-src 'self' blocks the inline script that injects axe.
const test = base.extend({
  deskCtx: [async ({ browser }, use) => {
    const context = await browser.newContext({ bypassCSP: true, viewport: { width: 1280, height: 800 } })
    await use(context)
    await context.close()
  }, { scope: 'worker' }],
  phoneCtx: [async ({ browser }, use) => {
    const context = await browser.newContext({
      bypassCSP: true, isMobile: true, hasTouch: true, deviceScaleFactor: 2, viewport: { width: 393, height: 873 },
    })
    await use(context)
    await context.close()
  }, { scope: 'worker' }],
})

test.afterEach(async ({ deskCtx, phoneCtx }) => {
  for (const c of [deskCtx, phoneCtx]) {
    for (const p of c.pages()) if (!p.isClosed()) await p.close().catch(() => {})
  }
})

const PAGES = [
  { path: '/', map: true },
  { path: '/en/', map: true },
  { path: '/about', map: false },
  { path: '/terms', map: false },
  { path: '/area/sofia', map: true },
]

// No real wind upstream: the folded wind note only shows once a forecast arrives.
const mockWind = (page) => page.route('**/api/v1/wind', (route) => route.fulfill({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify({
    generated_at: new Date().toISOString(),
    valid_at: new Date().toISOString(),
    model: 'Test Model',
    model_resolution_deg: 0.25,
    resolution_km: 25,
    forecast: true,
    vectors: [{ lon: 23.3, lat: 42.68, speed_ms: 3.2, direction_deg: 180 }],
  }),
}))

const violations = (page) => page.evaluate(async () => {
  const r = await window.axe.run(document, { runOnly: { type: 'rule', values: ['target-size'] } })
  return r.violations.flatMap((v) => v.nodes.map((n) => `${n.target.join(' ')} :: ${n.failureSummary}`))
})

for (const viewport of ['desktop', 'phone']) {
  for (const theme of ['light', 'dark']) {
    for (const { path, map } of PAGES) {
      test(`axe target-size: ${path} on ${viewport} in ${theme}`, async ({ deskCtx, phoneCtx }) => {
        const page = await (viewport === 'phone' ? phoneCtx : deskCtx).newPage()
        await page.addInitScript((v) => localStorage.setItem('kanarche:theme', v), theme)
        await mockWind(page)
        await page.goto(path)
        if (map) await mapSettled(page)
        await page.addScriptTag({ path: axeSource })
        expect(await violations(page)).toEqual([])
        await page.close()
      })
    }
  }
}
