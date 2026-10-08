// @vitest-environment jsdom
//
// The columns on the column source must follow the time-lapse frame, not the pre-play grid.
import { describe, it, expect, vi } from 'vitest'
import { installTimelapse } from '../timelapse-island.js'
import { installHexRise } from '../hexrise.js'
import { mountPlayer } from '../timelapse.js'
import { HEX_SOURCE_ID, HEX_COLUMN_SOURCE_ID, PAINT_EVENT } from '../mapids.js'

const BODY = {
  metric: 'P2', resolution_km: 15, cells: [[23, 42]],
  frames: [
    { t: '2026-09-08T06:00:00Z', v: [10] },
    { t: '2026-09-08T07:00:00Z', v: [20] },
    { t: '2026-09-08T08:00:00Z', v: [30] },
  ],
}

function setup(pitch) {
  const container = Object.assign(new EventTarget(), { clientWidth: 800 })
  const hex = { setData: vi.fn() }
  const columns = { setData: vi.fn() }
  const map = {
    pitch,
    getPitch() { return this.pitch },
    getZoom: () => 12,
    getBounds: () => ({ getWest: () => 23, getSouth: () => 42, getEast: () => 24, getNorth: () => 43 }),
    getContainer: () => container,
    getLayer: () => ({}),
    getSource: (id) => (id === HEX_SOURCE_ID ? hex : id === HEX_COLUMN_SOURCE_ID ? columns : undefined),
    setFeatureState: vi.fn(),
    setPaintProperty: vi.fn(),
    on: (evt, fn) => { if (evt === 'pitch') map.onPitch = fn },
    off: () => {},
  }
  installHexRise(map, { raf: (fn) => fn(), reducedMotion: () => false })
  const ui = mountPlayer(document.createElement('div'), { label: 'Time', playLabel: 'Play', pauseLabel: 'Pause', exitLabel: 'Now', speedLabel: 'Speed' })
  installTimelapse(map, { scales: null, hexUrl: null, window: '24h' }, { metric: 'P2', noDataColour: '#999', lang: 'en' }, { player: ui }, async () => BODY)
  const scrubTo = (i) => {
    ui.slider.value = String(i)
    ui.slider.dispatchEvent(new Event('input'))
  }
  const tilt = (p) => { map.pitch = p; map.onPitch() }
  return { map, hex, columns, ui, scrubTo, tilt }
}

const lastColumnValues = (columns) => columns.setData.mock.lastCall[0].features.map((f) => f.properties.value)

async function opened(h) {
  h.ui.button.click()
  await vi.waitFor(() => expect(h.hex.setData).toHaveBeenCalled())
}

describe('playback and the column source', () => {
  it('refreshes the columns on every frame while tilted', async () => {
    const h = setup(50)
    await opened(h)
    h.columns.setData.mockClear()
    h.scrubTo(1)
    expect(lastColumnValues(h.columns)).toEqual([20])
    h.scrubTo(2)
    expect(lastColumnValues(h.columns)).toEqual([30])
  })

  it('leaves the column source alone while flat', async () => {
    const h = setup(0)
    await opened(h)
    h.scrubTo(1)
    h.scrubTo(2)
    expect(h.hex.setData).toHaveBeenCalledTimes(3)
    expect(h.columns.setData).not.toHaveBeenCalled()
  })

  it('shows the current frame, not a stale one, when tilted mid-playback', async () => {
    const h = setup(0)
    await opened(h)
    h.scrubTo(2)
    h.tilt(50)
    expect(lastColumnValues(h.columns)).toEqual([30])
  })

  it('hands the live grid back to the columns when playback stops', async () => {
    const h = setup(50)
    await opened(h)
    h.scrubTo(2)
    const seen = []
    h.map.getContainer().addEventListener(PAINT_EVENT, (e) => seen.push(e.detail.source))
    h.ui.exit.click()
    await vi.waitFor(() => expect(seen).toContain(HEX_SOURCE_ID))
  })
})
