import { test, expect } from './fixtures.js'

// The harness sets KANARCHE_FEEDBACK_URL to this host (internal/e2e/e2e_test.go).
// It does not resolve: every request to it is answered here and recorded.
const SURVEY = 'https://feedback.example.test'

// Click `locate(page)` and return what the new tab loaded and what left the browser.
async function openSurvey(ctx, page, locate) {
  const hits = []
  await ctx.route(`${SURVEY}/**`, (route) => {
    hits.push(route.request())
    return route.fulfill({ status: 200, contentType: 'text/html', body: '<title>stub</title>' })
  })
  try {
    const target = locate(page)
    await expect(target).toBeVisible()
    // Links do not prefetch: nothing may reach the survey before the click.
    expect(hits).toHaveLength(0)
    const [popup] = await Promise.all([ctx.waitForEvent('page'), target.click()])
    await popup.waitForLoadState()
    const result = {
      url: popup.url(),
      opener: await popup.evaluate(() => window.opener),
      referer: hits[0] ? hits[0].headers()['referer'] : 'no request',
      requests: hits.length,
    }
    await popup.close()
    return result
  } finally {
    await ctx.unroute(`${SURVEY}/**`)
  }
}

for (const [prefix, lang] of [['', 'bg'], ['/en', 'en']]) {
  test(`masthead item opens the survey in a new tab (${lang})`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.setViewportSize({ width: 1280, height: 800 })
    await page.goto(`${prefix}/about`)
    const r = await openSurvey(ctx, page, (p) => p.locator('.masthead__link--feedback'))
    expect(r.url).toBe(`${SURVEY}/?lang=${lang}`)
    expect(r.requests).toBe(1)
    expect(r.opener).toBeNull()
    expect(r.referer).toBeUndefined()
    await page.close()
  })

  test(`About section link opens the survey in a new tab (${lang})`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.setViewportSize({ width: 1280, height: 800 })
    await page.goto(`${prefix}/about`)
    const r = await openSurvey(ctx, page, (p) => p.locator('.about-feedback a'))
    expect(r.url).toBe(`${SURVEY}/?lang=${lang}`)
    expect(r.requests).toBe(1)
    expect(r.opener).toBeNull()
    expect(r.referer).toBeUndefined()
    await page.close()
  })

  test(`narrow: the masthead item is hidden and the language picker carries it (${lang})`, async ({ ctx }) => {
    for (const width of [390, 481, 560]) {
      const page = await ctx.newPage()
      await page.setViewportSize({ width, height: 844 })
      await page.goto(`${prefix}/areas`)
      await expect(page.locator('.masthead__link--feedback'), `masthead item at ${width}`).toBeHidden()
      await page.locator('details.langpick').first().locator('summary').click()
      const r = await openSurvey(ctx, page, (p) => p.locator('.langpick__extra a[href^="' + SURVEY + '"]'))
      expect(r.url).toBe(`${SURVEY}/?lang=${lang}`)
      expect(r.opener).toBeNull()
      await page.close()
    }
  })

  test(`wide: the masthead item shows and the picker does not repeat it (${lang})`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.setViewportSize({ width: 561, height: 800 })
    await page.goto(`${prefix}/areas`)
    await expect(page.locator('.masthead__link--feedback')).toBeVisible()
    await expect(page.locator('.langpick__extra a[href^="' + SURVEY + '"]').first()).toBeHidden()
    await page.close()
  })

  // The fifth masthead item must not overflow or wrap anywhere the tab bar shows.
  test(`masthead does not overflow or wrap with the survey item (${lang})`, async ({ ctx }) => {
    for (const width of [481, 530, 560, 561, 600, 768, 1024, 1280]) {
      const page = await ctx.newPage()
      await page.setViewportSize({ width, height: 800 })
      await page.goto(`${prefix}/areas`)
      // Webfont swap changes the label widths; measure after it settles.
      await page.evaluate(() => document.fonts.ready)
      const m = await page.evaluate(() => ({
        over: document.documentElement.scrollWidth - document.documentElement.clientWidth,
        heights: [...document.querySelectorAll('.masthead__link')].map((a) => Math.round(a.getBoundingClientRect().height)),
      }))
      expect(m.over, `overflow at ${width}`).toBeLessThanOrEqual(0)
      for (const h of m.heights) expect(h, `wrapped link at ${width}`).toBeLessThanOrEqual(48)
      await page.close()
    }
  })
}
