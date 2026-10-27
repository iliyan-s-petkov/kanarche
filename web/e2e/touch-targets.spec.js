import { test, expect, mapSettled } from './fixtures.js'

// Every visible control on a touch device reaches a 44x44 CSS px hit area.
// Method: a control passes when its own box is 44x44, or when all four corners
// and the edge midpoints of a 44x44 square centred on it resolve (elementFromPoint)
// to the control or a node inside it. That counts ::before/::after hit extenders
// and fails when a neighbour sits in the way.
// Allowlist: inline text links inside a prose block (WCAG 2.5.8 inline exception),
// meaning an anchor whose p, li, dd, td or figcaption holds other text besides it.
// A checkbox or radio inside a label is measured as its label (the label carries the tap).
// Also allowlisted: the links in the map's credit line (.maplibregl-ctrl-attrib-inner), a run of
// text sized by its line height that only shows while the (i) is open.

const VIEWPORTS = [[360, 640], [390, 844], [844, 390]]
const PAGES = ['/', '/area/sofia', '/about', '/en']
const MIN = 44

const offenders = ({ min, root }) => {
  const sel = 'a[href], button, [role=button], summary, input:not([type=hidden]), select, textarea, [role=tab], [role=switch], [role=checkbox], [role=radio]'
  const out = []
  // With a root, only the controls inside it (or the root itself) are swept.
  const scope = root ? [...document.querySelectorAll(root)] : [document]
  for (const node of scope.flatMap((n) => [...(n.matches?.(sel) ? [n] : []), ...n.querySelectorAll(sel)])) {
    let el = node
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
    // A checkbox or radio inside its label is tapped through the label: the label is the target.
    if ((el.type === 'checkbox' || el.type === 'radio') && el.closest('label')) el = el.closest('label')
    el.scrollIntoView({ block: 'center', inline: 'center' })
    const r = el.getBoundingClientRect()
    let ok = r.width >= min - 0.5 && r.height >= min - 0.5
    let blocker = ''
    // In the open states, a box of the right size is no use under another layer: its centre and inset quarter points must hit it.
    if (ok && root) {
      for (const [fx, fy] of [[0.5, 0.5], [0.25, 0.25], [0.75, 0.25], [0.25, 0.75], [0.75, 0.75]]) {
        const x = Math.min(Math.max(r.left + r.width * fx, 0), innerWidth - 1)
        const y = Math.min(Math.max(r.top + r.height * fy, 0), innerHeight - 1)
        const hit = document.elementFromPoint(x, y)
        if (!(hit && (hit === el || el.contains(hit)))) {
          const name = (el.getAttribute('aria-label') || el.textContent || el.id || '').trim().replace(/\s+/g, ' ').slice(0, 30)
          out.push(`${el.tagName.toLowerCase()}.${el.className} "${name}" ${Math.round(r.width)}x${Math.round(r.height)} covered by ${hit ? hit.tagName.toLowerCase() + '.' + hit.className : 'none'} at ${Math.round(x)},${Math.round(y)}`)
          break
        }
      }
    }
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
          if (!(hit && (hit === el || el.contains(hit))) && !bad) bad = `${hit ? hit.tagName.toLowerCase() + "." + hit.className : "none"} at ${Math.round(x)},${Math.round(y)}, control at ${Math.round(r.left)},${Math.round(r.top)}`
        }
      }
      if (!bad) ok = true
      else blocker += ` [up ${up}: ${bad}]`
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
      expect(await page.evaluate(offenders, { min: MIN })).toEqual([])
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

// Open states: a menu, sheet or key that is closed at rest has its own controls.
// Each case opens one and sweeps only the controls inside its root, so a control
// the open surface covers elsewhere on the page is not reported.
const SEA_SITE = [23.45, 42.75]

const OPEN_STATES = [
  { name: 'layers menu', path: '/', root: '.map__layers', open: async (page) => { await page.locator('.map__layers .colmenu__btn').click() } },
  { name: 'language dropdown', path: '/', root: 'details.langpick[open]', open: async (page) => { await page.locator('.langpick__btn').first().click() } },
  { name: 'theme menu', path: '/', root: 'details.langpick[data-island="theme"][open]', open: async (page) => { await page.locator('details.langpick[data-island="theme"] > summary').click() } },
  { name: 'legend expanded', path: '/', root: '.scale--onmap', open: async (page) => {
    const legend = page.locator('.scale--onmap')
    if ((await legend.getAttribute('open')) === null) await legend.locator(':scope > .scale__toggle').click()
  } },
  { name: 'metric picker', path: '/area/sofia', root: '.colmenu:has(#metric-menu)', open: async (page) => { await page.locator('#metric-menu').click() } },
  { name: 'status picker', path: '/area/sofia', root: '.colmenu:has(#sensor-status-menu)', open: async (page) => { await page.locator('#sensor-status-menu').click() }, narrowOnly: true },
  { name: 'sensor panel', path: '/en/area/sofia#sensor=101', root: '.sensor-panel', open: async (page) => { await page.locator('.sensor-panel .chart-frame').waitFor() } },
  { name: 'sensor panel menu', path: '/en/area/sofia#sensor=101', root: '.panel-morewrap', narrowOnly: true, open: async (page) => {
    await page.locator('.sensor-panel .chart-frame').waitFor()
    await page.locator('.sensor-panel .panel-more').click()
  } },
  { name: 'sea panel', path: '/en/', root: '.map-sea', open: async (page) => {
    await page.evaluate((c) => {
      document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
      document.querySelector('[data-island="map"]').__map.jumpTo({ center: c, zoom: 10 })
    }, SEA_SITE)
    await mapSettled(page)
    await page.locator('.map__layers .colmenu__btn').click()
    await page.locator('.map__layers').getByLabel('Bathing water', { exact: true }).check()
    await page.keyboard.press('Escape')
    const at = () => page.evaluate((c) => {
      const map = document.querySelector('[data-island="map"]').__map
      if (!map.queryRenderedFeatures({ layers: ['sea-sites'] }).length) return null
      const box = map.getCanvas().getBoundingClientRect()
      const p = map.project(c)
      return { x: box.left + p.x, y: box.top + p.y }
    }, SEA_SITE)
    await expect.poll(at, { timeout: 20_000 }).not.toBeNull()
    const pt = await at()
    await page.mouse.click(pt.x, pt.y)
    await page.locator('.map-sea .sea-panel').waitFor()
  } },
]

