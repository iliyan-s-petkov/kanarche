// The shared answer to "how fresh is this, and is it staying fresh".
//
// Two islands touch it and neither owns it: the freshness pill asks for a
// reload and reports what happened, and the map is the thing that actually
// reloads. A store rather than a DOM event because there is real state here
// (in-flight, last success, last failure) and an event carries none of it, so
// every listener would keep its own copy and they would drift.
//
// NOT in viewstate: that store is the mirror of the URL hash, and a refresh is
// not a place. Putting it there would write a hash for an action with no
// destination and hand the reader a Back button that undoes a reload.
import { AUTO_CHOICES, AUTO_KEY, intervalMs, minutesFromStored } from './freshness.js'
import { safeStorage } from './storage.js'

export function createFreshness({ win = globalThis, doc = globalThis.document, storage = safeStorage(), now = () => Date.now() } = {}) {
  // Auto-refresh defaults to 5 minutes unless the reader chose otherwise. A map
  // of current air quality that quietly goes stale while someone watches it is
  // the one failure this page cannot report, because the numbers still look
  // like numbers.
  let minutes = $state(readMinutes())
  let busy = $state(false)
  let failed = $state(false)
  // The page was rendered from data the server had just read, so the reader is
  // looking at something that WAS fresh, and the line says so from the first
  // paint rather than staying blank until the first manual refresh.
  let at = $state(now())

  function readMinutes() {
    try {
      return minutesFromStored(storage?.getItem(AUTO_KEY))
    } catch {
      return minutesFromStored(null)
    }
  }

  // The map island registers what a reload actually means. Kept as a set so
  // the store does not care how many maps a page mounts, and so a page with no
  // map still has a working (if idle) button rather than a crash.
  const providers = new Set()
  let timer = null

  async function request() {
    if (busy) return
    busy = true
    failed = false
    try {
      await Promise.all([...providers].map((fn) => fn()))
      at = now()
    } catch (err) {
      // Reported, not swallowed: the status line is the only place a reader
      // can find out that the numbers they are looking at did not move.
      failed = true
      console.error('refresh:', err)
    } finally {
      busy = false
    }
  }

  const hidden = () => doc?.visibilityState === 'hidden'

  function schedule() {
    if (timer !== null) {
      win.clearInterval(timer)
      timer = null
    }
    if (minutes > 0 && !hidden()) timer = win.setInterval(request, intervalMs(minutes))
  }

  // Hidden tabs do not poll. On return, a reading older than the interval is
  // refreshed once right away, and the timer restarts from that moment.
  function onVisibility() {
    if (hidden()) {
      schedule()
      return
    }
    if (minutes > 0 && now() - at >= intervalMs(minutes)) request()
    schedule()
  }
  doc?.addEventListener('visibilitychange', onVisibility)
  schedule()

  return {
    get busy() { return busy },
    get failed() { return failed },
    get at() { return at },
    get minutes() { return minutes },
    setMinutes(next) {
      if (!AUTO_CHOICES.includes(next) || next === minutes) return
      minutes = next
      try {
        storage?.setItem(AUTO_KEY, String(next))
      } catch {
        /* private mode, or a full quota: the choice holds for this visit */
      }
      schedule()
    },
    provide(fn) {
      providers.add(fn)
      return () => providers.delete(fn)
    },
    request,
    destroy() {
      if (timer !== null) win.clearInterval(timer)
      timer = null
      doc?.removeEventListener('visibilitychange', onVisibility)
      providers.clear()
    },
  }
}

// One store per page, for the same reason viewstate keeps one: three islands
// reporting three different last-refresh times is worse than none.
let shared = null
export function getFreshness(opts) {
  if (shared === null) shared = createFreshness(opts)
  return shared
}

// TEST-ONLY reset seam, see the note on resetViewStateForTests. A singleton
// that survives between it() blocks makes the first test to touch it decide
// what every later one observes.
export function resetFreshnessForTests() {
  shared?.destroy()
  shared = null
}
