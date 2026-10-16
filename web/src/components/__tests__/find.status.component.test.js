// @vitest-environment jsdom
//
// The finder's polite status line: the result count after typing, and the
// confirmation after a pick on the map tab. Focus stays in the input, so a
// screen reader hears nothing about the list unless a live region says it.
import { describe, it, expect, afterEach } from 'vitest'
import { mount, unmount, tick } from 'svelte'
import AreaFind from '../AreaFind.svelte'

const areas = [
  { name: 'Варна', href: '/area/varna' },
  { name: 'Варненска', href: '/area/varnenska' },
  { name: 'Бургас', href: '/area/burgas' },
]
const status = { results: '{n} areas found', resultsOne: '1 area found', centred: 'Map centred on {name}' }

let component
afterEach(() => { if (component) unmount(component) })

function render(props = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  component = mount(AreaFind, {
    target,
    props: {
      areas, lang: 'en', label: 'Find an area', placeholder: 'Area name', hint: 'Use arrows',
      empty: 'No area by that name', onpick: () => {}, status, ...props,
    },
  })
  return target
}

const input = (t) => t.querySelector('input[role="combobox"]')
const live = (t) => t.querySelector('[role="status"]')

async function type(t, value) {
  const el = input(t)
  el.value = value
  el.dispatchEvent(new Event('input', { bubbles: true }))
  await tick()
}

async function enter(t) {
  input(t).dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
  await tick()
}

describe('AreaFind status', () => {
  it('has a polite live region, empty until the reader types', () => {
    const t = render()
    expect(live(t).getAttribute('aria-live')).toBe('polite')
    expect(live(t).textContent).toBe('')
  })

  it('does not announce the whole list just because the field took focus', async () => {
    const t = render()
    input(t).dispatchEvent(new FocusEvent('focus'))
    await tick()
    expect(live(t).textContent).toBe('')
  })

  it('announces how many areas match as the reader types', async () => {
    const t = render()
    await type(t, 'Вар')
    expect(live(t).textContent).toBe('2 areas found')
  })

  it('uses the singular for one match', async () => {
    const t = render()
    await type(t, 'Бур')
    expect(live(t).textContent).toBe('1 area found')
  })

  it('states the empty result with the no-match text', async () => {
    const t = render()
    await type(t, 'Zzz')
    expect(live(t).textContent).toBe('No area by that name')
  })

  it('clears when the field is emptied', async () => {
    const t = render()
    await type(t, 'Вар')
    await type(t, '')
    expect(live(t).textContent).toBe('')
  })

  it('confirms the pick on the map tab, naming the area', async () => {
    const t = render()
    await type(t, 'Бургас')
    await enter(t)
    expect(live(t).textContent).toBe('Map centred on Бургас')
  })

  it('stays silent after a pick where the page navigates instead', async () => {
    const t = render({ status: { ...status, centred: '' } })
    await type(t, 'Бургас')
    await enter(t)
    expect(live(t).textContent).not.toContain('centred')
  })
})