for (const [w, h] of VIEWPORTS) {
  for (const st of OPEN_STATES) {
    // The status picker is a menu button only on phone widths; wider it is the pills the resting sweep covers.
    if (st.narrowOnly && w > 672) continue
    test(`${w}x${h} touch ${st.path} ${st.name} open: every control has a 44x44 hit area`, async ({ browser }, testInfo) => {
      testInfo.setTimeout(90_000)
      const ctx = await browser.newContext({ viewport: { width: w, height: h }, isMobile: true, hasTouch: true })
      const page = await ctx.newPage()
      await page.goto(st.path)
      if (!st.path.includes('#')) await mapSettled(page)
      await page.evaluate(() => document.fonts.ready)
      await st.open(page)
      await page.waitForTimeout(400)
      if (process.env.KANARCHE_SHOT_DIR) await page.screenshot({ path: `${process.env.KANARCHE_SHOT_DIR}/${process.env.KANARCHE_SHOT_TAG}-${w}x${h}-${st.name.replace(/ /g, '_')}.png` })
      // An empty sweep proves nothing: the root must exist.
      expect(await page.locator(st.root).count(), `${st.root} not found`).toBeGreaterThan(0)
      expect(await page.evaluate(offenders, { min: MIN, root: st.root })).toEqual([])
      await ctx.close()
    })
  }
}

// The sensor bar sits in the 48px header row of a landscape phone: every pill is whole inside it.
test('844x390 touch /area/sofia landscape: no sensor-bar pill is clipped', async ({ browser }) => {
  const ctx = await browser.newContext({ viewport: { width: 844, height: 390 }, isMobile: true, hasTouch: true })
  const page = await ctx.newPage()
  await page.goto('/area/sofia')
  await mapSettled(page)
  await page.evaluate(() => document.fonts.ready)
  const res = await page.evaluate(() => {
    const host = document.querySelector('.toolbar--map [data-island="sensorbar"]')
    const h = host.getBoundingClientRect()
    const pills = [...host.querySelectorAll('.switcher__opt')].map((p) => p.getBoundingClientRect())
    return { pills: pills.length, scrollOver: host.scrollWidth - host.clientWidth, outside: pills.filter((p) => p.left < h.left - 0.5 || p.right > h.right + 0.5 || p.right > innerWidth).length }
  })
  expect(res.pills).toBeGreaterThan(0)
  expect(res.scrollOver).toBeLessThanOrEqual(0)
  expect(res.outside).toBe(0)
  await ctx.close()
})

// Portrait trail: it ends in an ellipsis inside the viewport (like landscape), and the
// downward hit extender of its links leaves every other control tappable.
for (const [w, h] of [[360, 640], [390, 844]]) {
  test(`${w}x${h} touch /area/sofia portrait: breadcrumb ends in an ellipsis and covers no control`, async ({ browser }) => {
    const ctx = await browser.newContext({ viewport: { width: w, height: h }, isMobile: true, hasTouch: true })
    const page = await ctx.newPage()
    await page.goto('/area/sofia')
    await mapSettled(page)
    await page.evaluate(() => document.fonts.ready)
    const res = await page.evaluate(() => {
      const nav = document.querySelector('nav.breadcrumb')
      const ol = nav.querySelector('ol')
      const cs = getComputedStyle(ol)
      const r = ol.getBoundingClientRect()
      // Controls outside the trail that sit within 30px below it must still win their own centre.
      const covered = [...document.querySelectorAll('a[href], button, summary, input, select')]
        .filter((e) => !nav.contains(e) && e.checkVisibility())
        .filter((e) => { const b = e.getBoundingClientRect(); return b.top < r.bottom + 30 && b.bottom > r.bottom && b.right > 0 && b.left < innerWidth })
        .filter((e) => { const b = e.getBoundingClientRect(); const hit = document.elementFromPoint(b.left + b.width / 2, Math.max(b.top, r.bottom + 1) + 1); return !(hit && (hit === e || e.contains(hit))) })
        .map((e) => `${e.tagName.toLowerCase()}.${e.className}`)
      return { overflow: cs.overflowX, ellipsis: cs.textOverflow, right: r.right, covered, vw: innerWidth }
    })
    expect(res.covered).toEqual([])
    expect(res.right).toBeLessThanOrEqual(res.vw)
    expect(res.overflow).toBe('hidden')
    expect(res.ellipsis).toBe('ellipsis')
    await ctx.close()
  })
}
