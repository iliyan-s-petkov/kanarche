// The open sensor's readings as a sheet inside the fullscreen frame, where the
// panel under the map cannot be seen. components/SensorSheetCard.svelte draws the
// title, star and gauges from the card model the panel island publishes
// (lib/sensorcard.js); the card under the map drops its own while the sheet is up.
import { tick, mount as mountComponent, unmount as unmountComponent, flushSync } from 'svelte'
import SensorSheetCard from '../components/SensorSheetCard.svelte'
import { panelDock } from './paneldock.svelte.js'
import { getSensorCard } from './sensorcard.js'
import { sectionHidden } from './panelhost.js'

const TITLE_ID = 'map-sensor-sheet-title'

export function createSensorSheet(frame, { closeLabel = '', historyLabel = '', exitFull = () => {} } = {}) {
  const doc = frame.ownerDocument
  let full = false
  let content = null
  let contentCard = null
  let returnTo = null
  let scrollOnExit = false
  let onClose = () => {}
  // A new map frame starts with nothing lent: a sheet torn down with its page never said it had gone.
  panelDock.sheet = false

  const el = doc.createElement('div')
  el.className = 'map-sensor-sheet'
  el.setAttribute('role', 'dialog')
  el.setAttribute('aria-labelledby', TITLE_ID)
  el.tabIndex = -1

  // SensorSheetCard mounts here, before the history button.

  // A button, not an <a href="#…">: a fragment link would rewrite the hash the view state lives in.
  let history = null
  if (historyLabel) {
    history = doc.createElement('button')
    history.type = 'button'
    history.className = 'map-sensor-sheet__history'
    history.textContent = historyLabel
    el.appendChild(history)
  }

  const panelHost = () => doc.querySelector('[data-island="panel"]')
  const panel = () => panelHost()?.querySelector('.sensor-panel') ?? null

  const mounted = () => el.parentNode === frame

  function dropContent() {
    if (content) unmountComponent(content)
    content = null
    contentCard = null
  }

  function unmount({ focus = true } = {}) {
    if (!mounted()) return
    dropContent()
    panelDock.sheet = false
    // The card under the map has its gauges back before the caller scrolls to it or reads it.
    flushSync()
    el.remove()
    const back = returnTo
    returnTo = null
    if (focus && back?.isConnected && typeof back.focus === 'function') back.focus({ preventScroll: true })
  }

  // Idempotent: reads the card as it is now and mounts, updates or unmounts.
  function sync() {
    const host = panelHost()
    const card = host ? getSensorCard() : null
    if (!full || !card?.open || frame.contains(host)) {
      unmount()
      return
    }
    // No link down to a section the wide home page hides.
    if (history) history.hidden = sectionHidden(doc.defaultView, host)
    if (card !== contentCard) {
      dropContent()
      contentCard = card
      content = mountComponent(SensorSheetCard, {
        target: el,
        anchor: history ?? undefined,
        props: { card, titleId: TITLE_ID, closeLabel, onclose: dismiss },
      })
    }
    if (!mounted()) {
      returnTo = doc.activeElement
      frame.appendChild(el)
      panelDock.sheet = true
      el.focus({ preventScroll: true })
    }
  }

  function dismiss() {
    unmount()
    onClose()
  }

  history?.addEventListener('click', () => {
    scrollOnExit = true
    exitFull()
  })

  // Capture phase, so it runs before mountFullscreen's faux-full Escape handler on the document.
  doc.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || !mounted()) return
    e.stopPropagation()
    e.preventDefault()
    dismiss()
  }, true)

  return {
    el,
    sync,
    setFull(on) {
      full = on
      sync()
      if (on || !scrollOnExit) return
      scrollOnExit = false
      const p = panel()
      if (!p || sectionHidden(doc.defaultView, p)) return
      // After the frame has left fullscreen and the page has its layout back.
      const go = () => {
        p.scrollIntoView({ block: 'start' })
        p.focus?.({ preventScroll: true })
      }
      if (typeof requestAnimationFrame === 'function') requestAnimationFrame(go)
      else go()
    },
    // Re-syncs after the panel has rendered the sensor the view state names.
    follow(vs, findSensor) {
      onClose = () => vs.closeSensor()
      return $effect.root(() => {
        $effect(() => {
          findSensor(vs.sensorId)
          tick().then(sync)
        })
      })
    },
  }
}
