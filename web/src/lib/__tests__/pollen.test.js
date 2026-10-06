import { describe, it, expect } from 'vitest'
import {
  POLLEN_LEVELS,
  pollenAttribution,
  pollenColours,
  pollenFeatures,
  pollenFillPaint,
  pollenHref,
  pollenVar,
  readPollenTexts,
} from '../pollen.js'

// Evaluates the ['match', ['coalesce', ['get','level'], ''], ...pairs, other] the layer uses.
function evalMatch(expr, level) {
  const pairs = expr.slice(2, -1)
  for (let i = 0; i < pairs.length; i += 2) if (pairs[i] === (level ?? '')) return pairs[i + 1]
  return expr[expr.length - 1]
}

const theme = {
  '--pollen-none': '#6f6f6f',
  '--pollen-low': '#0f62fe',
  '--pollen-moderate': '#0f62fe',
  '--pollen-high': '#f1c21b',
  '--pollen-very-high': '#da1e28',
}

describe('pollen colours', () => {
  it('reads one theme token per level, very_high as --pollen-very-high', () => {
    expect(pollenVar('very_high')).toBe('--pollen-very-high')
    expect(pollenColours((n) => ` ${theme[n]} `, 'grey')).toEqual({
      none: '#6f6f6f', low: '#0f62fe', moderate: '#0f62fe', high: '#f1c21b', very_high: '#da1e28',
    })
  })

  it('falls back where the theme has no token', () => {
    expect(pollenColours(() => '', 'grey').high).toBe('grey')
  })

  it('fills each level in its own colour and leaves no-data unfilled', () => {
    const paint = pollenFillPaint(pollenColours((n) => theme[n], 'grey'), 'grey')
    expect(evalMatch(paint['fill-color'], 'none')).toBe('#6f6f6f')
    expect(evalMatch(paint['fill-color'], 'high')).toBe('#f1c21b')
    expect(evalMatch(paint['fill-color'], 'very_high')).toBe('#da1e28')
    expect(evalMatch(paint['fill-opacity'], null)).toBe(0)
    // Low and moderate share a colour; the opacity tells them apart.
    expect(evalMatch(paint['fill-opacity'], 'low')).toBeLessThan(evalMatch(paint['fill-opacity'], 'moderate'))
    for (const l of POLLEN_LEVELS) expect(evalMatch(paint['fill-opacity'], l)).toBeGreaterThan(0)
  })
})

describe('pollenFeatures', () => {
  const boundaries = {
    type: 'FeatureCollection',
    features: [
      { type: 'Feature', geometry: { type: 'Polygon', coordinates: [] }, properties: { slug: 'sofia-oblast', name_en: 'Sofia' } },
      { type: 'Feature', geometry: { type: 'Polygon', coordinates: [] }, properties: { slug: 'oblast-1' } },
    ],
  }

  it('joins today\'s level onto the province outline by slug', () => {
    const fc = pollenFeatures(boundaries, { areas: [{ slug: 'sofia-oblast', level: 'high', species: 'ragweed' }] })
    expect(fc.features.map((f) => f.properties)).toEqual([
      { slug: 'sofia-oblast', level: 'high' },
      { slug: 'oblast-1', level: null },
    ])
  })

  it('keeps the outlines when there is no payload', () => {
    expect(pollenFeatures(boundaries, null).features).toHaveLength(2)
    expect(pollenFeatures(null, null).features).toEqual([])
  })
})

describe('pollenHref', () => {
  it('opens the clicked province\'s area page in the page\'s language', () => {
    expect(pollenHref({ properties: { slug: 'sofia-oblast' } }, '/en')).toBe('/en/area/sofia-oblast')
    expect(pollenHref({ properties: { slug: 'sofia-oblast' } }, '')).toBe('/area/sofia-oblast')
    expect(pollenHref({ properties: { slug: 'a b' } }, '')).toBe('/area/a%20b')
  })

  it('goes nowhere without a slug', () => {
    expect(pollenHref({ properties: {} }, '/en')).toBeNull()
    expect(pollenHref(undefined, '/en')).toBeNull()
  })
})

describe('pollenAttribution', () => {
  it('escapes the credit and links it', () => {
    expect(pollenAttribution('A & <B>', 'https://x/?a=1&b=2')).toBe(
      '<a href="https://x/?a=1&#38;b=2" rel="noopener noreferrer">A &#38; &#60;B&#62;</a>',
    )
    expect(pollenAttribution('A', '')).toBe('A')
    expect(pollenAttribution('', 'https://x')).toBe('')
  })
})

describe('readPollenTexts', () => {
  it('groups the data-t-pollen-* attributes', () => {
    const t = readPollenTexts({ tPollenToggle: 'Pollen', tPollenLegend: 'Pollen forecast', tPollenLevelVeryHigh: 'Very high' })
    expect(t.toggle).toBe('Pollen')
    expect(t.legend).toBe('Pollen forecast')
    expect(t.levels.very_high).toBe('Very high')
    expect(t.levels.none).toBe('')
  })
})
