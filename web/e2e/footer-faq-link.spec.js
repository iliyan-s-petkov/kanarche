import { test, expect } from './fixtures.js'

// OP 757: the footer FAQ link is on every page type and lands on the About FAQ.
const pages = [
  ['', '/'],
  ['', '/area/sofia'],
  ['', '/pm25'],
  ['/en', '/en/'],
]

for (const [lang, path] of pages) {
  test(`footer FAQ link on ${path} goes to ${lang}/about-the-data#faq`, async ({ ctx }) => {
    const page = await ctx.newPage()
    await page.goto(path)
    const link = page.locator(`footer a[href="${lang}/about-the-data#faq"]`)
    await expect(link).toHaveCount(1)
    await link.click()
    await expect(page).toHaveURL(new RegExp(`${lang}/about-the-data#faq$`))
    await expect(page.locator('#faq')).toHaveCount(1)
    await page.close()
  })
}
