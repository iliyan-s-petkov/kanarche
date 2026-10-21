// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { mount, unmount, flushSync } from 'svelte'
import SeaPanel from '../SeaPanel.svelte'
import { seaT, textsWith, colours, detail, sample } from './seafixtures.js'

const PAGE = 'https://rzi-varna.com/rezultati/'
const latest = {
  date: '2026-08-12', source: 'rzi_varna', e_coli: 600, e_coli_below_detection: false,
  enterococci: 40, enterococci_below_detection: true, unit: 'MPN/100ml', stale: false,
}
const rziRow = sample({ date: '2026-08-12', season: 2026, e_coli: 600, enterococci: 40, enterococci_below_detection: true, source: 'rzi_varna' })
const withRZI = (over = {}) => ({
  ...detail,
  site: { ...detail.site, rzi: { source: 'rzi_varna', page_url: PAGE, fetched_at: '2026-08-13T05:00:00Z', season_open: true } },
  latest,
  samples: [{ ...rziRow, remarks: '<img src=x onerror=alert(1)>' }, { ...detail.samples[0], source: 'eea' }],
  ...over,
})

let app
afterEach(() => {
  if (app) unmount(app)
  app = null
  document.body.innerHTML = ''
})
function render(body, t = seaT, extra = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  app = mount(SeaPanel, { target, props: { view: { id: 'BG1', detail: body, failed: false }, t, lang: 'en', colours, creditURL: 'https://www.eea.europa.eu/', onclose: () => {}, ...extra } })
  flushSync()
  return target.querySelector('.sea-panel')
}
const chipDates = (root) => [...root.querySelectorAll('.sea-chip__date')].map((n) => n.textContent)

describe('SeaPanel RZI samples', () => {
  it('shows an RZI sample as a chip with the mark of its worse indicator and names its source', () => {
    const root = render(withRZI())
    expect(chipDates(root)).toEqual(['Aug 12, 2026'])
    expect(root.querySelector('.sea-chip__mark').textContent).toBe('▲▲')
    expect(root.querySelector('.sea-panel__sources span').textContent).toBe('EEA, RHI Varna')
  })

  it('shows no RZI chip when the credit template is missing', () => {
    const root = render(withRZI({ samples: [rziRow, { ...rziRow, date: '2026-07-01', source: 'eea' }] }), textsWith({ tSeaCreditSource: '' }))
    expect(chipDates(root)).toEqual(['Jul 1, 2026'])
    expect(root.querySelector('.sea-panel__sources span').textContent).toBe('EEA')
  })

  it('shows no RZI chip when the source has no name', () => {
    const root = render(withRZI({ samples: [{ ...rziRow, source: 'rzi_unknown' }] }))
    expect(root.querySelector('.sea-chip')).toBeNull()
    expect(root.textContent).toContain('No lab samples this season')
  })

  it('shows the stale line with the date when the latest sample is stale', () => {
    const root = render(withRZI({ latest: { ...latest, stale: true } }))
    expect(root.querySelector('.sea-panel__fresh').textContent).toBe('No sample in the last 21 days. Latest: 12.08.2026.')
  })

  it('shows only samples of the newest season', () => {
    const root = render(withRZI({ samples: [rziRow, sample({ date: '2025-08-01', season: 2025, source: 'eea' })] }))
    expect(chipDates(root)).toEqual(['Aug 12, 2026'])
  })

  it('shows no stale line when the latest sample source has no name', () => {
    const root = render(withRZI({ latest: { ...latest, stale: true, source: 'rzi_unknown' } }))
    expect(root.querySelector('.sea-panel__fresh')).toBeNull()
  })

  it('shows no off-season line when no RZI sample is credited', () => {
    const body = withRZI({ latest: undefined, samples: [{ ...rziRow, source: 'rzi_unknown' }] })
    body.site.rzi.season_open = false
    expect(render(body).querySelector('.sea-panel__fresh')).toBeNull()
  })

  it('shows nothing extra when the latest sample is fresh', () => {
    expect(render(withRZI()).querySelector('.sea-panel__fresh')).toBeNull()
  })

  it('shows the off-season line when the season is closed and no sample is current', () => {
    const body = withRZI({ latest: undefined })
    body.site.rzi.season_open = false
    expect(render(body).querySelector('.sea-panel__fresh').textContent).toBe('Season closed. Results of 2026.')
  })

  it('shows no freshness line in season with only earlier samples', () => {
    const body = withRZI({ latest: undefined, samples: [{ ...rziRow, date: '2025-08-12', season: 2025 }] })
    const root = render(body)
    expect(root.querySelector('.sea-panel__fresh')).toBeNull()
    expect(root.textContent).not.toContain('Season closed')
  })

  it('renders no empty paragraph when the stale or off-season template is empty', () => {
    const off = withRZI({ latest: undefined })
    off.site.rzi.season_open = false
    expect(render(off, textsWith({ tSeaLatestOffSeason: '' })).querySelector('.sea-panel__fresh')).toBeNull()
    unmount(app)
    document.body.innerHTML = ''
    expect(render(withRZI({ latest: { ...latest, stale: true } }), textsWith({ tSeaLatestStale: '' })).querySelector('.sea-panel__fresh')).toBeNull()
  })

  it('adds the not-official notice to the sources explanation only with RZI samples', () => {
    const seen = []
    const a = render(withRZI(), seaT, { oninfo: (c) => seen.push(c) })
    a.querySelectorAll('button.sea-panel__info')[2].click()
    expect(seen[0].paragraphs).toContain('Not an official source.')
    unmount(app)
    document.body.innerHTML = ''
    const b = render(detail, seaT, { oninfo: (c) => seen.push(c) })
    b.querySelectorAll('button.sea-panel__info')[2].click()
    expect(seen[1].paragraphs).not.toContain('Not an official source.')
  })

  // I15: markup-like text stays text, in the source name and in a remark.
  it('renders markup-like text as text', () => {
    const hostile = '<img src=x onerror=alert(1)>'
    const root = render(withRZI(), textsWith({ tSeaSourceRziVarna: hostile }))
    expect(root.querySelector('img')).toBeNull()
    expect(root.querySelector('.sea-panel__sources span').textContent).toBe(`EEA, ${hostile}`)
  })
})

