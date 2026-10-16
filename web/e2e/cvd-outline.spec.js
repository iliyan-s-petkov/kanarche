import { test as base, expect, mapSettled } from './fixtures.js'

// A11Y-05: outline weight is the second channel beside colour. Under simulated
// colour-vision deficiency each class must still draw a heavier outline than the
// class before it, on the map and in the legend. Needs a VITE_E2E_MAP_HANDLE=1 build.
const DEFICIENCIES = ['protanopia', 'deuteranopia', 'tritanopia', 'achromatopsia']
const CENTRE = [23.32, 42.69]
const OUTLINE = 'kanarche-hex-outline'
const PROBE = 'cvd-probe'
const DPR = 2

const test = base.extend({
  cvdCtx: [async ({ browser }, use) => {
    const context = await browser.newContext({ bypassCSP: true, deviceScaleFactor: DPR, viewport: { width: 1280, height: 800 } })
    await use(context)
    await context.close()
  }, { scope: 'worker' }],
})

test.afterEach(async ({ cvdCtx }) => {
  for (const p of cvdCtx.pages()) if (!p.isClosed()) await p.close().catch(() => {})
})

const open = async (cvdCtx, contrast) => {
  const page = await cvdCtx.newPage()
  if (contrast) await page.emulateMedia({ contrast })
  await page.addInitScript(() => localStorage.setItem('kanarche:legend-open', 'true'))
  await page.goto('/en/')
  await mapSettled(page)
  await page.evaluate((center) => {
    document.querySelector('.map-shell').scrollIntoView({ block: 'start', behavior: 'instant' })
    document.querySelector('[data-island="map"]').__map.jumpTo({ center, zoom: 10 })
  }, CENTRE)
  await mapSettled(page)
  return page
}

// The session stays attached: detaching it drops the emulation.
const emulate = async (page, type) => {
  const cdp = await page.context().newCDPSession(page)
  await cdp.send('Emulation.setEmulatedVisionDeficiency', { type })
  return cdp
}

// Per line of [x, y] device pixels, how many change between two shots. Decoded in the page, which leaves them as captured.
const changedAlong = (page, on, off, lines) => page.evaluate(async ([a, b, lines]) => {
  const decode = async (b64) => {
    const img = new Image()
    img.src = `data:image/png;base64,${b64}`
    await img.decode()
    const c = document.createElement('canvas')
    c.width = img.width
    c.height = img.height
    const g = c.getContext('2d')
    g.drawImage(img, 0, 0)
    return { w: img.width, data: g.getImageData(0, 0, img.width, img.height).data }
  }
  const [p, q] = [await decode(a), await decode(b)]
  return lines.map((pts) => pts.filter(([x, y]) => {
    const i = (y * p.w + x) * 4
    return Math.max(Math.abs(p.data[i] - q.data[i]), Math.abs(p.data[i + 1] - q.data[i + 1]), Math.abs(p.data[i + 2] - q.data[i + 2])) > 24
  }).length)
}, [on.toString('base64'), off.toString('base64'), lines])

// Six squares in a row, one per class, stroked with the hex outline layer's own paint. Canvas px.
const addProbe = (page) => page.evaluate(([outline, probe]) => {
  const map = document.querySelector('[data-island="map"]').__map
  const boxes = Array.from({ length: 6 }, (_, band) => ({ band, x0: 200 + band * 140, x1: 300 + band * 140, y0: 250, y1: 350 }))
  const ring = (b) => [[b.x0, b.y0], [b.x1, b.y0], [b.x1, b.y1], [b.x0, b.y1], [b.x0, b.y0]].map((p) => map.unproject(p).toArray())
  map.addSource(probe, { type: 'geojson', data: { type: 'FeatureCollection', features: boxes.map((b) => ({ type: 'Feature', properties: { band: b.band }, geometry: { type: 'Polygon', coordinates: [ring(b)] } })) } })
  const paint = {}
  for (const k of ['line-width', 'line-offset', 'line-color']) paint[k] = map.getPaintProperty(outline, k)
  map.addLayer({ id: probe, type: 'line', source: probe, paint })
  return boxes
}, [OUTLINE, PROBE])

