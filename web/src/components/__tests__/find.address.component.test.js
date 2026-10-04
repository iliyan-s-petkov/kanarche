// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, unmount, tick } from 'svelte'
import AreaFind from '../AreaFind.svelte'

const areas = [
  { name: 'Варна', href: '/area/varna' },
  { name: 'Бургас', href: '/area/burgas' },
  { name: 'Велико Търново', href: '/area/veliko-tarnovo' },
]
const result = { label: 'бул. Витоша 1, София', lat: 42.69, lon: 23.32, bbox: [23.31, 42.68, 23.33, 42.7] }

let component
afterEach(() => { if (component) unmount(component) })

function deferred() {
  let resolve, reject
  const promise = new Promise((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

function render(over = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const address = {
    search: vi.fn().mockResolvedValue([result]),
    onpick: vi.fn(),
    onclear: vi.fn(),
    row: 'Търси адрес:',
    loading: 'Търсене…',
    none: 'Няма намерен адрес.',
    busy: 'Заето, опитайте след малко.',
    error: 'Търсенето не успя.',
    credit: '© сътрудниците на OpenStreetMap',
    creditHref: 'https://www.openstreetmap.org/copyright',
    ...over,
  }
  component = mount(AreaFind, {
    target,
    props: {
      areas, label: 'Търсене', placeholder: 'Област или адрес', hint: 'h', empty: 'Няма район',
      onpick: () => {}, address,
    },
  })
  return { t: target, address }
}

const input = (t) => t.querySelector('input[role="combobox"]')
const opts = (t) => [...t.querySelectorAll('.combobox__opt')]
const texts = (t) => opts(t).map((li) => li.textContent.trim())

async function type(t, value) {
  const el = input(t)
  el.value = value
  el.dispatchEvent(new Event('input', { bubbles: true }))
  await tick()
}
async function key(t, k) {
  const e = new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true })
  input(t).dispatchEvent(e)
  await tick()
  return e
}
const settle = async () => { for (let i = 0; i < 5; i++) { await Promise.resolve(); await tick() } }
const mousedown = (el) => el.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true }))

