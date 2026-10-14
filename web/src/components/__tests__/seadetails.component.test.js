// @vitest-environment jsdom
import { describe, it, expect, afterEach, beforeAll } from 'vitest'
import { mount, unmount, flushSync } from 'svelte'
import SeaDetails from '../SeaDetails.svelte'
import { seaT, textsWith, colours, detail, sample } from './seafixtures.js'

// jsdom has no showModal; the stand-in flips `open`.
beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function showModal() { this.open = true }
  HTMLDialogElement.prototype.close = function close() { this.open = false; this.dispatchEvent(new Event('close')) }
})

let app
afterEach(() => {
  if (app) unmount(app)
  app = null
  document.body.innerHTML = ''
})

const URL_EEA = 'https://www.eea.europa.eu/en/topics/in-depth/water/bathing-water'
function render(body = detail, over = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  app = mount(SeaDetails, { target, props: { d: body, t: seaT, lang: 'en', colours, creditURL: URL_EEA, ...over } })
  flushSync()
  return { target, dialog: target.querySelector('dialog'), api: app }
}
const key = (el, k) => { el.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true })); flushSync() }
const tabs = (dialog) => [...dialog.querySelectorAll('[role="tab"]')]
const panel = (dialog, id) => dialog.querySelector(`#sea-panel-${id}`)
const credits = (dialog, id) => [...panel(dialog, id).querySelectorAll('.sea-details__credits li')].map((li) => li.textContent)

const twoSeasons = {
  ...detail,
  classes: [{ season: 2025, quality: 'good', source: '' }, { season: 2024, quality: 'excellent', source: '' }],
  samples: [
    sample({ date: '2025-07-02', season: 2025, e_coli: 700, enterococci: 20 }),
    sample({ date: '2025-06-02', season: 2025, e_coli: 20, enterococci: 150 }),
    sample({ date: '2024-07-02', season: 2024, e_coli: 5, enterococci: 5 }),
  ],
}

describe('SeaDetails shell', () => {
  it('is a dialog named by the site, opened modally by show()', () => {
    const { dialog, api } = render()
    expect(dialog.getAttribute('aria-labelledby')).toBe('sea-details-title')
    expect(dialog.querySelector('#sea-details-title').textContent).toBe('Zlatni Pyasatsi')
    expect(dialog.open).toBe(false)
    api.show()
    expect(dialog.open).toBe(true)
  })

  it('closes from Back and from the close button, returning focus to the opener', () => {
    const { target, dialog, api } = render()
    const opener = document.createElement('button')
    target.appendChild(opener)
    api.show(opener)
    dialog.querySelector('.sea-details__back').click()
    expect(dialog.open).toBe(false)
    expect(document.activeElement).toBe(opener)
    opener.blur()
    api.show(opener)
    dialog.querySelector('.sea-details__close').click()
    expect(document.activeElement).toBe(opener)
  })

  it('returns focus to the opener when the browser closes the dialog on Escape', () => {
    const { target, dialog, api } = render()
    const opener = document.createElement('button')
    target.appendChild(opener)
    api.show(opener)
    dialog.open = false
    dialog.dispatchEvent(new Event('close'))
    expect(document.activeElement).toBe(opener)
  })

  // The listener sits on document, outside the Svelte mount target, so only stopPropagation can keep it quiet.
  it('keeps an Escape keydown from reaching the element that holds the dialog', () => {
    const { dialog, api } = render()
    let reached = 0
    const count = () => { reached++ }
    document.addEventListener('keydown', count)
    try {
      api.show()
      key(dialog.querySelector('.sea-details__title'), 'Escape')
      expect(reached).toBe(0)
      key(dialog.querySelector('.sea-details__title'), 'a')
      expect(reached).toBe(1)
    } finally {
      document.removeEventListener('keydown', count)
    }
  })
})

