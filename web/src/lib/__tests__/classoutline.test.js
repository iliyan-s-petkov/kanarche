// @vitest-environment jsdom
import { readFileSync } from 'node:fs'
import { describe, it, expect } from 'vitest'
import { expression } from '@maplibre/maplibre-gl-style-spec'
import {
  CLASS_OUTLINE_PX, bandIndex, classOutlineWidth, classOutlineExpr,
  outlineSwatch, outlineInk, applyOutlineInk, watchTheme,
} from '../classoutline.js'
import { HEX_OUTLINE_LAYER_ID, HEX_POINT_LAYER_ID, LAYER_ID, OFFICIAL_LAYER_ID } from '../mapids.js'

// A11Y-05: outline weight is the second channel beside colour. Worse class, heavier outline.
const BANDS = [
  { upper: 5 }, { upper: 15 }, { upper: 50 }, { upper: 90 }, { upper: 140 }, { upper: null },
]

function evalWidth(expr, band) {
  const parsed = expression.createExpression(expr, { type: 'number', 'property-type': 'data-driven', expression: { interpolated: false, parameters: ['zoom', 'feature'] } })
  expect(parsed.result).toBe('success')
  return parsed.value.evaluate({ zoom: 10 }, { type: 'Polygon', properties: band === undefined ? {} : { band } })
}

describe('CLASS_OUTLINE_PX', () => {
  it('has one weight per EAQI class, each heavier than the class before by at least 0.75px', () => {
    expect(CLASS_OUTLINE_PX).toHaveLength(6)
    for (let i = 1; i < CLASS_OUTLINE_PX.length; i++) {
      expect(CLASS_OUTLINE_PX[i] - CLASS_OUTLINE_PX[i - 1]).toBeGreaterThanOrEqual(0.75)
    }
  })

  // The table chip is server-rendered and styled from CSS, so the ladder is restated as tokens in both sheets.
  for (const path of ['../internal/web/static/theme.css', 'src/styles/theme.css']) {
    it(`is mirrored as --class-outline-N in ${path}`, () => {
      const css = readFileSync(path, 'utf8')
      CLASS_OUTLINE_PX.forEach((px, i) => {
        expect(css).toMatch(new RegExp(`--class-outline-${i}:\\s*${px}px;`))
      })
    })
  }

  it('outlines the table chip from those tokens, one rule per class', () => {
    const css = readFileSync('../internal/web/static/app.css', 'utf8')
    CLASS_OUTLINE_PX.forEach((_, i) => {
      expect(css).toMatch(new RegExp(`\\.chip__swatch\\[data-class="${i}"\\][^{]*\\{[^}]*var\\(--class-outline-${i}\\)`))
    })
  })
})

describe('bandIndex', () => {
  it('puts a boundary value in the lower band and an open top above the last upper', () => {
    expect(bandIndex(0, BANDS)).toBe(0)
    expect(bandIndex(5, BANDS)).toBe(0)
    expect(bandIndex(5.1, BANDS)).toBe(1)
    expect(bandIndex(50, BANDS)).toBe(2)
    expect(bandIndex(500, BANDS)).toBe(5)
  })

  it('is -1 for no reading or no band table', () => {
    expect(bandIndex(null, BANDS)).toBe(-1)
    expect(bandIndex(Number.NaN, BANDS)).toBe(-1)
    expect(bandIndex(12, [])).toBe(-1)
    expect(bandIndex(12, undefined)).toBe(-1)
  })
})

describe('classOutlineWidth', () => {
  it('maps a class to its weight, no class to the lightest and past the ladder to the heaviest', () => {
    expect(classOutlineWidth(0)).toBe(CLASS_OUTLINE_PX[0])
    expect(classOutlineWidth(3)).toBe(CLASS_OUTLINE_PX[3])
    expect(classOutlineWidth(-1)).toBe(CLASS_OUTLINE_PX[0])
    expect(classOutlineWidth(9)).toBe(CLASS_OUTLINE_PX[5])
  })
})

describe('classOutlineExpr', () => {
  it('evaluates to the same weight as classOutlineWidth for every class a feature can carry', () => {
    for (const band of [-1, 0, 1, 2, 3, 4, 5, 7]) {
      expect(evalWidth(classOutlineExpr(), band)).toBe(classOutlineWidth(band))
    }
    expect(evalWidth(classOutlineExpr(), undefined)).toBe(CLASS_OUTLINE_PX[0])
  })

  it('adds the extra px on top of the class weight', () => {
    expect(evalWidth(classOutlineExpr(1), 2)).toBe(CLASS_OUTLINE_PX[2] + 1)
  })
})

// The outline ink follows the theme: #161616 on light, the light foreground on dark.
describe('outline ink', () => {
  for (const path of ['../internal/web/static/theme.css', 'src/styles/theme.css']) {
    it(`is the theme foreground in ${path}`, () => {
      expect(readFileSync(path, 'utf8')).toMatch(/--class-outline-ink:\s*var\(--fg\);/)
    })
  }

  it('strokes every outlined swatch from the token, over its attribute', () => {
    const css = readFileSync('../internal/web/static/app.css', 'utf8')
    expect(css).toMatch(/\.class-outline\s*\{[^}]*stroke:\s*var\(--class-outline-ink\)/)
  })

  it('marks a swatch shape as outlined', () => {
    const shape = document.createElementNS('http://www.w3.org/2000/svg', 'rect')
    outlineSwatch(shape, 2, '#161616')
    expect(shape.classList.contains('class-outline')).toBe(true)
  })

  it('reads the ink from the page, else the fallback', () => {
    expect(outlineInk(document, '#161616')).toBe('#161616')
    document.documentElement.style.setProperty('--class-outline-ink', ' #f4f4f4')
    try {
      expect(outlineInk(document, '#161616')).toBe('#f4f4f4')
    } finally {
      document.documentElement.style.removeProperty('--class-outline-ink')
    }
  })

  it('repaints the outline and halo of every marker layer the map has', () => {
    const set = []
    const map = { getLayer: (id) => id !== OFFICIAL_LAYER_ID, setPaintProperty: (...a) => set.push(a) }
    applyOutlineInk(map, '#f4f4f4')
    expect(set).toEqual([
      [HEX_OUTLINE_LAYER_ID, 'line-color', '#f4f4f4'],
      [HEX_POINT_LAYER_ID, 'circle-stroke-color', '#f4f4f4'],
      [LAYER_ID, 'circle-stroke-color', '#f4f4f4'],
    ])
  })

  it('calls back when the theme attribute changes, until stopped', async () => {
    let calls = 0
    const stop = watchTheme(document, window, () => { calls++ })
    document.documentElement.setAttribute('data-theme', 'dark')
    await new Promise((r) => setTimeout(r, 0))
    expect(calls).toBe(1)
    stop()
    document.documentElement.setAttribute('data-theme', 'light')
    await new Promise((r) => setTimeout(r, 0))
    expect(calls).toBe(1)
    document.documentElement.removeAttribute('data-theme')
  })
})
