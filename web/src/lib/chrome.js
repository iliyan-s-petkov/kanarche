// The island seam for the map's DOM chrome: it builds the legend, the hint
// banner, the control cluster and the player UI beside the MapLibre canvas,
// and returns the handle mount() threads to everything else. Kept separate so
// the pure builders it calls stay importable on their own.
import { LEGEND_CLASSES, buildPollenRows, buildSeaRows, buildWindRow, legendRows, legendTitle, renderLegend, setPollenLegend, setWindRow } from './legend.js'
import { createScaleDialog } from './scaledialog.js'
import { createInfoDialog, pollenInfoContent, seaInfoContent } from './infodialog.js'
import { mountFullscreen, mountZoom, mountLocate, mountOrientation } from './mapcontrols.js'
import { mountLayers } from './maplayers.js'
import { setSensorStatus } from './sensorfilter.svelte.js'
import { readFlag, writeFlag, safeStorage } from './storage.js'
import { mountWindow, readWindow, windowOptions } from './mapwindow.js'
import { mountPlayer } from './timelapse.js'
import { RASTER_LAYER_ID } from './mapids.js'
import { LEGEND_FOLD_KEY } from './mapconfig.js'
import { setCellValues } from './mappaint.js'
import { createSensorSheet } from './sensorsheet.svelte.js'
import { createSideDock } from './sidedock.svelte.js'

// hintController owns the ONE rule about the hint banner: an error outranks the
// routine hint, permanently.
//
// showHint is called on every refresh with the text that applies right now, and
// with '' when none does — that clear-on-empty is what makes the tier hint
// disappear when it stops applying. It is also what silently erased the
// scales-failure explanation, because refresh runs immediately after the scales
// load and calls showHint('') whenever the zoom's tier is served as-is (the
// common case: zoom 7 on / and zoom ~10 on an area page). ANYONE ADDING A
// showHint CALL SHOULD KNOW IT CAN ERASE A REAL ERROR MESSAGE — use showError
// for anything the visitor must keep seeing.
//
// Pure and separate from the DOM on purpose: `render` is the only side effect,
// so the precedence rule itself can be driven by a test with an array as the
// sink instead of a browser, and the rule the test exercises is the same code
// the page runs.
export function hintController(render) {
  let stickyError = ''
  return {
    showHint(text) {
      // Deliberately not "only ignore the empty string": once the map is known
      // to be uncoloured, the tier hint is the lesser message too.
      if (stickyError) return
      render(text)
    },
    showError(text) {
      stickyError = text
      render(text)
    },
  }
}

// Landscape-phone query, matching app.css's height-gated breakpoint.
export const PHONE_LANDSCAPE_QUERY = '(orientation: landscape) and (max-height: 520px) and (hover: none)'

// True when the viewport is a phone in either orientation.
export function isPhoneViewport() {
  if (typeof matchMedia !== 'function') return false
  return matchMedia('(max-width: 672px)').matches || matchMedia(PHONE_LANDSCAPE_QUERY).matches
}

