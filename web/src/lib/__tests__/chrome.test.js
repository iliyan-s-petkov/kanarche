// @vitest-environment jsdom
//
// jsdom: mountChrome builds real DOM, which every test here drives directly.
import { readFileSync } from 'node:fs'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mountChrome, hintController, isPhoneViewport, PHONE_LANDSCAPE_QUERY } from '../chrome.js'
import { mountLayers, installLayers } from '../maplayers.js'
import { refreshHexes } from '../mapdata.js'
import { readConfig, LEGEND_FOLD_KEY } from '../mapconfig.js'
import { setSensorStatus, getSensorStatus, resetSensorFilterForTests } from '../sensorfilter.svelte.js'

const APP_CSS = '../internal/web/static/app.css'

// The cfg every mountChrome test hands in. metricLabels and metricUnits are
// keyed by metric because that is what readConfig produces (see byMetric): the
// key's caption is looked up by name, never by position.
const chromeCfg = (over = {}) => ({
  t: { tier: {} },
  noDataColour: '#999',
  lang: 'bg',
  metric: 'P2',
  metricLabels: { P1: 'ФПЧ10', P2: 'ФПЧ2.5' },
  metricUnits: { P1: 'µg/m³', P2: 'µg/m³' },
  ...over,
})

// Mounted in mountChrome and not in mount(), which is what puts it on both maps
// that carry this island — the home page and an area page — from one call.
describe('the averaging selector', () => {
  it('is mounted on the frame with the server-rendered words', () => {
    const el = document.createElement('div')
    el.dataset.tWindowLabel = 'Averaging period'
    el.dataset.tWindows = 'Now,Last 24 hours,Last 48 hours,Last week'
    document.body.appendChild(el)

    const { windowMenu } = mountChrome(el, readConfig(el))
    expect(windowMenu, 'no window menu in the chrome').toBeTruthy()
    expect(windowMenu.root.parentElement).toBe(el)
    expect(windowMenu.button.getAttribute('aria-label')).toBe('Averaging period')
    const radios = [...windowMenu.panel.querySelectorAll('input[type="radio"]')]
    expect(radios.map((r) => r.value)).toEqual(['', '24h', '48h', '7d'])
    expect(radios.map((r) => r.nextElementSibling.textContent))
      .toEqual(['Now', 'Last 24 hours', 'Last 48 hours', 'Last week'])
  })
})

// "Hide the basemap" must take down the ground and leave the readings standing.
// It used to walk every layer carrying an airbg:group — a marker the vector
// style set and the raster-only style cannot: the toggle reported itself on and
// hid nothing.
describe('the basemap toggle', () => {
  it('hides the ground, raster and vector detail alike, and no reading', () => {
    const el = document.createElement('div')
    document.body.appendChild(el)
    const { layerViews } = mountChrome(el, readConfig(el))
    const basemap = layerViews.find((v) => v.id === 'basemap')
    expect(basemap, 'no basemap view in the layers menu').toBeTruthy()

    const set = []
    const map = {
      setLayoutProperty: (...a) => set.push(a),
      // The ground is the raster PLUS the vector detail over it. Every layer
      // carrying an airbg:group came from the vector style; the readings carry
      // none, and must be left standing.
      getStyle: () => ({ layers: [
        { id: 'poi-shop', metadata: { 'airbg:group': 'poi-shop' } },
        { id: 'airbg-hex-fill' },
      ] }),
    }
    basemap.apply(false, map)

    expect(set).toEqual([
      ['airbg-raster-base', 'visibility', 'none'],
      ['poi-shop', 'visibility', 'none'],
    ])
  })
})

// cellValues starts ON on every viewport (wind is mapload.js's, covered in
// map.test.js); a stored choice always wins.
describe('mountChrome() defaults cellValues on, storage wins', () => {
  const stubMatchMedia = (matchesPhone) => {
    vi.stubGlobal('matchMedia', (query) => ({
      matches: matchesPhone && (query.includes('672px') || query === PHONE_LANDSCAPE_QUERY),
      media: query,
      addEventListener() {}, removeEventListener() {},
    }))
  }

  const cellValuesChecked = (stored) => {
    const store = new Map(stored ? [['kanarche:map-layers', JSON.stringify(stored)]] : [])
    const storage = {
      getItem: (k) => (store.has(k) ? store.get(k) : null),
      setItem: (k, v) => store.set(k, v),
    }
    const el = document.createElement('div')
    el.id = 'map'
    document.body.appendChild(el)
    const { layerViews } = mountChrome(el, readConfig(el))
    const cellValues = layerViews.find((v) => v.id === 'cellValues')

    const ui = mountLayers(document.createElement('div'), { label: 'Layers' })
    const map = {
      getStyle: () => ({ layers: [] }),
      getLayer: () => undefined,
      setLayerZoomRange: () => {},
    }
    installLayers(map, ui, { labels: {}, caption: 'c', views: [cellValues], storage })
    return ui.fieldset.querySelector('[data-layer-key="view:cellValues"]').checked
  }

  afterEach(() => { document.body.innerHTML = '' })

  it('phone, no stored value: on', () => {
    stubMatchMedia(true)
    expect(cellValuesChecked(null)).toBe(true)
  })

  it('phone, stored off: stays off', () => {
    stubMatchMedia(true)
    expect(cellValuesChecked({ 'view:cellValues': false })).toBe(false)
  })

  it('desktop, no stored value: on', () => {
    stubMatchMedia(false)
    expect(cellValuesChecked(null)).toBe(true)
  })

  it('desktop with no matchMedia at all: on', () => {
    vi.unstubAllGlobals()
    expect(cellValuesChecked(null)).toBe(true)
  })

  it('desktop, stored off: stays off', () => {
    stubMatchMedia(false)
    expect(cellValuesChecked({ 'view:cellValues': false })).toBe(false)
  })
})

