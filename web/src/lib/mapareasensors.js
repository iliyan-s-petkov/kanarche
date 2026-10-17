// Mounts the area page's sensor table into its server shell and opens a row's sensor on the map.
import { mount as mountComponent, unmount } from 'svelte'
import AreaSensors from '../components/AreaSensors.svelte'
import { readBandNames } from './bandname.js'
import { findSensor } from './sensors.svelte.js'
import { DEEP_LINK_ZOOM } from './placement.js'

function parseDistricts(raw) {
  try {
    const v = JSON.parse(raw || '{}')
    return v && typeof v === 'object' && !Array.isArray(v) ? v : {}
  } catch {
    return {}
  }
}

export function textsFrom(d) {
  return {
    summary: d.tSummary || '',
    summaryOne: d.tSummaryOne || '',
    caption: d.tCaption || '',
    colSensor: d.tColSensor || '',
    colDistrict: d.tColDistrict || '',
    colValue: d.tColValue || '',
    colBand: d.tColBand || '',
    colUpdated: d.tColUpdated || '',
    open: d.tOpen || '',
    filter: d.tFilter || '',
    filterPlaceholder: d.tFilterPlaceholder || '',
    filterHint: d.tFilterHint || '',
    status: d.tStatus || '',
    rangeOne: d.tRangeOne || '',
    empty: d.tEmpty || '',
    none: d.tNone || '',
    noValue: d.tNoValue || '',
    page: d.tPage || '',
    pageOf: d.tPageOf || '',
    pageFirst: d.tPageFirst || '',
    pagePrev: d.tPagePrev || '',
    pageNext: d.tPageNext || '',
    pageLast: d.tPageLast || '',
    pagerLabel: d.tPagerLabel || '',
    perPage: d.tPerPage || '',
    perPageAll: d.tPerPageAll || '',
    metrics: { P2: d.tP2 || 'PM2.5', P1: d.tP1 || 'PM10' },
    units: { P2: d.tUnitP2 || '', P1: d.tUnitP1 || '' },
  }
}

// el: the server-rendered [data-area-sensors] shell. onopen(id) opens the sensor's panel.
export function installAreaSensors({ map, el, cfg, onopen }) {
  const d = el.dataset
  // The district column is shown only with its header text, so a bad attribute drops both.
  const districts = d.tColDistrict ? parseDistricts(d.districts) : {}
  const open = (row) => {
    const s = findSensor(row.id)
    if (typeof s?.lon === 'number' && typeof s?.lat === 'number') {
      map.jumpTo({ center: [s.lon, s.lat], zoom: Math.max(map.getZoom(), DEEP_LINK_ZOOM) })
    }
    // The table sits below the map, so bring the map and its panel into view.
    map.getContainer()?.scrollIntoView?.({ block: 'nearest' })
    onopen(row.id)
  }
  const component = mountComponent(AreaSensors, {
    target: el,
    props: {
      texts: textsFrom(d),
      names: readBandNames(d),
      area: d.area || '',
      slug: cfg.slug,
      districts,
      lang: cfg.lang,
      onopen: open,
      // Shown only once this area's sensors are in, so a loading or adopted body never reads "(0)".
      onready: () => { el.hidden = false },
    },
  })
  return () => unmount(component)
}
