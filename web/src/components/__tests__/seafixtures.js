import { readSeaTexts, seaColours } from '../../lib/sea.js'

// Attributes as the templates deliver them. Wording equals the Task 6 draft; tests assert structure, not wording.
export const textAttrs = {
  tSeaToggle: 'Bathing water', tSeaLegend: 'Bathing water, EEA class',
  tSeaClassExcellent: 'Excellent', tSeaClassGood: 'Good', tSeaClassSufficient: 'Sufficient', tSeaClassPoor: 'Poor',
  tSeaClassNotClassified: 'Not classified', tSeaClassGoodOrSufficient: 'Good or sufficient',
  tSeaZoneCoastal: 'Sea', tSeaZoneLake: 'Lake', tSeaSeason: 'Class for {season}',
  tSeaSamples: 'Samples', tSeaDate: 'Date', tSeaEColi: 'E. coli', tSeaEnterococci: 'Enterococci',
  tSeaUnit: 'cfu/100 ml', tSeaLimits: 'Limits, excellent / good',
  tSeaOverExcellent: 'above the excellent limit', tSeaOverGood: 'above the good limit',
  tSeaBelowDetection: 'below the detection limit', tSeaPreSeason: 'pre-season',
  tSeaNoSamples: 'No samples published for this site.', tSeaNote: 'The class is the EEA assessment over the last four seasons.',
  tSeaCredit: 'Data: EEA, CC BY 4.0', tSeaProfile: 'Site profile', tSeaClose: 'Close', tSeaFailed: 'Could not load.',
  tSeaClassSourceDatahub: 'From the EEA {year} report', tSeaSamplesPending: 'Lab samples for {season} are not published yet.',
  tSeaSupplementNote: 'Dataset, edition {edition}, published {published}.',
  tSeaLatestHeading: 'Latest lab result', tSeaLatestSingleSampleNote: 'A single sample, not a class.',
  tSeaLatestStale: 'No sample in the last 21 days. Latest: {date}.', tSeaLatestOffSeason: 'Season closed. Results of {year}.',
  tSeaCreditRzi: 'Source: {region}, sample of {date}', tSeaNoticeUnofficial: 'Not an official source.',
  tSeaSourceRziVarna: 'RHI Varna', tSeaSourceRziBurgas: 'RHI Burgas', tSeaSourceRziDobrich: 'RHI Dobrich',
  tSeaSourceLink: 'Results on the RHI site', tSeaUnitMpn: 'MPN/100 ml', tSeaNewTab: 'opens in a new tab',
  tSeaInfoLabel: 'What the bathing water classes mean', tSeaInfoTitle: 'Bathing water class, EEA',
  tSeaInfoBody: 'The class is the EU classification.', tSeaInfoRzi: 'Lab results from the RHI sites.',
  tSeaInfoLinkMap: 'EEA map', tSeaInfoLinkEea: 'EEA, bathing water', tSeaInfoLinkDatahub: 'EEA dataset',
  // New in the redesign.
  tSeaInitialExcellent: 'E', tSeaInitialGood: 'G', tSeaInitialSufficient: 'S', tSeaInitialPoor: 'P',
  tSeaOfficialClass: 'Official class for {season}',
  tSeaChipsHeading: 'Latest lab samples', tSeaChipsKeyWithin: 'within the excellent limit',
  tSeaChipsNone: 'No lab samples this season', tSeaChipsInfoLabel: 'About the lab samples',
  tSeaSeasonsSince: 'Seasons since {first}', tSeaDetails: 'Details and charts', tSeaBack: 'Back',
  tSeaTabsLabel: 'Details sections', tSeaTabsSamples: 'Samples', tSeaTabsSeasons: 'Seasons', tSeaTabsAbout: 'About',
  tSeaMetricLabel: 'Indicator', tSeaSeasonPicker: 'Season',
  tSeaChartSummary: '{metric}, season {season}. Samples: {count}. Above the excellent limit: {over_excellent}. Above the good limit: {over_good}.',
  tSeaChartCaption: '{metric}, season {season}',
  tSeaChartShowTable: 'Show as table', tSeaChartShowChart: 'Show as chart',
  tSeaChartBandWithin: 'Within the excellent limit', tSeaChartBandBetween: 'Between the excellent and good limits',
  tSeaChartBandAbove: 'Above the good limit', tSeaChartHollow: 'Hollow dot: below the detection limit',
  tSeaCreditSource: 'Source: {region}', tSeaSourcesEea: 'EEA', tSeaSourcesInfoLabel: 'About the sources and licence',
}
export const seaT = readSeaTexts(textAttrs)
export const textsWith = (over = {}) => readSeaTexts({ ...textAttrs, ...over })

export const COLOURS = ['#0b4f9c', '#3a8fd9', '#8cc5e8', '#8e3a9c', '#9ca3af']
export const colours = seaColours(COLOURS)
export const limits = { e_coli: [250, 500], enterococci: [100, 200] }

export const sample = (over = {}) => ({
  date: '2024-07-02', season: 2024, e_coli: 15, e_coli_below_detection: false,
  enterococci: 40, enterococci_below_detection: false, pre_season: false, ...over,
})

// Newest first, as the API sends it.
export const detail = {
  imported_at: '2026-10-04T03:00:00Z',
  site: { id: 'BG1', name_bg: 'ЗЛАТНИ ПЯСЪЦИ', name_en: 'ZLATNI PYASATSI', zone: 'coastal', lat: 43.3, lon: 28.05, profile_url: 'https://example.org/p.pdf' },
  limits,
  classes: [{ season: 2024, quality: 'excellent', source: '' }, { season: 2023, quality: 'poor', source: '' }],
  samples: [
    sample({ date: '2024-07-02', e_coli: 15, e_coli_below_detection: true, enterococci: 150, pre_season: true }),
    sample({ date: '2023-07-03', season: 2023, e_coli: 600, enterococci: 40 }),
  ],
}