// isPhoneViewport itself: portrait width alone, landscape query alone, neither.
describe('isPhoneViewport', () => {
  it('is false with no matchMedia (jsdom default)', () => {
    expect(isPhoneViewport()).toBe(false)
  })

  it('is true on the portrait query alone', () => {
    vi.stubGlobal('matchMedia', (q) => ({ matches: q.includes('672px'), media: q }))
    expect(isPhoneViewport()).toBe(true)
  })

  it('is true on the landscape query alone', () => {
    vi.stubGlobal('matchMedia', (q) => ({ matches: q === PHONE_LANDSCAPE_QUERY, media: q }))
    expect(isPhoneViewport()).toBe(true)
  })

  it('is false when neither matches', () => {
    vi.stubGlobal('matchMedia', (q) => ({ matches: false, media: q }))
    expect(isPhoneViewport()).toBe(false)
  })
})

describe('the bathing-water key', () => {
  it('shows with the layer and survives a legend repaint', () => {
    const el = document.createElement('div')
    el.className = 'map'
    el.dataset.tSeaLegend = 'Bathing water'
    el.dataset.seaColours = '#0b4f9c,#3a8fd9,#8cc5e8,#8e3a9c,#9ca3af'
    document.body.appendChild(el)
    const c = mountChrome(el, readConfig(el))
    const key = () => el.querySelector('.scale__sea')
    expect(key().hidden).toBe(true)
    c.showSea(true)
    c.showLegend({ bands: [], tier: null, metric: 'pm25', scale: null })
    expect(key().hidden).toBe(false)
    expect(key().textContent).toContain('Bathing water')
    c.showSea(false)
    expect(key().hidden).toBe(true)
  })
})

describe('the pollen key', () => {
  const mount = (layer) => {
    const el = document.createElement('div')
    el.className = 'map'
    if (layer) el.dataset.pollenLayer = 'true'
    el.dataset.tPollenLegend = 'Pollen forecast'
    el.dataset.tPollenLevelHigh = 'High'
    el.dataset.tLegend = 'Air quality'
    document.body.appendChild(el)
    return { el, c: mountChrome(el, readConfig(el)) }
  }

  it('shows with the layer and survives a legend repaint', () => {
    const { el, c } = mount(true)
    const key = () => el.querySelector('.scale__pollen')
    expect(key().hidden).toBe(true)
    c.showPollen(true)
    c.showLegend({ bands: [], tier: null, metric: 'pm25', scale: null })
    expect(key().hidden).toBe(false)
    expect(key().querySelectorAll('.legend__row')).toHaveLength(5)
    expect(key().textContent).toContain('High')
    c.showPollen(false)
    expect(key().hidden).toBe(true)
  })

  // The owner's rule: with pollen on, the key is pollen only; the metric's title, bar and no-data row go.
  it('hides the metric key while pollen is on, through repaints, and restores it when off', () => {
    const { el, c } = mount(true)
    const legend = el.querySelector('.scale--onmap')
    const bands = [{ upper: 15, colour: '#3c9', label: 'Good', label_bg: 'Добро' }, { upper: null, colour: '#c33', label: 'Poor', label_bg: 'Лошо' }]
    const metricParts = () => ['.scale__label', '.scale__bands', '.scale__none'].map((s) => legend.querySelector(`:scope > ${s}`))
    const toggleLabel = () => legend.querySelector('.scale__toggle-label').textContent
    c.showLegend({ bands, tier: null, metric: 'P2', scale: null })
    const metricTitle = toggleLabel()
    expect(metricTitle).toBe('Air quality')
    c.showPollen(true)
    expect(legend.classList.contains('scale--pollen')).toBe(true)
    for (const part of metricParts()) expect(part.hidden).toBe(true)
    expect(toggleLabel()).toBe('Pollen forecast')
    c.showLegend({ bands, tier: null, metric: 'P2', scale: null })
    for (const part of metricParts()) expect(part.hidden).toBe(true)
    expect(toggleLabel()).toBe('Pollen forecast')
    c.showPollen(false)
    expect(legend.classList.contains('scale--pollen')).toBe(false)
    for (const part of metricParts()) expect(part.hidden).toBe(false)
    expect(toggleLabel()).toBe(metricTitle)
  })

  it('is absent from a map that does not offer the layer', () => {
    const { el, c } = mount(false)
    c.showPollen(true)
    expect(el.querySelector('.scale__pollen')).toBeNull()
  })
})

