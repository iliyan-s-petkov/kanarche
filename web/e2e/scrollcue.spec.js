import { test, expect, mapSettled } from './fixtures.js'

// The pull tab (OpenProject #574, plan Task 2, spike option C): a 56x28 tab
// hanging from the map's bottom edge, 64x44 hit area via ::before.
const VIEWPORTS = [
  { name: '393x873', width: 393, height: 873 },
  { name: '360x740', width: 360, height: 740 },
  { name: '873x393', width: 873, height: 393 },
  { name: '740x360', width: 740, height: 360 },
]

const PAGES = ['/', '/area/sofia']

// sofia is a city (boundary note, pollen chip); sofia-oblast has no note; mladost has the longest h1.
const FOLD_PAGES = ['/', '/area/sofia', '/area/sofia-oblast', '/area/mladost']
const FOLD_VIEWPORTS = [
  { name: '393x873', width: 393, height: 873 },
  { name: '873x393', width: 873, height: 393 },
  { name: '360x640', width: 360, height: 640 },
  { name: '375x667', width: 375, height: 667 },
  { name: '390x844', width: 390, height: 844 },
  { name: '412x915', width: 412, height: 915 },
]

const phoneCtx = (browser, vp) =>
  browser.newContext({ viewport: { width: vp.width, height: vp.height }, isMobile: true, hasTouch: true })

const overlaps = (a, b) =>
  a.x < b.x + b.width && a.x + a.width > b.x &&
  a.y < b.y + b.height && a.y + a.height > b.y

// The real tap target: ::before is out of DOM, so read its computed inset
// off the pseudo-element rather than assume the spike's numbers.
async function tapBox(cue) {
  return cue.evaluate((el) => {
    const r = el.getBoundingClientRect()
    const cs = getComputedStyle(el, '::before')
    const top = parseFloat(cs.top) || 0
    const right = parseFloat(cs.right) || 0
    const bottom = parseFloat(cs.bottom) || 0
    const left = parseFloat(cs.left) || 0
    return { x: r.x + left, y: r.y + top, width: r.width - left - right, height: r.height - top - bottom }
  })
}

// The key starts folded; put it in the requested state.
async function setLegend(page, open) {
  const legend = page.locator('details.scale--onmap')
  await expect(legend).toBeAttached()
  // Re-checked on every attempt: a repaint can replace the legend between the read and the click.
  await expect(async () => {
    if ((await legend.evaluate((el) => el.open)) !== open) await page.locator('.scale__toggle').click()
    if (open) await expect(legend).toHaveAttribute('open', '', { timeout: 1500 })
    else await expect(legend).not.toHaveAttribute('open', '', { timeout: 1500 })
  }).toPass({ timeout: 15_000 })
}

