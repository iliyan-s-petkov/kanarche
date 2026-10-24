// Phones and tablets (below the dock's 1024px): brings the sensor card under the map into view on open and returns to the map on close.
// `panel` may be a getter, because the section only exists while a sensor is open.
import { sectionHidden } from './panelhost.js'

export function createPanelScroll({ win = window, panel, isFull = () => false }) {
  let returnY = null
  const mq = (q) => win.matchMedia?.(q).matches ?? false
  const behavior = () => (mq('(prefers-reduced-motion: reduce)') ? 'auto' : 'smooth')
  const phone = () => mq('(hover: none), (max-width: 1023px)')

  // A map half scrolled off leaves the dock's header under the sticky masthead, out of reach.
  function revealDock(el) {
    const doc = el.ownerDocument
    const dock = doc?.querySelector('.map-dock')?.getBoundingClientRect()
    if (!dock) return
    const clear = doc.querySelector('.masthead')?.getBoundingClientRect().bottom ?? 0
    if (dock.top >= clear) return
    if (returnY === null) returnY = win.scrollY
    win.scrollBy({ top: dock.top - clear - 8, behavior: behavior() })
  }

  return {
    opened({ initial }) {
      const el = typeof panel === 'function' ? panel() : panel
      if (initial || !el || isFull()) return
      // From 1024px the sensor shows in the map's dock and this section is hidden.
      if (sectionHidden(win, el)) {
        revealDock(el)
        return
      }
      if (!phone()) return
      // Already in the upper part of the screen: a second tap must not jerk the page.
      const top = el.getBoundingClientRect().top
      if (top >= 0 && top < win.innerHeight * 0.6) return
      if (returnY === null) returnY = win.scrollY
      el.scrollIntoView({ behavior: behavior(), block: 'start' })
      el.focus?.({ preventScroll: true })
    },
    // True when it scrolled back, so the caller can hand focus to the map.
    closed() {
      if (returnY === null) return false
      const top = returnY
      returnY = null
      if (isFull()) return false
      win.scrollTo({ top, behavior: behavior() })
      return true
    },
  }
}