// The disclosure is why an unmeasured forecast layer is allowed on a map of
// measurements, so it is never dismissible — but two sentences and a model name
// unrolled over the map is most of a phone screen. Folded, it is a line the
// reader can open.
describe('the wind disclosure', () => {
  // Scoped to its own frame: the disclosure is one of two <details> the chrome
  // builds, and a document-wide query would find whichever came first.
  const chrome = () => {
    const el = document.createElement('div')
    el.className = 'map'
    el.dataset.tWindAbout = 'About the wind layer'
    document.body.appendChild(el)
    return { el, ...mountChrome(el, readConfig(el)) }
  }

  it('arrives folded, with the full text inside it', () => {
    const c = chrome()
    c.showWind(true, 'Wind forecast · valid now')
    const note = c.el.querySelector('.map-wind-label')
    expect(note.tagName).toBe('DETAILS')
    expect(note.open).toBe(false)
    expect(note.hidden).toBe(false)
    expect(note.textContent).toContain('Wind forecast · valid now')
  })

  it('names itself on the summary, so a folded line still says what it is', () => {
    const c = chrome()
    c.showWind(true, 'Wind forecast · valid now')
    const summary = c.el.querySelector('.map-wind-label summary')
    expect(summary).toBeTruthy()
    expect(summary.textContent.trim()).not.toBe('')
  })

  it('carries the i18n text in a labelled span, so it can be hidden on phones without losing the (i)', () => {
    const c = chrome()
    c.showWind(true, 'Wind forecast · valid now')
    const label = c.el.querySelector('.map-wind-label summary .map-wind-label__text-label')
    expect(label).toBeTruthy()
    expect(label.textContent.trim()).toBe('About the wind layer')
  })

  it('goes away with the arrows, and comes back folded', () => {
    const c = chrome()
    c.showWind(true, 'Wind forecast · valid now')
    const note = c.el.querySelector('.map-wind-label')
    note.open = true
    c.showWind(false, '')
    expect(note.hidden).toBe(true)
    c.showWind(true, 'Wind forecast · valid now')
    expect(note.open).toBe(false)
  })

  // An opened note, its parts, and the toggle that mirrors it.
  const opened = () => {
    const c = chrome()
    c.showWind(true, 'Wind forecast · valid now')
    const note = c.el.querySelector('.map-wind-label')
    const summary = note.querySelector('summary')
    note.open = true
    // A browser fires toggle after open changes; jsdom's is queued, so send it.
    note.dispatchEvent(new Event('toggle'))
    return { c, note, summary, body: note.querySelector('.map-wind-label__text') }
  }
  const click = (target) => target.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }))

  it('folds on a click on its body', () => {
    const { note, body, summary } = opened()
    click(body)
    expect(note.open).toBe(false)
    expect(summary.getAttribute('aria-expanded')).toBe('false')
  })

  it('keeps the summary toggling: jsdom toggles on click, so two clicks fold then reopen', () => {
    const { note, summary } = opened()
    click(summary)
    expect(note.open).toBe(false)
    click(summary)
    expect(note.open).toBe(true)
  })

  it('lets a link in the body navigate while folding', () => {
    const { note, body } = opened()
    const link = document.createElement('a')
    link.href = 'https://open-meteo.com/'
    body.appendChild(link)
    const ev = new MouseEvent('click', { bubbles: true, cancelable: true })
    link.dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(false)
    expect(note.open).toBe(false)
  })

  it('folds when the map reports a click', () => {
    const { c, note } = opened()
    c.foldWind()
    expect(note.open).toBe(false)
  })

  it('folds on Escape and returns focus to the summary', () => {
    const { note, summary } = opened()
    document.body.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(note.open).toBe(false)
    expect(document.activeElement).toBe(summary)
  })

  it('leaves Escape alone while folded', () => {
    const { note, summary } = opened()
    note.open = false
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(document.activeElement).not.toBe(summary)
  })

  it('keeps aria-expanded in step with the open state', () => {
    const { note, summary } = opened()
    expect(summary.getAttribute('aria-expanded')).toBe('true')
    note.open = false
    note.dispatchEvent(new Event('toggle'))
    expect(summary.getAttribute('aria-expanded')).toBe('false')
  })
})

// A sensor that has stopped reporting still has a cell on the grid, drawn in
// the no-data colour. At country zoom that is most of what a reader sees on a
// bad day for the network, and it reads as "nothing here" rather than "nobody
// is measuring here". The status filter already governed the markers; the grid
// is the tier that actually covers the country, so it is governed too.
describe('the inactive-stations toggle', () => {
  const hexCfg = { metric: 'P2', noDataColour: '#cccccc' }
  const scales = [{ metric: 'P2', bands: [{ upper: 10, colour: '#00ff00' }, { upper: null, colour: '#ff0000' }] }]
  const mixed = {
    resolution_km: 1,
    hexes: [
      { lon: 23.32, lat: 42.65, n: 1, values: { P2: 5 } },
      { lon: 23.34, lat: 42.66, n: 0, values: {} },
    ],
  }

  function hexMap() {
    const painted = []
    return {
      painted,
      getZoom: () => 12,
      getBounds: () => ({ getWest: () => 23.3, getSouth: () => 42.6, getEast: () => 23.4, getNorth: () => 42.7 }),
      getSource: (id) => (id === 'airbg-hexes' ? { setData: (d) => painted.push(d) } : undefined),
    }
  }

  afterEach(() => resetSensorFilterForTests())

  it('leaves the silent cells off the grid by default', async () => {
    const map = hexMap()
    await refreshHexes(map, { scales, hexUrl: null, hexBody: null }, hexCfg, async () => mixed)

    const values = map.painted[0].features.map((f) => f.properties.value)
    expect(values).toEqual([5])
  })

  it('draws them once the reader asks for them', async () => {
    setSensorStatus('all')
    const map = hexMap()
    await refreshHexes(map, { scales, hexUrl: null, hexBody: null }, hexCfg, async () => mixed)

    const values = map.painted[0].features.map((f) => f.properties.value)
    expect(values).toHaveLength(2)
    expect(values).toContain(null)
  })

  it('offers the option unticked, and flips the shared status', () => {
    const el = document.createElement('div')
    document.body.appendChild(el)
    const { inactiveView: view, layerViews } = mountChrome(el, readConfig(el))
    expect(view, 'no inactive-stations view').toBeTruthy()
    // The id is what visitors' saved choices are keyed on.
    expect(view.id).toBe('inactiveSensors')
    // Placed by the map loader, next to the other station toggles.
    expect(layerViews.map((v) => v.id)).not.toContain('inactiveSensors')

    // Unticked on arrival: the default is the quieter map.
    expect(view.defaultOff).toBe(true)
    view.apply(true)
    expect(getSensorStatus()).toBe('all')
    view.apply(false)
    expect(getSensorStatus()).toBe('active')
  })
})

