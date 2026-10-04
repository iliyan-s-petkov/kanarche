// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { flushSync } from 'svelte'
import { createSeaPanel } from '../../lib/seapanel.svelte.js'
import { readSeaTexts, seaColours } from '../../lib/sea.js'

const t = readSeaTexts({
  tSeaClassExcellent: 'Excellent', tSeaClassPoor: 'Poor', tSeaClassGood: 'Good',
  tSeaZoneCoastal: 'Sea', tSeaSeason: 'Class for {season}', tSeaHistory: 'Earlier seasons',
  tSeaSamples: 'Samples', tSeaDate: 'Date', tSeaEColi: 'E. coli', tSeaEnterococci: 'Enterococci',
  tSeaUnit: 'cfu/100 ml', tSeaLimits: 'Limits, excellent / good',
  tSeaOverExcellent: 'above the excellent limit', tSeaOverGood: 'above the good limit',
  tSeaBelowDetection: 'below the detection limit', tSeaPreSeason: 'pre-season',
  tSeaNoSamples: 'No samples.', tSeaNote: 'The class is the EEA assessment.',
  tSeaCredit: 'Data: EEA, CC BY 4.0', tSeaProfile: 'Site profile', tSeaClose: 'Close',
  tSeaFailed: 'Could not load.',
})
const cfg = {
  lang: 'en',
  seaColours: seaColours(['#0b4f9c', '#3a8fd9', '#8cc5e8', '#8e3a9c', '#9ca3af']),
  seaCreditURL: 'https://www.eea.europa.eu/en/topics/in-depth/water/bathing-water',
  t: { sea: t },
}
const detail = {
  imported_at: '2026-10-04T03:00:00Z',
  site: { id: 'BG1', name_bg: 'ЗЛАТНИ ПЯСЪЦИ', name_en: 'ZLATNI PYASATSI', zone: 'coastal', lat: 43.3, lon: 28.05, profile_url: 'https://example.org/p.pdf' },
  limits: { e_coli: [250, 500], enterococci: [100, 200] },
  classes: [{ season: 2024, quality: 'excellent' }, { season: 2023, quality: 'poor' }],
  samples: [
    { date: '2024-07-02', season: 2024, e_coli: 15, e_coli_below_detection: true, enterococci: 150, enterococci_below_detection: false, pre_season: true },
    { date: '2023-07-03', season: 2023, e_coli: 600, e_coli_below_detection: false, enterococci: 40, enterococci_below_detection: false, pre_season: false },
  ],
}

let frame
afterEach(() => { frame?.remove() })

async function opened(fetchJSON) {
  frame = document.createElement('div')
  document.body.appendChild(frame)
  const panel = createSeaPanel(frame, cfg, fetchJSON)
  await panel.open('BG1')
  flushSync()
  return panel
}

describe('SeaPanel', () => {
  it('shows the name, zone and current class with its season', async () => {
    await opened(async () => detail)
    const root = frame.querySelector('.sea-panel')
    expect(root.querySelector('h2').textContent).toBe('Zlatni Pyasatsi')
    expect(root.textContent).toContain('Sea')
    expect(root.querySelector('.sea-panel__class').textContent).toContain('Excellent')
    expect(root.querySelector('.sea-panel__class').textContent).toContain('Class for 2024')
    expect(root.querySelector('.sea-panel__history').textContent).toContain('2023')
    expect(root.querySelector('.sea-panel__history').textContent).toContain('Poor')
  })

  it('fetches the site by id', async () => {
    const fetchJSON = vi.fn(async () => detail)
    await opened(fetchJSON)
    expect(fetchJSON).toHaveBeenCalledWith('/api/v1/sea/sites/BG1')
  })

  // Marks are text, not colour: ▲ over the excellent limit, ▲▲ over the good one.
  it('marks each sample against the zone limits in text', async () => {
    await opened(async () => detail)
    const rows = [...frame.querySelectorAll('.sea-panel__samples tbody tr')]
    expect(rows).toHaveLength(2)
    const [ec2024, ie2024] = rows[0].querySelectorAll('td')
    expect(ec2024.textContent).toContain('<15')
    expect(ec2024.textContent).toContain('below the detection limit')
    expect(ie2024.textContent).toContain('150')
    expect(ie2024.textContent).toContain('▲')
    expect(ie2024.textContent).toContain('above the excellent limit')
    expect(ie2024.textContent).not.toContain('▲▲')
    const ec2023 = rows[1].querySelectorAll('td')[0]
    expect(ec2023.textContent).toContain('▲▲')
    expect(ec2023.textContent).toContain('above the good limit')
    expect(rows[0].textContent).toContain('pre-season')
    expect(frame.querySelector('.sea-panel__limits').textContent).toContain('250 / 500')
    expect(frame.querySelector('.sea-panel__limits').textContent).toContain('100 / 200')
  })

  it('credits the EEA and links the site profile', async () => {
    await opened(async () => detail)
    const links = [...frame.querySelectorAll('.sea-panel a')].map((a) => a.getAttribute('href'))
    expect(links).toContain(cfg.seaCreditURL)
    expect(links).toContain('https://example.org/p.pdf')
    expect(frame.textContent).toContain('Data: EEA, CC BY 4.0')
    expect(frame.textContent).toContain('The class is the EEA assessment.')
  })

  it('says so when the site cannot be loaded', async () => {
    await opened(async () => { throw new Error('404') })
    expect(frame.querySelector('.sea-panel').textContent).toContain('Could not load.')
  })

  it('closes from its button and from Escape', async () => {
    const panel = await opened(async () => detail)
    frame.querySelector('.sea-panel__close').click()
    flushSync()
    expect(frame.querySelector('.map-sea').hidden).toBe(true)
    expect(panel.id).toBeNull()
    await panel.open('BG1')
    flushSync()
    expect(frame.querySelector('.map-sea').hidden).toBe(false)
    frame.querySelector('.sea-panel').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    flushSync()
    expect(frame.querySelector('.map-sea').hidden).toBe(true)
  })
})