describe('AreaFind address search', () => {
  it('ends the dropdown with the address row once three characters are typed', async () => {
    const { t } = render()
    await type(t, 'вел')
    expect(texts(t)).toEqual(['Велико Търново', 'Търси адрес: вел'])
    expect(opts(t).at(-1).classList.contains('combobox__opt--address')).toBe(true)
  })

  it('keeps the address row last when no area matches, beside the no-area line', async () => {
    const { t } = render()
    await type(t, 'ул. Витоша')
    expect(texts(t)).toEqual(['Търси адрес: ул. Витоша'])
    expect(t.querySelector('.combobox__empty').textContent).toBe('Няма район')
  })

  it('offers no address row under three characters or with no address support', async () => {
    const { t } = render()
    await type(t, 'ва')
    expect(texts(t).some((x) => x.startsWith('Търси адрес'))).toBe(false)
    await type(t, '  в  ')
    expect(texts(t).some((x) => x.startsWith('Търси адрес'))).toBe(false)

    unmount(component)
    const target = document.createElement('div')
    document.body.appendChild(target)
    component = mount(AreaFind, { target, props: { areas, label: 'l', placeholder: 'p', hint: 'h', empty: 'e', onpick: () => {} } })
    await type(target, 'варна')
    expect(texts(target)).toEqual(['Варна'])
  })

  it('makes no request while typing, focusing or moving the cursor', async () => {
    const { t, address } = render()
    input(t).dispatchEvent(new Event('focus', { bubbles: true }))
    await type(t, 'в')
    await type(t, 'вит')
    await type(t, 'витоша 1')
    await key(t, 'ArrowDown')
    await key(t, 'ArrowUp')
    await settle()
    expect(address.search).not.toHaveBeenCalled()
  })

  it('searches on Enter when no row is highlighted and no area is unambiguous', async () => {
    const { t, address } = render()
    await type(t, '  бул. Витоша 1 ')
    await key(t, 'Enter')
    await settle()
    expect(address.search).toHaveBeenCalledTimes(1)
    expect(address.search).toHaveBeenCalledWith('бул. Витоша 1')
  })

  it('searches on Enter when the cursor is on the address row', async () => {
    const { t, address } = render()
    await type(t, 'вел')
    await key(t, 'ArrowUp') // wraps to the last row, the address row
    await key(t, 'Enter')
    await settle()
    expect(address.search).toHaveBeenCalledWith('вел')
  })

  it('searches on a click on the address row', async () => {
    const { t, address } = render()
    await type(t, 'вел')
    mousedown(opts(t).at(-1))
    await settle()
    expect(address.search).toHaveBeenCalledWith('вел')
  })

  it('does not search when an area row is highlighted: Enter picks the area', async () => {
    const onpick = vi.fn()
    const target = document.createElement('div')
    document.body.appendChild(target)
    const search = vi.fn().mockResolvedValue([])
    component = mount(AreaFind, {
      target,
      props: { areas, label: 'l', placeholder: 'p', hint: 'h', empty: 'e', onpick, address: { search, row: 'Търси адрес:' } },
    })
    await type(target, 'вел')
    await key(target, 'ArrowDown')
    await key(target, 'Enter')
    await settle()
    expect(onpick).toHaveBeenCalledWith(expect.objectContaining({ href: '/area/veliko-tarnovo' }))
    expect(search).not.toHaveBeenCalled()
  })

  it('never searches a query under three characters, even on Enter', async () => {
    const { t, address } = render()
    await type(t, 'ул')
    await key(t, 'Enter')
    await settle()
    expect(address.search).not.toHaveBeenCalled()
  })

  it('shows a loading line while the request is out', async () => {
    const d = deferred()
    const { t } = render({ search: vi.fn(() => d.promise) })
    await type(t, 'бул. Витоша')
    await key(t, 'Enter')
    expect(t.querySelector('.combobox__empty').textContent).toBe('Търсене…')
    expect(t.querySelector('.combobox__empty').getAttribute('data-state')).toBe('loading')
    d.resolve([result])
    await settle()
    expect(texts(t)).toEqual(['бул. Витоша 1, София'])
  })

  it('lists the results with the OSM credit linking to the copyright page', async () => {
    const { t } = render()
    await type(t, 'бул. Витоша')
    await key(t, 'Enter')
    await settle()
    expect(texts(t)).toEqual(['бул. Витоша 1, София'])
    const a = t.querySelector('.combobox__credit a')
    expect(a.textContent).toBe('© сътрудниците на OpenStreetMap')
    expect(a.getAttribute('href')).toBe('https://www.openstreetmap.org/copyright')
    expect(a.getAttribute('rel')).toContain('noopener')
    expect(t.querySelector('ul.combobox__list').hidden).toBe(false)
  })

  it('picks a result, closes the list and keeps focus on the input', async () => {
    const { t, address } = render()
    input(t).focus()
    await type(t, 'бул. Витоша')
    await key(t, 'Enter')
    await settle()
    await key(t, 'ArrowDown')
    await key(t, 'Enter')
    expect(address.onpick).toHaveBeenCalledWith(result)
    expect(t.querySelector('ul.combobox__list').hidden).toBe(true)
    expect(document.activeElement).toBe(input(t))
  })

  it('picks a result with the mouse', async () => {
    const { t, address } = render()
    await type(t, 'бул. Витоша')
    await key(t, 'Enter')
    await settle()
    mousedown(opts(t)[0])
    await tick()
    expect(address.onpick).toHaveBeenCalledWith(result)
  })

  it('says plainly when no address was found', async () => {
    const { t } = render({ search: vi.fn().mockResolvedValue([]) })
    await type(t, 'нищо такова')
    await key(t, 'Enter')
    await settle()
    expect(t.querySelector('.combobox__empty').textContent).toBe('Няма намерен адрес.')
    expect(t.querySelector('.combobox__empty').getAttribute('data-state')).toBe('none')
  })

  it('says so when the service is busy', async () => {
    const err = Object.assign(new Error('busy'), { kind: 'busy' })
    const { t } = render({ search: vi.fn().mockRejectedValue(err) })
    await type(t, 'нищо такова')
    await key(t, 'Enter')
    await settle()
    expect(t.querySelector('.combobox__empty').textContent).toBe('Заето, опитайте след малко.')
    expect(t.querySelector('.combobox__empty').getAttribute('data-state')).toBe('busy')
  })

  it('says so on a network error', async () => {
    const { t } = render({ search: vi.fn().mockRejectedValue(new TypeError('net')) })
    await type(t, 'нищо такова')
    await key(t, 'Enter')
    await settle()
    expect(t.querySelector('.combobox__empty').textContent).toBe('Търсенето не успя.')
    expect(t.querySelector('.combobox__empty').getAttribute('data-state')).toBe('error')
  })

  it('clears the previous pin when a new search starts', async () => {
    const { t, address } = render()
    await type(t, 'бул. Витоша')
    await key(t, 'Enter')
    await settle()
    expect(address.onclear).toHaveBeenCalledTimes(1)
    await type(t, 'бул. Витоша 2')
    await key(t, 'Enter')
    await settle()
    expect(address.onclear).toHaveBeenCalledTimes(2)
  })

  it('clears the pin on Escape and when the field is emptied', async () => {
    const { t, address } = render()
    await type(t, 'бул. Витоша')
    await key(t, 'Enter')
    await settle()
    address.onclear.mockClear()
    await key(t, 'Escape')
    expect(address.onclear).toHaveBeenCalled()
    address.onclear.mockClear()
    await type(t, '')
    expect(address.onclear).toHaveBeenCalled()
  })

  it('drops stale results once the reader types again', async () => {
    const d = deferred()
    const { t } = render({ search: vi.fn(() => d.promise) })
    await type(t, 'бул. Витоша')
    await key(t, 'Enter')
    await type(t, 'вел')
    d.resolve([result])
    await settle()
    expect(texts(t)).toEqual(['Велико Търново', 'Търси адрес: вел'])
  })

  it('returns to the area rows when the reader edits after results', async () => {
    const { t } = render()
    await type(t, 'бул. Витоша')
    await key(t, 'Enter')
    await settle()
    await type(t, 'бул. Витош')
    expect(texts(t)).toEqual(['Търси адрес: бул. Витош'])
  })
})