describe('SeaPanel RZI remarks', () => {
  const remark = 'Sample taken after heavy rain'
  const withRemark = (text = remark, source = 'rzi_varna') => withRZI({ samples: [{ ...rziRow, source, remarks: text }, { ...detail.samples[0], source: 'eea' }] })

  it('shows the remark under its sample with a label, the date and the credit', () => {
    const root = render(withRemark())
    const item = root.querySelector('.sea-remark')
    expect(item.querySelector('.sea-remark__label').textContent).toBe('RHI remark, Aug 12, 2026')
    expect(item.querySelector('.sea-remark__text').textContent).toBe(remark)
    expect(item.querySelector('.sea-remark__credit').textContent).toBe('Source: RHI Varna')
  })

  it('names the inspectorate of the sample in the credit', () => {
    expect(render(withRemark(remark, 'rzi_burgas'), textsWith({ tSeaSourceRziBurgas: 'RHI Burgas' })).querySelector('.sea-remark__credit').textContent).toBe('Source: RHI Burgas')
  })

  it('ties the label to the text for assistive tech', () => {
    const group = render(withRemark()).querySelector('.sea-remark [role="group"]')
    expect(document.getElementById(group.getAttribute('aria-labelledby')).className).toBe('sea-remark__label')
  })

  it('renders nothing for a sample with no remark', () => {
    const root = render(withRemark(''))
    expect(root.querySelector('.sea-remark')).toBeNull()
    expect(root.textContent).not.toContain('RHI remark')
  })

  it('renders nothing when the label template is missing', () => {
    expect(render(withRemark(), textsWith({ tSeaRemarkLabel: '' })).querySelector('.sea-remark')).toBeNull()
  })

  it('shows HTML in a remark as text', () => {
    const hostile = '<img src=x onerror=alert(1)><b>bold</b>'
    const root = render(withRemark(hostile))
    expect(root.querySelector('img')).toBeNull()
    expect(root.querySelector('.sea-remark b')).toBeNull()
    expect(root.querySelector('.sea-remark__text').textContent).toBe(hostile)
  })
})
