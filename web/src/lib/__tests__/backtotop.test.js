// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { createBackToTop } from '../backtotop.js'

// Stub IntersectionObserver: the test fires the callback by hand, jsdom has no geometry.
function stubIO() {
  let cb = null
  const observe = vi.fn()
  function IO(fn, opts) { cb = fn; this.opts = opts; this.observe = observe }
  return { IO, observe, // rootTop is the top of the observed band: half the viewport height once rootMargin trims the top by 50%.
    fire: (isIntersecting, bottom, rootTop = 450) => cb([{ isIntersecting, boundingClientRect: { bottom }, rootBounds: { top: rootTop } }]) }
}

function page({ withMap = true } = {}) {
  document.body.innerHTML = `
    ${withMap ? '<div class="map-shell"></div>' : ''}
    <main><h1>Title</h1></main>
    <div class="back-to-top-sentinel"></div>
    <button type="button" class="back-to-top"></button>`
  return {
    btn: document.querySelector('.back-to-top'),
    h1: document.querySelector('h1'),
    shell: document.querySelector('.map-shell'),
    sentinel: document.querySelector('.back-to-top-sentinel'),
  }
}

function env({ reduce = false } = {}) {
  const io = stubIO()
  const scrollTo = vi.fn()
  const win = { scrollTo, matchMedia: (q) => ({ matches: q.includes('reduced-motion') && reduce }) }
  return { ...io, win, scrollTo, doc: document }
}

const VISIBLE = 'back-to-top--visible'

describe('createBackToTop', () => {
  it('does nothing without the button', () => {
    document.body.innerHTML = '<div class="map-shell"></div>'
    const e = env()
    expect(createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })).toBeNull()
  })

  it('does nothing without IntersectionObserver', () => {
    page()
    const e = env()
    expect(createBackToTop({ doc: e.doc, win: e.win, IO: null })).toBeNull()
  })

  it('watches the map shell when the page has one, with threshold 0', () => {
    const { shell } = page()
    const e = env()
    const r = createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })
    expect(e.observe).toHaveBeenCalledWith(shell)
    expect(r.observer.opts.threshold).toBe(0)
  })

  it('falls back to the sentinel on a page with no map', () => {
    const { sentinel } = page({ withMap: false })
    const e = env()
    createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })
    expect(e.observe).toHaveBeenCalledWith(sentinel)
  })

  it('is hidden at first, shown once the map has left through the top', () => {
    const { btn } = page()
    const e = env()
    createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })
    expect(btn.classList.contains(VISIBLE)).toBe(false)
    e.fire(false, -10)
    expect(btn.classList.contains(VISIBLE)).toBe(true)
  })

  it('observes only the lower half of the viewport', () => {
    page()
    const e = env()
    const r = createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })
    expect(r.observer.opts.rootMargin).toBe('-50% 0px 0px 0px')
  })

  it('shows when the map bottom has risen above mid-viewport but is still on screen', () => {
    const { btn } = page()
    const e = env()
    createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })
    e.fire(false, 287)
    expect(btn.classList.contains(VISIBLE)).toBe(true)
  })

  it('stays hidden while the map bottom is still in the lower half', () => {
    const { btn } = page()
    const e = env()
    createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })
    e.fire(true, 600)
    expect(btn.classList.contains(VISIBLE)).toBe(false)
  })

  it('hides again when the map comes back', () => {
    const { btn } = page()
    const e = env()
    createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })
    e.fire(false, -10)
    e.fire(true, 300)
    expect(btn.classList.contains(VISIBLE)).toBe(false)
  })

  it('stays hidden when the target is off-screen below, not scrolled past', () => {
    const { btn } = page()
    const e = env()
    createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })
    e.fire(false, 2000)
    expect(btn.classList.contains(VISIBLE)).toBe(false)
  })

  it('smooth-scrolls to the top and focuses the heading without scrolling', () => {
    const { btn, h1 } = page()
    h1.focus = vi.fn()
    const e = env()
    createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })
    btn.click()
    expect(e.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' })
    expect(h1.focus).toHaveBeenCalledWith({ preventScroll: true })
    expect(h1.getAttribute('tabindex')).toBe('-1')
  })

  it('scrolls instantly under prefers-reduced-motion', () => {
    const { btn } = page()
    const e = env({ reduce: true })
    createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })
    btn.click()
    expect(e.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'instant' })
  })

  it('keeps a tabindex the heading already has', () => {
    const { btn, h1 } = page()
    h1.setAttribute('tabindex', '0')
    const e = env()
    createBackToTop({ doc: e.doc, win: e.win, IO: e.IO })
    btn.click()
    expect(h1.getAttribute('tabindex')).toBe('0')
  })
})
