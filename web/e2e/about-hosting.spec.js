import { test, expect } from './fixtures.js'

const sizes = [
  { width: 360, height: 640 },
  { width: 390, height: 844 },
  { width: 1440, height: 900 },
]

// '' is the BG root, '/en' the English one.
for (const lang of ['', '/en']) {
  for (const theme of ['light', 'dark']) {
    for (const size of sizes) {
      test(`hosting credit at ${size.width} ${theme} (${lang || '/bg'})`, async ({ ctx }) => {
        const page = await ctx.newPage()
        await page.setViewportSize(size)
        await page.addInitScript((t) => localStorage.setItem('kanarche:theme', t), theme)
        await page.goto(`${lang}/about`)
        const link = page.locator('#hosting a[href="https://hostellation.com/"]')
        await link.scrollIntoViewIfNeeded()
        await expect(link).toBeVisible()

        const m = await page.evaluate(() => {
          const visible = [...document.querySelectorAll('#hosting img')].filter((i) => i.getClientRects().length > 0)
          const more = document.querySelector('#more').getBoundingClientRect()
          return {
            over: document.documentElement.scrollWidth - document.documentElement.clientWidth,
            linkHeight: document.querySelector('#hosting a').getBoundingClientRect().height,
            visibleSrcs: visible.map((i) => i.src),
            hostingTop: document.querySelector('#hosting').getBoundingClientRect().top,
            moreBottom: more.bottom,
          }
        })
        expect(m.over).toBeLessThanOrEqual(0)
        expect(m.linkHeight).toBeGreaterThanOrEqual(44)
        // src, not currentSrc: a lazy image has no currentSrc until it loads. One logo shows, and it is the one named for the active theme.
        expect(m.visibleSrcs).toHaveLength(1)
        expect(m.visibleSrcs[0]).toContain(`host-hostellation-${theme}`)
        expect(m.hostingTop).toBeGreaterThanOrEqual(m.moreBottom)
        await page.close()
      })
    }
  }
}
