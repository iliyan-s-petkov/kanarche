import { scaleInfo } from './scaleinfo.js'
import { outlineSwatch } from './classoutline.js'
import { fillSlots } from './sea.js'

// The (i) beside the key, and the dialog it opens.
//
// A native <dialog> with showModal(), not a div: the focus trap, the Escape
// key, the inert background and the return of focus to the button are all
// browser behaviour here, and every hand-rolled version of them is a
// keyboard-accessibility bug waiting to be filed.
//
// Swatches are SVG <rect fill="…"> for the reason legend.js gives: band colours
// are server data, style-src has no 'unsafe-inline', and a presentation
// attribute is not an inline style.

// createScaleDialog builds the dialog once and returns a handle. `show` repaints
// it from a scale and opens it; the button's own visibility is the caller's
// business, because only the caller knows whether a scale has loaded yet.
// `t` holds the catalogue strings: title and unit templates, the corner mark's
// own name, and the range words.
export function createScaleDialog(doc, { closeLabel, sourceLabel, disclaimer, lang, t = {} }) {
  const el = doc.createElement('dialog')
  el.className = 'scaleinfo'

  // The corner mark, so a reader who has scrolled the table need not scroll
  // back to the button to leave. Its glyph is CSS content for the reason the
  // (i) button gives: a character here is announced beside the aria-label.
  const dismiss = doc.createElement('button')
  dismiss.type = 'button'
  dismiss.className = 'scaleinfo__dismiss'
  dismiss.setAttribute('aria-label', t.dismiss || closeLabel || '')
  dismiss.addEventListener('click', () => el.close())
  el.appendChild(dismiss)

  const heading = doc.createElement('h2')
  heading.className = 'scaleinfo__title'
  el.appendChild(heading)

  const units = doc.createElement('p')
  units.className = 'scaleinfo__units'
  el.appendChild(units)

  const notes = doc.createElement('p')
  notes.className = 'scaleinfo__notes'
  el.appendChild(notes)

  const list = doc.createElement('ol')
  list.className = 'scaleinfo__bands'
  el.appendChild(list)

  const note = doc.createElement('p')
  note.className = 'scaleinfo__disclaimer'
  note.textContent = disclaimer || ''
  note.hidden = !disclaimer
  el.appendChild(note)

  const source = doc.createElement('a')
  source.className = 'link scaleinfo__source'
  source.textContent = sourceLabel || ''
  // The guideline is somebody else's site: rel keeps the opener unreachable
  // from it, which is the one thing target="_blank" would otherwise hand over.
  source.target = '_blank'
  source.rel = 'noopener noreferrer'
  el.appendChild(source)

  const close = doc.createElement('button')
  close.type = 'button'
  close.className = 'scaleinfo__close'
  close.textContent = closeLabel || ''
  close.addEventListener('click', () => el.close())
  el.appendChild(close)

  // `metric` is the reader-facing metric name; the scale's own id is the fallback.
  function paint(scale, metric) {
    const info = scaleInfo(scale, lang, t.range)
    if (!info) return false
    heading.textContent = fillSlots(t.title, { metric: metric || info.name })
    units.textContent = info.unit ? fillSlots(t.unit, { unit: info.unit }) : ''
    units.hidden = !info.unit
    notes.textContent = info.notes
    notes.hidden = !info.notes

    list.replaceChildren()
    for (const [i, row] of info.rows.entries()) {
      const item = doc.createElement('li')
      item.className = 'scaleinfo__band'
      const mark = swatch(doc, row.colour)
      // Same class outline as the key and the map (A11Y-05).
      outlineSwatch(mark.firstChild, i)
      item.appendChild(mark)

      const name = doc.createElement('span')
      name.className = 'scaleinfo__band-name'
      name.textContent = row.label
      item.appendChild(name)

      const range = doc.createElement('span')
      range.className = 'scaleinfo__band-range'
      range.textContent = row.range
      item.appendChild(range)

      list.appendChild(item)
    }

    // A table that cites nobody gets no link at all rather than one pointing
    // nowhere: the weather bands orient a reader and claim no authority, and
    // a dead "official guideline" would claim one for them.
    source.hidden = !info.source
    if (info.source) source.href = info.source
    else source.removeAttribute('href')
    return true
  }

  return {
    el,
    // Paints without opening, so the source link has an href before the (i) is clicked.
    prime(scale, metric) {
      paint(scale, metric)
    },
    show(scale, metric) {
      if (!paint(scale, metric)) return
      // showModal throws if it is already open — repainting an open dialog is
      // what a metric switch behind it does.
      if (!el.open) el.showModal()
    },
  }
}

function swatch(doc, colour) {
  const NS = 'http://www.w3.org/2000/svg'
  const svg = doc.createElementNS(NS, 'svg')
  svg.setAttribute('class', 'scaleinfo__swatch')
  svg.setAttribute('viewBox', '0 0 1 1')
  svg.setAttribute('preserveAspectRatio', 'none')
  svg.setAttribute('aria-hidden', 'true')
  const rect = doc.createElementNS(NS, 'rect')
  rect.setAttribute('width', '1')
  rect.setAttribute('height', '1')
  rect.setAttribute('fill', colour)
  svg.appendChild(rect)
  return svg
}