const showProbe = (page, on) => page.evaluate(([probe, on]) => new Promise((done) => {
  const map = document.querySelector('[data-island="map"]').__map
  map.once('render', () => setTimeout(done, 300))
  map.setLayoutProperty(probe, 'visibility', on ? 'visible' : 'none')
  map.triggerRepaint()
}), [PROBE, on])

// Outline px per class on the map probe: the row through each square's middle.
const mapWeights = async (page, boxes) => {
  const canvas = page.locator('[data-island="map"] canvas.maplibregl-canvas')
  const shot = async (on) => {
    await showProbe(page, on)
    return canvas.screenshot()
  }
  const on = await shot(true)
  const off = await shot(false)
  const lines = boxes.map((b) => {
    const row = []
    for (let x = (b.x0 - 8) * DPR; x <= (b.x1 + 8) * DPR; x++) row.push([x, ((b.y0 + b.y1) / 2) * DPR])
    return row
  })
  return changedAlong(page, on, off, lines)
}

// Outline px per class in the legend: the column through each band swatch's middle.
const legendWeights = async (page) => {
  const bands = page.locator('.scale--onmap .scale__bands')
  const swatches = bands.locator('.scale__band-swatch')
  await expect(swatches).toHaveCount(6)
  const strokes = (on) => swatches.evaluateAll((els, on) => els.forEach((e) => {
    const shape = e.firstElementChild
    if (!on) shape.dataset.w = shape.getAttribute('stroke-width')
    shape.setAttribute('stroke-width', on ? shape.dataset.w : '0')
  }), on)
  const on = await bands.screenshot()
  const boxes = await bands.evaluate((list) => {
    const o = list.getBoundingClientRect()
    return [...list.querySelectorAll('.scale__band-swatch')].map((e) => {
      const r = e.getBoundingClientRect()
      return { x: r.x - o.x + r.width / 2, y0: r.y - o.y, y1: r.y - o.y + r.height }
    })
  })
  // The list runs worst first and is laid out reversed; left to right on screen is best to worst.
  boxes.sort((a, b) => a.x - b.x)
  await strokes(false)
  const off = await bands.screenshot()
  await strokes(true)
  const lines = boxes.map((b) => {
    const col = []
    for (let y = Math.ceil(b.y0 * DPR); y < Math.floor(b.y1 * DPR); y++) col.push([Math.round(b.x * DPR), y])
    return col
  })
  return changedAlong(page, on, off, lines)
}

const strictlyRising = (ws) => ws.every((w, i) => i === 0 || w > ws[i - 1])

