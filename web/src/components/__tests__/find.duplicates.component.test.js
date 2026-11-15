// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { mount, unmount, flushSync } from 'svelte'
import AreaFind from '../AreaFind.svelte'

// Production has districts with the same name in different cities (Младост in
// Sofia and in Varna). A keyed list on the name throws each_key_duplicate,
// which aborts the whole effect flush and with it the desktop map dock.
const areas = [
  { name: 'Младост', href: '/area/mladost', area: { kind: 'neighbourhood' } },
  { name: 'Младост', href: '/area/mladost-varna', area: { kind: 'neighbourhood' } },
  { name: 'София', href: '/area/sofia', area: { kind: 'city' } },
]

let component
afterEach(() => { if (component) unmount(component); component = null })

function render(groups) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  component = mount(AreaFind, {
    target,
    props: { areas, lang: 'bg', groups, label: 'Търсене', placeholder: '', hint: '', empty: 'Няма', onpick: () => {} },
  })
  flushSync()
  return target
}

const names = (t) => [...t.querySelectorAll('.combobox__opt')].map((li) => li.textContent)

describe('AreaFind with duplicate names', () => {
  it('lists both districts when grouped', () => {
    const t = render({ district: 'Райони', place: 'Градове' })
    expect(names(t)).toEqual(['Младост', 'Младост', 'София'])
  })

  it('lists both districts when not grouped', () => {
    const t = render(undefined)
    expect(names(t)).toEqual(['Младост', 'Младост', 'София'])
  })
})
