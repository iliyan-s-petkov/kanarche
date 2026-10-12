// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { flushSync } from 'svelte'
import { createSeaPanel } from '../../lib/seapanel.svelte.js'
import { readSeaTexts, seaColours } from '../../lib/sea.js'

const base = {
  tSeaClassExcellent: 'Excellent', tSeaClassPoor: 'Poor', tSeaClassGood: 'Good',
  tSeaZoneCoastal: 'Sea', tSeaSeason: 'Class for {season}', tSeaHistory: 'Earlier seasons',
  tSeaSamples: 'Samples', tSeaDate: 'Date', tSeaEColi: 'E. coli', tSeaEnterococci: 'Enterococci',
  tSeaUnit: 'cfu/100 ml', tSeaLimits: 'Limits, excellent / good',
  tSeaOverExcellent: 'above the excellent limit', tSeaOverGood: 'above the good limit',
  tSeaBelowDetection: 'below the detection limit', tSeaPreSeason: 'pre-season',
  tSeaNoSamples: 'No samples.', tSeaNote: 'The class is the EEA assessment.',
  tSeaCredit: 'Data: EEA, CC BY 4.0', tSeaProfile: 'Site profile', tSeaClose: 'Close', tSeaFailed: 'Could not load.',
  tSeaLatestHeading: 'Latest lab result',
  tSeaLatestSingleSampleNote: 'A single sample, not a class.',
  tSeaLatestStale: 'No sample in the last 21 days. Latest: {date}.',
  tSeaLatestOffSeason: 'Season closed. Results of {year}.',
  tSeaCreditRzi: 'Source: {region}, sample of {date}',
  tSeaNoticeUnofficial: 'Not an official source.',
  tSeaSourceRziVarna: 'RHI Varna', tSeaSourceRziBurgas: 'RHI Burgas', tSeaSourceRziDobrich: 'RHI Dobrich',
  tSeaSourceLink: 'Results on the RHI site', tSeaUnitMpn: 'MPN/100 ml',
}
const cfgFor = (over = {}) => ({
  lang: 'en',
  seaColours: seaColours(['#0b4f9c', '#3a8fd9', '#8cc5e8', '#8e3a9c', '#9ca3af']),
  seaCreditURL: 'https://www.eea.europa.eu/en/topics/in-depth/water/bathing-water',
  t: { sea: readSeaTexts({ ...base, ...over }) },
})
const PAGE = 'https://rzi-varna.com/rezultati/'
const detail = {
  imported_at: '2026-10-04T03:00:00Z',
  site: { id: 'BG1', name_bg: 'ЗЛАТНИ ПЯСЪЦИ', name_en: 'ZLATNI PYASATSI', zone: 'coastal', lat: 43.3, lon: 28.05, profile_url: 'https://example.org/p.pdf' },
  limits: { e_coli: [250, 500], enterococci: [100, 200] },
  classes: [{ season: 2025, quality: 'excellent' }],
  samples: [{ date: '2025-07-02', season: 2025, e_coli: 15, e_coli_below_detection: true, enterococci: 150, enterococci_below_detection: false, pre_season: false }],
}
const latest = {
  date: '2026-08-12', source: 'rzi_varna', e_coli: 600, e_coli_below_detection: false,
  enterococci: 40, enterococci_below_detection: true, unit: 'MPN/100ml', stale: false,
}
const rziRow = { date: '2026-08-12', season: 2026, e_coli: 600, e_coli_below_detection: false, enterococci: 40, enterococci_below_detection: true, pre_season: false, source: 'rzi_varna' }
const withRZI = (over = {}) => ({
  ...detail,
  site: { ...detail.site, rzi: { source: 'rzi_varna', page_url: PAGE, fetched_at: '2026-08-13T05:00:00Z', season_open: true } },
  latest,
  samples: [{ ...rziRow, remarks: '<img src=x onerror=alert(1)>' }, { ...detail.samples[0], source: 'eea' }],
  ...over,
})

