// The open sensor as a panel along the bottom of the map on wide screens: title, gauges, a chart.
// components/MapDockCard.svelte draws them from the card model the panel island publishes (lib/sensorcard.js);
// this file owns the frame around them: mounting, the grip and height, fold, the history and area buttons.
// The chart is drawn a second time while the panel is open (SensorChart place="dock"); getJSON serves it from cache.
import { tick, mount as mountComponent, unmount as unmountComponent, flushSync } from 'svelte'
import MapDockCard from '../components/MapDockCard.svelte'
import { panelDock } from './paneldock.svelte.js'
import { getSensorCard } from './sensorcard.js'
import { sectionHidden } from './panelhost.js'
import { readFlag, writeFlag } from './storage.js'
import { areaLine } from './arealine.js'
import { getSensors, getSensorArea } from './sensors.svelte.js'
import { getMapAreas } from './mapareas.svelte.js'
import { stackReserve, heightBounds, clampHeight, keyHeight, readHeight, writeHeight, clearHeight } from './dockheight.js'

const WIDE = '(min-width: 1024px)'
// Set on <html> only while the dock is mounted. app.css hides the card under the map under it,
// so a dock that fails to mount leaves the card visible.
const DOCKED_ROOT = 'has-map-dock'
const TITLE_ID = 'map-dock-title'
const FOLD_KEY = 'kanarche:panel-folded'
const SVG_NS = 'http://www.w3.org/2000/svg'
// The right-hand control rail; when it does not fit above the panel, the panel clears the rail.
const COLUMN = '.map-rail'
const GAP = 8
// The top row, and the bottom bar that rides up with the panel; the open legend stands above the bar.
const TOP_ROW = '.map__layers, .map-controls'
const RIDERS = '.map-bar, .map-bar > .scale--onmap'

function icon(doc, cls, d) {
  const svg = doc.createElementNS(SVG_NS, 'svg')
  svg.setAttribute('class', cls)
  svg.setAttribute('viewBox', '0 0 16 16')
  svg.setAttribute('aria-hidden', 'true')
  const path = doc.createElementNS(SVG_NS, 'path')
  path.setAttribute('d', d)
  path.setAttribute('fill', 'none')
  path.setAttribute('stroke', 'currentColor')
  path.setAttribute('stroke-width', '1.5')
  path.setAttribute('stroke-linecap', 'round')
  path.setAttribute('stroke-linejoin', 'round')
  svg.appendChild(path)
  return svg
}

