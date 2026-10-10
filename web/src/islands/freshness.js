// The freshness pill: what time the numbers are from, a refresh-now button and
// the auto-refresh interval, behind one icon. It is the only refresh control on
// a page; the home list tab has none and relies on this one.
import { mount as mountComponent } from 'svelte'
import DataFreshness from '../components/DataFreshness.svelte'
import { getFreshness } from '../lib/freshness.svelte.js'
import { statusText } from '../lib/freshness.js'

export function mount(el, doc = document) {
  const d = el.dataset
  const fresh = getFreshness()
  const t = { updated: d.tUpdated || '', loading: d.tLoading || '', failed: d.tFailed || '' }
  const lang = doc.documentElement.getAttribute('lang') || 'bg'
  const options = [
    { value: 0, text: d.tOff || '' },
    { value: 5, text: d.tM5 || '' },
    { value: 15, text: d.tM15 || '' },
    { value: 30, text: d.tM30 || '' },
  ]
  return mountComponent(DataFreshness, {
    target: el,
    props: {
      labels: { trigger: d.tLabel || '', now: d.tNow || '', group: d.tAuto || '' },
      options,
      get status() { return statusText(fresh, t, lang) },
      get minutes() { return fresh.minutes },
      get busy() { return fresh.busy },
      onpick: (m) => fresh.setMinutes(m),
      onrefresh: () => fresh.request(),
    },
  })
}