// Real fullscreen renders the frame and nothing else. The key is anchored to
// the shell — deliberately, so a wide window does not float it over the page —
// which meant going full screen took the colour key off the map, on the one
// view where the map is all there is.
describe('mountChrome() keeps the key on the map in fullscreen', () => {
  const chromeFrame = () => {
    const shell = document.createElement('div')
    shell.className = 'map-shell'
    const el = document.createElement('div')
    el.className = 'map'
    shell.appendChild(el)
    document.body.appendChild(shell)
    return { shell, el }
  }

  it('moves the key into the frame and back out again', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    expect(legend, 'no key on the shell').toBeTruthy()

    el.querySelector('.map__full').click()
    expect(legend.parentElement, 'the key stayed outside the fullscreen frame').toBe(el)

    el.querySelector('.map__full').click()
    expect(legend.parentElement).toBe(shell)
  })

  it('leaves it a details, so it can still be folded away', () => {
    const { el } = chromeFrame()
    mountChrome(el, readConfig(el))
    el.querySelector('.map__full').click()

    const legend = el.querySelector('details.scale')
    expect(legend.tagName).toBe('DETAILS')
    legend.open = true
    expect(legend.open, 'the key cannot be unfolded').toBe(true)
  })
})

// The fold and the layers-menu option are two different controls: the menu says
// whether there is a key at all, the triangle says whether it is unrolled. A
// fold that forgets is the one that reads as broken — the reader folds the key
// away, reloads, and it is back over the map.
describe('mountChrome() remembers whether the key is folded', () => {
  const chromeFrame = () => {
    const shell = document.createElement('div')
    shell.className = 'map-shell'
    const el = document.createElement('div')
    el.className = 'map'
    shell.appendChild(el)
    document.body.appendChild(shell)
    return { shell, el }
  }

  // This jsdom has no localStorage of its own, so the seam is stubbed rather
  // than cleared — which also proves the code reaches for the real one.
  let store
  beforeEach(() => {
    store = new Map()
    vi.stubGlobal('localStorage', {
      getItem: (k) => (store.has(k) ? store.get(k) : null),
      setItem: (k, v) => store.set(k, String(v)),
      removeItem: (k) => store.delete(k),
    })
  })
  // Left stubbed on purpose: vi.unstubAllGlobals would also drop the fetch stub
  // the later suites install, and an empty store reads exactly like the absent
  // localStorage this jsdom otherwise has.

  it('folds the key on a first visit', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    expect(shell.querySelector('details.scale').open).toBe(false)
  })

  it('records the fold when the reader closes it', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')

    legend.open = false
    legend.dispatchEvent(new Event('toggle'))

    expect(store.get(LEGEND_FOLD_KEY)).toBe('false')
  })

  it('opens folded on the next visit, and records the reopening', () => {
    store.set(LEGEND_FOLD_KEY, 'false')
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    expect(legend.open, 'the key ignored the remembered fold').toBe(false)

    legend.open = true
    legend.dispatchEvent(new Event('toggle'))
    expect(store.get(LEGEND_FOLD_KEY)).toBe('true')
  })
})

