import { test, expect, mapSettled } from './fixtures.js'

// Every visible control on a touch device reaches a 44x44 CSS px hit area.
// Method: a control passes when its own box is 44x44, or when all four corners
// and the edge midpoints of a 44x44 square centred on it resolve (elementFromPoint)
// to the control or a node inside it. That counts ::before/::after hit extenders
// and fails when a neighbour sits in the way.
// Allowlist: inline text links inside a prose block (WCAG 2.5.8 inline exception),
// meaning an anchor whose p, li, dd, td or figcaption holds other text besides it.
// Also allowlisted: the links in the map's credit line (.maplibregl-ctrl-attrib-inner), a run of
// text sized by its line height that only shows while the (i) is open.

const VIEWPORTS = [[360, 640], [390, 844], [844, 390]]
const PAGES = ['/', '/area/sofia', '/about', '/en']
const MIN = 44

const offenders = (min) => {
  const sel = 'a[href], button, [role=button], summary, input:not([type=hidden]), select, textarea, [role=tab], [role=switch], [role=checkbox], [role=radio]'
  const out = []
  for (const el of document.querySelectorAll(sel)) {
    if (!el.checkVisibility({ visibilityProperty: true, opacityProperty: true })) continue
    const r0 = el.getBoundingClientRect()
    // Visually hidden helpers and off-canvas skip links are not targets.
    if (r0.width <= 2 || r0.height <= 2) continue
    if (r0.right < 0 || r0.left > document.documentElement.scrollWidth) continue
    if (el.closest('[inert], [aria-hidden=true], .maplibregl-ctrl-attrib-inner')) continue
    if (el.tagName === 'A') {
      const block = el.closest('p, li, dd, td, figcaption')
      if (block && block.textContent.trim().length > el.textContent.trim().length + 2) continue
    }
    el.scrollIntoView({ block: 'center', inline: 'center' })
    const r = el.getBoundingClientRect()
    let ok = r.width >= min - 0.5 && r.height >= min - 0.5
    let blocker = ''
    // Try 44px squares centred sideways, growing the box up or down by any share of the shortfall.
    // Samples sit 2px inside the square so a rounded corner of the control itself is not a miss.
    const cx = (r.left + r.right) / 2
    const gapY = Math.max(0, min - r.height)
    for (let up = 0; !ok && up <= gapY + 0.5; up += 2) {
      const t = r.height >= min ? (r.top + r.bottom) / 2 - min / 2 : r.top - up
      let bad = ''
      for (const dx of [-min / 2 + 2, 0, min / 2 - 2]) {
        for (const dy of [2, min / 2, min - 2]) {
          const x = Math.min(Math.max(cx + dx, 0), innerWidth - 1)
          const y = Math.min(Math.max(t + dy, 0), innerHeight - 1)
          const hit = document.elementFromPoint(x, y)
          if (!(hit && (hit === el || el.contains(hit))) && !bad) bad = hit ? `${hit.tagName.toLowerCase()}.${hit.className}` : 'none'
        }
      }
      if (!bad) ok = true
      else blocker = bad
    }
    if (!ok) {
      const name = (el.getAttribute('aria-label') || el.textContent || el.id || '').trim().replace(/\s+/g, ' ').slice(0, 30)
      const tag = (n) => n.tagName.toLowerCase() + (typeof n.className === 'string' && n.className ? '.' + n.className.trim().split(/\s+/).join('.') : '')
      out.push(`${tag(el)} "${name}" ${Math.round(r.width)}x${Math.round(r.height)} in ${tag(el.parentElement)} < ${el.parentElement.parentElement ? tag(el.parentElement.parentElement) : ''}${blocker ? ` blocked by ${blocker}` : ''}`)
    }
  }
  return out
}

for (const [w, h] of VIEWPORTS) {
  for (const path of PAGES) {
    test(`${w}x${h} touch ${path}: every control has a 44x44 hit area`, async ({ browser }) => {
      const ctx = await browser.newContext({ viewport: { width: w, height: h }, isMobile: true, hasTouch: true })
      const page = await ctx.newPage()
      await page.goto(path)
      if (path === '/' || path.startsWith('/area/')) await mapSettled(page)
      await page.evaluate(() => document.fonts.ready)
      // Fold screenshots for review, only when asked for.
      if (process.env.KANARCHE_SHOT_DIR) await page.screenshot({ path: `${process.env.KANARCHE_SHOT_DIR}/${process.env.KANARCHE_SHOT_TAG}-${w}x${h}${path.replace(/\//g, '_')}.png` })
      expect(await page.evaluate(offenders, MIN)).toEqual([])
      await ctx.close()
    })
  }
}

// The 44px rail rule used to stop at 672px wide, leaving landscape phones at 40px.
for (const [w, h] of [[740, 360], [844, 390], [915, 412]]) {
  test(`${w}x${h} touch landscape /: the map rail buttons are 44px`, async ({ browser }) => {
    const ctx = await browser.newContext({ viewport: { width: w, height: h }, isMobile: true, hasTouch: true })
    const page = await ctx.newPage()
    await page.goto('/')
    await mapSettled(page)
    const small = await page.evaluate(() => [...document.querySelectorAll('.maplibregl-ctrl-group button, .map-rail :is(button, a):not(.map-locate-hint__btn)')]
      .filter((e) => e.checkVisibility())
      .map((e) => [e.className, e.getBoundingClientRect()])
      .filter(([, r]) => r.width < 43.5 || r.height < 43.5)
      .map(([c, r]) => `${c} ${Math.round(r.width)}x${Math.round(r.height)}`))
    expect(small).toEqual([])
    await ctx.close()
  })
}
