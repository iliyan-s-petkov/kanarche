// @vitest-environment jsdom
//
// jsdom for the dialog half: showModal, the <a rel> and the SVG fill are real
// DOM behaviour, and a string of markup would assert none of them.
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, it, expect, vi } from 'vitest'
import { bandRange, scaleFor, scaleInfo } from '../scaleinfo.js'
import { createScaleDialog } from '../scaledialog.js'

// Shaped like /api/v1/scales, including the fields the key never reads: notes
// in both languages and the guideline link, which are the whole point of the
// dialog.
const EAQI = {
  name: 'eaqi', metric: 'P2', unit: 'µg/m³', ceiling: 500,
  notes: 'EAQI bands', notes_bg: 'Ленти по EAQI',
  source: 'https://airindex.eea.europa.eu/',
  bands: [
    { upper: 5, colour: '#3c9', label: 'Good', label_bg: 'Добро' },
    { upper: 10, colour: '#fc3', label: 'Fair', label_bg: 'Задоволително' },
    { upper: null, colour: '#c33', label: 'Poor', label_bg: 'Лошо' },
  ],
}
const METEO = { name: 'meteo', metric: 'temperature', unit: '°C', source: '', notes: 'Weather', notes_bg: 'Време', bands: [] }
const SCALES = [EAQI, METEO]

// The range words the page hands over, with the slots the server catalogue uses.
const WORDS = { upTo: 'up to {to}', between: '{from} to {to}', above: 'above {from}' }
const DIALOG_T = {
  title: '{metric} colour scale', unit: 'Values are in {unit}.', dismiss: 'Close colour scale', range: WORDS,
}

describe('scaleFor', () => {
  // Matched on metric, never on position: the same rule bandsFor applies, so a
  // reordered response cannot make the dialog describe a table the map is not
  // painting from.
  it('finds the table by metric, not by order', () => {
    expect(scaleFor(SCALES, 'temperature')).toBe(METEO)
    expect(scaleFor(SCALES, 'noise_LAeq')).toBeNull()
    expect(scaleFor(null, 'P2')).toBeNull()
  })
})

describe('bandRange', () => {
  // A band states only its own upper bound; the range a reader needs is between
  // that and the band below. Getting this wrong shifts every row by one band —
  // it would still look like a plausible table.
  it('reads each band lower edge off the band below it', () => {
    expect(EAQI.bands.map((_, i) => bandRange(EAQI.bands, i, WORDS))).toEqual(['up to 5', '5 to 10', 'above 10'])
  })

  // An en dash is read as "minus" by some readers and breaks the copy rule.
  it('writes ranges as words with no dash or symbol', () => {
    const rows = EAQI.bands.map((_, i) => bandRange(EAQI.bands, i, WORDS))
    for (const r of rows) expect(r).not.toMatch(/[–—<>-]/)
  })

  it('says nothing for a band bounded at neither end', () => {
    expect(bandRange([{ upper: null }], 0, WORDS)).toBe('')
  })
})

describe('scaleInfo', () => {
  it('prints the rows in the reader language', () => {
    expect(scaleInfo(EAQI, 'bg').rows.map((r) => r.label)).toEqual(['Добро', 'Задоволително', 'Лошо'])
    expect(scaleInfo(EAQI, 'en').rows.map((r) => r.label)).toEqual(['Good', 'Fair', 'Poor'])
    expect(scaleInfo(EAQI, 'bg').notes).toBe('Ленти по EAQI')
  })

  it('carries the source through, and none where the scale cites none', () => {
    expect(scaleInfo(EAQI, 'en').source).toBe('https://airindex.eea.europa.eu/')
    expect(scaleInfo(METEO, 'en').source).toBe('')
  })

  it('is null for no scale at all, so the caller can withhold the button', () => {
    expect(scaleInfo(null, 'bg')).toBeNull()
  })
})

