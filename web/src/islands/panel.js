// The panel island. Wires SensorPanel.svelte to the same viewstate singleton
// the map and switcher share (see viewstate.svelte.js), and to the sensor
// registry the map island publishes into (lib/sensors.svelte.js) — never a
// fetch of its own for the panel's own content, see that file's comment.
//
// No component tree here reacts imperatively: every prop below is a plain
// getter, the same idiom islands/switcher.js already uses for `selected`.
// SensorPanel.svelte's own $props()/template reads are the reactive
// consumer, so a read of vs.sensorId or the registry made INSIDE one of
// these getters is tracked exactly as if it happened in a .svelte file —
// dependency tracking follows the read, not the file it is written in. That
// is what lets this file stay plain .js (no $effect, no wrapper component)
// while still repainting on a metric-store-shared mutation it did not
// itself trigger (a marker click handled in islands/map.js).
import { mount as mountComponent, unmount, createRawSnippet } from 'svelte'
import SensorPanel from '../components/SensorPanel.svelte'
import SensorChart from '../components/SensorChart.svelte'
import { panelRows, detailRows, stationMeta, networkText, lastUpdated } from '../lib/sensorview.js'
import { createPanelLink } from '../lib/panellink.svelte.js'
import { parseMetricList, zipLabels } from '../lib/metrics.js'
import { getViewState } from '../lib/viewstate.svelte.js'
import { getFreshness } from '../lib/freshness.svelte.js'
import { getFavourite } from '../lib/favourite.svelte.js'
import { findSensor, getScales, normaliseSensor } from '../lib/sensors.svelte.js'
import { gaugeModel } from '../lib/gauge.js'
import { bandName, readBandNames } from '../lib/bandname.js'
import { bandsFor } from '../lib/mappaint.js'
import { createPanelScroll } from '../lib/panelscroll.js'
import { watchPanelScroll, returnFocus } from '../lib/panelwatch.svelte.js'

// Re-exported, not re-implemented: the projection lives in
// lib/sensors.svelte.js because findSensor (the registry's own lookup) needs
// it too, and a lib module cannot import from an island (islands depend on
// lib, never the reverse — see every other island in this directory).
// Exported here as well because this is the file the task's own interface
// names as normaliseSensor's home.
export { normaliseSensor }

// Only 'out_of_range', 'stuck', 'clamped', 'spatial_outlier' and 'source_invalid' have
// catalogue entries (panel.flag.*, internal/i18n/{bg,en}.json) — 'ok' and 'no_neighbours'
// deliberately do not, because neither is a failure. flagTextFor's lookup
// miss (any flag not a key of `catalogue`, including those two, and
// including anything this frontend does not yet recognise) falls through to
// '', never to the server's i18n miss-marker ('!key!'): that marker is a
// Go-side concept (internal/i18n/i18n.go) produced by Catalogue.T, and
// mount() below never calls it — it only reads the four data-t-flag-*
// attributes the template renders (see area.gohtml), so '!key!' cannot
// reach this function's input in the first place. Exported (rather than a
// closure inside mount()) so this guard is provable without mounting a
// component or touching the DOM.
export function flagTextFor(flag, catalogue) {
  return catalogue[flag] || ''
}

// Pulled out of mount() so the catalogue-construction logic is testable
// without a DOM.
export function flagCatalogueFrom(d) {
  return {
    out_of_range: d.tFlagOutOfRange || '',
    stuck: d.tFlagStuck || '',
    spatial_outlier: d.tFlagSpatialOutlier || '',
    source_invalid: d.tFlagSourceInvalid || '',
    clamped: d.tFlagClamped || '',
  }
}

// flagTextForSensor names the failed metrics: "Temperature, Humidity: <text>",
// one sentence per distinct flag. A payload without the flags column falls back
// to the worst-flag text with no metric named.
export function flagTextForSensor(sensor, catalogue, labels = {}) {
  if (!sensor) return ''
  if (!sensor.flags) return flagTextFor(sensor.flag, catalogue)
  const byFlag = new Map()
  for (const [metric, flag] of Object.entries(sensor.flags)) {
    if (!flagTextFor(flag, catalogue)) continue
    if (!byFlag.has(flag)) byFlag.set(flag, [])
    byFlag.get(flag).push(labels[metric] || metric)
  }
  return [...byFlag].map(([flag, names]) => `${names.join(', ')}: ${catalogue[flag]}`).join(' ')
}

