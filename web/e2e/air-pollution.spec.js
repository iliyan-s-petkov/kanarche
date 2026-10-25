import { createRequire } from 'node:module'
import { test, expect } from './fixtures.js'

// The air pollution explainer: a static page, so it loads, its FAQ folds open and axe finds nothing.
const axeSource = createRequire(import.meta.url).resolve('axe-core/axe.min.js')

for (const [lang, path, h1] of [
  ['en', '/en/air-pollution', 'Air pollution: what we breathe and where it comes from'],
  ['bg', '/air-pollution', 'Замърсяване на въздуха: какво дишаме и откъде идва'],
]) {
  test(`the explainer loads, its FAQ opens and axe is clean (${lang})`, async ({ browser }) => {
    // bypassCSP lets the test inject axe-core; the page itself is unchanged.
    const context = await browser.newContext({ bypassCSP: true })
    const page = await context.newPage()
    try {
      await page.goto(path)
      await expect(page.locator('h1')).toHaveText(h1)

      const first = page.locator('details.faq__item').first()
      await expect(page.locator('details.faq__item')).toHaveCount(8)
      await expect(first.locator('.faq__answer')).toBeHidden()
      await first.locator('summary').click()
      await expect(first).toHaveAttribute('open', '')
      await expect(first.locator('.faq__answer')).toBeVisible()

      await page.addScriptTag({ path: axeSource })
      const violations = await page.evaluate(async () => {
        const r = await window.axe.run(document)
        return r.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)
      })
      expect(violations).toEqual([])
    } finally {
      await context.close()
    }
  })
}
