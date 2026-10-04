// The open sensor as a panel along the bottom of the map on wide screens: title, gauges, a chart.
// The panel's own .gauges node is moved in and put back, the same move-not-duplicate idiom as sensorsheet.
// The chart is drawn a second time while the panel is open (see SensorChart); getJSON serves it from cache.
import { tick } from 'svelte'
import { createStarSlot } from './panelstar.js'
import { panelDock } from './paneldock.svelte.js'
import { sectionHidden } from './panelhost.js'
import { readFlag, writeFlag } from './storage.js'
import { areaLine } from './arealine.js'
import { getSensors, getSensorArea } from './sensors.svelte.js'
import { getMapAreas } from './mapareas.svelte.js'
import { stackReserve, heightBounds, clampHeight, keyHeight, readHeight, writeHeight, clearHeight } from './dockheight.js'

const WIDE = '(min-width: 1024px)'
const TITLE_ID = 'map-dock-title'
const FOLD_KEY = 'kanarche:panel-folded'
const SVG_NS = 'http://www.w3.org/2000/svg'
// The right-hand control column; when it and the locate button do not fit above the panel, the panel clears the column.
const COLUMN = '.map__full, .map-zoom, .map-orient'
const GAP = 8
// The left stack: the top row, and the controls that ride up with the panel (legend, freshness card).
const TOP_ROW = '.map__layers, .map-controls, .map-note'
const RIDERS = '.scale--onmap, .map-freshness'

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
  let gauges = null
  let marker = null
  let returnTo = null
  let onClose = () => {}
  const starSlot = createStarSlot(doc)
  const infoSlot = createStarSlot(doc, '.panel-info')

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

  const head = doc.createElement('div')
  head.className = 'map-dock__head'
  const title = doc.createElement('h2')
  title.id = TITLE_ID
  title.className = 'map-dock__title'
  const close = doc.createElement('button')
  close.type = 'button'
  close.className = 'map-dock__close'
  close.setAttribute('aria-label', closeLabel)
  close.title = closeLabel
  close.appendChild(icon(doc, 'map-dock__close-ico', 'M3.5 3.5l9 9M12.5 3.5l-9 9'))

  // Named for what it does next, like the fullscreen button; the chevron points the way it will move.
  const fold = doc.createElement('button')
  fold.type = 'button'
  fold.className = 'map-dock__fold'
  fold.appendChild(icon(doc, 'map-dock__fold-ico', 'M3.5 6l4.5 4.5L12.5 6'))
  head.append(title, fold, close)
  el.appendChild(head)

  const body = doc.createElement('div')
  body.className = 'map-dock__body'
  el.appendChild(body)

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

  let folded = readFlag(FOLD_KEY, false)
  function paintFold() {
    el.classList.toggle('map-dock--folded', folded)
    grip.hidden = folded
    const label = folded ? expandLabel : foldLabel
    fold.setAttribute('aria-label', label)
    fold.title = label
    moreText.textContent = folded && moreShortLabel ? moreShortLabel : moreLabel
  }
  paintFold()

  const panel = () => doc.querySelector('[data-island="panel"] .sensor-panel')
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

  function restoreGauges() {
    if (marker && gauges) marker.replaceWith(gauges)
    marker = null
    gauges = null
  }

  // How much of the map's bottom the panel covers, inset included: the controls and the camera clear this much.
  const listeners = new Set()
  let lastHeight = -1
  function measure() {
    // The legend folding or the map resizing moves the cap; the saved choice is only clamped, never rewritten.
    if (mounted()) applyHeight()
    const h = mounted() ? Math.max(0, Math.round(frame.getBoundingClientRect().bottom - el.getBoundingClientRect().top)) : 0
    shell?.classList.toggle('map-shell--crowded', mounted() && crowded())
    if (h !== lastHeight) {
      if (h > 0) shell?.style.setProperty('--map-panel-h', `${h}px`)
      else shell?.style.removeProperty('--map-panel-h')
      lastHeight = h
    }
    listeners.forEach((fn) => fn(h))
  }
  function crowded() {
    const top = frame.getBoundingClientRect().top
    const lowest = Math.max(0, ...[...(shell ?? frame).querySelectorAll(COLUMN)].map((n) => n.getBoundingClientRect().bottom - top))
    const locate = (shell ?? frame).querySelector('.map-locate')?.offsetHeight ?? 0
    return el.getBoundingClientRect().top - top < lowest + locate + 2 * GAP
  }
  const watcher = typeof win?.ResizeObserver === 'function' ? new win.ResizeObserver(measure) : null

  // The chart's panel copy is rendered by SensorChart under the map, then moved here. Svelte removes it
  // from anywhere when only its own flag goes, but not when a whole chart is torn down, so stale ones are dropped here.
  const chartNodes = () => [...el.children].filter((n) => n.classList.contains('panel-chart__dock'))
  function adoptChart() {
    if (!mounted()) return
    const node = panel()?.querySelector('.panel-chart__dock')
    if (node && node.parentNode !== el) el.insertBefore(node, more.parentNode === el ? more : null)
    if (node) chartNodes().filter((n) => n !== node).forEach((n) => n.remove())
    measure()
  }

  function unmount() {
    if (!mounted()) return
    restoreGauges()
    starSlot.release()
    infoSlot.release()
    panelDock.on = false
    chartNodes().forEach((n) => n.remove())
    watcher?.disconnect()
    el.remove()
    shell?.classList.remove('map-shell--docked')
    measure()
    const back = returnTo
    returnTo = null
    if (back?.isConnected && back !== doc.body && typeof back.focus === 'function') back.focus({ preventScroll: true })
  }

  // Idempotent: reads the panel as it is now and mounts, updates or unmounts.
  function sync() {
    const p = panel()
    // Gauges already moved out of this panel are its own, not a panel without them.
    const own = gauges && body.contains(gauges) && p?.contains(marker) ? gauges : null
    const next = p?.querySelector('.gauges') ?? own
    if (!enabled || full || !mq?.matches || !p || !next) {
      unmount()
      return
    }
    title.textContent = p.querySelector('h2')?.textContent ?? ''
    // No way down to a section that is not shown.
    more.hidden = sectionHidden(win, p)
    if (next !== gauges) {
      restoreGauges()
      marker = doc.createComment('gauges')
      next.before(marker)
      gauges = next
      body.appendChild(next)
    }
    starSlot.take(p, head, fold)
    infoSlot.take(p, head, fold)
    if (!mounted()) {
      returnTo = doc.activeElement
      frame.appendChild(el)
      shell?.classList.add('map-shell--docked')
      watcher?.observe(el)
      ;[...(shell ?? frame).querySelectorAll(RIDERS + ', ' + TOP_ROW)].forEach((n) => watcher?.observe(n))
      applyHeight()
      panelDock.on = true
      el.focus({ preventScroll: true })
    }
    adoptChart()
    // The chart node exists only after Svelte has rendered the flag set above.
    tick().then(adoptChart)
  }

  function dismiss() {
    unmount()
    onClose()
  }

  close.addEventListener('click', dismiss)
  fold.addEventListener('click', () => {
    folded = !folded
    writeFlag(FOLD_KEY, folded)
    paintFold()
    measure()
  })
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