let frame
afterEach(() => { frame?.remove() })

async function opened(body, over) {
  frame = document.createElement('div')
  document.body.appendChild(frame)
  const panel = createSeaPanel(frame, cfgFor(over), async () => body)
  await panel.open('BG1')
  flushSync()
  return frame.querySelector('.sea-panel')
}

describe('SeaPanel latest lab result', () => {
  it('shows a separate block after the class block with both indicators coloured against the limits', async () => {
    const root = await opened(withRZI())
    const block = root.querySelector('.sea-panel__latest')
    expect(block.querySelector('h3').textContent).toBe('Latest lab result')
    // After the class paragraph, in document order.
    const cls = root.querySelector('.sea-panel__class')
    expect(cls.compareDocumentPosition(block) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(block.contains(cls)).toBe(false)
    const [ec, ie] = block.querySelectorAll('.sea-panel__latest-value')
    expect(ec.textContent).toContain('600')
    expect(ec.textContent).toContain('▲▲')
    expect(ec.textContent).toContain('above the good limit')
    expect(ie.textContent).toContain('<40')
    expect(ie.textContent).toContain('below the detection limit')
    expect(ie.textContent).not.toContain('▲')
    expect(block.textContent).toContain('MPN/100 ml')
    expect(block.textContent).toContain('A single sample, not a class.')
    expect(block.textContent).toContain('Not an official source.')
  })

  it('leaves the class block as it was', async () => {
    const root = await opened(withRZI())
    expect(root.querySelector('.sea-panel__class').textContent).toContain('Excellent')
    expect(root.querySelector('.sea-panel__class').textContent).toContain('Class for 2025')
  })

  it('credits the source and the sample date from the template', async () => {
    const root = await opened(withRZI())
    expect(root.querySelector('.sea-panel__latest .sea-panel__credit-rzi').textContent).toBe('Source: RHI Varna, sample of 12.08.2026')
  })

  it('renders no RZI value when the credit template is missing', async () => {
    const root = await opened(withRZI(), { tSeaCreditRzi: '' })
    expect(root.querySelector('.sea-panel__latest-value')).toBeNull()
    expect(root.textContent).not.toContain('600')
    expect(root.querySelector('.sea-panel__credit-rzi')).toBeNull()
    // No heading-only section either.
    expect(root.querySelector('.sea-panel__latest')).toBeNull()
    expect(root.querySelector('.sea-panel__row-credit')).toBeNull()
  })

  it('renders no RZI value when the source has no name', async () => {
    const root = await opened(withRZI({ latest: { ...latest, source: 'rzi_unknown' } }))
    expect(root.querySelector('.sea-panel__latest')).toBeNull()
  })

  it('links to the RZI page, never to a file', async () => {
    const root = await opened(withRZI())
    const a = root.querySelector('.sea-panel__latest a')
    expect(a.getAttribute('href')).toBe(PAGE)
    expect(a.textContent).toBe('Results on the RHI site')
    expect(a.getAttribute('rel')).toBe('noopener noreferrer')
  })

  it('does not link a page that is not http(s)', async () => {
    const body = withRZI()
    body.site.rzi.page_url = 'javascript:alert(1)'
    const root = await opened(body)
    expect(root.querySelector('.sea-panel__latest a')).toBeNull()
  })

  it('shows the stale state with the date and no values', async () => {
    const root = await opened(withRZI({ latest: { ...latest, stale: true } }))
    const block = root.querySelector('.sea-panel__latest')
    expect(block.querySelector('.sea-panel__stale').textContent).toBe('No sample in the last 21 days. Latest: 12.08.2026.')
    expect(block.querySelector('.sea-panel__latest-value')).toBeNull()
    expect(block.querySelector('.sea-panel__credit-rzi').textContent).toBe('Source: RHI Varna, sample of 12.08.2026')
  })

  it('shows the off-season state when only the RZI page is known', async () => {
    const body = withRZI({ latest: undefined })
    body.site.rzi.season_open = false
    const root = await opened(body)
    const block = root.querySelector('.sea-panel__latest')
    expect(block.querySelector('.sea-panel__off-season').textContent).toBe('Season closed. Results of 2026.')
    expect(block.querySelector('.sea-panel__latest-value')).toBeNull()
    expect(block.querySelector('a').getAttribute('href')).toBe(PAGE)
  })

  it('gives every RZI row its credit, and EEA rows none', async () => {
    const root = await opened(withRZI())
    const rows = [...root.querySelectorAll('.sea-panel__samples tbody tr')]
    expect(rows).toHaveLength(2)
    expect(rows[0].querySelector('.sea-panel__row-credit').textContent).toBe('Source: RHI Varna, sample of 12.08.2026')
    expect(rows[1].querySelector('.sea-panel__row-credit')).toBeNull()
  })

  it('credits each of several RZI rows with its own date and source', async () => {
    const body = withRZI()
    body.samples = [
      { ...rziRow, date: '2026-08-12' },
      { ...rziRow, date: '2026-07-29', source: 'rzi_burgas' },
    ]
    const root = await opened(body)
    const credits = [...root.querySelectorAll('.sea-panel__row-credit')].map((n) => n.textContent)
    expect(credits).toEqual(['Source: RHI Varna, sample of 12.08.2026', 'Source: RHI Burgas, sample of 29.07.2026'])
  })

  it('in season with prior-year rows only: no latest block, credited rows, no off-season text', async () => {
    const body = withRZI({ latest: undefined })
    body.samples = [{ ...rziRow, date: '2025-08-12', season: 2025 }]
    const root = await opened(body)
    expect(root.querySelector('.sea-panel__latest')).toBeNull()
    expect(root.textContent).not.toContain('Season closed')
    expect(root.querySelector('.sea-panel__row-credit').textContent).toBe('Source: RHI Varna, sample of 12.08.2025')
  })

  it('renders no empty paragraph when the stale or off-season template is empty', async () => {
    const off = withRZI({ latest: undefined })
    off.site.rzi.season_open = false
    const a = await opened(off, { tSeaLatestOffSeason: '' })
    expect(a.querySelector('.sea-panel__latest')).toBeNull()
    const b = await opened(withRZI({ latest: { ...latest, stale: true } }), { tSeaLatestStale: '' })
    expect(b.querySelector('.sea-panel__stale')).toBeNull()
    expect(b.querySelector('.sea-panel__latest')).toBeNull()
  })

  it('ties the unit to both lab values', async () => {
    const root = await opened(withRZI())
    const unit = root.querySelector('.sea-panel__latest-unit')
    for (const v of root.querySelectorAll('.sea-panel__latest-value')) {
      expect(v.getAttribute('aria-describedby')).toBe(unit.id)
    }
  })

  // I15: remarks are not in the view, and any text that looks like markup stays text.
  it('renders markup-like text as text and never shows remarks', async () => {
    const hostile = '<img src=x onerror=alert(1)>'
    const root = await opened(withRZI(), { tSeaSourceRziVarna: hostile })
    expect(root.querySelector('img')).toBeNull()
    expect(root.querySelector('.sea-panel__credit-rzi').textContent).toBe(`Source: ${hostile}, sample of 12.08.2026`)
    expect(root.querySelector('.sea-panel__row-credit').textContent).toBe(`Source: ${hostile}, sample of 12.08.2026`)
    const plain = await opened(withRZI())
    expect(plain.innerHTML).not.toContain('onerror')
  })

  it('shows no lab block and no source marker without RZI data', async () => {
    const root = await opened(detail)
    expect(root.querySelector('.sea-panel__latest')).toBeNull()
    expect(root.querySelector('.sea-panel__row-credit')).toBeNull()
    expect(root.textContent).not.toContain('Not an official source.')
  })
})