describe('createScaleDialog', () => {
  const open = (scale, lang = 'bg') => {
    const d = createScaleDialog(document, {
      closeLabel: 'Затвори', sourceLabel: 'Официалният документ',
      disclaimer: 'Ориентировъчни данни', lang, t: DIALOG_T,
    })
    document.body.appendChild(d.el)
    // jsdom implements <dialog> but not showModal in every version the project
    // pins; stubbing it keeps the test about the contents, which is what this
    // module owns.
    d.el.showModal = vi.fn(() => { d.el.open = true })
    d.show(scale, 'PM2.5')
    return d.el
  }

  it('names the scale, its unit and every band with its range', () => {
    const el = open(EAQI)
    expect(el.querySelector('.scaleinfo__units').textContent).toBe('Values are in µg/m³.')
    expect([...el.querySelectorAll('.scaleinfo__band-range')].map((n) => n.textContent))
      .toEqual(['up to 5', '5 to 10', 'above 10'])
  })

  // The heading was the raw scale id and unit ("eaqi, µg/m³"), which named
  // nothing a reader would recognise. The metric name leads and the unit moves
  // to its own line.
  it('titles the dialog with the metric name and keeps the unit out of the heading', () => {
    const title = open(EAQI).querySelector('.scaleinfo__title').textContent
    expect(title).toBe('PM2.5 colour scale')
    expect(title).not.toContain('µg')
  })

  it('has no unit line for a scale with no unit', () => {
    expect(open({ ...EAQI, unit: '' }).querySelector('.scaleinfo__units').hidden).toBe(true)
  })

  // The same CSP rule legend.js paints under: style-src has no 'unsafe-inline',
  // so a swatch coloured by a style attribute renders invisible.
  it('paints swatches with an SVG fill and no inline style', () => {
    const el = open(EAQI)
    expect([...el.querySelectorAll('rect')].map((r) => r.getAttribute('fill')))
      .toEqual(['#3c9', '#fc3', '#c33'])
    expect(el.querySelectorAll('[style]')).toHaveLength(0)
  })

  // A11Y-05: the dialog's swatches carry the same class outline as the map and the key.
  it('marks each band swatch with its class outline', () => {
    const rects = [...open(EAQI).querySelectorAll('rect')]
    rects.forEach((r, i) => {
      expect(r.classList.contains('class-outline')).toBe(true)
      expect(r.getAttribute('data-class')).toBe(String(Math.min(i, 5)))
      expect(r.getAttribute('vector-effect')).toBe('non-scaling-stroke')
    })
  })

  // The reason the dialog exists: a reader can check the claim instead of
  // taking the colours on trust.
  it('links the guideline, and opens it without handing over the opener', () => {
    const link = open(EAQI).querySelector('.scaleinfo__source')
    expect(link.hidden).toBe(false)
    expect(link.getAttribute('href')).toBe('https://airindex.eea.europa.eu/')
    expect(link.rel).toBe('noopener noreferrer')
  })

  // A weather axis claims no authority; an "official guideline" link pointing
  // nowhere would claim one for it.
  it('shows no link for a scale that cites nobody', () => {
    const link = open(METEO).querySelector('.scaleinfo__source')
    expect(link.hidden).toBe(true)
    expect(link.hasAttribute('href')).toBe(false)
  })

  // Every page that explains the bands has to say the readings are indicative.
  it('carries the disclaimer', () => {
    expect(open(EAQI).querySelector('.scaleinfo__disclaimer').textContent).toBe('Ориентировъчни данни')
  })

  // The guideline link and the close button shared a line and overlapped, so
  // the end of the link sat underneath the button and could not be clicked.
  it('puts the close button last, after the guideline link', () => {
    const el = open(EAQI)
    const close = el.querySelector('.scaleinfo__close')
    expect(el.lastElementChild).toBe(close)
    expect(close.compareDocumentPosition(el.querySelector('.scaleinfo__source')))
      .toBe(Node.DOCUMENT_POSITION_PRECEDING)
  })

  // A modal that can only be dismissed from the bottom of a long table makes
  // the reader scroll to leave it; the corner mark is where one is looked for.
  it('offers a named dismiss mark in the corner, before anything else', () => {
    const el = open(EAQI)
    const dismiss = el.querySelector('.scaleinfo__dismiss')
    expect(el.firstElementChild).toBe(dismiss)
    expect(dismiss.type).toBe('button')
    expect(dismiss.getAttribute('aria-label')).toBe('Close colour scale')
    // The glyph is CSS content: a character in the markup is read out beside
    // the label, which is the same rule the (i) button follows.
    expect(dismiss.textContent).toBe('')
  })

  // Two buttons both named Close are indistinguishable in a controls list.
  it('gives the corner mark and the bottom button different names', () => {
    const el = open(EAQI)
    const names = ['.scaleinfo__dismiss', '.scaleinfo__close'].map((sel) => {
      const b = el.querySelector(sel)
      return b.getAttribute('aria-label') || b.textContent
    })
    expect(names).toEqual(['Close colour scale', 'Затвори'])
  })

  it('closes from the corner mark as well as from the button', () => {
    for (const sel of ['.scaleinfo__dismiss', '.scaleinfo__close']) {
      const el = open(EAQI)
      el.close = vi.fn(() => { el.open = false })
      el.querySelector(sel).click()
      expect(el.close, `${sel} did not close the dialog`).toHaveBeenCalled()
    }
  })

  // The link must not sit crawlable-but-hrefless from first paint, before the
  // (i) is ever clicked: prime paints without opening the dialog.
  it('prime paints the link without opening the dialog', () => {
    const d = createScaleDialog(document, {
      closeLabel: 'x', sourceLabel: 'Официалният документ', disclaimer: '', lang: 'en', t: DIALOG_T,
    })
    document.body.appendChild(d.el)
    d.el.showModal = vi.fn(() => { d.el.open = true })
    d.prime(EAQI)
    const link = d.el.querySelector('.scaleinfo__source')
    expect(link.getAttribute('href')).toBe('https://airindex.eea.europa.eu/')
    expect(d.el.showModal).not.toHaveBeenCalled()
    expect(d.el.open).toBeFalsy()
  })

  // Repainting an already-open dialog is what a metric switch behind it does,
  // and showModal throws on an open dialog.
  it('repaints in place rather than reopening', () => {
    const d = createScaleDialog(document, { closeLabel: 'x', sourceLabel: 'y', disclaimer: '', lang: 'en', t: DIALOG_T })
    document.body.appendChild(d.el)
    d.el.showModal = vi.fn(() => { d.el.open = true })
    d.show(EAQI)
    d.show(METEO)
    expect(d.el.showModal).toHaveBeenCalledTimes(1)
    expect(d.el.querySelectorAll('.scaleinfo__band')).toHaveLength(0)
  })
})