// A stubbed matchMedia stands in for the 672px breakpoint, since jsdom has
// none of its own. The key starts folded on phone and desktop alike.
describe('mountChrome() folds the key by default on every viewport', () => {
  const chromeFrame = () => {
    const shell = document.createElement('div')
    shell.className = 'map-shell'
    const el = document.createElement('div')
    el.className = 'map'
    shell.appendChild(el)
    document.body.appendChild(shell)
    return { shell, el }
  }

  let store
  beforeEach(() => {
    store = new Map()
    vi.stubGlobal('localStorage', {
      getItem: (k) => (store.has(k) ? store.get(k) : null),
      setItem: (k, v) => store.set(k, String(v)),
      removeItem: (k) => store.delete(k),
    })
    vi.stubGlobal('matchMedia', (query) => ({
      matches: query.includes('672px'), media: query,
      addEventListener() {}, removeEventListener() {},
    }))
  })

  it('mounts folded on a phone with no stored flag', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    expect(shell.querySelector('details.scale').open).toBe(false)
  })

  it('mounts folded on desktop (no matchMedia) with no stored flag', () => {
    vi.unstubAllGlobals()
    vi.stubGlobal('localStorage', {
      getItem: (k) => (store.has(k) ? store.get(k) : null),
      setItem: (k, v) => store.set(k, String(v)),
      removeItem: (k) => store.delete(k),
    })
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    expect(shell.querySelector('details.scale').open).toBe(false)
  })

  it('mounts folded on a phone when a reader stored false', () => {
    store.set(LEGEND_FOLD_KEY, 'false')
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    expect(shell.querySelector('details.scale').open).toBe(false)
  })

  it('mounts open when a reader stored true', () => {
    store.set(LEGEND_FOLD_KEY, 'true')
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    expect(shell.querySelector('details.scale').open).toBe(true)
  })

  it('mounts folded on a landscape phone with no stored flag', () => {
    vi.stubGlobal('matchMedia', (query) => ({
      matches: query === PHONE_LANDSCAPE_QUERY, media: query,
      addEventListener() {}, removeEventListener() {},
    }))
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    expect(shell.querySelector('details.scale').open).toBe(false)
  })

  it('still opens on a landscape phone when a reader stored true', () => {
    store.set(LEGEND_FOLD_KEY, 'true')
    vi.stubGlobal('matchMedia', (query) => ({
      matches: query === PHONE_LANDSCAPE_QUERY, media: query,
      addEventListener() {}, removeEventListener() {},
    }))
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    expect(shell.querySelector('details.scale').open).toBe(true)
  })

  it('an embed folds the key like any page, and a stored true still opens it', () => {
    document.body.classList.add('embed')
    try {
      const a = chromeFrame()
      mountChrome(a.el, readConfig(a.el))
      expect(a.shell.querySelector('details.scale').open).toBe(false)
      document.body.innerHTML = ''
      store.set(LEGEND_FOLD_KEY, 'true')
      const b = chromeFrame()
      mountChrome(b.el, readConfig(b.el))
      expect(b.shell.querySelector('details.scale').open).toBe(true)
    } finally { document.body.classList.remove('embed') }
  })

  // The open attribution card covers the key's corner on a phone.
  const openAttribution = async (el) => {
    const attrib = document.createElement('div')
    attrib.className = 'maplibregl-ctrl-attrib'
    const button = document.createElement('button')
    button.className = 'maplibregl-ctrl-attrib-button'
    attrib.appendChild(button)
    el.appendChild(attrib)
    // MapLibre's own handler opens the card, then the click bubbles to the chrome.
    button.addEventListener('click', () => attrib.classList.add('maplibregl-compact-show'))
    button.click()
    await new Promise((r) => setTimeout(r, 0))
    return attrib
  }

  // MapLibre opens the card by itself on load; that must not fold the key.
  it('the card opening on its own (no click) leaves the key open', async () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    shell.querySelector('details.scale').open = true
    const attrib = document.createElement('div')
    attrib.className = 'maplibregl-ctrl-attrib maplibregl-compact-show'
    el.appendChild(attrib)
    await new Promise((r) => setTimeout(r, 0))
    expect(shell.querySelector('details.scale').open).toBe(true)
  })

  it('opening the attribution card folds the key on a phone without persisting', async () => {
    store.set(LEGEND_FOLD_KEY, 'true')
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    expect(legend.open).toBe(true)

    await openAttribution(el)

    expect(legend.open).toBe(false)
    expect(store.get(LEGEND_FOLD_KEY)).toBe('true')
  })

  it('opening the attribution card leaves the key open on desktop', async () => {
    vi.stubGlobal('matchMedia', (query) => ({ matches: false, media: query, addEventListener() {}, removeEventListener() {} }))
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    shell.querySelector('details.scale').open = true
    await openAttribution(el)
    expect(shell.querySelector('details.scale').open).toBe(true)
  })

  // The map itself dispatches movestart; chrome only exposes closeLegend for
  // whoever holds the map instance (see islands/map.js).
  it('closeLegend closes an open key without writing the fold flag', () => {
    store.set(LEGEND_FOLD_KEY, 'true')
    const { shell, el } = chromeFrame()
    const chrome = mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    expect(legend.open).toBe(true)

    chrome.closeLegend()
    legend.dispatchEvent(new Event('toggle'))

    expect(legend.open).toBe(false)
    expect(store.get(LEGEND_FOLD_KEY), 'auto-close must not persist').toBe('true')
  })

  // The transport bar unfolds into the corner the open key covers. Folded, not
  // hidden: the reader can unroll it again while the animation runs.
  it('folds the key when the transport bar opens, and leaves it in the DOM', () => {
    store.set(LEGEND_FOLD_KEY, 'true')
    const { shell, el } = chromeFrame()
    const chrome = mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    expect(legend.open).toBe(true)

    chrome.player.show(3)
    legend.dispatchEvent(new Event('toggle'))

    expect(legend.open).toBe(false)
    expect(legend.isConnected).toBe(true)
    expect(store.get(LEGEND_FOLD_KEY), 'auto-close must not persist').toBe('true')
  })

  it('leaves the key alone when the bar collapses', () => {
    store.set(LEGEND_FOLD_KEY, 'true')
    const { shell, el } = chromeFrame()
    const chrome = mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')

    chrome.player.show(0)

    expect(legend.open).toBe(true)
  })
})

