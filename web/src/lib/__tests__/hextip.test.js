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
    expect(live().getAttribute('aria-live')).toBe('polite')
  })

  it('shows text on the card without speaking it', () => {
    const tip = createHexTip(container)
    tip.show('PM2.5: 10 µg/m³ · fair', { x: 40, y: 80 })
    expect(card().hidden).toBe(false)
    expect(card().textContent).toBe('PM2.5: 10 µg/m³ · fair')
    expect(live().textContent).toBe('')
  })

  it('speaks only when announce is called, and hide clears both', () => {
    const tip = createHexTip(container)
    tip.show('a', { x: 1, y: 1 })
    tip.announce('PM2.5: 10 µg/m³ · fair')
    expect(live().textContent).toBe('PM2.5: 10 µg/m³ · fair')
    tip.hide()
    expect(card().hidden).toBe(true)
    expect(live().textContent).toBe('')
  })

  it('removes its elements on destroy', () => {
    createHexTip(container).destroy()
    expect(card()).toBeNull()
    expect(live()).toBeNull()
  })
})
