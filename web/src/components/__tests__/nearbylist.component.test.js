// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { mount, unmount } from 'svelte'
import NearbyList from '../NearbyList.svelte'

let component = null
afterEach(() => {
  if (component) unmount(component)
  component = null
  document.body.innerHTML = ''
})

function render(items) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  component = mount(NearbyList, {
    target,
    props: { items, total: items.length, rest: 0, title: 'Sensors in view', empty: '', more: '', onopen() {}, onpreview() {}, onunpreview() {} },
  })
  return target
}

const item = { id: 1, name: 'Sensor 1', place: '2 km north of the map centre', reading: '12.3 µg/m³ · good', readingSpoken: '12.3 micrograms per cubic metre · good' }

describe('NearbyList heading', () => {
  it('puts the title and count in an h3 inside the summary', () => {
    const el = render([item])
    expect(el.querySelector('summary > h3').textContent).toBe('Sensors in view (1)')
  })
})

describe('NearbyList spoken reading', () => {
  it('hides the printed reading from speech and gives the spoken one', () => {
    const el = render([item])
    const printed = el.querySelector('.nearby__reading')
    expect(printed.textContent).toBe('12.3 µg/m³ · good')
    expect(printed.getAttribute('aria-hidden')).toBe('true')
    expect(el.querySelector('.nearby__item .sr-only').textContent).toBe('12.3 micrograms per cubic metre · good')
  })

  it('does not double a reading that has no separate spoken form', () => {
    const el = render([{ ...item, readingSpoken: '' }])
    expect(el.querySelector('.nearby__reading').getAttribute('aria-hidden')).toBe(null)
    expect(el.querySelector('.nearby__item .sr-only')).toBe(null)
  })
})