// The refresh pill and the window button are two overlays in the same corner.
// A phone has room for one, so the pill moves into the window panel — and back
// out again when the reader turns the phone, which is a media-query change
// rather than a new page load.
describe('mountChrome() hosts the refresh controls in the window panel on a phone', () => {
  let listeners
  let matches

  const query = () => ({
    get matches() { return matches },
    media: '(max-width: 672px)',
    // Honours the abort signal, as a real MediaQueryList does — otherwise
    // dispose() could not be told apart from a listener that never fired.
    addEventListener: (type, fn, opts) => {
      if (type !== 'change') return
      listeners.push(fn)
      opts?.signal?.addEventListener('abort', () => {
        const i = listeners.indexOf(fn)
        if (i >= 0) listeners.splice(i, 1)
      })
    },
    removeEventListener() {},
  })

  const flipTo = (yes) => {
    matches = yes
    for (const fn of listeners) fn({ matches: yes })
  }

  const freshFrame = () => {
    const shell = document.createElement('div')
    shell.className = 'map-shell'
    const el = document.createElement('div')
    el.className = 'map'
    const fresh = document.createElement('div')
    fresh.className = 'map-freshness'
    // The island host, as the templates render it: Svelte fills it in later,
    // so moving the host is what keeps the move race-free on a real page.
    const island = document.createElement('div')
    island.dataset.island = 'freshness'
    const refresh = document.createElement('p')
    refresh.className = 'data-refresh'
    island.appendChild(refresh)
    fresh.appendChild(island)
    shell.append(el, fresh)
    document.body.appendChild(shell)
    return { shell, el, fresh, island, refresh }
  }

  beforeEach(() => {
    listeners = []
    matches = true
    vi.stubGlobal('matchMedia', query)
  })

  it('moves the refresh controls into the panel at mount', () => {
    const { el, fresh, refresh } = freshFrame()
    const { windowMenu } = mountChrome(el, readConfig(el))
    expect(windowMenu.panel.contains(refresh)).toBe(true)
    expect(refresh.closest('.map-window__footer')).toBe(windowMenu.footer)
    // Still under .map-freshness — the whole menu is — but no longer a child
    // of it, which is what the corner row lays out.
    expect([...fresh.children].some((c) => c.contains(refresh) && c !== windowMenu.root)).toBe(false)
  })

  it('puts them back when the query stops matching, and takes them again when it does', () => {
    const { el, fresh, island, refresh } = freshFrame()
    const { windowMenu } = mountChrome(el, readConfig(el))

    flipTo(false)
    expect(windowMenu.panel.contains(refresh)).toBe(false)
    // Ahead of the window button and the player, which were appended after it.
    expect(fresh.firstElementChild).toBe(island)

    flipTo(true)
    expect(windowMenu.panel.contains(refresh)).toBe(true)
  })

  it('drops the listener on dispose, so a later flip moves nothing', () => {
    const { el, fresh, island } = freshFrame()
    const chrome = mountChrome(el, readConfig(el))
    chrome.dispose()

    flipTo(false)

    expect(chrome.windowMenu.footer.contains(island)).toBe(true)
    expect(fresh.firstElementChild).not.toBe(island)
  })

  it('leaves the pill in the corner on a desktop width', () => {
    matches = false
    const { el, fresh, island } = freshFrame()
    const { windowMenu } = mountChrome(el, readConfig(el))
    expect(island.parentElement).toBe(fresh)
    expect(windowMenu.footer.children).toHaveLength(0)
  })
})

// Where the key and the tier line LAND is load-bearing, not decoration, and
// both defects it guards were found in a browser rather than here.
//
// The key is absolutely positioned against .map-shell. Put it inside #map and
// the kit's phone rule — which turns it static so it sits UNDER the map below
// 672px — leaves it under the map but still inside it, over the canvas corner.
// Put the tier paragraph inside the shell and the shell grows taller than the
// map, so the key's inset-block-end:16px is measured from a bottom edge 16px
// below the map's own: measured live at -16px before this.
describe('mountChrome anchors the key to the shell and the tier line outside it', () => {
  const chrome = () => {
    const shell = document.createElement('div')
    shell.className = 'map-shell'
    const el = document.createElement('div')
    el.className = 'map map--hero'
    shell.appendChild(el)
    const host = document.createElement('div')
    host.append(shell)
    mountChrome(el, chromeCfg())
    return { shell, el, host }
  }

  it('puts the key in the shell, not in the map', () => {
    const { shell, el } = chrome()
    expect(el.querySelector('.scale--onmap')).toBeNull()
    expect(shell.querySelector(':scope > .scale--onmap')).not.toBeNull()
  })

  it('puts the tier line after the shell, so the shell stays the map box', () => {
    const { shell, host } = chrome()
    expect(shell.querySelector('.map-tier')).toBeNull()
    expect(host.lastElementChild.className).toContain('map-tier')
  })

  // The banners stay children of the map: they are messages about the map and
  // they do not have the phone rule the key has.
  it('leaves the hint and note inside the map', () => {
    const { el } = chrome()
    expect(el.querySelector('.map-hint')).not.toBeNull()
    expect(el.querySelector('.map-note')).not.toBeNull()
  })

  // Without a shell the key still has to render. It anchors to the map instead,
  // which loses the phone layout but shows a key rather than throwing.
  it('falls back to the map when no shell wraps it', () => {
    const el = document.createElement('div')
    document.createElement('div').appendChild(el)
    mountChrome(el, chromeCfg())
    expect(el.querySelector('.scale--onmap')).not.toBeNull()
  })

  // OpenProject #609: the server renders this element (PageData.InitialTier)
  // so it paints with the HTML rather than waiting on the map bundle for LCP.
  // mountChrome must reuse that node, not insert a second one beside it —
  // two would mean the LCP element Lighthouse measured is not the one the
  // island goes on to update.
  it('adopts a server-rendered tier line instead of creating a new one', () => {
    const shell = document.createElement('div')
    shell.className = 'map-shell'
    const el = document.createElement('div')
    el.className = 'map map--hero'
    shell.appendChild(el)
    const ssrTier = document.createElement('p')
    ssrTier.className = 'legend__tier map-tier'
    ssrTier.textContent = 'Всяка клетка е медиана за площта под нея'
    const host = document.createElement('div')
    host.append(shell, ssrTier)

    mountChrome(el, chromeCfg())

    expect(host.querySelectorAll('.map-tier')).toHaveLength(1)
    expect(host.querySelector('.map-tier')).toBe(ssrTier)
  })

  // The mount-time bootstrap draw (tier: null, before any real tier is known)
  // must not blank the server-rendered text — that would be exactly the flash
  // of empty content SSR was meant to avoid.
  it('leaves the server-rendered tier text alone until a real tier arrives', () => {
    const shell = document.createElement('div')
    shell.className = 'map-shell'
    const el = document.createElement('div')
    el.className = 'map map--hero'
    shell.appendChild(el)
    const ssrTier = document.createElement('p')
    ssrTier.className = 'legend__tier map-tier'
    ssrTier.textContent = 'Всяка клетка е медиана за площта под нея'
    const host = document.createElement('div')
    host.append(shell, ssrTier)

    mountChrome(el, chromeCfg())

    expect(ssrTier.textContent).toBe('Всяка клетка е медиана за площта под нея')
    expect(ssrTier.hidden).toBe(false)
  })
})