for (const vp of VIEWPORTS) {
  for (const path of PAGES) {
    for (const legendState of ['folded', 'open']) {
      test(`${vp.name} ${path} legend ${legendState}: pull tab sits on the map edge, clear of controls`, async ({ browser }) => {
        const ctx = await phoneCtx(browser, vp)
        const page = await ctx.newPage()
        await page.goto(path)
        await mapSettled(page)
        // Area chrome varies above the map; the guarantee is for the map at the
        // top, same convention the old strip spec used.
        if (path.includes('/area/')) {
          // The switcher and sensorbar hydrate above the map; scrolling first let them push it down (CI: 767.5 > 740).
          await expect(page.locator('[data-island="sensorbar"] .sensor-bar').first()).toBeVisible()
          await expect.poll(async () => {
            const top = () => page.evaluate(() => document.querySelector('.map-shell').getBoundingClientRect().top + scrollY)
            const a = await top()
            await page.waitForTimeout(200)
            return a === await top()
          }).toBe(true)
          await page.evaluate(() => document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' }))
        }
        await setLegend(page, legendState === 'open')

        const cue = page.locator('a.scroll-cue')
        await expect(cue).toBeVisible()
        // The map island mounts after FCP and can still shift the cue, so the whole
        // geometry is judged in one retried read rather than from separate snapshots.
        const mapEl = page.locator('#map, #area-map').first()
        await expect.poll(async () => {
          const box = await cue.boundingBox()
          const map = await mapEl.boundingBox()
          if (!box || !map) return 'unmeasured'
          // Straddles the map's bottom edge, fully inside the first viewport.
          if (Math.abs(box.y - (map.y + map.height)) > 2) return 'edge'
          if (box.y < 0 || box.y + box.height > vp.height) return 'viewport'
          // Visual tab is the spike's small pull tab, not the old 44px strip.
          if (box.width > 57 || box.height > 29) return 'size'
          // The tap target (::before) is still >= 44px tall.
          if ((await tapBox(cue)).height < 44) return 'tap'
          for (const sel of ['.scale--onmap', '.map-play', '.map-freshness', '.map-locate', '.scale__info']) {
            const target = page.locator(sel).first()
            // A control folded away has no box on screen; its stale layout rect is not an overlap.
            if (!(await target.isVisible())) continue
            const o = await target.boundingBox()
            if (o && overlaps(box, o)) return sel
          }
          return 'ok'
        }).toBe('ok')

        await ctx.close()
      })
    }
  }
}

test('the full 64x44 tap target is hit-testable, not just the visual tab', async ({ browser }) => {
  const vp = VIEWPORTS[0]
  const ctx = await phoneCtx(browser, vp)
  const page = await ctx.newPage()
  await page.goto('/')
  await mapSettled(page)
  await setLegend(page, false)
  const cue = page.locator('a.scroll-cue')
  await expect(cue).toBeVisible()
  // Box and hit tests are redone together on each attempt, so a late reflow
  // cannot leave the points computed from one layout and tested against another.
  await expect.poll(() => page.evaluate(() => {
    const cueEl = document.querySelector('a.scroll-cue')
    const box = cueEl.getBoundingClientRect()
    const cx = box.x + box.width / 2
    const cy = box.y + box.height / 2
    const points = {
      '10px above top edge': { x: cx, y: box.y - 10 },
      '10px below bottom edge': { x: cx, y: box.y + box.height + 10 },
      '2px inside left extension': { x: box.x - 2, y: cy },
      '2px inside right extension': { x: box.x + box.width + 2, y: cy },
    }
    const missed = []
    for (const [label, p] of Object.entries(points)) {
      const el = document.elementFromPoint(p.x, p.y)
      if (!(el && (el === cueEl || cueEl.contains(el)))) missed.push(label)
    }
    return missed
  })).toEqual([])

  await ctx.close()
})

for (const vp of FOLD_VIEWPORTS) {
  for (const path of FOLD_PAGES) {
    test(`${vp.name} ${path}: the pull tab's bottom stays in the first viewport`, async ({ browser }) => {
      const ctx = await phoneCtx(browser, vp)
      const page = await ctx.newPage()
      await page.goto(path)
      await mapSettled(page)
      const cue = page.locator('a.scroll-cue')
      await expect(cue).toBeVisible()
      // Polled: the chrome around the map reflows as islands hydrate, so one
      // measurement can catch the page mid-layout.
      await expect.poll(async () => {
        const box = await cue.boundingBox()
        return box.y + box.height <= vp.height
      }).toBe(true)
      // The area summary sits below the map on touch phones.
      if (path.includes('/area/')) {
        await expect.poll(async () => {
          const box = await cue.boundingBox()
          const summary = await page.locator('.area-summary').boundingBox()
          return summary.y > box.y
        }).toBe(true)
      }
      const portrait = vp.height > vp.width
      // Portrait: only breadcrumb, h1, chips and toolbar above the map; the scope note follows the map.
      if (path.includes('/area/') && portrait) {
        await expect.poll(() => page.evaluate(() => {
          const cue = document.querySelector('a.scroll-cue').getBoundingClientRect()
          const map = document.querySelector('.map--wide').getBoundingClientRect()
          const note = document.querySelector('.notice-inline')
          const above = ['.breadcrumb', '.page-head h1', '.toolbar--map']
            .every((s) => document.querySelector(s).getBoundingClientRect().bottom <= map.top + 1)
          return above && (!note || note.getBoundingClientRect().top >= cue.bottom)
        })).toBe(true)
        // The map fills the screen: under the tab, at most the line a one-line h1 leaves spare.
        await expect.poll(() => page.evaluate(() => {
          const h1 = document.querySelector('.page-head h1')
          const lh = parseFloat(getComputedStyle(h1).lineHeight)
          const spare = Math.round(h1.getBoundingClientRect().height / lh) >= 2 ? 0 : lh
          const unused = innerHeight - document.querySelector('a.scroll-cue').getBoundingClientRect().bottom
          return unused >= 0 && unused <= spare + 16
        })).toBe(true)
        // Same without the pollen line: the fixture area has pollen, prod areas mostly do not.
        const unused = await page.evaluate(() => {
          document.querySelector('.toolbar--map .pollen-chip')?.remove()
          const h1 = document.querySelector('.page-head h1')
          const lh = parseFloat(getComputedStyle(h1).lineHeight)
          const spare = Math.round(h1.getBoundingClientRect().height / lh) >= 2 ? 0 : lh
          return [innerHeight - document.querySelector('a.scroll-cue').getBoundingClientRect().bottom, spare]
        })
        expect(unused[0]).toBeGreaterThanOrEqual(0)
        expect(unused[0]).toBeLessThanOrEqual(unused[1] + 16)
      }
      // Worst case on the fixture: a two-line h1, the pollen line, no sensor bar, a no-coverage notice.
      if (path.includes('/area/')) {
        const { cueBottom, noticeTop, mapBottom, mapH } = await page.evaluate(() => {
          document.querySelector('[data-island="sensorbar"]')?.remove()
          const toolbar = document.querySelector('.frame > .toolbar')
          if (!toolbar.querySelector('.pollen-chip')) {
            const chip = document.createElement('a')
            chip.className = 'pollen-chip'
            chip.href = '#pollen'
            chip.textContent = 'Pollen'
            toolbar.append(chip)
          }
          const h1 = document.querySelector('.page-head h1')
          const lh = parseFloat(getComputedStyle(h1).lineHeight)
          while (h1.getBoundingClientRect().height < 1.5 * lh) h1.textContent += ' Long'
          const notice = document.createElement('div')
          notice.className = 'notice'
          notice.innerHTML = '<p><strong>No coverage</strong></p><p>Two lines of detail.</p>'
          toolbar.before(notice)
          const map = document.querySelector('.map--wide').getBoundingClientRect()
          return {
            cueBottom: document.querySelector('a.scroll-cue').getBoundingClientRect().bottom,
            noticeTop: notice.getBoundingClientRect().top, mapBottom: map.bottom, mapH: map.height,
          }
        })
        expect(cueBottom).toBeLessThanOrEqual(vp.height)
        expect(mapH).toBeGreaterThanOrEqual(portrait ? 240 : 190)
        // The notice moves below the map on touch phones rather than pushing it down.
        expect(noticeTop).toBeGreaterThanOrEqual(mapBottom)
      }
      await ctx.close()
    })
  }
}

test('the pull tab still scrolls to #below-map without changing the hash', async ({ browser }) => {
  const ctx = await phoneCtx(browser, VIEWPORTS[0])
  const page = await ctx.newPage()
  for (const path of PAGES) {
    await page.goto(path)
    await mapSettled(page)
    const hash = await page.evaluate(() => location.hash)
    await page.locator('a.scroll-cue').click()
    const strict = path.includes('/area/')
    await expect.poll(() => page.evaluate((strict) => {
      const top = Math.abs(document.getElementById('below-map').getBoundingClientRect().top)
      const atEnd = scrollY >= document.documentElement.scrollHeight - innerHeight - 1
      return top <= 8 || (!strict && atEnd && scrollY > 0)
    }, strict)).toBe(true)
    expect(await page.evaluate(() => location.hash)).toBe(hash)
  }
  await ctx.close()
})

test('the pull tab is hidden while the map is full screen', async ({ browser }) => {
  const ctx = await phoneCtx(browser, VIEWPORTS[0])
  const page = await ctx.newPage()
  await page.goto('/')
  await mapSettled(page)
  await expect(page.locator('a.scroll-cue')).toBeVisible()
  await page.locator('.map__full').click()
  await expect.poll(() => page.evaluate(() => {
    const map = document.querySelector('#map')
    return document.fullscreenElement === map || map.classList.contains('map--faux-full')
  })).toBe(true)
  await expect(page.locator('a.scroll-cue')).toBeHidden()
  await ctx.close()
})

test('the pull tab is shown on the 1280x800 desktop home map only, not on an area page', async ({ ctx }) => {
  const page = await ctx.newPage()
  await page.setViewportSize({ width: 1280, height: 800 })
  for (const path of PAGES) {
    await page.goto(path)
    await mapSettled(page)
    await expect(page.locator('#map, #area-map').first()).toBeVisible()
    if (path === '/') await expect(page.locator('a.scroll-cue')).toBeVisible()
    else await expect(page.locator('a.scroll-cue')).toBeHidden()
    // On desktop the area summary stays under the title, above the map.
    if (path.includes('/area/')) {
      await expect.poll(async () => {
        const summary = await page.locator('.area-summary').boundingBox()
        const map = await page.locator('#area-map').boundingBox()
        return summary.y + summary.height <= map.y
      }).toBe(true)
    }
  }
  await page.close()
})

test('the cue holds two stacked chevrons', async ({ browser }) => {
  const ctx = await phoneCtx(browser, VIEWPORTS[0])
  const page = await ctx.newPage()
  await page.goto('/')
  await mapSettled(page)
  const chevrons = page.locator('a.scroll-cue .scroll-cue__chevron')
  await expect(chevrons).toHaveCount(2)
  await expect.poll(async () => {
    const [a, b] = await chevrons.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().y))
    return b > a
  }).toBe(true)
  await expect(page.locator('a.scroll-cue')).toHaveAttribute('aria-label', /\S/)
  await ctx.close()
})