describe('SeaDetails tabs', () => {
  it('follows the WAI-ARIA tabs pattern', () => {
    const { dialog } = render()
    expect(dialog.querySelector('[role="tablist"]').getAttribute('aria-label')).toBe('Details sections')
    expect(tabs(dialog).map((t) => t.textContent)).toEqual(['Samples', 'Seasons', 'About'])
    for (const t of tabs(dialog)) {
      const id = t.id.replace('sea-tab-', '')
      expect(t.getAttribute('aria-controls')).toBe(`sea-panel-${id}`)
      expect(panel(dialog, id).getAttribute('role')).toBe('tabpanel')
      expect(panel(dialog, id).getAttribute('aria-labelledby')).toBe(t.id)
    }
  })

  it('starts on Samples with only that tab in the tab order and only that panel visible', () => {
    const { dialog } = render()
    expect(tabs(dialog).map((t) => t.getAttribute('aria-selected'))).toEqual(['true', 'false', 'false'])
    expect(tabs(dialog).map((t) => t.getAttribute('tabindex'))).toEqual(['0', '-1', '-1'])
    expect(['samples', 'seasons', 'about'].map((id) => panel(dialog, id).hidden)).toEqual([false, true, true])
  })

  it('moves and selects with the arrow keys, wrapping, and focuses the new tab', () => {
    const { dialog, api } = render()
    api.show()
    const [samples, seasons, about] = tabs(dialog)
    samples.focus()
    key(samples, 'ArrowRight')
    expect(seasons.getAttribute('aria-selected')).toBe('true')
    expect(document.activeElement).toBe(seasons)
    key(seasons, 'ArrowRight')
    key(about, 'ArrowRight')
    expect(samples.getAttribute('aria-selected')).toBe('true')
    key(samples, 'ArrowLeft')
    expect(about.getAttribute('aria-selected')).toBe('true')
  })

  it('jumps with Home and End and ignores other keys', () => {
    const { dialog, api } = render()
    api.show()
    const [samples, , about] = tabs(dialog)
    key(samples, 'End')
    expect(about.getAttribute('aria-selected')).toBe('true')
    key(about, 'Home')
    expect(samples.getAttribute('aria-selected')).toBe('true')
    key(samples, 'x')
    expect(samples.getAttribute('aria-selected')).toBe('true')
  })

  it('makes every tab panel reachable by keyboard', () => {
    const { dialog } = render()
    for (const id of ['samples', 'seasons', 'about']) expect(panel(dialog, id).getAttribute('tabindex')).toBe('0')
  })

  it('selects a tab on click and shows its panel', () => {
    const { dialog } = render()
    tabs(dialog)[1].click()
    flushSync()
    expect(panel(dialog, 'seasons').hidden).toBe(false)
    expect(panel(dialog, 'samples').hidden).toBe(true)
  })
})