// The key's caption is the metric and its unit, not a fixed phrase. "Качество
// на въздуха" is simply false when the map is painting temperature, and it is
// the same words for every metric — so it says nothing about which one is on
// screen. The unit comes from the server-rendered catalogue rather than
// /api/v1/scales, which the key must caption without waiting on.
describe('the key names the metric it is a key to', () => {
  const captionOf = (cfg) => {
    const el = document.createElement('div')
    document.createElement('div').appendChild(el)
    return { el, chrome: mountChrome(el, cfg) }
  }

  it('composes the name and the unit on the first paint', () => {
    const { el } = captionOf(chromeCfg())
    expect(el.querySelector('.scale__label').textContent).toBe('ФПЧ2.5, µg/m³')
  })

  // cfg.metric is already the new metric by the time onMetricChange calls
  // refresh, and refresh passes it through — so the caption follows the
  // switcher without the key subscribing to anything.
  it('follows the metric switch', () => {
    const { el, chrome } = captionOf(chromeCfg())
    chrome.showLegend({ bands: [], tier: null, metric: 'P1' })
    expect(el.querySelector('.scale__label').textContent).toBe('ФПЧ10, µg/m³')
  })

  // A metric the catalogue has no unit for still gets its name.
  it('drops to the name alone when the metric has no unit', () => {
    const cfg = chromeCfg({ metricUnits: { P1: '', P2: '' } })
    const { el } = captionOf(cfg)
    expect(el.querySelector('.scale__label').textContent).toBe('ФПЧ2.5')
  })

  // Only a metric with no name at all falls back, because a caption reading
  // just "µg/m³" would name nothing.
  it('falls back to the generic title when the metric has no name', () => {
    const cfg = chromeCfg({ metricLabels: {}, t: { tier: {}, legend: 'Качество на въздуха' } })
    const { el } = captionOf(cfg)
    expect(el.querySelector('.scale__label').textContent).toBe('Качество на въздуха')
  })
})

// The map is a canvas, so the banner is the only part of its running commentary
// a screen reader can reach. Without aria-live the text changes silently.
describe('mountChrome() announces the hint banner', () => {
  it('marks it as a polite live region', () => {
    const shell = document.createElement('div')
    shell.className = 'map-shell'
    const el = document.createElement('div')
    el.className = 'map'
    shell.appendChild(el)
    document.body.appendChild(shell)

    mountChrome(el, readConfig(el))
    expect(el.querySelector('.map-hint')?.getAttribute('aria-live')).toBe('polite')
  })
})

// hintController is the precedence rule: an error outranks the routine tier
// hint permanently. `render` is the only side effect, so these drive the real
// rule with an array as the sink — no DOM, and no second implementation that
// could disagree with the one the page runs.
describe('hintController', () => {
  it('shows and clears the routine hint while no error is outstanding', () => {
    const rendered = []
    const c = hintController((t) => rendered.push(t))

    c.showHint('Select an area')
    c.showHint('')

    expect(rendered).toEqual(['Select an area', ''])
  })

  it('refuses to let a later showHint erase an error', () => {
    const rendered = []
    const c = hintController((t) => rendered.push(t))

    c.showError('Map data is unavailable right now')
    c.showHint('')
    c.showHint('Select an area')

    expect(rendered).toEqual(['Map data is unavailable right now'])
  })
})