// mountChrome builds the legend and the hint banner as plain DOM, appended
// beside the MapLibre canvas inside the same container. Plain DOM rather than
// Svelte: two static-ish text nodes and a class toggle need no reactivity
// system, and pulling in Svelte for this would be a dependency with nothing to
// show for it.
//
// Classes only, never `el.style` — the CSP's style-src has no 'unsafe-inline',
// so an inline style written from JS is silently dropped by the browser, not
// merely a lint complaint.
// Exported for mount() and for the two test files that drive it directly.
// It touches no MapLibre object, so where the key and the tier line
// land in the DOM is checkable without a WebGL context — and that placement is
// load-bearing (see the shell comments below), not decoration.
export function mountChrome(el, cfg) {
  // Resolve storage once for threaded access to player and legend prefs. Must
  // be called before any caller can access chrome.storage.
  const storage = cfg.storage ?? safeStorage()

  // The key and the tier line go on the SHELL, not on #map, and they are the
  // only two things here that do. The kit turns .scale--onmap static below
  // 672px so the key sits under the map on a phone — and inside .map, "under
  // the map" is still inside the map, over the corner of the canvas. The shell
  // exists to be the positioning context for exactly this. Falling back to `el`
  // keeps a map mounted without a shell rendering something rather than
  // throwing, at the cost of the phone layout.
  const shell = el.closest('.map-shell') ?? el

  // <details>: it owns the open state, the keyboard and the accessible name, so
  // nothing else in the DOM has to record whether the key is folded. Folded by
  // default on every viewport: the pill names the metric and an open card
  // covers the map.
  //
  // The fold is remembered, like every other map preference. It is a different
  // control from the layers menu's "Legend": the menu says whether there is a
  // key at all, the triangle says whether it is unrolled, and a reader who
  // folds the key on a small screen wants it folded on the next page too.
  // matchMedia is missing under jsdom — absent means "not a phone".
  // Portrait-only: the rotation listener below only tracks the 672px breakpoint.
  const phoneQuery = typeof matchMedia === 'function' ? matchMedia('(max-width: 672px)') : null

  // Phone check for the legend auto-close and the refresh controls' placement.
  const phone = isPhoneViewport()
  const legend = document.createElement('details')
  legend.className = LEGEND_CLASSES
  // Folded by default everywhere; a stored choice still wins.
  legend.open = readFlag(LEGEND_FOLD_KEY, false)
  // #579: true while the layers list is the reason the legend is folded, so
  // closing the list can restore it — but only that fold, never a reader's own.
  let legendFoldedForLayers = false
  // A programmatic legend.open write (map move, the layers list) must not
  // persist as a reader's own choice. <details> fires `toggle` as a queued
  // task, coalescing several synchronous writes into one event by the time it
  // runs — so the guard records the TARGET state rather than a boolean, and
  // the listener only skips writeFlag when the event still matches it.
  let autoTarget = null
  const setLegendOpen = (open) => {
    if (legend.open === open) return
    autoTarget = open
    legend.open = open
  }

  // The layers menu takes the frame's top-left corner, which is why the hint
  // and the note below are offset past it in app.css rather than sharing it.
  // Built here, filled later: its options are read off the mounted style, which
  // does not exist until MapLibre has loaded one.
  // #579: opening the list folds an open legend (not persisted); closing the
  // list restores it, but only when the list is what folded it.
  const layers = mountLayers(el, {
    label: cfg.t.layersButton,
    onToggle: (open) => {
      if (open) {
        if (legend.open) {
          legendFoldedForLayers = true
          setLegendOpen(false)
        }
      } else if (legendFoldedForLayers) {
        legendFoldedForLayers = false
        setLegendOpen(true)
      }
    },
  })

  legend.addEventListener('toggle', () => {
    if (autoTarget !== null && legend.open === autoTarget) {
      autoTarget = null
      return
    }
    autoTarget = null
    writeFlag(LEGEND_FOLD_KEY, legend.open)
    // Opening the legend, by any real interaction, closes an open layers list.
    if (legend.open && layers.button.getAttribute('aria-expanded') === 'true') {
      legendFoldedForLayers = false
      layers.open(false)
    }
  })
  shell.appendChild(legend)

  // #580: a tap anywhere on the open legend folds it, same as the triangle —
  // except its interactive children, which keep their own behaviour.
  legend.addEventListener('click', (e) => {
    if (!legend.open) return
    if (e.target.closest('button, a, summary')) return
    legend.open = false
  })
  legend.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || !legend.open) return
    legend.open = false
    legend.querySelector(':scope > .scale__toggle')?.focus()
  })

  // Fold without persisting.
  const closeLegend = () => {
    if (!phone) return
    setLegendOpen(false)
  }

  // The key says which colour is worse; it cannot say what 25 µg/m³ IS, whose
  // rule that is, or where to read it. That belongs behind an (i), not on the
  // map: it is a paragraph, and the map is the page.
  //
  // On the frame rather than the shell, because a modal in fullscreen must be
  // inside the fullscreen element or the browser renders it nowhere.
  const scaleDialog = createScaleDialog(el.ownerDocument, {
    closeLabel: cfg.t.close,
    sourceLabel: cfg.t.legendSource,
    disclaimer: cfg.t.disclaimer,
    lang: cfg.lang,
  })
  el.appendChild(scaleDialog.el)
  // The pollen and bathing-water sections' (i) share one dialog.
  const infoDialog = createInfoDialog(el.ownerDocument, { closeLabel: cfg.t.close })
  el.appendChild(infoDialog.el)

  // What a dot aggregates at this zoom. Under the map as prose, not inside the
  // key: the key is an overlay with no panel behind it (the kit's §5.2d — a box
  // there would hide the map it explains), and a sentence of that length haloed
  // over a choropleth is not readable. It is also not part of the ramp.
  // AFTER the shell, not inside it. The shell is the box the key is anchored
  // to — inset-block-end:16px is measured from the shell's bottom — so anything
  // else placed in it makes the shell taller than the map and pushes the key
  // down past the map's own edge. Measured live: 16px below it.
  // Adopt the server-rendered caption (PageData.InitialTier) when the page has one.
  const ssrTierLine = shell.nextElementSibling
  const adoptingTierLine = ssrTierLine?.classList.contains('map-tier')
  const tierLine = adoptingTierLine ? ssrTierLine : document.createElement('p')
  tierLine.className = 'legend__tier map-tier'
  if (!adoptingTierLine) {
    shell.after(tierLine)
    tierLine.hidden = true
  }

  // Full screen and zoom go on the FRAME, not the shell: they are furniture on
  // the canvas and belong over it at every width, which is the opposite of the
  // key's rule directly above. Fullscreen wires itself — it drives the element,
  // not the camera — while the zoom stack is returned unwired, because the
  // MapLibre map is constructed after this function returns.
  // The key rides along. It is anchored to the shell (see above), and in real
  // fullscreen the frame IS the viewport — so a reader who went full screen
  // lost the colour key on the one view where the map is all there is. Moved
  // rather than duplicated: it stays one <details>, so its folded state, its
  // contents and the layers menu's "show the key" toggle all keep pointing at
  // the same element on both sides of the trip.
  // The open sensor's gauges ride along too, in a sheet (lib/sensorsheet.svelte.js).
  let fullButton = null
  // The overlay rows (top controls, bottom-left refresh/window/play) ride along too: real fullscreen paints only the frame.
  const rows = ['.map-controls', '.map-freshness']
    .map((sel) => el.closest('.map-shell')?.querySelector(sel))
    .filter(Boolean)
    .map((node) => {
      // A marker, not the next sibling: that sibling may be a row that is itself away in the frame.
      const home = document.createComment('')
      node.before(home)
      return { node, home }
    })
  const sheet = createSensorSheet(el, {
    closeLabel: cfg.t.close,
    historyLabel: cfg.t.sheetHistory,
    exitFull: () => { if (fullButton?.getAttribute('aria-pressed') === 'true') fullButton.click() },
  })
  // Wide screens open the same card as a panel along the map's bottom instead; it stands down in fullscreen.
  const dock = createSideDock(el, {
    closeLabel: cfg.t.close,
    moreLabel: cfg.t.panelHistory,
    moreShortLabel: cfg.t.panelHistoryShort,
    areaBelow: cfg.t.panelAreaBelow,
    areaBelowUnnamed: cfg.t.panelAreaBelowUnnamed,
    foldLabel: cfg.t.panelFold,
    expandLabel: cfg.t.panelExpand,
    resizeLabel: cfg.t.panelResize,
  })
  fullButton = mountFullscreen(el, {
    label: cfg.t.fullscreen,
    exitLabel: cfg.t.fullscreenExit,
    onChange: (full) => {
      (full ? el : shell).appendChild(legend)
      for (const { node, home } of rows) {
        if (full) el.appendChild(node)
        else home.before(node)
      }
      // The gauges have one home at a time: the dock lets go before the sheet takes them, and the reverse on exit.
      if (full) { dock.setFull(true); sheet.setFull(true) } else { sheet.setFull(false); dock.setFull(false) }
    },
  })
  const zoom = mountZoom(el, {
    inLabel: cfg.t.zoomIn,
    outLabel: cfg.t.zoomOut,
    resetLabel: cfg.t.zoomReset,
  })
  const orient = mountOrientation(el, {
    label: cfg.t.orientLabel,
    tip: cfg.t.orientTip,
    tiltLabel: cfg.t.orientTilt,
    headingLabel: cfg.t.orientHeading,
    northLabel: cfg.t.orientNorth,
    closeLabel: cfg.t.close,
  })

  // The averaging window. Built with the chrome and wired by mount(), which
  // owns what a pick costs — see lib/mapwindow.js on why it is a menu in the
  // bottom-left cluster rather than a select across the top of the map.
  //
  // The refresh controls ride inside its panel on a phone: they and the window
  // button are two overlays in one corner, and a phone has room for one. The
  // ISLAND HOST is what moves, not the `.data-refresh` it renders — that host
  // is server-rendered and always here, while the Svelte island that fills it
  // mounts on its own schedule.
  const freshBox = el.closest('.map-shell')?.querySelector('.map-freshness') ?? null
  const refreshBox = freshBox?.querySelector('[data-island="freshness"], .data-refresh') ?? null
  const windowMenu = mountWindow(el, {
    label: cfg.t.windowLabel,
    options: windowOptions(cfg.windowLabels),
    value: readWindow(),
    // Into the freshness pill's own box, so the two are one flex row: an
    // absolute offset here would be this file's guess at how wide that pill is,
    // and it is one icon wide on some pages and two on others.
    host: freshBox ?? el,
    footer: phone && refreshBox ? refreshBox : undefined,
  })

  // Rotation crosses the breakpoint without a page load, so the move is a
  // listener rather than a one-off read. prepend: the pill led the corner row
  // before the window button and the player were appended after it.
  // Signal lets dispose() drop the listener so the chrome closure is not retained.
  const live = new AbortController()
  phoneQuery?.addEventListener?.('change', (e) => {
    if (!refreshBox || !freshBox) return
    if (e.matches) windowMenu.footer.appendChild(refreshBox)
    else freshBox.prepend(refreshBox)
  }, { signal: live.signal })

  // The open attribution card covers the key's corner on a phone: fold it then.
  // A click on its (i), not a class watch: MapLibre opens the card itself on load.
  el.addEventListener('click', (e) => {
    if (!e.target.closest?.('.maplibregl-ctrl-attrib-button')) return
    queueMicrotask(() => {
      if (el.querySelector('.maplibregl-ctrl-attrib.maplibregl-compact-show')) closeLegend()
    })
  }, { signal: live.signal })

  // Third in the bottom-left cluster: refresh, then which window, then play.
  const player = mountPlayer(el, {
    label: cfg.t.timeLabel,
    playLabel: cfg.t.playLabel,
    pauseLabel: cfg.t.pauseLabel,
    exitLabel: cfg.t.exitLabel,
    speedLabel: cfg.t.speedLabel,
    host: freshBox ?? el,
  })

  // Fold the key when the bar opens into the same corner; folded, not hidden (spec 7.4).
  const showPlayer = player.show
  player.show = (count) => {
    showPlayer(count)
    if (count > 0) closeLegend()
  }

  // Two toggles about the SCREEN rather than about the basemap, listed above
  // the categories rather than smuggled in beside "Shops" as if they were one
  // more kind of place.
  //
  // The basemap one hides the ground — the raster and the vector detail drawn
  // over it — and only that. The kit's own version walks every layer in the
  // style, which on this map would take the readings down with it. "Hide the
  // basemap" has to leave the measurements standing, or it is not the control
  // it says it is.
  const layerViews = [
    { id: 'legend', label: cfg.t.viewLegend, apply: (on) => { legend.hidden = !on } },
    {
      id: 'cellValues',
      label: cfg.t.viewCellValues,
      // No needsMap: the cells are this island's own layer and are drawn on a
      // map served without tiles like any other.
      // On by default (no defaultOff); a stored choice still wins.
      apply: (on, map) => setCellValues(map, on),
    },
    {
      id: 'basemap',
      label: cfg.t.viewBasemap,
      needsMap: true,
      apply: (on, map) => {
        const v = on ? 'visible' : 'none'
        map.setLayoutProperty(RASTER_LAYER_ID, 'visibility', v)
        // The raster AND the vector detail over it: hiding one and leaving the
        // other would strand road lines and POI pins over a blank canvas.
        for (const l of map.getStyle()?.layers ?? []) {
          if (l.metadata?.['airbg:group']) map.setLayoutProperty(l.id, 'visibility', v)
        }
      },
    },
  ]

  // Not in layerViews: the map loader places it beside the station toggles.
  const inactiveView = {
    // The id stays 'inactiveSensors': visitors' saved choices are keyed on it.
    id: 'inactiveSensors',
    label: cfg.t.viewInactiveStations,
    // Off by default: a sensor that stopped reporting has no reading to show,
    // and a grid full of no-data cells reads as an empty country rather than
    // a quiet one. The subscription in mount() repaints both tiers.
    defaultOff: true,
    // A blank slot, so the text lines up with the marked rows around it.
    mark: 'blank',
    apply: (on) => setSensorStatus(on ? 'all' : 'active'),
  }

  const hint = document.createElement('div')
  hint.className = 'map-hint'
  // The banner is the map's only running commentary — the fallback tier, a
  // failed load, both networks unticked — and none of it is visible to a screen
  // reader otherwise, because the map itself is a canvas. polite, not assertive:
  // nothing here interrupts what the reader is doing.
  hint.setAttribute('aria-live', 'polite')
  hint.hidden = true
  el.appendChild(hint)

  // The unscaled-metric explanation. A separate element from the hint/error
  // banner above, on purpose: hintController's whole reason to exist is the
  // precedence rule between a routine hint and a sticky error, and a note
  // about the CURRENT metric having no band table is neither of those — it is
  // not routine (it does not come and go with the viewport) and it is not an
  // error (nothing failed). Conflating it with hint/error would either let a
  // real error hide the note or let the note block a real error from showing.
  const note = document.createElement('div')
  note.className = 'map-note'
  note.hidden = true
  el.appendChild(note)

  // The find-me button: precise, user-initiated geolocation (see locateMe in
  // this file). The click handler is wired by mount(), which is where `state`
  // (the loaded area list locateMe reads) and the real `map` first exist;
  // mountChrome only owns the DOM.
  const locateButton = mountLocate(el, { label: cfg.t.locateButton })

  // The wind overlay's disclosure. Its control is a checkbox in the layers
  // menu, with the other overlays — it was a button of its own in the corner,
  // which said the layer was a different kind of thing from the rest of what
  // the map draws when it is not. The checkbox's own checked state is what
  // announces the layer is on, since a screen reader cannot see the arrows.
  //
  // The disclosure stays a sibling of the map rather than anything inside that
  // menu: it must remain visible while the layer is, and the menu closes.
  //
  // Folded, and folded again every time the layer comes back: unrolled it is
  // two sentences and a model name over the map, which on a phone is most of
  // the screen the arrows are drawn on. The summary keeps it a line that says
  // what it is, so nothing is hidden — only rolled up.
  const windNote = document.createElement('details')
  windNote.className = 'map-wind-label'
  windNote.hidden = true
  const windSummary = document.createElement('summary')
  windSummary.className = 'map-wind-label__toggle'
  // Wrapped so the phone rule below can hide the text and keep only the
  // ::before (i), the same idiom the layers/locate buttons already use.
  const windSummaryText = document.createElement('span')
  windSummaryText.className = 'map-wind-label__text-label'
  windSummaryText.textContent = cfg.t.windAbout || cfg.t.windToggle
  windSummary.appendChild(windSummaryText)
  const windText = document.createElement('div')
  windText.className = 'map-wind-label__text'
  windNote.append(windSummary, windText)
  el.appendChild(windNote)

  // Folds on a click on the note body (links still navigate) and on Escape.
  // A map click calls foldWind from the map island. aria-expanded mirrors open.
  const syncWindExpanded = () => windSummary.setAttribute('aria-expanded', String(windNote.open))
  const foldWind = () => {
    windNote.open = false
    syncWindExpanded()
  }
  syncWindExpanded()
  windNote.addEventListener('toggle', syncWindExpanded)
  windNote.addEventListener('click', (event) => {
    if (windNote.open && !windSummary.contains(event.target)) foldWind()
  })
  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape' || !windNote.open) return
    foldWind()
    windSummary.focus()
  }, { signal: live.signal })

  // The precedence rule lives in hintController; this is only the wiring from
  // its decision to the banner. textContent, never innerHTML.
  const hintCtl = hintController((text) => {
    hint.textContent = text
    hint.hidden = !text
  })

  // renderLegend clears the key, so the wind row is re-inserted after each repaint.
  const windRow = buildWindRow(cfg.t.legendWind)
  const seaRows = cfg.t.sea
    ? buildSeaRows(cfg.t.sea, cfg.seaColours ?? {}, () => infoDialog.show(seaInfoContent(cfg.t.sea.info)))
    : null
  const pollenRows = cfg.pollenLayer && cfg.t.pollen
    ? buildPollenRows(cfg.t.pollen, () => infoDialog.show(pollenInfoContent(cfg.t.pollen.info)))
    : null
  // The metric key's title, kept so switching pollen off can put it back on the folded pill.
  let metricTitle = ''
  const applyPollenLegend = () => {
    if (!pollenRows) return
    const on = !pollenRows.hidden
    setPollenLegend(legend, on, on ? cfg.t.pollen.legend : metricTitle)
  }
  // The caption describes the hex cells, which pollen replaces.
  const applyTierLine = () => {
    tierLine.hidden = !tierLine.textContent || (pollenRows ? !pollenRows.hidden : false)
  }
  const showLegend = ({ bands, tier, metric, scale }) => {
    if (scale) scaleDialog.prime(scale)
    metricTitle = legendTitle({
      label: cfg.metricLabels[metric],
      unit: cfg.metricUnits[metric],
      fallback: cfg.t.legend,
    })
    renderLegend(legend, {
      // Repainted with the bands, which is the only way it stays right: the
      // bands change with the metric, and so does the name of what they band.
      title: metricTitle,
      toggleLabel: cfg.t.legendToggle,
      ...legendRows(bands, {
        noDataColour: cfg.noDataColour,
        noDataLabel: cfg.t.legendNoData,
        lang: cfg.lang,
      }),
      // No scale, no (i): before the tables load, and for a metric none of them
      // claim, the dialog would open on nothing.
      info: scale ? { label: cfg.t.legendAbout, onOpen: () => scaleDialog.show(scale) } : null,
    })
    legend.insertBefore(windRow, legend.querySelector(':scope > .scale__info'))
    if (seaRows) legend.insertBefore(seaRows, legend.querySelector(':scope > .scale__info'))
    if (pollenRows) legend.insertBefore(pollenRows, legend.querySelector(':scope > .scale__info'))
    applyPollenLegend()
    // Bootstrap call: keep the server-rendered caption.
    if (tier === null) return
    const text = cfg.t.tier[tier] ?? ''
    tierLine.textContent = text
    applyTierLine()
  }

  // Drawn once at mount, before any scales have loaded, so the key is never an
  // empty overlay: with no bands that is the title and the no-data row, both of
  // which are true at that moment.
  showLegend({ bands: [], tier: null, metric: cfg.metric, scale: null })

  return {
    ...hintCtl,
    storage,
    showNote(text) {
      note.textContent = text
      note.hidden = !text
    },
    showLegend,
    foldWind,
    dock,
    zoomButtons: zoom.buttons,
    orient,
    windowMenu,
    player,
    layersUI: layers,
    layerViews,
    inactiveView,
    locateButton,
    sheet,
    // Called on movestart and when the replay bar opens.
    closeLegend,
    // Removes the media-query listener.
    dispose() { live.abort() },
    // Both halves move together: the disclosure is shown exactly when the
    // arrows are, so no caller can turn one on without the other.
    showSea(on) {
      if (seaRows) seaRows.hidden = !on
    },
    showPollen(on) {
      if (pollenRows) pollenRows.hidden = !on
      applyPollenLegend()
      applyTierLine()
    },
    showWind(on, text, mode = 'streaks') {
      setWindRow(windRow, { on, mode })
      windText.textContent = on ? text : ''
      windNote.hidden = !on || !text
      if (!on) windNote.open = false
    },
  }
}