export function mount(el) {
  const d = el.dataset
  const metrics = parseMetricList(d.metrics)
  const options = zipLabels(metrics, parseMetricList(d.metricLabels))
  const vs = getViewState({ metrics, defaultMetric: d.metric })
  const favourite = getFavourite()
  // The embed is a third-party frame: it neither saves nor shows a favourite.
  const canStar = !document.body.classList.contains('embed')

  const flagCatalogue = flagCatalogueFrom(d)
  const bandNames = readBandNames(d)
  const metricLabels = Object.fromEntries(options.map((o) => [o.metric, o.label]))

  // Memoised by sensor id: `chart` below is read on EVERY reactive
  // re-evaluation of the props object (a pan that brings in new sensors, a
  // metric switch that does not even touch this sensor's rows, scales
  // finally loading) — none of those should tear down and remount the chart
  // that is already on screen and mid-fetch. Only the OPEN sensor changing
  // should.
  let chartId = null
  let chartSnippet = null
  // One link per open station: it carries the chart's metric to the gauges.
  let link = createPanelLink()
  function chartFor(sensor) {
    const id = sensor?.id ?? null
    if (id === chartId) return chartSnippet
    chartId = id
    link = createPanelLink()
    chartSnippet = id === null ? null : buildChartSnippet(sensor, options, d, link)
    return chartSnippet
  }

  const detailLabels = {
    devices: d.tDetailDevices || '',
    hardware: d.tDetailHardware || '',
    since: d.tDetailSince || '',
    updated: d.tDetailUpdated || '',
    coords: d.tDetailCoords || '',
  }
  const stationLabels = {
    code: d.tStationCode || '',
    type: d.tStationType || '',
    area: d.tStationArea || '',
  }
  // The page's own language, so dates read the way the rest of the page does.
  // document.documentElement.lang is what the server rendered; undefined (the
  // browser's own locale) only if the attribute is missing.
  const locale = document.documentElement.lang || undefined

  mountComponent(SensorPanel, {
    target: el,
    props: {
      get open() { return findSensor(vs.sensorId) !== null },
      // Each row carries its own gauge model (SensorPanel stays dumb: it only
      // hands the model to Gauge.svelte, never reads scales itself).
      get rows() {
        const sensor = findSensor(vs.sensorId)
        if (!sensor) return []
        const scales = getScales()
        return panelRows(sensor, options, scales).map((row) => ({
          ...row,
          model: gaugeModel(row, scales),
          // The band in words, so the arc's colour is not the only thing that says it.
          band: row.missing ? '' : bandName(row.value, bandsFor(scales, row.metric), bandNames),
        }))
      },
      // Composed from the (non-templated) i18n label plus the sensor id —
      // see SensorPanel.svelte's own comment on why `sensor` itself is not
      // one of its props. An official station names itself (station_name);
      // the synthetic 9e9 sensor id it would otherwise fall back to is not
      // a useful label for a reader.
      get title() {
        const sensor = findSensor(vs.sensorId)
        if (!sensor) return ''
        return sensor.stationName || `${d.tTitle || ''} ${sensor.id}`
      },
      get flagText() {
        const sensor = findSensor(vs.sensorId)
        return flagTextForSensor(sensor, flagCatalogue, metricLabels)
      },
      closeLabel: d.tClose || '',
      favSave: d.tFavSave || '',
      favRemove: d.tFavRemove || '',
      get favourite() { return vs.sensorId !== null && favourite.id === vs.sensorId },
      onstar: canStar ? () => { if (vs.sensorId !== null) favourite.toggle(vs.sensorId) } : null,
      noValue: d.tNoValue || '',
      detailsLabel: d.tDetails || '',
      get details() {
        const sensor = findSensor(vs.sensorId)
        return sensor ? detailRows(sensor, detailLabels, locale) : []
      },
      get meta() {
        const sensor = findSensor(vs.sensorId)
        return sensor ? stationMeta(sensor, stationLabels) : []
      },
      get network() {
        const sensor = findSensor(vs.sensorId)
        return sensor ? networkText(sensor, d.tNetwork || '') : ''
      },
      get source() {
        const sensor = findSensor(vs.sensorId)
        return sensor ? sensor.source || 'sensor.community' : ''
      },
      get updated() {
        const sensor = findSensor(vs.sensorId)
        return sensor ? lastUpdated(sensor) : null
      },
      locale,
      // Read after `chart`, which is what swaps the link on a new station.
      get link() {
        chartFor(findSensor(vs.sensorId))
        return link
      },
      onclose: () => vs.closeSensor(),
      // Keyed by STATION now, not by the device charted: the chart component
      // owns which metric it is drawing (and therefore which device it asks),
      // so remounting it when the reader switches metric would throw away the
      // selection that caused the switch.
      get chart() {
        return chartFor(findSensor(vs.sensorId))
      },
    },
  })

  // Phones: tap scrolls the card into view; close scrolls back to the map.
  const scroll = createPanelScroll({
    win: window,
    panel: () => el.querySelector('.sensor-panel'),
    isFull: () => !!document.querySelector('.map:fullscreen, .map--faux-full'),
  })
  watchPanelScroll({
    shownId: () => (findSensor(vs.sensorId) !== null ? vs.sensorId : null),
    initialId: vs.sensorId,
    scroll,
    onReturn: (opener) => returnFocus(opener),
  })
}

