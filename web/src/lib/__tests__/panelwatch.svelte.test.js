// @vitest-environment jsdom
import { describe, it, expect, afterEach, vi } from 'vitest'
import { flushSync } from 'svelte'
import { watchPanelScroll, returnFocus } from '../panelwatch.svelte.js'

let stop
afterEach(() => {
  stop?.()
  stop = null
  document.body.replaceChildren()
})

function setup() {
  const state = $state({ id: null })
  const onReturn = vi.fn()
  const scroll = { opened: vi.fn(), closed: () => true }
  stop = watchPanelScroll({ shownId: () => state.id, initialId: null, scroll, onReturn })
  flushSync()
  return { state, onReturn }
}

describe('watchPanelScroll', () => {
  it('hands the close the element that had focus when the sensor opened', () => {
    const { state, onReturn } = setup()
    const opener = document.createElement('button')
    document.body.append(opener)
    opener.focus()
    state.id = 7
    flushSync()
    document.body.focus()
    state.id = null
    flushSync()
    expect(onReturn.mock.calls.at(-1)[0]).toBe(opener)
  })

  it('keeps the first opener while the reader moves between sensors', () => {
    const { state, onReturn } = setup()
    const opener = document.createElement('button')
    const other = document.createElement('button')
    document.body.append(opener, other)
    opener.focus()
    state.id = 7
    flushSync()
    other.focus()
    state.id = 8
    flushSync()
    state.id = null
    flushSync()
    expect(onReturn.mock.calls.at(-1)[0]).toBe(opener)
  })
})

describe('returnFocus', () => {
  function canvas() {
    const c = document.createElement('canvas')
    c.className = 'maplibregl-canvas'
    c.tabIndex = 0
    document.body.append(c)
    return c
  }

  it('focuses the opener while it is on the page', () => {
    canvas()
    const opener = document.createElement('button')
    document.body.append(opener)
    returnFocus(opener)
    expect(document.activeElement).toBe(opener)
  })

  it('falls back to the map canvas when the opener is gone', () => {
    const c = canvas()
    returnFocus(document.createElement('button'))
    expect(document.activeElement).toBe(c)
  })

  it('falls back to the map canvas when nothing had focus', () => {
    const c = canvas()
    returnFocus(document.body)
    expect(document.activeElement).toBe(c)
  })
})
