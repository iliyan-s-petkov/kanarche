// The bathing-site card over the map. One component, mounted on first open; the view state drives it.
import { mount } from 'svelte'
import SeaPanel from '../components/SeaPanel.svelte'
import { getJSON } from './api.js'

export function createSeaPanel(frame, cfg, fetchJSON = getJSON) {
  const view = $state({ id: null, detail: null, failed: false })
  const host = document.createElement('div')
  host.className = 'map-sea'
  host.hidden = true
  frame.appendChild(host)
  let mounted = false

  function close() {
    view.id = null
    view.detail = null
    view.failed = false
    host.hidden = true
  }

  async function open(id) {
    view.id = id
    view.detail = null
    view.failed = false
    host.hidden = false
    if (!mounted) {
      mount(SeaPanel, {
        target: host,
        props: { view, t: cfg.t.sea, lang: cfg.lang, colours: cfg.seaColours, creditURL: cfg.seaCreditURL, onclose: close },
      })
      mounted = true
    }
    try {
      const d = await fetchJSON(`/api/v1/sea/sites/${encodeURIComponent(id)}`)
      // A later click wins; a slow answer for an earlier site is dropped.
      if (view.id === id) view.detail = d
    } catch {
      if (view.id === id) view.failed = true
    }
  }

  return {
    open,
    close,
    get id() { return view.id },
  }
}