// buildChartSnippet wraps Chart.svelte in a snippet built from vanilla JS via
// createRawSnippet — the escape hatch for exactly this: panel.js is plain
// .js, so it cannot write `{#snippet}...{/snippet}` (compiler syntax, only
// valid inside .svelte files), but SensorPanel.svelte's `chart` prop is a
// snippet, not a component reference.
//
// The metric and period the chart OPENS on come from data-metric/data-period
// on the panel's own container, not from vs.metric: the panel lists every
// metric a sensor reports regardless of which metric the map is currently
// coloured by (see lib/sensorview.js), so tying the embedded chart to the
// map's live metric would make the panel disagree with itself the moment a
// visitor switches metric while it is open. After that the reader owns both,
// inside SensorChart.
//
// The metric list offered is this station's, not the map's whole set: the switcher
// must not offer a metric whose only possible answer is an empty plot.
function buildChartSnippet(sensor, options, d, link) {
  const measured = options.filter(({ metric }) => Object.hasOwn(sensor.values, metric))
  return createRawSnippet(() => ({
    render: () => '<div></div>',
    setup: (node) => {
      const component = mountComponent(SensorChart, {
        target: node,
        props: {
          stationId: sensor.id,
          sources: sensor.sources,
          options: measured,
          periods: parseMetricList(d.periods),
          periodLabels: parseMetricList(d.periodLabels),
          periodShortLabels: parseMetricList(d.periodShortLabels),
          link,
          get stamp() { return getFreshness().at },
          moreLabel: d.tMore || '',
          aboutLabel: d.tDetails || '',
          shareLabel: d.tShare || '',
          embedLabel: d.tEmbed || '',
          shareDone: d.tShareDone || '',
          embedDone: d.tEmbedDone || '',
          copyFailed: d.tCopyFailed || '',
          initialPeriod: d.period,
          initialMetric: d.metric,
          metricLegend: d.tChartMetricLegend || '',
          periodLegend: d.tChartPeriodLegend || '',
          customLabel: d.tPeriodCustom || '',
          fromLabel: d.tPeriodFrom || '',
          toLabel: d.tPeriodTo || '',
          nowLabel: d.tPeriodNow || '',
          resetLabel: d.tChartReset || '',
          rangeInvalid: d.tChartRangeInvalid || '',
          nearbyLegend: d.tNearbyLegend || '',
          nearbyOff: d.tNearbyOff || '',
          nearbySingleOnly: d.tNearbySingleOnly || '',
          nearbyLabels: {
            low: d.tNearbyLow || '',
            median: d.tNearbyMedian || '',
            high: d.tNearbyHigh || '',
          },
          // Config (frontend.chart_*_colour), not CSS: a canvas inherits no
          // custom property. The panel draws as many lines as the reader ticks,
          // so the palette runs on past the two named colours.
          colours: [d.lineColour, d.compareColour, ...parseMetricList(d.seriesColours)],
          timeLabel: d.tChartTime || '',
          empty: d.tChartEmpty || '',
          unavailable: d.tChartUnavailable || '',
          text: {
            summary: d.tChartTextSummary || '',
            none: d.tChartTextNone || '',
            captionHourly: d.tChartTextCaptionByHour || '',
            captionDaily: d.tChartTextCaptionByDay || '',
          },
        },
      })
      return () => unmount(component)
    },
  }))
}

