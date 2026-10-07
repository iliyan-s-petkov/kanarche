// Island loader: one entry module, one pass over the document.
//
// A registry of DYNAMIC imports rather than static ones, so Rollup splits each
// island into its own chunk and the index page downloads the map chunk and never
// the chart chunk. A `for` loop over [data-island] rather than per-page entry
// points, because the server stays ignorant of which bundles exist — it only
// emits the attributes.
//
// Static, unlike the islands below: the masthead is on every page and this is
// a few lines, so a chunk boundary would cost a request to save nothing.
import { soleOpen } from './lib/disclosure.js'
import { scrollCue } from './lib/scrollcue.js'
import { createBackToTop } from './lib/backtotop.js'
import { localizeTimes } from './lib/localtime.js'
import { afterIdle } from './lib/idle.js'

const ISLANDS = {
  map: () => import('./islands/map.js'),
  chart: () => import('./islands/chart.js'),
  switcher: () => import('./islands/switcher.js'),
  finder: () => import('./islands/finder.js'),
  table: () => import('./islands/table.js'),
  refresh: () => import('./islands/refresh.js'),
  freshness: () => import('./islands/freshness.js'),
  panel: () => import('./islands/panel.js'),
  readouts: () => import('./islands/readouts.svelte.js'),
  sensorbar: () => import('./islands/sensorbar.js'),
  theme: () => import('./islands/theme.js'),
  clearsettings: () => import('./islands/clearsettings.js'),
  copycode: () => import('./islands/copycode.js'),
  visitors: () => import('./islands/visitors.js'),
}

// resolveLoader is the pure lookup at the heart of "leave the server-rendered
// fallback for anything this bundle does not know about". Exported so a test
// can pin the decision (known name -> a function, unknown name -> null)
// without touching the DOM.
export function resolveLoader(islandName) {
  return ISLANDS[islandName] ?? null
}

// runIsland mounts one island and swallows any failure so it cannot break the
// loop over the other islands, or leave an unhandled rejection on the page. It
// is a plain async function — not a `.then`/`.catch` chain built inline in the
// loop — precisely so it can be driven from a test with a plain {dataset}
// object and stub `load`/`mount` functions, with no real DOM element and no
// real MapLibre/Svelte bundle involved.
//
// Logged, not silent: every island's container sits BESIDE server-rendered
// content, never replacing it, so a broken bundle degrades to the
// server-rendered page instead of a blank div — but a developer still needs to
// see the failure in the console.
export async function runIsland(el, load, log = console.error) {
  try {
    const mod = await load()
    mod.mount(el)
    return true
  } catch (err) {
    log('island failed:', el.dataset.island, err)
    return false
  }
}

// Upper bound on waiting for the FCP entry; a hidden tab never paints, and must still mount.
export const FCP_WAIT_MAX_MS = 2000

// Runs callback on a fresh task once first-contentful-paint is recorded, so the map chunk request starts after FCP rather than racing it.
export function whenFirstPaintReported(
  callback,
  perf = globalThis.performance,
  Observer = globalThis.PerformanceObserver,
  timer = globalThis.setTimeout,
  clear = globalThis.clearTimeout,
) {
  if (perf?.getEntriesByName?.('first-contentful-paint').length) {
    timer(callback, 0)
    return
  }
  if (!Observer?.supportedEntryTypes?.includes('paint')) {
    callback()
    return
  }
  let done = false
  let obs
  let fallback
  const run = () => {
    if (done) return
    done = true
    obs?.disconnect()
    clear(fallback)
    timer(callback, 0)
  }
  obs = new Observer((list) => {
    if (list.getEntriesByName('first-contentful-paint').length) run()
  })
  obs.observe({ type: 'paint', buffered: true })
  fallback = timer(run, FCP_WAIT_MAX_MS)
}

// Two rAFs: a single rAF callback runs before its frame paints; then the FCP entry is awaited.
export function scheduleAfterFirstPaint(callback, raf = globalThis.requestAnimationFrame) {
  if (typeof raf === 'function') {
    raf(() => raf(() => whenFirstPaintReported(callback)))
  } else {
    setTimeout(callback, 0)
  }
}

// Islands below the map wait for the map to mount and the main thread to idle.
// The sensor card host is not here: the side dock looks the card up when a sensor opens, so a click before it mounted showed nothing.
const DEFERRED = new Set(['readouts', 'table', 'visitors', 'copycode', 'clearsettings'])
// A sensor in the URL means the readouts are wanted on first paint, not after idle.
const SENSOR_NOW = new Set(['readouts'])

export function deferIsland(name, hash) {
  if (!DEFERRED.has(name)) return false
  if (SENSOR_NOW.has(name) && new URLSearchParams(String(hash).replace(/^#/, '')).has('sensor')) return false
  return true
}

// Deferred past first paint so MapLibre evaluation does not delay the SSR'd LCP text.
function mountIslands() {
  const first = []
  const later = []
  for (const el of document.querySelectorAll('[data-island]')) {
    const load = resolveLoader(el.dataset.island)
    if (!load) continue // unknown island: leave the server-rendered fallback
    if (deferIsland(el.dataset.island, window.location.hash)) later.push([el, load])
    else first.push(runIsland(el, load))
  }
  // runIsland never rejects, so allSettled is just "the map chunk is done evaluating".
  Promise.allSettled(first).then(() => {
    for (const [el, load] of later) afterIdle(() => runIsland(el, load))
  })
}

function init() {
  // The masthead's two pickers are independent <details> and would otherwise
  // open on top of each other. Wired before the islands: the theme picker's
  // element is already in the DOM, empty, and soleOpen re-queries on each event.
  soleOpen(document.querySelector('.masthead__nav') ?? document.body)
  scrollCue(document, window)
  createBackToTop({ doc: document, win: window })
  localizeTimes(document, document.documentElement.lang || undefined)

  scheduleAfterFirstPaint(mountIslands)
}

// Guarded so this module can be imported by a Vitest run (no `document`
// global, and none should be added — pure-logic-only, no jsdom) to exercise
// resolveLoader and runIsland without ever executing the DOM-walking loop.
if (typeof document !== 'undefined') init()