describe('Samples tab', () => {
  const chartLabel = (dialog) => dialog.querySelector('svg.sea-chart__svg').getAttribute('aria-label')
  const metricButtons = (dialog) => [...dialog.querySelectorAll('.sea-seg__btn')]

  it('opens on the newest season with E. coli and a described chart', () => {
    const { dialog } = render(twoSeasons)
    expect(chartLabel(dialog)).toBe('E. coli, season 2025. Samples: 2. Above the excellent limit: 1. Above the good limit: 1.')
  })

  it('switches the indicator with a two-button group that reports its state', () => {
    const { dialog } = render(twoSeasons)
    expect(dialog.querySelector('.sea-seg').getAttribute('role')).toBe('group')
    expect(dialog.querySelector('.sea-seg').getAttribute('aria-label')).toBe('Indicator')
    expect(metricButtons(dialog).map((b) => [b.textContent, b.getAttribute('aria-pressed')])).toEqual([['E. coli', 'true'], ['Enterococci', 'false']])
    metricButtons(dialog)[1].click()
    flushSync()
    expect(metricButtons(dialog).map((b) => b.getAttribute('aria-pressed'))).toEqual(['false', 'true'])
    expect(chartLabel(dialog)).toBe('Enterococci, season 2025. Samples: 2. Above the excellent limit: 1. Above the good limit: 0.')
  })

  it('keeps the indicator and the season while the dialog lives', () => {
    const { dialog } = render(twoSeasons)
    const select = dialog.querySelector('select')
    select.value = '2024'
    select.dispatchEvent(new Event('change', { bubbles: true }))
    metricButtons(dialog)[1].click()
    tabs(dialog)[2].click()
    tabs(dialog)[0].click()
    flushSync()
    expect(chartLabel(dialog)).toContain('Enterococci, season 2024')
  })

  it('offers a season picker, newest first, only when more than one season has samples', () => {
    const { dialog } = render(twoSeasons)
    const select = dialog.querySelector('select')
    expect([...select.options].map((o) => o.value)).toEqual(['2025', '2024'])
    expect(dialog.querySelector('label').textContent).toContain('Season')
    select.value = '2024'
    select.dispatchEvent(new Event('change', { bubbles: true }))
    flushSync()
    expect(chartLabel(dialog)).toContain('season 2024')
  })

  it('has no season picker with a single season', () => {
    const oneSeason = { ...detail, samples: [detail.samples[0]] }
    const { dialog } = render(oneSeason)
    expect(dialog.querySelector('select')).toBeNull()
    expect(dialog.querySelector('svg.sea-chart__svg')).not.toBeNull()
  })

  it('credits each source once under the chart', () => {
    const { dialog } = render(twoSeasons)
    expect(credits(dialog, 'samples')).toEqual(['Data: EEA, CC BY 4.0'])
  })

  it('credits an RZI source by its region next to the EEA', () => {
    const body = { ...twoSeasons, samples: [sample({ date: '2025-07-02', season: 2025, source: 'rzi_varna' }), sample({ date: '2025-06-02', season: 2025, source: 'eea' })] }
    const { dialog } = render(body)
    expect(credits(dialog, 'samples')).toEqual(['Source: RHI Varna', 'Data: EEA, CC BY 4.0'])
  })

  it('keeps the pending note for a datahub class with no samples yet', () => {
    const body = { ...twoSeasons, classes: [{ season: 2026, quality: 'good', source: 'datahub' }, ...twoSeasons.classes] }
    expect(render(body).dialog.querySelector('.sea-details__pending').textContent).toBe('Lab samples for 2026 are not published yet.')
  })

  it('drops the pending note once samples for that season exist', () => {
    const body = { ...twoSeasons, classes: [{ season: 2025, quality: 'good', source: 'datahub' }, ...twoSeasons.classes] }
    expect(render(body).dialog.querySelector('.sea-details__pending')).toBeNull()
  })

  it('has no pending note for a class that is not from the datahub', () => {
    const body = { ...twoSeasons, classes: [{ season: 2026, quality: 'good', source: 'discodata' }, ...twoSeasons.classes] }
    expect(render(body).dialog.querySelector('.sea-details__pending')).toBeNull()
  })

  it('counts a sample without a credit as existing, so the pending note stays away', () => {
    const body = { ...twoSeasons, classes: [{ season: 2026, quality: 'good', source: 'datahub' }, ...twoSeasons.classes], samples: [...twoSeasons.samples, sample({ date: '2026-06-01', season: 2026, source: 'rzi_unknown' })] }
    expect(render(body).dialog.querySelector('.sea-details__pending')).toBeNull()
  })

  it('shows a message and no chart when no season has samples', () => {
    const { dialog } = render({ ...detail, samples: [] })
    expect(dialog.querySelector('svg.sea-chart__svg')).toBeNull()
    expect(dialog.querySelector('.sea-seg')).toBeNull()
    expect(panel(dialog, 'samples').textContent).toContain('No samples published for this site.')
  })

  it('offers the sample table behind the chart toggle, with the readings and their marks', () => {
    const { dialog } = render(twoSeasons)
    expect(dialog.querySelector('table.sea-chart__table')).toBeNull()
    dialog.querySelector('.sea-chart__toggle').click()
    flushSync()
    expect(dialog.querySelector('svg.sea-chart__svg')).toBeNull()
    const rows = [...dialog.querySelectorAll('table.sea-chart__table tbody tr')]
    expect(rows).toHaveLength(2)
    expect(rows.map((r) => r.querySelector('th').textContent)).toEqual(['Jun 2, 2025', 'Jul 2, 2025'])
    expect(rows[1].textContent).toContain('700')
    expect(rows[1].textContent).toContain('above the good limit')
  })

  it('copes with null samples and null classes', () => {
    expect(() => render({ ...detail, samples: null, classes: null })).not.toThrow()
  })
})

describe('Seasons tab', () => {
  it('shows the grid, newest first, with the same names as the strip, and the class key', () => {
    const { dialog } = render()
    const p = panel(dialog, 'seasons')
    expect([...p.querySelectorAll('.sea-cell')].map((c) => c.getAttribute('aria-label'))).toEqual(['2024 Excellent', '2023 Poor'])
    expect(p.querySelectorAll('.sea-key__item')).toHaveLength(5)
  })

  it('marks datahub seasons with their source and shows the supplement note', () => {
    const body = {
      ...detail,
      classes: [{ season: 2025, quality: 'good', source: 'datahub' }, { season: 2024, quality: 'excellent', source: '' }],
      supplement: { edition: '2025 v1.0', published: '2026-09-30', url: 'https://example.test/d' },
    }
    const p = panel(render(body).dialog, 'seasons')
    expect([...p.querySelectorAll('.sea-details__source')].map((n) => n.textContent)).toEqual(['2025: From the EEA 2025 report'])
    expect(p.querySelector('.sea-details__supplement').textContent).toBe('Dataset, edition 2025 v1.0, published Sep 30, 2026.')
  })

  it('keeps a published value that is not a date as it came', () => {
    const body = { ...detail, supplement: { edition: 'E', published: 'autumn', url: '' } }
    expect(panel(render(body).dialog, 'seasons').querySelector('.sea-details__supplement').textContent).toContain('published autumn.')
  })

  it('shows neither source line nor supplement without them', () => {
    const p = panel(render().dialog, 'seasons')
    expect(p.querySelector('.sea-details__source')).toBeNull()
    expect(p.querySelector('.sea-details__supplement')).toBeNull()
  })
})

