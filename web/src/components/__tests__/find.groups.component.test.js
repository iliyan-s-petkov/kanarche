// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { mount, unmount, tick } from 'svelte'
import AreaFind from '../AreaFind.svelte'

const areas = [
  { name: 'Varna', area: { kind: 'city' } },
  { name: 'Lozenets', area: { kind: 'neighbourhood' } },
  { name: 'Sofia', area: { kind: 'city' } },
  { name: 'Vitosha', area: { kind: 'neighbourhood' } },
]
const groups = { district: 'City districts', place: 'Cities and provinces' }

let component
afterEach(() => { if (component) unmount(component) })

function render(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  component = mount(AreaFind, {
    target,
    props: {
      areas, lang: 'en', groups, label: 'Find', placeholder: '', hint: '', empty: 'None',
      onpick: () => {}, ...props,
    },
  })
  return target
}

const input = (t) => t.querySelector('input[role="combobox"]')
const names = (t) => [...t.querySelectorAll('.combobox__opt')].map((li) => li.textContent)
const headings = (t) => [...t.querySelectorAll('.combobox__group-label')].map((h) => h.textContent)

async function type(t, value) {
  input(t).value = value
  input(t).dispatchEvent(new Event('input', { bubbles: true }))
  await tick()
}
async function key(t, k) {
  input(t).dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }))
  await tick()
}

describe('AreaFind groups', () => {
  it('shows districts above the rest under two labelled groups', () => {
    const t = render()
    expect(headings(t)).toEqual(['City districts', 'Cities and provinces'])
    expect(names(t)).toEqual(['Lozenets', 'Vitosha', 'Sofia', 'Varna'])
  })

  it('wraps each group in role=group labelled by its heading', () => {
    const t = render()
    const gs = [...t.querySelectorAll('[role="group"]')]
    expect(gs).toHaveLength(2)
    for (const g of gs) {
      const label = t.querySelector(`#${g.getAttribute('aria-labelledby')}`)
      expect(label.classList.contains('combobox__group-label')).toBe(true)
    }
    expect(gs[0].querySelectorAll('[role="option"]')).toHaveLength(2)
  })

  it('hides the heading of a group with no matches, and both when only one kind is left', async () => {
    const t = render()
    await type(t, 'loz')
    expect(headings(t)).toEqual([])
    expect(names(t)).toEqual(['Lozenets'])
    await type(t, 'sof')
    expect(headings(t)).toEqual([])
  })

  it('shows no headings when no entry carries a kind', () => {
    const t = render({ areas: [{ name: 'Varna', href: '/a' }, { name: 'Sofia', href: '/b' }] })
    expect(headings(t)).toEqual([])
    expect(names(t)).toEqual(['Sofia', 'Varna'])
  })

  it('arrow keys step over options only, never onto a heading', async () => {
    const t = render()
    await key(t, 'ArrowDown')
    for (let i = 0; i < 3; i++) await key(t, 'ArrowDown')
    const id = input(t).getAttribute('aria-activedescendant')
    expect(t.querySelector(`#${id}`).textContent).toBe('Varna')
    await key(t, 'ArrowDown')
    expect(t.querySelector(`#${input(t).getAttribute('aria-activedescendant')}`).textContent).toBe('Lozenets')
    expect(t.querySelectorAll('[role="option"]')).toHaveLength(4)
  })

  it('keeps the address row last, outside the groups', async () => {
    const t = render({ areas: [{ name: 'Sofia', area: { kind: 'city' } }, { name: 'Sofia Vitosha', area: { kind: 'neighbourhood' } }], address: { search: async () => [], onpick() {}, row: 'Search address:', loading: '', none: '', busy: '', error: '', credit: '', creditHref: '' } })
    await type(t, 'sof')
    expect(headings(t)).toHaveLength(2)
    const opts = [...t.querySelectorAll('[role="option"]')]
    expect(opts.at(-1).textContent).toContain('Search address:')
    expect(opts.at(-1).closest('[role="group"]')).toBeNull()
  })
})
