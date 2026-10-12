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
  tSeaClassSourceDatahub: 'From the EEA {year} report', tSeaSamplesPending: 'Lab samples for {season} are not published yet.',
  tSeaSupplementNote: 'Dataset, edition {edition}, published {published}.',
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

  describe('datahub classes', () => {
    const datahub = {
      ...detail,
      classes: [{ season: 2025, quality: 'good', source: 'datahub' }, { season: 2024, quality: 'excellent', source: 'discodata' }],
      supplement: { edition: '2025 v1.0', published: '2026-09-30', url: 'https://example.test/d' },
    }
    const discodata = {
      ...detail,
      classes: detail.classes.map((c) => ({ ...c, source: 'discodata' })),
    }

    it('marks a datahub class and says its lab samples are pending', async () => {
      await opened(async () => datahub)
      expect(frame.querySelector('.sea-panel__class .sea-panel__source').textContent).toBe('From the EEA 2025 report')
      expect(frame.querySelector('.sea-panel__pending').textContent).toBe('Lab samples for 2025 are not published yet.')
      // The earlier discodata season carries no marker.
      expect(frame.querySelectorAll('.sea-panel__source')).toHaveLength(1)
      expect(frame.querySelector('.sea-panel__history .sea-panel__source')).toBeNull()
    })

    it('marks an earlier datahub season in the history', async () => {
      const older = { ...datahub, classes: [{ season: 2025, quality: 'good', source: 'discodata' }, { season: 2024, quality: 'good', source: 'datahub' }], supplement: undefined }
      await opened(async () => older)
      expect(frame.querySelector('.sea-panel__history .sea-panel__source').textContent).toBe('From the EEA 2024 report')
      expect(frame.querySelector('.sea-panel__pending')).toBeNull()
    })

    it('shows neither marker nor pending note for discodata classes', async () => {
      await opened(async () => discodata)
      expect(frame.querySelector('.sea-panel__source')).toBeNull()
      expect(frame.querySelector('.sea-panel__pending')).toBeNull()
    })

    it('shows the supplement note with a formatted date only when the API sends one', async () => {
      await opened(async () => datahub)
      expect(frame.querySelector('.sea-panel__supplement').textContent).toBe('Dataset, edition 2025 v1.0, published Sep 30, 2026.')
    })

    it('has no supplement note without a supplement', async () => {
      await opened(async () => discodata)
      expect(frame.querySelector('.sea-panel__supplement')).toBeNull()
    })

    it('keeps a published value that is not a date as it came', async () => {
      await opened(async () => ({ ...datahub, supplement: { ...datahub.supplement, published: 'autumn' } }))
      expect(frame.querySelector('.sea-panel__supplement').textContent).toContain('published autumn.')
    })

    it('drops the pending note once samples for that season exist', async () => {
      const sampled = { ...datahub, samples: [{ ...detail.samples[0], season: 2025, date: '2025-07-02' }] }
      await opened(async () => sampled)
      expect(frame.querySelector('.sea-panel__pending')).toBeNull()
    })
  })

  // Pinned from the panel before the lab-result block existed: a site with no RZI data renders as it did.
  it('renders a site without RZI data exactly as before', async () => {
    await opened(async () => detail)
    // Svelte's empty anchor comments are not content.
    expect(frame.querySelector('.sea-panel').outerHTML.replaceAll('<!---->', '')).toMatchInlineSnapshot(`"<section class="sea-panel" aria-labelledby="sea-panel-title"><header class="sea-panel__head"><h2 class="sea-panel__title" id="sea-panel-title">Zlatni Pyasatsi</h2> <button type="button" class="sea-panel__close" aria-label="Close"><svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true" focusable="false"><path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"></path></svg></button></header> <p class="sea-panel__zone">Sea</p> <p class="sea-panel__class"><svg class="sea-panel__swatch" viewBox="0 0 1 1" aria-hidden="true" focusable="false"><rect width="1" height="1" fill="#0b4f9c"></rect></svg> <strong>Excellent</strong> <span class="sea-panel__season">Class for 2024</span> </p> <h3 class="sea-panel__h">Earlier seasons</h3> <ul class="sea-panel__history"><li><svg class="sea-panel__swatch" viewBox="0 0 1 1" aria-hidden="true" focusable="false"><rect width="1" height="1" fill="#8e3a9c"></rect></svg>2023: Poor</li></ul>   <h3 class="sea-panel__h">Samples</h3>  <p class="sea-panel__limits">Limits, excellent / good (cfu/100 ml): E. coli 250 / 500, Enterococci 100 / 200</p> <div class="sea-panel__scroll"><table class="sea-panel__samples"><thead><tr><th scope="col">Date</th><th scope="col">E. coli</th><th scope="col">Enterococci</th></tr></thead><tbody><tr><th scope="row">Jul 2, 2024<span class="sea-panel__pre">pre-season</span></th><td>&lt;15 <span class="sr-only">, below the detection limit</span> </td><td>150<span aria-hidden="true" class="sea-panel__mark">▲</span>  <span class="sr-only">, above the excellent limit</span></td></tr><tr><th scope="row">Jul 3, 2023</th><td>600<span aria-hidden="true" class="sea-panel__mark">▲▲</span>  <span class="sr-only">, above the good limit</span></td><td>40  </td></tr></tbody></table></div> <p class="sea-panel__key"><span aria-hidden="true">▲</span> above the excellent limit · <span aria-hidden="true">▲▲</span> above the good limit</p> <p class="sea-panel__note">The class is the EEA assessment.</p> <p class="sea-panel__credit"><a rel="noopener noreferrer" target="_blank" href="https://www.eea.europa.eu/en/topics/in-depth/water/bathing-water">Data: EEA, CC BY 4.0</a> · <a rel="noopener noreferrer" target="_blank" href="https://example.org/p.pdf">Site profile</a></p></section>"`)
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
