// The readout for one hex cell: a small text card at the pointer or the focused list item.
// The card is for sight (aria-hidden). Speech comes from a separate polite live region
// that only announce() writes, so a mouse sweeping the map never talks.

// Gap between the point and the card, matching .hex-tip in app.css.
const GAP = 12

export function createHexTip(container, doc = document) {
  const tip = doc.createElement('div')
  tip.className = 'hex-tip'
  tip.setAttribute('aria-hidden', 'true')
  tip.hidden = true
  const live = doc.createElement('div')
  live.className = 'visually-hidden'
  live.setAttribute('role', 'status')
  live.setAttribute('aria-live', 'polite')
  container.append(tip, live)

  return {
    // point: {x, y} in the container's own pixels.
    show(text, point) {
      tip.textContent = text
      tip.hidden = false
      const w = tip.offsetWidth
      const half = w / 2
      const x = Math.min(Math.max(point.x, half), Math.max(half, container.clientWidth - half))
      // Below the point when there is no room above it.
      tip.classList.toggle('hex-tip--below', point.y < tip.offsetHeight + GAP)
      tip.style.left = `${x}px`
      tip.style.top = `${point.y}px`
    },
    hide() {
      tip.hidden = true
      live.textContent = ''
    },
    announce(text) {
      live.textContent = text
    },
    destroy() {
      tip.remove()
      live.remove()
    },
  }
}
