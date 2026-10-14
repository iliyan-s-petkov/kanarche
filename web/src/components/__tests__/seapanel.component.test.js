// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest'
import { mount, unmount, flushSync } from 'svelte'
import SeaPanel from '../SeaPanel.svelte'
import { createSeaPanel } from '../../lib/seapanel.svelte.js'
import { seaT, textsWith, colours, COLOURS, detail, sample } from './seafixtures.js'

// jsdom has no showModal; the stand-in flips `open`.
beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function showModal() { this.open = true }
  HTMLDialogElement.prototype.close = function close() { this.open = false; this.dispatchEvent(new Event('close')) }
})

let app
let frame
afterEach(() => {
  if (app) unmount(app)
  app = null
  frame?.remove()
  document.body.innerHTML = ''
})

function renderPanel(body = detail, over = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const calls = { info: [], close: 0 }
  app = mount(SeaPanel, {
    target,
    props: {
      view: { id: 'BG1', detail: body, failed: false },
      t: seaT, lang: 'en', colours,
      creditURL: 'https://www.eea.europa.eu/en/topics/in-depth/water/bathing-water',
      onclose: () => { calls.close++ },
      oninfo: (c) => calls.info.push(c),
      ...over,
    },
  })
  flushSync()
  return { root: target.querySelector('.sea-panel'), calls, target }
}

const withSamples = (rows, over = {}) => ({ ...detail, samples: rows, ...over })

