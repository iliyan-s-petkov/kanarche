// The readout for one hex cell: a small text card at the pointer or the focused list item.
// Sight only (aria-hidden): a focused list button already speaks its own text, so a live
// region here would make screen readers say every reading twice.

// Gap between the point and the card, matching .hex-tip in app.css.
const GAP = 12

export function createHexTip(container, doc = document) {
  const tip = doc.createElement('div')
  tip.className = 'hex-tip'
  tip.setAttribute('aria-hidden', 'true')
  tip.hidden = true
  container.append(tip)

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
    },
    destroy() {
      tip.remove()
    },
  }
}
