// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { mount, unmount } from 'svelte'
import Readouts from '../Readouts.svelte'

let component = null
afterEach(() => {
  if (component) unmount(component)
  component = null
  document.body.innerHTML = ''
})

function render(cards) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  component = mount(Readouts, { target, props: { cards } })
  return target
}

const gauged = { label: 'Максимум · ФПЧ2.5', metric: 'ФПЧ2.5', value: '30', unit: 'µg/m³', tier: 'Овча купел · 9 сензора', band: { colour: '#a00', word: 'лошо' } }
const plain = { label: 'Този сензор', metric: 'ФПЧ2.5', value: '1', unit: 'от 9', tier: 'Под медианата', band: null }

describe('Readouts spoken units', () => {
  afterEach(() => { document.head.innerHTML = '' })

  it('hides the symbol from speech and says the words beside it', () => {
    document.head.innerHTML = '<meta name="spoken-units" content=\'{"µg/m³":"micrograms per cubic metre"}\'>'
    const el = render([gauged])
    const unit = el.querySelector('.readout__unit')
    expect(unit.textContent).toBe('µg/m³')
    expect(unit.getAttribute('aria-hidden')).toBe('true')
    expect(el.querySelector('.readout__value .sr-only').textContent.trim()).toBe('micrograms per cubic metre')
  })

  it('leaves a unit that is not a symbol alone', () => {
    document.head.innerHTML = '<meta name="spoken-units" content=\'{"µg/m³":"micrograms per cubic metre"}\'>'
    const el = render([plain])
    expect(el.querySelector('.readout__unit').getAttribute('aria-hidden')).toBe(null)
    expect(el.querySelector('.sr-only')).toBe(null)
  })
})

describe('Readouts', () => {
  it('renders one card per figure, in the classes the server uses', () => {
    const el = render([gauged, plain])
    expect(el.querySelectorAll('.readout.card')).toHaveLength(2)
    expect(el.querySelector('.readout__label').textContent).toBe('Максимум · ФПЧ2.5')
    expect(el.querySelector('.readout__tier').textContent).toBe('Овча купел · 9 сензора')
  })

  // style-src is 'self' with no 'unsafe-inline': a style attribute would be
  // dropped and the bar would never paint. Presentation attributes, as in
  // base.gohtml's own strip.
  it('paints the band bar with a presentation attribute and no inline style', () => {
    const el = render([gauged])
    const rect = el.querySelector('.readout__bar rect')
    expect(rect.getAttribute('fill')).toBe('#a00')
    expect(el.querySelector('.readout__bar').getAttribute('style')).toBe(null)
    expect(el.querySelector('.readout__band').textContent).toBe('лошо')
    expect(el.querySelector('.gauge')).toBe(null)
  })

  // A count or an axis metric has no health class: no bar and no word.
  it('leaves a card with no band a plain figure', () => {
    const el = render([plain])
    expect(el.querySelector('.readout__bar')).toBe(null)
    expect(el.querySelector('.readout__band')).toBe(null)
    // The unit sits inside the value, as it does in base.gohtml, with the
    // space Svelte would otherwise trim out of the markup.
    expect(el.querySelector('.readout__value').textContent).toBe('1 от 9')
    expect(el.querySelector('.readout__unit').textContent).toBe('от 9')
  })

  it('renders an empty strip for no cards at all', () => {
    expect(render([]).querySelectorAll('.readout')).toHaveLength(0)
  })

  // Task 9 round 1: the phone rule hides .readout__metric on this island's
  // cards too, same as the server's own strip. The island's high/low/median
  // labels put the metric last ("Highest · PM2.5"), so the span holds the
  // metric and the rest still prints, in its original order.
  it('splits the metric into its own span, rest in the original order', () => {
    const el = render([gauged])
    const span = el.querySelector('.readout__metric')
    expect(span.textContent).toBe(' · ФПЧ2.5')
    expect(el.querySelector('.readout__label').textContent).toBe('Максимум · ФПЧ2.5')
  })

  // A label the metric text does not appear in (e.g. "This sensor") gets no
  // span at all — nothing to hide, nothing wrapped.
  it('leaves a label with no metric segment as plain text', () => {
    const el = render([plain])
    expect(el.querySelector('.readout__metric')).toBe(null)
    expect(el.querySelector('.readout__label').textContent).toBe('Този сензор')
  })
})
