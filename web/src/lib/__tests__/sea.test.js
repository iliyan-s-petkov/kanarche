import { describe, it, expect } from 'vitest'
import {
  SEA_CLASSES, seaClass, seaColours, seaFeatures, seaLevel, seaName, fillSeason, readSeaTexts,
} from '../sea.js'

const COLOURS = ['#0b4f9c', '#3a8fd9', '#8cc5e8', '#8e3a9c', '#9ca3af']

describe('seaClass', () => {
  it('passes the four directive classes through', () => {
    for (const q of ['excellent', 'good', 'sufficient', 'poor']) expect(seaClass(q)).toBe(q)
  })
  // The legacy combined class cannot claim "good"; it shares sufficient's marker.
  it('draws the legacy combined class as sufficient', () => {
    expect(seaClass('good_or_sufficient')).toBe('sufficient')
  })
  it('draws no class, and anything unknown, as not classified', () => {
    expect(seaClass(null)).toBe('not_classified')
    expect(seaClass('not_classified')).toBe('not_classified')
    expect(seaClass('splendid')).toBe('not_classified')
  })
})

describe('seaColours', () => {
  it('maps the positional list onto the classes in order', () => {
    const c = seaColours(COLOURS)
    expect(SEA_CLASSES.map((k) => c[k])).toEqual(COLOURS)
  })
})

describe('seaFeatures', () => {
  const body = {
    sites: [
      { id: 'BG1', lat: 43.3, lon: 28.05, zone: 'coastal', season: 2024, quality: 'poor' },
      { id: 'BG2', lat: 43.49, lon: 26.47, zone: 'lake', season: null, quality: null },
    ],
  }
  it('builds one point per site, coloured by its latest class', () => {
    const fc = seaFeatures(body, seaColours(COLOURS))
    expect(fc.type).toBe('FeatureCollection')
    expect(fc.features).toHaveLength(2)
    expect(fc.features[0].geometry).toEqual({ type: 'Point', coordinates: [28.05, 43.3] })
    expect(fc.features[0].properties).toEqual({ id: 'BG1', quality: 'poor', colour: '#8e3a9c' })
    expect(fc.features[1].properties.colour).toBe('#9ca3af')
  })
  it('is empty for a missing body', () => {
    expect(seaFeatures(null, seaColours(COLOURS)).features).toEqual([])
  })
})

describe('seaLevel', () => {
  const limits = [250, 500]
  it('marks a sample by the limits it is over', () => {
    expect(seaLevel(250, false, limits)).toBe(0)
    expect(seaLevel(251, false, limits)).toBe(1)
    expect(seaLevel(500, false, limits)).toBe(1)
    expect(seaLevel(501, false, limits)).toBe(2)
  })
  // A censored value is the detection limit, not a measurement over anything.
  it('never marks a value below detection', () => {
    expect(seaLevel(1000, true, limits)).toBe(0)
  })
})

describe('seaName', () => {
  const site = { name_bg: 'ЗЛАТНИ ПЯСЪЦИ-ПСОВ', name_en: 'ZLATNI PYASATSI-PSOV' }
  it('picks the page language and drops the register capitals', () => {
    expect(seaName(site, 'bg')).toBe('Златни Пясъци-Псов')
    expect(seaName(site, 'en')).toBe('Zlatni Pyasatsi-Psov')
  })
  it('falls back to the other name when one is empty', () => {
    expect(seaName({ name_bg: '', name_en: 'VARNA' }, 'bg')).toBe('Varna')
  })
})

describe('fillSeason', () => {
  it('substitutes the season', () => {
    expect(fillSeason('Class for {season}', 2024)).toBe('Class for 2024')
  })
})

describe('readSeaTexts', () => {
  it('groups the data-t-sea-* attributes', () => {
    const t = readSeaTexts({
      tSeaToggle: 'Bathing water', tSeaClassPoor: 'Poor', tSeaZoneLake: 'Lake', tSeaEColi: 'E. coli',
    })
    expect(t.toggle).toBe('Bathing water')
    expect(t.classes.poor).toBe('Poor')
    expect(t.classes.excellent).toBe('')
    expect(t.zones.lake).toBe('Lake')
    expect(t.eColi).toBe('E. coli')
  })
})
