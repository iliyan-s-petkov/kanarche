// How old the numbers on screen are, and what to say about it.
//
// Every rule that does not need a browser lives here; the reactive state and
// the timer are in freshness.svelte.js. The split is the same one viewstate
// makes, and for the same reason: a rune only compiles in a .svelte.js file,
// and the decisions are worth testing without one.

export const AUTO_KEY = 'kanarche:auto-refresh'

// Minutes between automatic refreshes, 0 meaning off. Five is the default: the
// network's own cadence is "every few minutes", so faster asks the origin for
// numbers that have not changed, and a wall display drifts if it is much slower.
export const AUTO_CHOICES = [0, 5, 15, 30]
export const DEFAULT_MINUTES = 5

export function intervalMs(minutes) {
  return minutes * 60 * 1000
}

// The key used to hold a boolean: "true" meant the fixed five minutes and
// "false" meant off, so they migrate to 5 and 0. Anything else is unset.
export function minutesFromStored(raw) {
  if (raw === 'true') return DEFAULT_MINUTES
  if (raw === 'false') return 0
  return AUTO_CHOICES.find((m) => String(m) === raw) ?? DEFAULT_MINUTES
}

// statusText is the whole of what the freshness line says, in one place.
//
// The three states are not decorations of one another: "refreshing" is a
// promise about the near future, a time is a fact about the past, and a
// failure is neither. An absence is stated plainly rather than dressed as an
// error (DESIGN.md §2.3) — a page that has never loaded says nothing at all
// rather than announcing that it has no time to show.
export function statusText({ busy, dataAt, failed }, t, lang = 'bg') {
  if (busy) return t.loading
  if (failed) return t.failed
  // Only the server's snapshot time. The client clock is not the server's, so it is never written as "Data from".
  if (dataAt == null) return ''
  return `${t.updated} ${formatTime(dataAt, lang)}`
}

// A snapshot older than this is shown in the stale colour. The ingest cycle is a few minutes.
export const STALE_MS = 15 * 60 * 1000

export function isStale(at, now) {
  return at != null && now - at > STALE_MS
}

// The snapshot time (ms) of the first body with a parseable generated_at, else null.
export function snapshotTime(...bodies) {
  for (const body of bodies) {
    const ms = Date.parse(body?.generated_at)
    if (Number.isFinite(ms)) return ms
  }
  return null
}

// The snapshot time of the bodies a refresh actually replaced, comparing each slot before and after by identity.
// A body that was not reloaded (a failed fetch, a tier that keeps none) must not date this refresh.
export function reloadedSnapshot(before, after) {
  return snapshotTime(...after.filter((body, i) => body != null && body !== before[i]))
}

// 24-hour HH:MM in every language: the label is a few characters wide on a phone, and h23 keeps midnight 00:xx.
export function formatTime(at, lang = 'bg') {
  return new Date(at).toLocaleTimeString(lang, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })
}
