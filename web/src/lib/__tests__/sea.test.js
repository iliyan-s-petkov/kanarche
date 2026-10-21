import { describe, it, expect } from 'vitest'
import {
  SEA_CLASSES, seaClass, seaColours, seaFeatures, seaLevel, seaName, fillSeason, fillSlots, readSeaTexts, safePageURL,
  SEA_METRICS, LEVEL_MARKS, seasonsWithSamples, samplePoints, chipLevel, classInitial, classLabel,
  sourceCredit, creditedSamples, latestSamples, sampleSources, seasonCells, cellName, inkFor, nextTab,
  pendingSeason, formatDay, levelText, httpURL, offSeasonText, rhiName, remarkedSamples,
} from '../sea.js'
import { seaT, textsWith, limits, sample, textAttrs } from '../../components/__tests__/seafixtures.js'

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

describe('fillSlots', () => {
  it('substitutes every named slot and leaves unknown ones', () => {
    expect(fillSlots('Edition {edition}, published {published} {x}', { edition: '2025 v1.0', published: '30 Sept 2026' }))
      .toBe('Edition 2025 v1.0, published 30 Sept 2026 {x}')
  })
})

describe('readSeaTexts datahub copy', () => {
  it('reads the datahub keys, and the legend link under info', () => {
    const t = readSeaTexts({
      tSeaClassSourceDatahub: 'From the EEA {year} report', tSeaSamplesPending: 'Pending {season}',
      tSeaSupplementNote: 'Edition {edition}', tSeaInfoLinkDatahub: 'Dataset',
    })
    expect(t.classSourceDatahub).toBe('From the EEA {year} report')
    expect(t.samplesPending).toBe('Pending {season}')
    expect(t.supplementNote).toBe('Edition {edition}')
    expect(t.info.linkDatahub).toBe('Dataset')
  })
  it('reads missing keys as empty text', () => {
    const t = readSeaTexts({})
    expect([t.classSourceDatahub, t.samplesPending, t.supplementNote, t.info.linkDatahub]).toEqual(['', '', '', ''])
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

describe('safePageURL', () => {
  it('keeps an http(s) page and returns the parsed href', () => {
    expect(safePageURL('https://rzi-varna.com/rezultati/')).toBe('https://rzi-varna.com/rezultati/')
    expect(safePageURL('http://example.org')).toBe('http://example.org/')
  })
  it.each([
    ' javascript:alert(1)', 'JAVASCRIPT:alert(1)', 'javascript:alert(1)', '//evil.com/x', 'data:text/html,<b>x</b>',
    '/\\evil.com', '\\\\evil.com\\x', 'ftp://example.org/', 'https://user:pw@example.org/', 'https://user@example.org/', '', null, undefined,
    'https://rzi-varna.com/files/r.pdf', 'https://rzi-varna.com/r.XLS', 'https://rzi-varna.com/r.xlsx?x=1',
    'https://rzi-varna.com/r.csv', 'https://rzi-varna.com/r.doc', 'https://rzi-varna.com/r.docx',
  ])('rejects %s', (u) => {
    expect(safePageURL(u)).toBe('')
  })
})

describe('seasonsWithSamples', () => {
  it('lists each season once, newest first', () => {
    expect(seasonsWithSamples([{ season: 2023 }, { season: 2024 }, { season: 2023 }])).toEqual([2024, 2023])
  })
  it('is empty for no samples and for null', () => {
    expect(seasonsWithSamples([])).toEqual([])
    expect(seasonsWithSamples(null)).toEqual([])
  })
})

describe('samplePoints', () => {
  const rows = [
    sample({ date: '2024-07-02', e_coli: 15, e_coli_below_detection: true, enterococci: 150 }),
    sample({ date: '2024-05-20', e_coli: 600, enterococci: 20, pre_season: true, source: 'rzi_varna' }),
    sample({ date: '2023-07-03', season: 2023, e_coli: 9, enterococci: 9 }),
  ]
  it('keeps one season, oldest first, with the chosen metric', () => {
    expect(samplePoints(rows, 2024, 'e_coli')).toEqual([
      { date: '2024-05-20', value: 600, below: false, preSeason: true, source: 'rzi_varna' },
      { date: '2024-07-02', value: 15, below: true, preSeason: false, source: '' },
    ])
    expect(samplePoints(rows, 2024, 'enterococci').map((p) => p.value)).toEqual([20, 150])
  })
  it('is empty for an unknown season and for null', () => {
    expect(samplePoints(rows, 1999, 'e_coli')).toEqual([])
    expect(samplePoints(null, 2024, 'e_coli')).toEqual([])
  })
})

describe('chipLevel', () => {
  const at = (ec, ie, over = {}) => sample({ e_coli: ec, enterococci: ie, ...over })
  it('treats a value exactly on a limit as within', () => {
    expect(chipLevel(at(250, 100), limits)).toBe(0)
  })
  it('is 1 just over excellent and 2 just over good', () => {
    expect(chipLevel(at(251, 10), limits)).toBe(1)
    expect(chipLevel(at(500, 10), limits)).toBe(1)
    expect(chipLevel(at(501, 10), limits)).toBe(2)
  })
  it('takes the worse of the two indicators', () => {
    expect(chipLevel(at(600, 150), limits)).toBe(2)
    expect(chipLevel(at(10, 150), limits)).toBe(1)
    expect(chipLevel(at(10, 201), limits)).toBe(2)
  })
  it('counts a value below detection as within, whatever the number', () => {
    expect(chipLevel(at(9999, 9999, { e_coli_below_detection: true, enterococci_below_detection: true }), limits)).toBe(0)
  })
})

describe('classInitial and classLabel', () => {
  it('reads the initial for each class', () => {
    expect(['excellent', 'good', 'sufficient', 'poor'].map((q) => classInitial(q, seaT))).toEqual(['E', 'G', 'S', 'P'])
  })
  it('gives the legacy combined class the sufficient initial and its own full label', () => {
    expect(classInitial('good_or_sufficient', seaT)).toBe('S')
    expect(classLabel('good_or_sufficient', seaT)).toBe('Good or sufficient')
  })
  it('reads no class and unknown classes as not classified, with no initial', () => {
    expect(classInitial(null, seaT)).toBe('')
    expect(classInitial('splendid', seaT)).toBe('')
    expect(classLabel(null, seaT)).toBe('Not classified')
  })
})

describe('source credits', () => {
  const rzi = sample({ date: '2026-08-12', season: 2026, source: 'rzi_varna' })
  const eea = sample({ date: '2024-07-02' })
  it('credits EEA with the EEA line and an RZI source with its region', () => {
    expect(sourceCredit(seaT, '')).toBe('Data: EEA, CC BY 4.0')
    expect(sourceCredit(seaT, 'eea')).toBe('Data: EEA, CC BY 4.0')
    expect(sourceCredit(seaT, 'rzi_varna')).toBe('Source: RHI Varna')
  })
  it('has no credit for an RZI source without a name or without the template', () => {
    expect(sourceCredit(seaT, 'rzi_unknown')).toBe('')
    expect(sourceCredit(textsWith({ tSeaCreditSource: '' }), 'rzi_varna')).toBe('')
  })
  it('drops an RZI sample that cannot be credited and keeps the EEA one', () => {
    expect(creditedSamples([rzi, eea], textsWith({ tSeaCreditSource: '' }))).toEqual([eea])
    expect(creditedSamples([rzi, eea], seaT)).toEqual([rzi, eea])
    expect(creditedSamples(null, seaT)).toEqual([])
  })
  it('lists each source once, in order of first appearance', () => {
    const rows = [rzi, sample({ date: '2026-07-29', source: 'rzi_varna' }), eea, sample({ date: '2023-07-03', source: 'eea' })]
    expect(sampleSources(rows, seaT)).toEqual([
      { id: 'rzi_varna', name: 'RHI Varna', credit: 'Source: RHI Varna' },
      { id: 'eea', name: 'EEA', credit: 'Data: EEA, CC BY 4.0' },
    ])
  })
})

describe('latestSamples', () => {
  it('returns the newest three, newest first, whatever the input order', () => {
    const rows = ['2024-05-01', '2024-07-01', '2023-07-01', '2024-06-01'].map((date) => sample({ date }))
    expect(latestSamples(rows, seaT).map((s) => s.date)).toEqual(['2024-07-01', '2024-06-01', '2024-05-01'])
  })
  it('returns fewer when fewer exist and nothing for null', () => {
    expect(latestSamples([sample()], seaT)).toHaveLength(1)
    expect(latestSamples(null, seaT)).toEqual([])
  })
})

describe('seasonCells', () => {
  it('runs oldest to newest and fills a missing year as not classified', () => {
    const cells = seasonCells([{ season: 2025, quality: 'excellent', source: '' }, { season: 2023, quality: 'poor', source: 'datahub' }])
    expect(cells).toEqual([
      { season: 2023, quality: 'poor', source: 'datahub' },
      { season: 2024, quality: null, source: '' },
      { season: 2025, quality: 'excellent', source: '' },
    ])
  })
  it('is empty without classes', () => {
    expect(seasonCells([])).toEqual([])
    expect(seasonCells(null)).toEqual([])
  })
  it('names a cell by year and class label, the legacy class by its full label', () => {
    expect(cellName({ season: 2019, quality: 'excellent' }, seaT)).toBe('2019 Excellent')
    expect(cellName({ season: 2018, quality: null }, seaT)).toBe('2018 Not classified')
    expect(cellName({ season: 2017, quality: 'good_or_sufficient' }, seaT)).toBe('2017 Good or sufficient')
  })
})

describe('inkFor', () => {
  it('picks the letter colour that reads on the swatch', () => {
    expect(inkFor('#0b4f9c')).toBe('light')
    expect(inkFor('#8e3a9c')).toBe('light')
    expect(inkFor('#3a8fd9')).toBe('dark')
    expect(inkFor('#8cc5e8')).toBe('dark')
    expect(inkFor('#9ca3af')).toBe('dark')
  })
  it('falls back to dark for a value that is not a six-digit hex', () => {
    expect(inkFor('')).toBe('dark')
    expect(inkFor(undefined)).toBe('dark')
    expect(inkFor('blue')).toBe('dark')
  })
})

describe('nextTab', () => {
  it('moves with the arrows and wraps at both ends', () => {
    expect(nextTab(0, 'ArrowRight', 3)).toBe(1)
    expect(nextTab(2, 'ArrowRight', 3)).toBe(0)
    expect(nextTab(1, 'ArrowLeft', 3)).toBe(0)
    expect(nextTab(0, 'ArrowLeft', 3)).toBe(2)
  })
  it('jumps with Home and End', () => {
    expect(nextTab(2, 'Home', 3)).toBe(0)
    expect(nextTab(0, 'End', 3)).toBe(2)
  })
  it('ignores every other key', () => {
    expect(nextTab(1, 'Enter', 3)).toBeNull()
    expect(nextTab(1, 'a', 3)).toBeNull()
  })
})

describe('pendingSeason', () => {
  const datahub = { classes: [{ season: 2025, quality: 'good', source: 'datahub' }], samples: [sample({ season: 2024 })] }
  it('names the season of a datahub class that has no samples yet', () => {
    expect(pendingSeason(datahub)).toBe(2025)
  })
  it('is null once that season has a sample, for other sources and without a class', () => {
    expect(pendingSeason({ ...datahub, samples: [sample({ season: 2025 })] })).toBeNull()
    expect(pendingSeason({ ...datahub, classes: [{ season: 2025, quality: 'good', source: '' }] })).toBeNull()
    expect(pendingSeason({ classes: [], samples: [] })).toBeNull()
    expect(pendingSeason(null)).toBeNull()
  })
})

describe('formatDay', () => {
  it('formats an ISO day in the page language as UTC', () => {
    expect(formatDay('2026-08-12', 'en')).toBe('Aug 12, 2026')
  })
  it('returns anything else as sent', () => {
    expect(formatDay('autumn', 'en')).toBe('autumn')
    expect(formatDay(null, 'en')).toBe('')
  })
})

describe('constants', () => {
  it('fixes the metric order and the marks', () => {
    expect(SEA_METRICS).toEqual(['e_coli', 'enterococci'])
    expect(LEVEL_MARKS).toEqual(['✓', '▲', '▲▲'])
  })
})

describe('readSeaTexts redesign copy', () => {
  it('reads every new attribute into its field', () => {
    const t = readSeaTexts(textAttrs)
    expect(t.initials).toEqual({ excellent: 'E', good: 'G', sufficient: 'S', poor: 'P' })
    expect(t.officialClass).toBe('Official class for {season}')
    expect(t.chips).toEqual({ heading: 'Latest lab samples', keyWithin: 'within the excellent limit', none: 'No lab samples this season', infoLabel: 'About the lab samples' })
    expect([t.seasonsSince, t.details, t.back, t.metricLabel, t.seasonPicker]).toEqual(['Seasons since {first}', 'Details and charts', 'Back', 'Indicator', 'Season'])
    expect(t.tabs).toEqual({ label: 'Details sections', samples: 'Samples', seasons: 'Seasons', about: 'About' })
    expect(Object.keys(t.chart).sort()).toEqual(['bandAbove', 'bandBetween', 'bandWithin', 'caption', 'hollow', 'showChart', 'showTable', 'summary'])
    expect([t.creditSource, t.sourceEea, t.sourcesInfoLabel]).toEqual(['Source: {region}', 'EEA', 'About the sources and licence'])
  })
  it('reads missing attributes as empty text, never undefined', () => {
    const t = readSeaTexts({})
    expect(t.initials.excellent).toBe('')
    expect(t.chips.heading).toBe('')
    expect(t.tabs.about).toBe('')
    expect(t.chart.summary).toBe('')
    expect(t.creditSource).toBe('')
  })
})

describe('levelText', () => {
  it('names the level, with the given text for within', () => {
    expect(levelText(seaT, 2)).toBe(seaT.overGood)
    expect(levelText(seaT, 1)).toBe(seaT.overExcellent)
    expect(levelText(seaT, 0)).toBe('')
    expect(levelText(seaT, 0, 'within')).toBe('within')
  })
})

describe('httpURL', () => {
  it('passes http and https only', () => {
    expect(httpURL('https://example.org/p.pdf')).toBe('https://example.org/p.pdf')
    expect(httpURL('http://example.org/')).toBe('http://example.org/')
    expect(httpURL('javascript:alert(1)')).toBe('')
    expect(httpURL(undefined)).toBe('')
    expect(httpURL('nope')).toBe('')
  })
})

describe('offSeasonText', () => {
  const rzi = [sample({ date: '2025-07-02', season: 2025, source: 'rzi_varna' })]
  const closed = { site: { rzi: { season_open: false } }, latest: null }
  it('names the year of the newest RZI sample while the season is closed', () => {
    expect(offSeasonText(seaT, closed, rzi)).toBe(seaT.latest.offSeason.replace('{year}', '2025'))
  })
  it('is empty while open, with a latest result, with no RZI sample, or with no template', () => {
    expect(offSeasonText(seaT, { site: { rzi: { season_open: true } }, latest: null }, rzi)).toBe('')
    expect(offSeasonText(seaT, { ...closed, latest: {} }, rzi)).toBe('')
    expect(offSeasonText(seaT, closed, [])).toBe('')
    expect(offSeasonText({ ...seaT, latest: { ...seaT.latest, offSeason: '' } }, closed, rzi)).toBe('')
  })
})

describe('RZI remarks', () => {
  it('names the inspectorate without its RHI or РЗИ prefix', () => {
    expect(rhiName(seaT, 'rzi_varna')).toBe('Varna')
    expect(rhiName(textsWith({ tSeaSourceRziBurgas: 'РЗИ Бургас' }), 'rzi_burgas')).toBe('Бургас')
    expect(rhiName(seaT, 'rzi_unknown')).toBe('')
  })

  it('keeps only RZI samples with a non-blank remark', () => {
    const rzi = sample({ source: 'rzi_varna', remarks: 'rain' })
    const rows = [rzi, sample({ source: 'rzi_varna', remarks: '  ' }), sample({ source: 'rzi_varna' }), sample({ source: 'eea', remarks: 'x' })]
    expect(remarkedSamples(rows, seaT)).toEqual([rzi])
    expect(remarkedSamples(rows, textsWith({ tSeaRemarkCredit: '' }))).toEqual([])
  })
})
