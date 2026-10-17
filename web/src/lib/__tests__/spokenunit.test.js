// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest'
import { readSpokenUnits, spokenUnit } from '../spokenunit.js'

const publish = (table) => {
  document.head.innerHTML = `<meta name="spoken-units" content='${JSON.stringify(table)}'>`
}

describe('spokenUnit', () => {
  beforeEach(() => { document.head.innerHTML = '' })

  it('swaps a symbol for the published words', () => {
    publish({ 'µg/m³': 'micrograms per cubic metre', '°C': 'degrees Celsius' })
    expect(spokenUnit('µg/m³')).toBe('micrograms per cubic metre')
    expect(spokenUnit('°C')).toBe('degrees Celsius')
  })

  it('keeps a symbol it has no words for', () => {
    publish({ 'µg/m³': 'micrograms per cubic metre' })
    expect(spokenUnit('MPN/100 ml')).toBe('MPN/100 ml')
  })

  it('falls back to the symbol when the page published nothing', () => {
    expect(spokenUnit('µg/m³')).toBe('µg/m³')
  })

  it('survives a malformed table', () => {
    document.head.innerHTML = '<meta name="spoken-units" content="{oops">'
    expect(readSpokenUnits()).toEqual({})
    expect(spokenUnit('hPa')).toBe('hPa')
  })

  it('says nothing for a missing unit', () => {
    publish({ 'µg/m³': 'x' })
    expect(spokenUnit('')).toBe('')
    expect(spokenUnit(undefined)).toBe('')
  })
})
