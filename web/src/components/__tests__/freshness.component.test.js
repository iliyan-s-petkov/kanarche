// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { mount, unmount, flushSync } from 'svelte'
import DataFreshness from '../DataFreshness.svelte'

let component
afterEach(() => {
  if (component) unmount(component)
  component = null
  document.body.innerHTML = ''
})

function render(props) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  component = mount(DataFreshness, { target, props })
  return target
}

const labels = { trigger: 'Обнови', now: 'Обнови сега', group: 'Автоматично обновяване' }
const options = [
  { value: 0, text: 'Изключено' },
  { value: 5, text: 'На всеки 5 мин' },
  { value: 15, text: 'На всеки 15 мин' },
  { value: 30, text: 'На всеки 30 мин' },
]
const base = { status: '', minutes: 5, labels, options, onpick: () => {}, onrefresh: () => {} }

describe('DataFreshness', () => {
  // Clipped, never hidden: display:none would silence the announcement.
  it('carries a live region even before it has anything to report', () => {
    const el = render({ ...base })
    const status = el.querySelector('.data-refresh__status')
    expect(status.getAttribute('role')).toBe('status')
    expect(status.classList.contains('sr-only')).toBe(true)
    expect(status.textContent).toBe('')
  })

  it('shows whatever the store says', () => {
    const el = render({ ...base, status: 'Данни от 14:07' })
    expect(el.querySelector('.data-refresh__status').textContent).toBe('Данни от 14:07')
  })

  it('puts the time beside the name where a hover reaches it', () => {
    const el = render({ ...base, status: 'Данни от 14:07' })
    const btn = el.querySelector('.data-refresh__btn--icon')
    expect(btn.title).toBe('Обнови · Данни от 14:07')
    expect(btn.getAttribute('aria-label')).toBe(btn.title)
  })

  // A dangling "·" is the tell that the empty status was interpolated anyway.
  it('states the name alone when there is no reading to date', () => {
    expect(render({ ...base }).querySelector('.data-refresh__btn--icon').title).toBe('Обнови')
  })

  it('is one icon button that starts closed', () => {
    const el = render({ ...base })
    const btn = el.querySelector('.data-refresh__btn--icon')
    expect(btn.getAttribute('aria-expanded')).toBe('false')
    expect(btn.getAttribute('aria-controls')).toBe(el.querySelector('.data-refresh__panel').id)
    expect(el.querySelector('.data-refresh__panel').hidden).toBe(true)
    expect(el.querySelector('[role="switch"]')).toBe(null)
  })

  it('opens on a tap and closes on a second', () => {
    const el = render({ ...base })
    const btn = el.querySelector('.data-refresh__btn--icon')
    btn.click()
    flushSync()
    expect(btn.getAttribute('aria-expanded')).toBe('true')
    expect(el.querySelector('.data-refresh__panel').hidden).toBe(false)
    btn.click()
    flushSync()
    expect(el.querySelector('.data-refresh__panel').hidden).toBe(true)
  })

  it('offers Refresh now and a labelled radiogroup of the four intervals', () => {
    const el = render({ ...base, minutes: 15 })
    expect(el.querySelector('.data-refresh__now').textContent.trim()).toBe('Обнови сега')
    const group = el.querySelector('[role="radiogroup"]')
    expect(document.getElementById(group.getAttribute('aria-labelledby')).textContent).toBe('Автоматично обновяване')
    const radios = [...group.querySelectorAll('input[type="radio"]')]
    expect(radios.map((r) => r.parentElement.textContent.trim())).toEqual(options.map((o) => o.text))
    expect(radios.map((r) => r.checked)).toEqual([false, false, true, false])
  })

  it('reports the interval picked', () => {
    const seen = []
    const el = render({ ...base, onpick: (m) => seen.push(m) })
    const radios = el.querySelectorAll('input[type="radio"]')
    radios[3].click()
    flushSync()
    radios[0].click()
    flushSync()
    expect(seen).toEqual([30, 0])
  })

  it('refreshes and returns focus to the icon on Refresh now', () => {
    let asked = 0
    const el = render({ ...base, onrefresh: () => { asked += 1 } })
    const btn = el.querySelector('.data-refresh__btn--icon')
    btn.click()
    flushSync()
    el.querySelector('.data-refresh__now').click()
    flushSync()
    expect(asked).toBe(1)
    expect(el.querySelector('.data-refresh__panel').hidden).toBe(true)
    expect(document.activeElement).toBe(btn)
  })

  it('closes on Escape, returns focus and does not let the key reach a parent panel', () => {
    const el = render({ ...base })
    const btn = el.querySelector('.data-refresh__btn--icon')
    let reached = 0
    document.body.addEventListener('keydown', () => { reached += 1 })
    btn.click()
    flushSync()
    el.querySelector('input[type="radio"]').focus()
    el.querySelector('input[type="radio"]').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    flushSync()
    expect(el.querySelector('.data-refresh__panel').hidden).toBe(true)
    expect(document.activeElement).toBe(btn)
    expect(reached).toBe(0)
  })

  it('closes on a press outside and keeps open for one inside', () => {
    const el = render({ ...base })
    el.querySelector('.data-refresh__btn--icon').click()
    flushSync()
    el.querySelector('.data-refresh__panel').dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    flushSync()
    expect(el.querySelector('.data-refresh__panel').hidden).toBe(false)
    document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    flushSync()
    expect(el.querySelector('.data-refresh__panel').hidden).toBe(true)
  })

  // Not disabled: a disabled button would drop keyboard focus mid-request.
  it('marks itself busy without going unfocusable', () => {
    const btn = render({ ...base, busy: true }).querySelector('.data-refresh__btn--icon')
    expect(btn.getAttribute('aria-busy')).toBe('true')
    expect(btn.disabled).toBe(false)
  })
})
