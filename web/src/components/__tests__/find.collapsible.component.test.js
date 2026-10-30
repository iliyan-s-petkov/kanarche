// @vitest-environment jsdom
//
// The finder's icon form on a narrow touch screen: a named button that opens
// the field, and the ways the field closes again. jsdom has no media queries,
// so matchMedia is stubbed to say the screen is narrow.
import { describe, it, expect, afterEach, vi } from 'vitest'
import { mount, unmount, tick } from 'svelte'
import AreaFind from '../AreaFind.svelte'

const areas = [{ name: 'Varna', href: '/area/varna' }]

let component
afterEach(() => {
  if (component) unmount(component)
  component = null
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

function render({ narrow = true, ...props } = {}) {
  vi.stubGlobal('matchMedia', () => ({ matches: narrow }))
  const target = document.createElement('div')
  document.body.appendChild(target)
  component = mount(AreaFind, {
    target,
    props: {
      areas, lang: 'en', label: 'Find an area', placeholder: 'Area or address', hint: 'Use arrows',
      empty: 'No area by that name', onpick: () => {}, collapsible: true, ...props,
    },
  })
  return target
}

const btn = (t) => t.querySelector('.find-toggle')
const input = (t) => t.querySelector('input[role="combobox"]')
const root = (t) => t.querySelector('.combobox')

describe('collapsible finder', () => {
  it('has no button unless collapsible', () => {
    const t = render({ collapsible: false })
    expect(btn(t)).toBeNull()
  })

  it('renders a real button named by the finder label, closed', () => {
    const t = render()
    expect(btn(t).tagName).toBe('BUTTON')
    expect(btn(t).getAttribute('aria-label')).toBe('Find an area')
    expect(btn(t).getAttribute('aria-expanded')).toBe('false')
    expect(root(t).classList.contains('find--open')).toBe(false)
  })

  it('opens on click and moves focus into the input', async () => {
    const t = render()
    btn(t).click()
    await tick()
    await tick()
    expect(btn(t).getAttribute('aria-expanded')).toBe('true')
    expect(root(t).classList.contains('find--open')).toBe(true)
    expect(document.activeElement).toBe(input(t))
  })

  it('Escape closes it and focus returns to the button', async () => {
    const t = render()
    btn(t).click()
    await tick()
    await tick()
    input(t).dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await tick()
    await tick()
    expect(root(t).classList.contains('find--open')).toBe(false)
    expect(document.activeElement).toBe(btn(t))
  })

  it('blur with an empty field closes it, with text it stays', async () => {
    vi.useFakeTimers()
    const t = render()
    btn(t).click()
    await tick()
    await tick()
    input(t).value = 'Var'
    input(t).dispatchEvent(new Event('input', { bubbles: true }))
    input(t).blur()
    vi.runAllTimers()
    await tick()
    expect(root(t).classList.contains('find--open')).toBe(true)
    input(t).value = ''
    input(t).dispatchEvent(new Event('input', { bubbles: true }))
    input(t).focus()
    input(t).blur()
    vi.runAllTimers()
    await tick()
    expect(root(t).classList.contains('find--open')).toBe(false)
    vi.useRealTimers()
  })

  it('choosing a result closes it and focuses the button', async () => {
    const picked = []
    const t = render({ onpick: (m) => picked.push(m.name) })
    btn(t).click()
    await tick()
    await tick()
    input(t).value = 'Var'
    input(t).dispatchEvent(new Event('input', { bubbles: true }))
    await tick()
    input(t).dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    await tick()
    await tick()
    expect(picked).toEqual(['Varna'])
    expect(root(t).classList.contains('find--open')).toBe(false)
    expect(document.activeElement).toBe(btn(t))
  })

  it('on a wide screen Escape keeps its two stages and nothing collapses', async () => {
    const t = render({ narrow: false })
    input(t).focus()
    input(t).value = 'Var'
    input(t).dispatchEvent(new Event('input', { bubbles: true }))
    await tick()
    input(t).dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await tick()
    expect(input(t).value).toBe('Var')
    expect(document.activeElement).toBe(input(t))
  })
})
