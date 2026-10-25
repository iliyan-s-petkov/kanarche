// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { mount, unmount, flushSync } from 'svelte'
import DataFreshness from '../DataFreshness.svelte'
import { formatTime, STALE_MS } from '../../lib/freshness.js'

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
const base = { status: '', announce: '', minutes: 5, labels, options, onpick: () => {}, onrefresh: () => {} }

describe('DataFreshness time beside the icon', () => {
  const NOW = Date.parse('2026-10-25T14:30:00Z')
  const timed = (over = {}) => render({ ...base, lang: 'en', at: NOW, nowFn: () => NOW, ...over })
  const time = (el) => el.querySelector('.data-refresh__time')

  it('writes the snapshot time inside the button, in the page locale', () => {
    const el = timed()
    expect(time(el).textContent).toBe(formatTime(NOW, 'en'))
    expect(el.querySelector('.data-refresh__btn--icon').contains(time(el))).toBe(true)
  })

  // WCAG 2.5.3: the visible time must be inside the accessible name, and the word stays so voice control still finds it.
  it('keeps the visible time inside the accessible name', () => {
    const name = timed().querySelector('.data-refresh__btn--icon').getAttribute('aria-label')
    expect(name).toBe(`Обнови ${formatTime(NOW, 'en')}`)
  })

  it('shows no time before there is one, and keeps the accessible name static', () => {
    const el = timed({ at: null })
    expect(time(el)).toBe(null)
    expect(el.querySelector('.data-refresh__btn--icon').getAttribute('aria-label')).toBe('Обнови')
  })

  it('marks a snapshot older than the stale limit', () => {
    expect(time(timed({ at: NOW - STALE_MS - 1000 })).classList.contains('data-refresh__time--stale')).toBe(true)
  })

  it('does not mark a snapshot inside the stale limit', () => {
    expect(time(timed({ at: NOW - STALE_MS + 1000 })).classList.contains('data-refresh__time--stale')).toBe(false)
  })

  it('keeps the time and flags aria-busy while a refresh is in flight', () => {
    const el = timed({ busy: true })
    expect(el.querySelector('.data-refresh__btn--icon').getAttribute('aria-busy')).toBe('true')
    expect(time(el).textContent).toBe(formatTime(NOW, 'en'))
  })

  it('greys the time while a replay is open', () => {
    expect(time(timed({ paused: true })).classList.contains('data-refresh__time--paused')).toBe(true)
    expect(time(timed({ paused: false })).classList.contains('data-refresh__time--paused')).toBe(false)
  })
})

describe('DataFreshness', () => {
  // Clipped, never hidden: display:none would silence the announcement.
  it('carries a live region even before it has anything to report', () => {
    const el = render({ ...base })
    const status = el.querySelector('.data-refresh__status')
    expect(status.getAttribute('role')).toBe('status')
    expect(status.classList.contains('sr-only')).toBe(true)
    expect(status.textContent).toBe('')
  })

  // D4: the plain time is never live. Only a band change or a failure is spoken.
  it('keeps the plain time out of the live region', () => {
    const el = render({ ...base, status: 'Данни от 14:07' })
    expect(el.querySelector('[role="status"]').textContent).toBe('')
  })

  it('speaks the announcement it is given', () => {
    const el = render({ ...base, status: 'Данни от 14:07', announce: 'София: качеството на въздуха вече е лошо, преди беше добро.' })
    expect(el.querySelector('[role="status"]').textContent).toBe('София: качеството на въздуха вече е лошо, преди беше добро.')
  })

  // The accessible name is the static word. A time inside it changes the name
  // on every refresh and breaks voice control (WCAG 2.5.3, 4.1.2). The time
  // stays in the title for hover only.
  it('puts the time in the hover title but keeps the accessible name static', () => {
    const el = render({ ...base, status: 'Данни от 14:07' })
    const btn = el.querySelector('.data-refresh__btn--icon')
    expect(btn.title).toBe('Обнови · Данни от 14:07')
    expect(btn.getAttribute('aria-label')).toBe('Обнови')
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