describe('SeaPanel card', () => {
  it('shows the name, the zone and the official class badge first', () => {
    const { root } = renderPanel()
    expect(root.querySelector('h2').textContent).toBe('Zlatni Pyasatsi')
    expect(root.querySelector('.sea-panel__zone').textContent).toBe('Sea')
    const badge = root.querySelector('.sea-panel__badge')
    expect(badge.querySelector('strong').textContent).toBe('Excellent')
    expect(badge.querySelector('text').textContent).toBe('E')
    expect(badge.querySelector('rect').getAttribute('fill')).toBe(COLOURS[0])
    expect(badge.textContent).toContain('Official class for 2024')
    // Class first: the badge comes before the chips.
    expect(badge.compareDocumentPosition(root.querySelector('.sea-panel__chips')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('marks a class that came from the datahub and says nothing of pending samples on the card', () => {
    const { root } = renderPanel({ ...detail, classes: [{ season: 2025, quality: 'good', source: 'datahub' }, { season: 2024, quality: 'excellent', source: '' }] })
    expect(root.querySelector('.sea-panel__badge .sea-panel__source').textContent).toBe('From the EEA 2025 report')
    expect(root.querySelector('.sea-panel__pending')).toBeNull()
  })

  it('shows a grey badge with the dash initial and "Not classified" when there is no class', () => {
    const { root } = renderPanel({ ...detail, classes: [] })
    const badge = root.querySelector('.sea-panel__badge')
    expect(badge.querySelector('strong').textContent).toBe('Not classified')
    expect(badge.querySelector('text').textContent).toBe('')
    expect(badge.querySelector('rect').getAttribute('fill')).toBe(COLOURS[4])
    expect(root.querySelector('.sea-seasons')).toBeNull()
  })

  it('draws the legacy class in the sufficient colour and initial with its full label', () => {
    const { root } = renderPanel({ ...detail, classes: [{ season: 2017, quality: 'good_or_sufficient', source: '' }] })
    const badge = root.querySelector('.sea-panel__badge')
    expect(badge.querySelector('strong').textContent).toBe('Good or sufficient')
    expect(badge.querySelector('text').textContent).toBe('S')
    expect(badge.querySelector('rect').getAttribute('fill')).toBe(COLOURS[2])
  })

  describe('lab sample chips', () => {
    const four = [
      sample({ date: '2024-07-30', e_coli: 600, enterococci: 10 }),
      sample({ date: '2024-07-16', e_coli: 300, enterococci: 10 }),
      sample({ date: '2024-07-02', e_coli: 20, enterococci: 10 }),
      sample({ date: '2024-06-18', e_coli: 5, enterococci: 5 }),
    ]
    const chips = (root) => [...root.querySelectorAll('.sea-panel__chips .sea-chip')]

    it('shows the newest three, newest first, with the date', () => {
      const { root } = renderPanel(withSamples(four))
      expect(chips(root).map((c) => c.querySelector('.sea-chip__date').textContent)).toEqual(['Jul 30, 2024', 'Jul 16, 2024', 'Jul 2, 2024'])
    })

    it('marks each chip in text: over good, over excellent, within', () => {
      const { root } = renderPanel(withSamples(four))
      const [a, b, c] = chips(root)
      expect(a.querySelector('.sea-chip__mark').textContent).toBe('▲▲')
      expect(a.textContent).toContain('above the good limit')
      expect(b.querySelector('.sea-chip__mark').textContent).toBe('▲')
      expect(b.textContent).toContain('above the excellent limit')
      expect(c.querySelector('.sea-chip__mark').textContent).toBe('✓')
      expect(c.textContent).toContain('within the excellent limit')
      expect(a.querySelector('.sea-chip__mark').getAttribute('aria-hidden')).toBe('true')
    })

    it('takes the worse of the two indicators for the mark', () => {
      const { root } = renderPanel(withSamples([sample({ e_coli: 10, enterococci: 150 })]))
      expect(chips(root)[0].querySelector('.sea-chip__mark').textContent).toBe('▲')
    })

    it('counts a value below detection as within', () => {
      const { root } = renderPanel(withSamples([sample({ e_coli: 9999, e_coli_below_detection: true, enterococci: 5 })]))
      expect(chips(root)[0].querySelector('.sea-chip__mark').textContent).toBe('✓')
    })

    it('marks a pre-season sample', () => {
      const { root } = renderPanel(withSamples([sample({ pre_season: true })]))
      expect(chips(root)[0].textContent).toContain('pre-season')
    })

    it('shows only the chips that exist', () => {
      const { root } = renderPanel(withSamples([sample()]))
      expect(chips(root)).toHaveLength(1)
    })

    it('says so when there are no samples', () => {
      const { root } = renderPanel(withSamples([]))
      expect(chips(root)).toHaveLength(0)
      expect(root.textContent).toContain('No lab samples this season')
    })

    it('keeps the one-line key under the chips', () => {
      const { root } = renderPanel(withSamples(four))
      const key = root.querySelector('.sea-panel__key').textContent
      expect(key).toContain('within the excellent limit')
      expect(key).toContain('above the excellent limit')
      expect(key).toContain('above the good limit')
    })

    // Review Focus 3.
    it('shows two samples of the same date without throwing', () => {
      const { root } = renderPanel(withSamples([sample({ source: 'eea' }), sample({ source: 'eea', e_coli: 700 })]))
      expect(chips(root)).toHaveLength(2)
    })

    // Review Focus 4.
    it('copes with a null samples list', () => {
      const { root } = renderPanel({ ...detail, samples: null })
      expect(root.textContent).toContain('No lab samples this season')
    })
  })

  describe('season strip and key', () => {
    it('shows "Seasons since" the first class, the strip and the five-class key', () => {
      const { root } = renderPanel()
      expect(root.textContent).toContain('Seasons since 2023')
      expect([...root.querySelectorAll('.sea-seasons--strip .sea-cell')].map((c) => c.getAttribute('aria-label'))).toEqual(['2023 Poor', '2024 Excellent'])
      // The card holds the details dialog, which has its own key.
      expect([...root.querySelectorAll('.sea-key__item')].filter((n) => !n.closest('dialog'))).toHaveLength(5)
    })
  })

  describe('(i) buttons', () => {
    const infos = (root) => [...root.querySelectorAll('button.sea-panel__info')]

    it('has an info button for the class, the lab samples and the sources, each with a name', () => {
      const { root } = renderPanel()
      expect(infos(root).map((b) => b.getAttribute('aria-label'))).toEqual(['What the bathing water classes mean', 'About the lab samples', 'About the sources and licence'])
    })

    it('opens the class explanation, with the datahub link only when the API sent a supplement', () => {
      const a = renderPanel()
      infos(a.root)[0].click()
      expect(a.calls.info[0].title).toBe('Bathing water class, EEA')
      expect(a.calls.info[0].paragraphs).toContain('The class is the EU classification.')
      expect(a.calls.info[0].links.map((l) => l.label)).toEqual(['EEA map', 'EEA, bathing water'])
      unmount(app)
      document.body.innerHTML = ''
      const b = renderPanel({ ...detail, supplement: { edition: '2025 v1.0', published: '2026-09-30', url: 'https://example.test/d' } })
      infos(b.root)[0].click()
      expect(b.calls.info[0].links.map((l) => l.href)).toContain('https://example.test/d')
    })

    it('opens the lab sample note and the sources and licence', () => {
      const { root, calls } = renderPanel()
      infos(root)[1].click()
      expect(calls.info[0].paragraphs).toContain('A single sample, not a class.')
      infos(root)[2].click()
      expect(calls.info[1].paragraphs).toContain('Data: EEA, CC BY 4.0')
      expect(calls.info[1].links[0].href).toBe('https://www.eea.europa.eu/en/topics/in-depth/water/bathing-water')
    })
  })

  describe('footer', () => {
    it('names each source of the shown chips once', () => {
      const { root } = renderPanel(withSamples([sample({ date: '2024-07-02' }), sample({ date: '2024-06-01' })]))
      expect(root.querySelector('.sea-panel__sources span').textContent).toBe('EEA')
    })

    it('names the EEA for the class even with no samples, so it is never a lone info button', () => {
      const { root } = renderPanel(withSamples([]))
      expect(root.querySelector('.sea-panel__sources span').textContent).toBe('EEA')
    })

    it('names the EEA for the class next to an RZI-only chip source', () => {
      const { root } = renderPanel(withSamples([sample({ date: '2024-07-02', source: 'rzi_varna' })]))
      expect(root.querySelector('.sea-panel__sources span').textContent).toBe('EEA, RHI Varna')
    })
  })

  it('has a Details and charts button and a details dialog, closed until pressed', () => {
    const { root } = renderPanel()
    expect(root.querySelector('button.sea-panel__details').textContent).toBe('Details and charts')
    expect(root.querySelector('dialog.sea-details').open).toBe(false)
  })

  it('tells assistive tech the Details button opens a dialog', () => {
    expect(renderPanel().root.querySelector('button.sea-panel__details').getAttribute('aria-haspopup')).toBe('dialog')
  })

  it('shows the loading mark with no detail and the failure text when it failed', () => {
    expect(renderPanel(null, { view: { id: 'BG1', detail: null, failed: false } }).root.querySelector('[aria-busy="true"]')).not.toBeNull()
    unmount(app)
    document.body.innerHTML = ''
    expect(renderPanel(null, { view: { id: 'BG1', detail: null, failed: true } }).root.textContent).toContain('Could not load.')
  })

  // Review Focus 4.
  it('mounts with no copy and an empty body without throwing', () => {
    expect(() => renderPanel({ ...detail, classes: [], samples: [] }, { t: textsWith({}) })).not.toThrow()
    unmount(app)
    document.body.innerHTML = ''
    expect(() => renderPanel(detail, { t: textsWith(Object.fromEntries(['tSeaChipsHeading', 'tSeaDetails', 'tSeaSeasonsSince'].map((k) => [k, '']))) })).not.toThrow()
  })
})

describe('createSeaPanel lifecycle', () => {
  const cfg = { lang: 'en', seaColours: colours, seaCreditURL: 'https://www.eea.europa.eu/', t: { sea: seaT } }
  async function opened(fetchJSON) {
    frame = document.createElement('div')
    document.body.appendChild(frame)
    const panel = createSeaPanel(frame, cfg, fetchJSON)
    await panel.open('BG1')
    flushSync()
    return panel
  }

  it('fetches the site by id', async () => {
    const fetchJSON = vi.fn(async () => detail)
    await opened(fetchJSON)
    expect(fetchJSON).toHaveBeenCalledWith('/api/v1/sea/sites/BG1')
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

  it('opens the (i) explanation in one shared modal dialog on the frame', async () => {
    await opened(async () => detail)
    frame.querySelector('button.sea-panel__info').click()
    flushSync()
    const dialogs = frame.querySelectorAll('dialog.scaleinfo')
    expect(dialogs).toHaveLength(1)
    expect(dialogs[0].open).toBe(true)
    expect(dialogs[0].querySelector('h2').textContent).toBe('Bathing water class, EEA')
    frame.querySelectorAll('button.sea-panel__info')[1].click()
    flushSync()
    expect(frame.querySelectorAll('dialog.scaleinfo')).toHaveLength(1)
  })
})
