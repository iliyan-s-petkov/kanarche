// Pure-logic tests for the hexagon number: size per viewport, font, halo and
// the digits it prints. The formatter is run through the real style-spec
// evaluator, so what is asserted is what MapLibre would draw.
import { describe, it, expect } from 'vitest'
import { expression } from '@maplibre/maplibre-gl-style-spec'
import { hexLabelLayout, labelPaint } from '../../lib/mappaint.js'
import { PHONE_BREAKPOINT_PX } from '../../lib/hexes.js'
import { contrastRatio } from '../../lib/axiscolour.js'

const cfg = { lang: 'en', markerStrokeColour: '#ffffff', labelColour: '#161616' }

// Evaluates the layout's text-field for one value, as MapLibre does.
function printed(value, lang = 'en') {
  const field = hexLabelLayout({ ...cfg, lang })['text-field']
  const parsed = expression.createExpression(field, {
    type: 'formatted', overridable: true, transition: false, 'property-type': 'data-driven',
  })
  if (parsed.result !== 'success') throw new Error(JSON.stringify(parsed.value))
  const out = parsed.value.evaluate({ zoom: 10 }, { type: 'Point', properties: { value } })
  return out.toString()
}

describe('the hexagon number', () => {
  it('is 14px on a desktop map and 12px on a phone', () => {
    expect(hexLabelLayout(cfg)['text-size']).toBe(14)
    expect(hexLabelLayout(cfg, 1280)['text-size']).toBe(14)
    expect(hexLabelLayout(cfg, PHONE_BREAKPOINT_PX + 1)['text-size']).toBe(14)
    expect(hexLabelLayout(cfg, PHONE_BREAKPOINT_PX)['text-size']).toBe(12)
    expect(hexLabelLayout(cfg, 390)['text-size']).toBe(12)
  })

  it('uses the medium weight of the basemap fontstack', () => {
    expect(hexLabelLayout(cfg)['text-font']).toEqual(['Noto Sans Medium'])
  })

  it('stays centred on its cell and thinned on collision', () => {
    const layout = hexLabelLayout(cfg)
    expect(layout['text-anchor']).toBe('center')
    expect(layout['text-allow-overlap']).toBe(false)
  })

  it('keeps one decimal below 100', () => {
    expect(printed(99.9)).toBe('99.9')
    expect(printed(5)).toBe('5.0')
    expect(printed(31)).toBe('31.0')
  })

  it('drops the decimal from 100 up', () => {
    expect(printed(100)).toBe('100')
    expect(printed(120.4)).toBe('120')
    expect(printed(120.6)).toBe('121')
  })

  // 99.96 would round to "100.0" with one decimal, so it takes the whole branch.
  it('never prints 100.0', () => {
    expect(printed(99.96)).toBe('100')
  })

  it('still localises the separator', () => {
    expect(printed(12.4, 'bg')).toBe('12,4')
  })
})

describe('the number halo', () => {
  it('is 2px wide in the stroke colour', () => {
    const paint = labelPaint(cfg)
    expect(paint['text-halo-width']).toBe(2)
    expect(paint['text-halo-color']).toBe('#ffffff')
    expect(paint['text-color']).toBe('#161616')
  })

  // The halo is what the digit sits on, so text against halo must clear AA.
  it('gives the text at least 4.5:1 against its halo', () => {
    const paint = labelPaint(cfg)
    expect(contrastRatio(paint['text-color'], paint['text-halo-color'])).toBeGreaterThanOrEqual(4.5)
  })
})