test('393x873: the cue tap target is at least 44px tall', async ({ browser }) => {
  const ctx = await phoneCtx(browser, VIEWPORTS[0])
  const page = await ctx.newPage()
  await page.goto('/')
  await mapSettled(page)
  await expect.poll(async () => (await tapBox(page.locator('a.scroll-cue'))).height).toBeGreaterThanOrEqual(44)
  await ctx.close()
})

test('reduced motion: neither chevron animates', async ({ browser }) => {
  const ctx = await phoneCtx(browser, VIEWPORTS[0])
  const page = await ctx.newPage()
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/')
  await mapSettled(page)
  const names = await page.locator('.scroll-cue__chevron')
    .evaluateAll((els) => els.map((el) => getComputedStyle(el).animationName))
  expect(names).toEqual(['none', 'none'])
  await ctx.close()
})

test('motion allowed: both chevrons animate', async ({ browser }) => {
  const ctx = await phoneCtx(browser, VIEWPORTS[0])
  const page = await ctx.newPage()
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  await page.goto('/')
  await mapSettled(page)
  const names = await page.locator('.scroll-cue__chevron')
    .evaluateAll((els) => els.map((el) => getComputedStyle(el).animationName))
  expect(names).toHaveLength(2)
  for (const n of names) expect(n).not.toBe('none')
  await ctx.close()
})