// The DOM order alone did not separate them: an <a> and a <button> are both
// inline, so they laid out on one line however they were ordered.
describe('the dialog footer', () => {
  const appCSS = () => {
    const here = dirname(fileURLToPath(import.meta.url))
    return readFileSync(join(here, '..', '..', '..', '..', 'internal', 'web', 'static', 'app.css'), 'utf8')
  }
  const rule = (css, selector) => {
    const at = css.indexOf(`${selector} {`)
    return at < 0 ? '' : css.slice(at, css.indexOf('}', at))
  }

  it('gives the guideline link its own line', () => {
    expect(rule(appCSS(), '.scaleinfo__source'), 'link still inline beside the button')
      .toContain('display: block')
  })

  it('centres the close button under everything', () => {
    const r = rule(appCSS(), '.scaleinfo__close')
    expect(r, 'button still inline').toContain('display: block')
    expect(r, 'button not centred').toContain('margin-inline: auto')
  })

  it('anchors the dismiss mark to the dialog corner', () => {
    const css = appCSS()
    expect(rule(css, '.scaleinfo'), 'dialog is not the positioning ancestor')
      .toContain('position: relative')
    const r = rule(css, '.scaleinfo__dismiss')
    expect(r, 'mark not taken out of flow').toContain('position: absolute')
    expect(r, 'mark not in the top-right corner').toContain('inset-inline-end')
    expect(css, 'mark has no glyph').toContain('.scaleinfo__dismiss::before')
    // The title runs the full width and would slide under the mark.
    expect(rule(css, '.scaleinfo__title'), 'title can run under the mark')
      .toContain('padding-inline-end')
  })
})
