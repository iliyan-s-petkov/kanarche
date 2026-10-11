import { createRequire } from 'node:module'
import { test, expect, mapSettled } from './fixtures.js'

// WCAG 1.4.10 Reflow at 320px and a unique name per landmark role (axe
// landmark-unique). Findings from the OpenProject #662 audit.
const axeSource = createRequire(import.meta.url).resolve('axe-core/axe.min.js')

const PAGES = [
  { path: '/', map: true },
  { path: '/en/', map: true },
  { path: '/area/sofia', map: true },
  { path: '/en/area/sofia', map: true },
  { path: '/about', map: false },
  { path: '/en/about', map: false },
  { path: '/about-the-data', map: false },
  { path: '/privacy', map: false },
  { path: '/licences', map: false },
  { path: '/terms', map: false },
]

// bypassCSP: the site's script-src 'self' blocks the inline script that injects axe.
const open = async (browser, path, map, width) => {
  const context = await browser.newContext({ bypassCSP: true, viewport: { width, height: 640 } })
  const page = await context.newPage()
  await page.goto(path)
  if (map) await mapSettled(page)
  return { context, page }
}

// Elements past the right edge, ignoring the map canvas and anything inside a
// scroll container of its own (code blocks, tables with overflow).
const overflowing = (page) => page.evaluate(() => {
  const vw = document.documentElement.clientWidth
  const clipped = (el) => {
    for (let p = el.parentElement; p && p !== document.body; p = p.parentElement) {
      if (/(auto|scroll|hidden|clip)/.test(getComputedStyle(p).overflowX)) return true
    }
    return false
  }
  const bad = []
  for (const el of document.body.querySelectorAll('*')) {
    if (el.closest('[data-island="map"], .maplibregl-map, canvas')) continue
    const r = el.getBoundingClientRect()
    if (r.width === 0 || r.right <= vw + 0.5 || clipped(el)) continue
    bad.push(`${el.tagName.toLowerCase()}.${String(el.className).slice(0, 40)} right=${Math.round(r.right)}`)
  }
  return { scrollWidth: document.documentElement.scrollWidth, vw, bad: bad.slice(0, 8) }
})

for (const { path, map } of PAGES) {
  test(`no horizontal overflow at 320px: ${path}`, async ({ browser }) => {
    const { context, page } = await open(browser, path, map, 320)
    try {
      const r = await overflowing(page)
      expect(r.scrollWidth, JSON.stringify(r.bad)).toBeLessThanOrEqual(r.vw)
      expect(r.bad).toEqual([])
    } finally {
      await context.close()
    }
  })

  test(`landmarks are uniquely named per role: ${path}`, async ({ browser }) => {
    const { context, page } = await open(browser, path, map, 1280)
    try {
      await page.addScriptTag({ path: axeSource })
      const found = await page.evaluate(async () => {
        const r = await window.axe.run(document, { runOnly: { type: 'rule', values: ['landmark-unique'] } })
        return r.violations.flatMap((v) => v.nodes.map((n) => {
          const twins = n.any.flatMap((a) => a.relatedNodes.map((r) => r.html.slice(0, 120)))
          return `${n.target.join(' ')} shares its name with ${twins.join(' | ')}`
        }))
      })
      expect(found).toEqual([])
    } finally {
      await context.close()
    }
  })
}