describe('About tab', () => {
  const about = (body = detail, over = {}) => panel(render(body, over).dialog, 'about')
  const reset = () => { unmount(app); document.body.innerHTML = '' }

  it('explains the class, the four-season assessment and both limits', () => {
    const p = about()
    expect(p.textContent).toContain('The class is the EU classification.')
    expect(p.textContent).toContain('The class is the EEA assessment over the last four seasons.')
    expect(p.textContent).toContain('Limits, excellent / good (cfu/100 ml)')
    expect(p.textContent).toContain('E. coli 250 / 500')
    expect(p.textContent).toContain('Enterococci 100 / 200')
  })

  it('credits the EEA and links the site profile with the new-tab hint', () => {
    const links = [...about().querySelectorAll('a')]
    expect(links.map((a) => a.getAttribute('href'))).toEqual([URL_EEA, 'https://example.org/p.pdf'])
    for (const a of links) {
      expect(a.getAttribute('target')).toBe('_blank')
      expect(a.getAttribute('rel')).toBe('noopener noreferrer')
      expect(a.querySelector('.sr-only').textContent).toBe(', opens in a new tab')
    }
  })

  it('adds no new-tab hint when the text is missing', () => {
    const links = about(detail, { t: textsWith({ tSeaNewTab: '' }) }).querySelectorAll('a[target="_blank"]')
    expect(links.length).toBeGreaterThan(0)
    for (const a of links) expect(a.querySelector('.sr-only')).toBeNull()
  })

  it('has no stray separator when there is no credit link', () => {
    const p = about(detail, { creditURL: '' })
    expect(p.querySelector('.sea-details__links').textContent.trim().startsWith('·')).toBe(false)
    expect([...p.querySelectorAll('a')].map((a) => a.getAttribute('href'))).toEqual(['https://example.org/p.pdf'])
  })

  it('links the site profile only when it is an http(s) URL', () => {
    const bad = { ...detail, site: { ...detail.site, profile_url: 'javascript:alert(1)' } }
    expect([...about(bad).querySelectorAll('a')].map((a) => a.getAttribute('href'))).toEqual([URL_EEA])
  })

  it('credits every distinct source of the samples once', () => {
    const body = {
      ...twoSeasons,
      samples: [
        sample({ date: '2025-07-02', season: 2025, source: 'rzi_varna' }),
        sample({ date: '2025-06-20', season: 2025, source: 'rzi_varna' }),
        sample({ date: '2024-07-02', season: 2024, source: 'eea' }),
        sample({ date: '2024-06-02', season: 2024, source: 'rzi_burgas' }),
      ],
    }
    const { dialog } = render(body)
    expect(credits(dialog, 'about')).toEqual(['Source: RHI Varna', 'Data: EEA, CC BY 4.0', 'Source: RHI Burgas'])
  })

  const PAGE = 'https://rzi-varna.com/rezultati/'
  const latest = { date: '2026-08-12', source: 'rzi_varna', e_coli: 600, e_coli_below_detection: false, enterococci: 40, enterococci_below_detection: true, unit: 'MPN/100ml', stale: false }
  const rziBody = (page = PAGE, over = {}) => ({
    ...detail,
    site: { ...detail.site, rzi: { source: 'rzi_varna', page_url: page, season_open: true } },
    samples: [sample({ date: '2026-08-12', season: 2026, source: 'rzi_varna' })],
    ...over,
  })
  const resultsLink = (p) => [...p.querySelectorAll('a')].find((a) => a.textContent.startsWith('Results on the RHI site'))

  it('shows the not-official notice, the RZI note and the results link only with RZI data', () => {
    const p = about(rziBody())
    expect(p.textContent).toContain('Not an official source.')
    expect(p.textContent).toContain('Lab results from the RHI sites.')
    expect(p.textContent).toContain('Source: RHI Varna')
    const link = resultsLink(p)
    expect(link.getAttribute('href')).toBe(PAGE)
    expect(link.getAttribute('target')).toBe('_blank')
    expect(link.getAttribute('rel')).toBe('noopener noreferrer')
    expect(link.textContent).toBe('Results on the RHI site, opens in a new tab')
    reset()
    const plain = about()
    expect(plain.textContent).not.toContain('Not an official source.')
    expect(resultsLink(plain)).toBeUndefined()
  })

  it('never links a page that is not http(s) or that names a file', () => {
    expect(resultsLink(about(rziBody('javascript:alert(1)')))).toBeUndefined()
    reset()
    expect(resultsLink(about(rziBody('https://rzi-varna.com/results.pdf')))).toBeUndefined()
  })

  it('renders a hostile source name as text', () => {
    const p = about(rziBody(), { t: textsWith({ tSeaSourceRziVarna: '<img src=x onerror=alert(1)>' }) })
    expect(p.querySelector('img')).toBeNull()
    expect(p.textContent).toContain('Source: <img src=x onerror=alert(1)>')
  })

  describe('latest lab result block', () => {
    const block = (p) => p.querySelector('.sea-details__latest')

    it('shows both indicators against the limits, the unit, the credit and the single-sample note', () => {
      const b = block(about(rziBody(PAGE, { latest })))
      expect(b.querySelector('h3').textContent).toBe('Latest lab result')
      const [ec, ie] = b.querySelectorAll('.sea-details__latest-value')
      expect(ec.textContent).toContain('600')
      expect(ec.textContent).toContain('▲▲')
      expect(ec.textContent).toContain('above the good limit')
      expect(ie.textContent).toContain('<40')
      expect(ie.textContent).toContain('below the detection limit')
      expect(ie.textContent).not.toContain('▲')
      for (const v of [ec, ie]) expect(v.textContent).toContain('MPN/100 ml')
      expect(b.querySelector('.sea-details__credit-rzi').textContent).toBe('Source: RHI Varna, sample of 12.08.2026')
      expect(b.textContent).toContain('A single sample, not a class.')
    })

    it('shows the stale state with the date and no values', () => {
      const b = block(about(rziBody(PAGE, { latest: { ...latest, stale: true } })))
      expect(b.querySelector('.sea-details__stale').textContent).toBe('No sample in the last 21 days. Latest: 12.08.2026.')
      expect(b.querySelector('.sea-details__latest-value')).toBeNull()
      expect(b.querySelector('.sea-details__credit-rzi').textContent).toBe('Source: RHI Varna, sample of 12.08.2026')
    })

    it('shows the off-season state when only the RZI page is known', () => {
      const body = rziBody(PAGE)
      body.site.rzi.season_open = false
      const p = about(body)
      expect(block(p).querySelector('.sea-details__off-season').textContent).toBe('Season closed. Results of 2026.')
      expect(block(p).querySelector('.sea-details__latest-value')).toBeNull()
      expect(resultsLink(p).getAttribute('href')).toBe(PAGE)
    })

    it('has no block without a latest sample or when the season is open', () => {
      expect(block(about(rziBody()))).toBeNull()
      reset()
      expect(block(about())).toBeNull()
    })

    it('renders no value when the credit template or the source name is missing', () => {
      expect(block(about(rziBody(PAGE, { latest }), { t: textsWith({ tSeaCreditRzi: '' }) }))).toBeNull()
      reset()
      expect(block(about(rziBody(PAGE, { latest: { ...latest, source: 'rzi_unknown' } })))).toBeNull()
    })

    it('renders no empty paragraph when the stale or off-season template is empty', () => {
      const off = rziBody(PAGE)
      off.site.rzi.season_open = false
      expect(block(about(off, { t: textsWith({ tSeaLatestOffSeason: '' }) }))).toBeNull()
      reset()
      const stale = rziBody(PAGE, { latest: { ...latest, stale: true } })
      expect(block(about(stale, { t: textsWith({ tSeaLatestStale: '' }) }))).toBeNull()
    })

    it('renders the credit with a hostile source name as text', () => {
      const hostile = '<img src=x onerror=alert(1)>'
      const p = about(rziBody(PAGE, { latest }), { t: textsWith({ tSeaSourceRziVarna: hostile }) })
      expect(p.querySelector('img')).toBeNull()
      expect(p.querySelector('.sea-details__credit-rzi').textContent).toBe(`Source: ${hostile}, sample of 12.08.2026`)
    })
  })
})
