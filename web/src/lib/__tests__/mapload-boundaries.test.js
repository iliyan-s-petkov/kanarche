import { describe, it, expect, vi, beforeEach } from 'vitest'

const h = vi.hoisted(() => ({
  views: [],
  setBoundaries: vi.fn(async () => true),
  openInit: null,
  release: null,
}))

vi.mock('../maplayers.js', () => ({ installLayers: (_m, _ui, o) => { h.views = o.views } }))
vi.mock('../mapboundaries.js', () => ({ setBoundaries: h.setBoundaries }))
// initData stands for the first data paint: it settles when the test says so.
vi.mock('../mapdata.js', () => ({
  refresh: vi.fn(), refreshHexes: vi.fn(), onMetricChange: vi.fn(), applyMetricColours: vi.fn(),
  mapHint: vi.fn(), repaintSensors: vi.fn(), setSourceViewAvailability: vi.fn(), mapInlineSize: () => 400,
  initData: () => new Promise((resolve) => { h.openInit = resolve }),
}))
vi.mock('../idle.js', () => ({ whenRenderSettled: () => Promise.resolve() }))
vi.mock('../placement.js', () => ({
  locateVisitor: vi.fn(), openDeepLinkedSensor: vi.fn(), placeVisitor: vi.fn(), prefetchPlacement: vi.fn(), LOCATE_TIMEOUT_MS: 1,
}))
vi.mock('../mapstyle.js', () => ({ addBasemapOverlay: vi.fn() }))
vi.mock('../mapwind-lazy.js', () => ({ setWind: vi.fn(async () => true), refreshWind: vi.fn() }))
vi.mock('../timelapse-lazy.js', () => ({ installTimelapseLazy: () => ({}) }))
vi.mock('../hexrise.js', () => ({ hexExtrusionPaint: () => ({}), installHexRise: vi.fn() }))
vi.mock('../lastview.js', () => ({ restoreLastView: vi.fn() }))
vi.mock('../favourite.js', () => ({ openFavouriteSensor: vi.fn() }))
vi.mock('../markericon.js', async (orig) => ({ ...(await orig()), diamondImage: () => ({}), squareImage: () => ({}) }))
vi.mock('../../islands/windarrow.js', () => ({
  WIND_SOURCE_ID: 'w', WIND_LAYER_ID: 'wl', ARROW_IMAGE_ID: 'wi', arrowImage: () => ({}), arrowLayout: () => ({}), arrowPaint: () => ({}),
}))

import { installMapLoad } from '../mapload.js'

function fakeMap() {
  const handlers = {}
  return {
    on: (ev, fn) => { handlers[ev] = fn },
    addSource: vi.fn(), addLayer: vi.fn(), addImage: vi.fn(), setLayoutProperty: vi.fn(), getLayer: vi.fn(),
    fire: (ev) => handlers[ev](),
  }
}

async function boot(slug) {
  const map = fakeMap()
  const cfg = { slug, t: { pollen: {}, sea: {}, layers: {} }, pollenLayer: false, metrics: [], metric: 'p' }
  installMapLoad({
    map, state: { slug }, cfg, chrome: { layerViews: [], inactiveView: {}, layersUI: {}, windowMenu: { onpick: vi.fn() }, showHint: vi.fn() },
    vs: { metric: 'p', onMetricChange: vi.fn() }, windState: {}, boundaryState: {}, onMoveEnd: { cancel: vi.fn() }, subs: {},
  })
  const done = map.fire('load')
  await vi.waitFor(() => expect(h.views.length).toBeGreaterThan(0))
  return { done }
}

const boundaryView = () => h.views.find((v) => v.id === 'boundaries')

describe('installMapLoad outlines', () => {
  beforeEach(() => {
    h.views = []
    h.setBoundaries.mockClear()
  })

  it('home map: outlines wait for the first data paint, then apply', async () => {
    const { done } = await boot('')
    expect(boundaryView().apply(true)).toBe(true)
    await new Promise((r) => setTimeout(r, 10))
    expect(h.setBoundaries).not.toHaveBeenCalled()
    h.openInit()
    await done
    await vi.waitFor(() => expect(h.setBoundaries).toHaveBeenCalledTimes(1))
  })

  it('area page: outlines apply at once', async () => {
    const { done } = await boot('sofia')
    boundaryView().apply(true)
    expect(h.setBoundaries).toHaveBeenCalledTimes(1)
    h.openInit()
    await done
  })
})
