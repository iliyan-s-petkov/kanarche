import { readFileSync } from 'node:fs'
import { describe, it, expect } from 'vitest'
import { expression } from '@maplibre/maplibre-gl-style-spec'
import { CLASS_OUTLINE_PX, bandIndex, classOutlineWidth, classOutlineExpr } from '../classoutline.js'

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