// #579: the legend and the layers list are two on-map popovers that collide on
// a phone. Opening one folds the other, and closing the list restores the
// legend only if the list is what folded it — a reader's own fold must stick.
describe('mountChrome() keeps the legend and the layers list mutually exclusive', () => {
  const chromeFrame = () => {
    const shell = document.createElement('div')
    shell.className = 'map-shell'
    const el = document.createElement('div')
    el.className = 'map'
    shell.appendChild(el)
    document.body.appendChild(shell)
    return { shell, el }
  }

  beforeEach(() => { localStorage.removeItem(LEGEND_FOLD_KEY) })
  afterEach(() => { document.body.innerHTML = '' })

  // jsdom never fires toggle on a programmatic .open write, unlike a real
  // browser's queued task — dispatch it by hand so the guard is actually
  // exercised, not just the synchronous .open flip.
  it('opening the layers list folds an open legend, without persisting the fold', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    const layersBtn = el.querySelector('.map__layers .colmenu__btn')
    legend.open = true

    layersBtn.click()
    legend.dispatchEvent(new Event('toggle'))

    expect(legend.open).toBe(false)
    expect(localStorage.getItem(LEGEND_FOLD_KEY)).toBeNull()
  })

  it('closing the layers list re-opens a legend it folded', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    const layersBtn = el.querySelector('.map__layers .colmenu__btn')
    legend.open = true

    layersBtn.click()
    legend.dispatchEvent(new Event('toggle'))
    expect(legend.open).toBe(false)
    layersBtn.click()
    legend.dispatchEvent(new Event('toggle'))

    expect(legend.open).toBe(true)
    expect(localStorage.getItem(LEGEND_FOLD_KEY)).toBeNull()
  })

  it('does not re-open the legend on close when it was already folded', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    legend.open = false
    legend.dispatchEvent(new Event('toggle'))
    const layersBtn = el.querySelector('.map__layers .colmenu__btn')

    layersBtn.click()
    layersBtn.click()

    expect(legend.open).toBe(false)
  })

  it('opening the legend closes an open layers list', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    const layersBtn = el.querySelector('.map__layers .colmenu__btn')
    legend.open = false
    legend.dispatchEvent(new Event('toggle'))

    layersBtn.click()
    expect(layersBtn.getAttribute('aria-expanded')).toBe('true')

    legend.open = true
    legend.dispatchEvent(new Event('toggle'))

    expect(layersBtn.getAttribute('aria-expanded')).toBe('false')
  })
})

// #580: the disclosure triangle is a hard target on a phone; the whole open
// legend body folds it instead, except its interactive children.
describe('mountChrome() folds the open legend on a tap anywhere inside it', () => {
  const chromeFrame = () => {
    const shell = document.createElement('div')
    shell.className = 'map-shell'
    const el = document.createElement('div')
    el.className = 'map'
    shell.appendChild(el)
    document.body.appendChild(shell)
    return { shell, el }
  }

  afterEach(() => { document.body.innerHTML = '' })

  it('folds on a click on the legend body', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    expect(legend.open).toBe(true)

    legend.querySelector('.scale__label').dispatchEvent(new MouseEvent('click', { bubbles: true }))

    expect(legend.open).toBe(false)
  })

  it('leaves the info button to open the scale dialog instead of folding', () => {
    const { shell, el } = chromeFrame()
    const chrome = mountChrome(el, readConfig(el))
    chrome.showLegend({ bands: [], tier: null, metric: 'P2', scale: { bands: [] } })
    const legend = shell.querySelector('details.scale')
    const info = legend.querySelector('.scale__info')
    expect(info, 'no info button rendered').toBeTruthy()
    // jsdom does not implement showModal in every version the project targets.
    el.querySelector('dialog').showModal = vi.fn(function () { this.open = true })

    info.dispatchEvent(new MouseEvent('click', { bubbles: true }))

    expect(legend.open, 'the info button folded the legend').toBe(true)
  })

  it('leaves a link inside the legend to navigate rather than fold', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    const link = document.createElement('a')
    link.href = '#somewhere'
    legend.appendChild(link)

    let followed = false
    link.addEventListener('click', (e) => { e.preventDefault(); followed = true })
    link.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }))

    expect(followed, 'the link never received the click').toBe(true)
    expect(legend.open, 'the link click folded the legend instead of following it').toBe(true)
  })

  it('ignores a click whose target is the summary (native toggle owns it)', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    const summary = legend.querySelector(':scope > .scale__toggle')

    // Simulates the summary's own click, with no state change from OUR
    // handler: closest('summary') must short-circuit before touching .open.
    const event = new MouseEvent('click', { bubbles: true, cancelable: true })
    Object.defineProperty(event, 'target', { value: summary })
    legend.dispatchEvent(event)

    expect(legend.open, 'the click-anywhere handler acted on a summary target').toBe(true)
  })

  it('Escape folds the open legend and returns focus to the toggle', () => {
    const { shell, el } = chromeFrame()
    mountChrome(el, readConfig(el))
    const legend = shell.querySelector('details.scale')
    document.body.appendChild(legend)

    legend.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))

    expect(legend.open).toBe(false)
    expect(document.activeElement).toBe(legend.querySelector(':scope > .scale__toggle'))
  })
})

// Drift guard: app.css's landscape breakpoint must match chrome.js's
// PHONE_LANDSCAPE_QUERY, or the two disagree on what a landscape phone is.
describe('PHONE_LANDSCAPE_QUERY matches app.css', () => {
  it('equals the @media condition app.css gates its landscape block on', () => {
    const css = readFileSync(APP_CSS, 'utf8')
    const match = css.match(/@media (\([^{]+?\)) \{\n\s*\.map-shell:has\(\.map--hero\)/)
    expect(match).not.toBeNull()
    expect(match[1]).toBe(PHONE_LANDSCAPE_QUERY)
  })
})
