import { test, expect } from './fixtures.js'

// A readout card draws its figure under the caption, inside the card, at the
// value size, with no ring around it.
const widths = [390, 1024, 1440]

async function measure(page) {
  return page.evaluate(() => {
    const cards = [...document.querySelectorAll('.readouts .readout.card')]
      .filter((c) => c.offsetParent !== null)
    return cards.map((c) => {
      const card = c.getBoundingClientRect()
      const top = c.querySelector('.readout__label').getBoundingClientRect().bottom
      const v = c.querySelector('.readout__value')
      const range = document.createRange()
      // The figure's own text node: the spoken unit beside it is visually hidden.
      range.selectNodeContents(v.firstChild.nodeType === 3 ? v.firstChild : v)
      const r = range.getBoundingClientRect()
      return {
        text: v.textContent.trim(),
        card: { l: card.left, r: card.right },
        free: { top },
        ring: !!c.querySelector('.gauge'),
        size: parseFloat(getComputedStyle(v).fontSize),
        value: { l: r.left, r: r.right, t: r.top, b: r.bottom, h: r.height },
        overflow: v.scrollWidth > v.clientWidth + 1,
      }
    })
  })
}

for (const path of ['/en/', '/en/areas']) {
  for (const width of widths) {
    test(`number cards sit inside their card: ${path} @${width}`, async ({ ctx }) => {
      const page = await ctx.newPage()
      await page.setViewportSize({ width, height: 900 })
      await page.goto(path)
      await page.waitForTimeout(800)
      const cards = await measure(page)
      if (path === '/en/') expect(cards.length).toBeGreaterThan(0)
      for (const c of cards) {
        expect(c.ring, `${c.text} has no ring`).toBe(false)
        expect(c.size, `${c.text} value size`).toBeGreaterThanOrEqual(32)
        expect(c.value.l, `${c.text} inside card`).toBeGreaterThanOrEqual(c.card.l)
        expect(c.value.r, `${c.text} inside card`).toBeLessThanOrEqual(c.card.r)
        expect(c.value.t, `${c.text} under its caption`).toBeGreaterThanOrEqual(c.free.top - 1)
        expect(c.overflow).toBe(false)
      }
      await page.close()
    })
  }
}

// Review screenshots, only when NUMCARDS_SHOT=<before|after> is set.
const shot = process.env.NUMCARDS_SHOT
for (const [lang, path, width, scheme] of [['en', '/en/', 1440, 'light'], ['bg', '/', 390, 'dark']]) {
  test(`screenshot ${lang} @${width}`, async ({ ctx }) => {
    test.skip(!shot)
    const page = await ctx.newPage()
    await page.setViewportSize({ width, height: 900 })
    await page.emulateMedia({ colorScheme: scheme })
    await page.goto(path)
    await page.waitForTimeout(1000)
    await page.locator('.readouts:visible').first().scrollIntoViewIfNeeded()
    await page.screenshot({ path: `/tmp/airbg-verify/numcards-${shot}-${lang}-${width}-${scheme}.png` })
    await page.close()
  })
}
