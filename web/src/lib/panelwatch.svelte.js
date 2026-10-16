// Drives lib/panelscroll.js from the shown sensor id; a rune module because islands/panel.js is plain .js.
import { tick, untrack } from 'svelte'

// shownId: the open sensor once the registry can render it, else null.
// initialId: the #sensor= the page loaded with; its first showing is not a user tap.
// onReturn(opener): the panel closed; opener had focus when it opened.
export function watchPanelScroll({ shownId, initialId, scroll, onReturn = () => {} }) {
  let prev = null
  let first = initialId != null
  // Focus when a sensor opened from nothing, handed back on close so a list or table row gets it again.
  let opener = null
  return $effect.root(() => {
    $effect(() => {
      const id = shownId()
      untrack(() => {
        if (id === prev) return
        const was = prev
        prev = id
        if (id === null) {
          const back = opener
          opener = null
          if (scroll.closed()) onReturn(back)
          return
        }
        if (was === null) opener = globalThis.document?.activeElement ?? null
        const initial = first && was === null && id === initialId
        first = false
        // After the panel's own render, so the section exists and has its final position.
        tick().then(() => { if (prev === id) scroll.opened({ initial }) })
      })
    })
  })
}

// Back to the opener if it is still on the page and takes focus, else to the map canvas.
export function returnFocus(opener, doc = globalThis.document) {
  const ok = opener?.isConnected && opener !== doc.body && typeof opener.focus === 'function'
  if (ok) {
    opener.focus({ preventScroll: true })
    if (doc.activeElement === opener) return
  }
  doc.querySelector('.maplibregl-canvas')?.focus({ preventScroll: true })
}