export function createSideDock(frame, { closeLabel = '', moreLabel = '', moreShortLabel = '', areaBelow = '', areaBelowUnnamed = '', foldLabel = '', expandLabel = '', resizeLabel = '' } = {}) {
  const doc = frame.ownerDocument
  const win = doc.defaultView
  const shell = frame.closest('.map-shell')
  // The embed page never docks: its card is not under the map either.
  const enabled = !doc.body.classList.contains('embed')
  const mq = win?.matchMedia?.(WIDE) ?? null
  let full = false
  let content = null
  let contentCard = null
  let returnTo = null
  let onClose = () => {}
  // A new map frame starts with nothing lent: a dock torn down with its page never said it had gone.
  panelDock.on = false

  const el = doc.createElement('div')
  el.className = 'map-dock'
  el.setAttribute('role', 'group')
  el.setAttribute('aria-labelledby', TITLE_ID)
  el.tabIndex = -1

  // The top edge's resize handle: a separator whose value is the panel's height in pixels.
  const grip = doc.createElement('div')
  grip.className = 'map-dock__grip'
  grip.setAttribute('role', 'separator')
  grip.setAttribute('aria-orientation', 'horizontal')
  grip.setAttribute('aria-label', resizeLabel)
  grip.title = resizeLabel
  grip.tabIndex = 0
  el.appendChild(grip)

  // MapDockCard mounts here, after the grip and before the history and area buttons.
  // A button, not an <a href="#…">: a fragment link would rewrite the hash the view state lives in.
  const more = doc.createElement('button')
  more.type = 'button'
  more.className = 'map-dock__more'
  more.appendChild(icon(doc, 'map-dock__more-ico', 'M8 3v9M4 8.5l4 4 4-4'))
  const moreText = doc.createElement('span')
  more.appendChild(moreText)
  if (moreLabel) el.appendChild(more)

  // The footer row: a pointer to the area figures under the map, shown while the readouts have an area row.
  const area = doc.createElement('button')
  area.type = 'button'
  area.className = 'map-dock__area'
  area.hidden = true
  area.appendChild(icon(doc, 'map-dock__area-ico', 'M8 3v9M4 8.5l4 4 4-4'))
  const areaText = doc.createElement('span')
  area.appendChild(areaText)
  el.appendChild(area)
  // The island only skips its sensor row on the area page, where the card says it all.
  const figures = () => doc.querySelector('[data-island="readouts"]:not([data-sensor-row="off"])')

  function paintArea(vs) {
    const lang = doc.documentElement.getAttribute('lang') || 'bg'
    const text = figures() ? areaLine({
      body: getSensors(), metric: vs.metric, sensorId: vs.sensorId, areas: getMapAreas(), slug: getSensorArea(), lang,
      t: { areaBelow, areaBelowUnnamed },
    }) : ''
    areaText.textContent = text
    area.hidden = text === ''
    el.classList.toggle('map-dock--area', text !== '')
    if (mounted()) applyHeight()
  }

  // The fold button (in MapDockCard) is named from ui.folded; the rest of the fold is the frame's.
  const ui = $state({ folded: readFlag(FOLD_KEY, false) })
  function paintFold() {
    el.classList.toggle('map-dock--folded', ui.folded)
    grip.hidden = ui.folded
    moreText.textContent = ui.folded && moreShortLabel ? moreShortLabel : moreLabel
  }
  paintFold()

  const panelHost = () => doc.querySelector('[data-island="panel"]')
  const panel = () => panelHost()?.querySelector('.sensor-panel') ?? null
  const mounted = () => el.parentNode === frame

  // The saved height is the reader's choice; what shows is that clamped to the map as it is now.
  let chosen = readHeight()
  const rem = () => Number.parseFloat(win?.getComputedStyle?.(doc.documentElement).fontSize ?? '') || 16
  // The riders' height above the panel is measured as laid out now, less the panel height already applied to them.
  function reserve() {
    const host = shell ?? frame
    const f = frame.getBoundingClientRect()
    const shown = (sel) => [...host.querySelectorAll(sel)].filter((n) => n.getClientRects().length > 0).map((n) => n.getBoundingClientRect())
    const applied = Number.parseFloat(shell?.style.getPropertyValue('--map-panel-h') ?? '') || 0
    const rise = Math.max(0, ...shown(RIDERS).map((r) => f.bottom - r.top - applied))
    const topBottom = Math.max(0, ...shown(TOP_ROW).map((r) => r.bottom - f.top))
    return stackReserve({ rise, topBottom, gap: GAP })
  }
  // The area line takes 2rem of the panel, so the floor rises by as much while it shows.
  const bounds = () => {
    const b = heightBounds(frame.clientHeight, rem(), mounted() ? reserve() : 0)
    const extra = area.hidden ? 0 : Math.round(2 * rem())
    return { min: b.min + extra, max: Math.max(b.min + extra, b.max) }
  }
  function applyHeight() {
    const b = bounds()
    if (chosen === null) el.style.removeProperty('--map-dock-h')
    else el.style.setProperty('--map-dock-h', `${clampHeight(chosen, b)}px`)
    grip.setAttribute('aria-valuemin', String(b.min))
    grip.setAttribute('aria-valuemax', String(b.max))
    grip.setAttribute('aria-valuenow', String(chosen === null ? clampHeight(el.offsetHeight, b) : clampHeight(chosen, b)))
  }
  applyHeight()

  // How much of the map's bottom the panel covers, inset included: the controls and the camera clear this much.
  const listeners = new Set()
  let lastHeight = -1
  function measure() {
    // The legend folding or the map resizing moves the cap; the saved choice is only clamped, never rewritten.
    if (mounted()) applyHeight()
    const h = mounted() ? Math.max(0, Math.round(frame.getBoundingClientRect().bottom - el.getBoundingClientRect().top)) : 0
    const room = mounted() ? railRoom() : Infinity
    // Crowded: the panel itself reaches the rail. Tight: only the bar and attribution above it do.
    shell?.classList.toggle('map-shell--crowded', room < 0)
    shell?.classList.toggle('map-shell--tight', room < above())
    if (h !== lastHeight) {
      if (h > 0) shell?.style.setProperty('--map-panel-h', `${h}px`)
      else shell?.style.removeProperty('--map-panel-h')
      lastHeight = h
    }
    listeners.forEach((fn) => fn(h))
  }
  // Space between the rail's foot (plus two gaps) and the panel's top; negative when they meet.
  function railRoom() {
    const top = frame.getBoundingClientRect().top
    const lowest = Math.max(0, ...[...(shell ?? frame).querySelectorAll(COLUMN)].map((n) => n.getBoundingClientRect().bottom - top))
    return el.getBoundingClientRect().top - top - (lowest + 2 * GAP)
  }
  // The bar rides GAP above the panel and the attribution stands on the bar.
  function above() {
    const h = (sel) => (shell ?? frame).querySelector(sel)?.getBoundingClientRect().height ?? 0
    return h('.map-bar') + h('.maplibregl-ctrl-bottom-right') + GAP
  }
  const watcher = typeof win?.ResizeObserver === 'function' ? new win.ResizeObserver(measure) : null

  function dropContent() {
    if (content) unmountComponent(content)
    content = null
    contentCard = null
  }

  function unmount() {
    if (!mounted()) return
    dropContent()
    panelDock.on = false
    watcher?.disconnect()
    el.remove()
    shell?.classList.remove('map-shell--docked')
    doc.documentElement.classList.remove(DOCKED_ROOT)
    measure()
    const back = returnTo
    returnTo = null
    // An opener that re-rendered away (the nearby list does) or never held focus falls back to the map canvas.
    const usable = back?.isConnected && back !== doc.body && typeof back.focus === 'function'
    ;(usable ? back : frame.querySelector('canvas'))?.focus({ preventScroll: true })
  }

  // Idempotent: reads the card as it is now and mounts, updates or unmounts. Plain reads, no effect:
  // the panel's MutationObserver below calls this too.
  function sync() {
    const host = panelHost()
    const card = host ? getSensorCard() : null
    if (!enabled || full || !mq?.matches || !card?.open) {
      unmount()
      return
    }
    // No way down to a section that is not shown.
    more.hidden = sectionHidden(win, host)
    if (card !== contentCard) {
      dropContent()
      contentCard = card
      content = mountComponent(MapDockCard, {
        target: el,
        anchor: more.parentNode === el ? more : area,
        props: { card, titleId: TITLE_ID, closeLabel, foldLabel, expandLabel, ui, onfold: toggleFold, onclose: dismiss },
      })
    }
    if (!mounted()) {
      returnTo = doc.activeElement
      frame.appendChild(el)
      shell?.classList.add('map-shell--docked')
      doc.documentElement.classList.add(DOCKED_ROOT)
      watcher?.observe(el)
      ;[...(shell ?? frame).querySelectorAll(RIDERS + ', ' + TOP_ROW)].forEach((n) => watcher?.observe(n))
      applyHeight()
      panelDock.on = true
      el.focus({ preventScroll: true })
    }
    measure()
  }

  function dismiss() {
    unmount()
    onClose()
  }

  function toggleFold() {
    ui.folded = !ui.folded
    writeFlag(FOLD_KEY, ui.folded)
    paintFold()
    // The button's new name and the folded layout are read back at once (measure, and the reader's next key).
    flushSync()
    measure()
  }
  // Pointer events cover mouse, touch and pen; the capture keeps the drag when the pointer leaves the handle.
  let drag = null
  let queued = false
  grip.addEventListener('pointerdown', (e) => {
    if (e.button !== 0) return
    e.preventDefault()
    drag = { y: e.clientY, h: el.getBoundingClientRect().height, to: null }
    try { grip.setPointerCapture?.(e.pointerId) } catch { /* a synthetic pointer has nothing to capture */ }
    el.classList.add('map-dock--resizing')
  })
  grip.addEventListener('pointermove', (e) => {
    if (!drag) return
    drag.to = drag.h + drag.y - e.clientY
    if (queued) return
    queued = true
    // One height per frame; the ResizeObserver then republishes --map-panel-h and the map's padding.
    ;(win?.requestAnimationFrame ?? ((fn) => setTimeout(fn, 16)))(() => {
      queued = false
      if (drag?.to == null) return
      chosen = clampHeight(drag.to, bounds())
      applyHeight()
    })
  })
  function endDrag() {
    if (!drag) return
    if (drag.to != null) {
      chosen = clampHeight(drag.to, bounds())
      applyHeight()
      writeHeight(chosen)
    }
    drag = null
    el.classList.remove('map-dock--resizing')
  }
  grip.addEventListener('pointerup', endDrag)
  grip.addEventListener('pointercancel', endDrag)
  grip.addEventListener('lostpointercapture', endDrag)
  grip.addEventListener('dblclick', () => {
    chosen = null
    clearHeight()
    applyHeight()
  })
  grip.addEventListener('keydown', (e) => {
    const b = bounds()
    const next = keyHeight(e.key, chosen === null ? clampHeight(el.offsetHeight, b) : clampHeight(chosen, b), b)
    if (next === null) return
    e.preventDefault()
    chosen = next
    applyHeight()
    writeHeight(chosen)
  })
  win?.addEventListener?.('resize', () => { if (mounted()) applyHeight() })

  // Scrolls like the cue under the map; the figures take focus so the next Tab reads them.
  area.addEventListener('click', () => {
    const target = figures()
    if (!target) return
    const reduce = win?.matchMedia?.('(prefers-reduced-motion: reduce)').matches
    target.tabIndex = -1
    target.scrollIntoView({ block: 'start', behavior: reduce ? 'auto' : 'smooth' })
    target.focus({ preventScroll: true })
  })
  more.addEventListener('click', () => panel()?.scrollIntoView({ block: 'start', behavior: 'smooth' }))
  mq?.addEventListener?.('change', sync)

  // The panel's own DOM also drives sync, so the dock does not depend on the shared Svelte
  // effect flush: a throw in another island aborts that flush. One sync per batch of changes.
  // sync is idempotent, so the mutations it makes itself end in a no-op pass.
  const watchedHost = panelHost()
  let syncQueued = false
  const panelWatch = watchedHost && typeof win?.MutationObserver === 'function'
    ? new win.MutationObserver(() => {
      // A dock whose map has left the page must not reach into the next page's panel.
      if (syncQueued || !frame.isConnected) return
      syncQueued = true
      queueMicrotask(() => { syncQueued = false; sync() })
    })
    : null
  panelWatch?.observe(watchedHost, { childList: true, subtree: true })

  // Capture phase, like the sheet, so a faux-fullscreen Escape handler never sees a dock Escape.
  doc.addEventListener('keydown', (e) => {
    // The station sheet opened from the info button owns its own Escape.
    if (e.key !== 'Escape' || !mounted() || !el.isConnected || doc.querySelector('.about-sheet')) return
    // An open menu in the panel closes first; its own handler takes this Escape.
    if (el.querySelector('.colmenu__panel:not([hidden]), .panel-menu')) return
    e.stopPropagation()
    e.preventDefault()
    dismiss()
  }, true)

  return {
    el,
    sync,
    measure,
    // fn(height) on every size change and on close (0); returns the unsubscribe.
    onLayout(fn) {
      listeners.add(fn)
      return () => listeners.delete(fn)
    },
    setFull(on) {
      full = on
      sync()
    },
    // Re-syncs after the panel has rendered the sensor the view state names.
    follow(vs, findSensor) {
      onClose = () => vs.closeSensor()
      return $effect.root(() => {
        $effect(() => {
          findSensor(vs.sensorId)
          tick().then(sync)
        })
        $effect(() => paintArea(vs))
      })
    },
  }
}
