// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest'
import { createHexTip } from '../hextip.js'

let container
beforeEach(() => {
  container = document.createElement('div')
  document.body.replaceChildren(container)
})

const card = () => container.querySelector('.hex-tip')
const live = () => container.querySelector('[role="status"]')

describe('createHexTip', () => {
  it('starts hidden, with the card out of the accessibility tree', () => {
    createHexTip(container)
    expect(card().hidden).toBe(true)
    expect(card().getAttribute('aria-hidden')).toBe('true')
  })

  it('shows text on the card and never adds a live region', () => {
    const tip = createHexTip(container)
    tip.show('PM2.5: 10 µg/m³ · fair', { x: 40, y: 80 })
    expect(card().hidden).toBe(false)
    expect(card().textContent).toBe('PM2.5: 10 µg/m³ · fair')
    expect(live()).toBeNull()
    expect(container.querySelector('[aria-live]')).toBeNull()
  })

  it('hides on hide and removes its element on destroy', () => {
    const tip = createHexTip(container)
    tip.show('a', { x: 1, y: 1 })
    tip.hide()
    expect(card().hidden).toBe(true)
    tip.destroy()
    expect(card()).toBeNull()
  })
})
