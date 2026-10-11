// Wires the nearby-sensors list to the map: rebuilt after every hex repaint, from the
// features that repaint carried, so it costs no request and never disagrees with the map.
import { mount as mountComponent, unmount } from 'svelte'
import NearbyList from '../components/NearbyList.svelte'
import { buildNearby, describeReading } from './nearbylist.js'
import { bandsFor } from './mappaint.js'
import { findSensor } from './sensors.svelte.js'
import { HEX_SOURCE_ID, PAINT_EVENT } from './mapids.js'

// How many sensors the list names; the rest is a count.
export const NEARBY_LIMIT = 10

// el: the server-rendered [data-nearby] host. onopen(item) opens the sensor, onpreview(item, keyboard) / onunpreview() mark its cell.
export function installNearbyList({ map, el, cfg, state, onopen, onpreview, onunpreview }) {
  const d = el.dataset
  const model = $state({ items: [], total: 0, rest: 0 })
  const container = map.getContainer()

  const rebuild = (features) => {
    const c = map.getCenter()
    const b = map.getBounds()
    const read = {
      bands: bandsFor(state.scales, cfg.metric),
      names: cfg.bandNames,
      unit: cfg.metricUnits[cfg.metric] ?? '',
      lang: cfg.lang,
    }
    const next = buildNearby(features, {
      centre: [c.lng, c.lat],
      bounds: [b.getWest(), b.getSouth(), b.getEast(), b.getNorth()],
      limit: NEARBY_LIMIT,
      nameOf: (id) => findSensor(id)?.stationName || `${d.tSensor || ''} ${id}`.trim(),
      readingOf: (value) => describeReading(value, read),
    })
    model.items = next.items
    model.total = next.total
    model.rest = next.rest
  }

  const onPaint = (e) => {
    if (e.detail?.source === HEX_SOURCE_ID) rebuild(e.detail.features)
  }
  container.addEventListener(PAINT_EVENT, onPaint)

  const component = mountComponent(NearbyList, {
    target: el,
    props: {
      get items() { return model.items },
      get total() { return model.total },
      get rest() { return model.rest },
      title: d.tTitle || '',
      empty: d.tEmpty || '',
      more: d.tMore || '',
      onopen,
      onpreview,
      onunpreview,
    },
  })
  el.hidden = false

  return () => {
    container.removeEventListener(PAINT_EVENT, onPaint)
    unmount(component)
  }
}