test.describe('class outline weight under colour-vision deficiency', () => {
  test('rendered hexes carry their class and the outline paint reads it', async ({ cvdCtx }) => {
    const page = await open(cvdCtx)
    const bands = () => page.evaluate((layer) => document.querySelector('[data-island="map"]').__map
      .queryRenderedFeatures({ layers: [layer] }).map((f) => f.properties.band), OUTLINE)
    await expect.poll(() => bands().then((b) => b.length), { timeout: 30_000 }).toBeGreaterThan(0)
    expect((await bands()).every((b) => Number.isInteger(b) && b >= -1 && b <= 5)).toBe(true)
    expect(JSON.stringify(await page.evaluate((layer) => document.querySelector('[data-island="map"]').__map
      .getPaintProperty(layer, 'line-width'), OUTLINE))).toContain('"band"')
  })

  for (const type of DEFICIENCIES) {
    test(`${type}: each map class outline is heavier than the one before`, async ({ cvdCtx }) => {
      const page = await open(cvdCtx)
      const boxes = await addProbe(page)
      const before = await mapWeights(page, boxes)
      await emulate(page, type)
      const weights = await mapWeights(page, boxes)
      expect(strictlyRising(before), `none ${before}`).toBe(true)
      expect(strictlyRising(weights), `${type} ${weights}`).toBe(true)
    })

    test(`${type}: each legend swatch outline is heavier than the one before`, async ({ cvdCtx }) => {
      const page = await open(cvdCtx)
      const bar = page.locator('.scale--onmap .scale__bands')
      const before = await bar.screenshot()
      await emulate(page, type)
      // The simulation is on: the same legend renders differently.
      expect(Buffer.compare(before, await bar.screenshot())).not.toBe(0)
      const weights = await legendWeights(page)
      expect(strictlyRising(weights), `${type} ${weights}`).toBe(true)
    })
  }

  test('high contrast keeps the legend outlines visible and rising', async ({ cvdCtx }) => {
    const page = await open(cvdCtx, 'more')
    await emulate(page, 'achromatopsia')
    const weights = await legendWeights(page)
    expect(weights[0]).toBeGreaterThan(0)
    expect(strictlyRising(weights), `contrast more ${weights}`).toBe(true)
  })

  // Page frames take the theme foreground. The basemap stays light in both themes, so the map paint keeps the dark ink.
  const INK = { light: 'rgb(22, 22, 22)', dark: 'rgb(244, 244, 244)' }
  const MAP_INK = INK.light
  const inks = (page) => page.evaluate((layers) => {
    const map = document.querySelector('[data-island="map"]').__map
    const css = (sel) => getComputedStyle(document.querySelector(sel)).stroke
    const rgb = (c) => {
      const el = document.createElement('i')
      el.style.color = c
      document.body.append(el)
      const v = getComputedStyle(el).color
      el.remove()
      return v
    }
    return {
      paint: layers.map(([id, prop]) => rgb(map.getPaintProperty(id, prop))),
      legend: css('.scale--onmap .scale__band-swatch .class-outline'),
    }
  }, [[OUTLINE, 'line-color'], ['kanarche-hex-point', 'circle-stroke-color'], ['kanarche-markers', 'circle-stroke-color'], ['kanarche-markers-official', 'icon-halo-color']])

  for (const theme of ['light', 'dark']) {
    for (const contrast of ['no-preference', 'more']) {
      test(`${theme} theme, contrast ${contrast}: frames use the ${theme} ink, the map the dark ink`, async ({ cvdCtx }) => {
        const page = await cvdCtx.newPage()
        await page.emulateMedia({ contrast })
        await page.addInitScript((v) => {
          localStorage.setItem('kanarche:theme', v)
          localStorage.setItem('kanarche:legend-open', 'true')
        }, theme)
        await page.goto('/en/')
        await mapSettled(page)
        await expect(page.locator('.scale--onmap .scale__band-swatch .class-outline').first()).toBeAttached()
        const got = await inks(page)
        expect(got.paint).toEqual([MAP_INK, MAP_INK, MAP_INK, MAP_INK])
        expect(got.legend).toBe(INK[theme])
        await page.goto('/en/areas')
        expect(await page.locator('svg.chip__swatch polygon').first().evaluate((e) => getComputedStyle(e).stroke)).toBe(INK[theme])
      })
    }
  }

  test('switching the theme turns the frames light and leaves the map paint dark', async ({ cvdCtx }) => {
    const page = await cvdCtx.newPage()
    await page.addInitScript(() => {
      localStorage.setItem('kanarche:theme', 'light')
      localStorage.setItem('kanarche:legend-open', 'true')
    })
    await page.goto('/en/')
    await mapSettled(page)
    await expect(page.locator('.scale--onmap .scale__band-swatch .class-outline').first()).toBeAttached()
    await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'dark'))
    await expect.poll(async () => (await inks(page)).legend).toBe(INK.dark)
    expect((await inks(page)).paint).toEqual([MAP_INK, MAP_INK, MAP_INK, MAP_INK])
  })

  test('the area table chip is outlined and the chip rules step up by class', async ({ cvdCtx }) => {
    const page = await cvdCtx.newPage()
    await page.goto('/en/areas')
    const chip = page.locator('svg.chip__swatch[data-class] polygon').first()
    expect(parseFloat(await chip.evaluate((e) => getComputedStyle(e).strokeWidth))).toBeGreaterThan(0)
    // The fixture areas share one class, so every class is set on a copy of the chip.
    const widths = await chip.evaluate((e) => [0, 1, 2, 3, 4, 5].map((c) => {
      const svg = e.parentElement.cloneNode(true)
      svg.dataset.class = String(c)
      e.parentElement.after(svg)
      const w = parseFloat(getComputedStyle(svg.firstElementChild).strokeWidth)
      svg.remove()
      return w
    }))
    expect(strictlyRising(widths), `chips ${widths}`).toBe(true)
  })
})
