import { describe, it, expect, vi } from 'vitest'
import { paintSource } from '../mapdata.js'
import { installHexRise } from '../hexrise.js'
import { HEX_SOURCE_ID, HEX_COLUMN_SOURCE_ID, PAINT_EVENT } from '../mapids.js'

// Pins the literal name: the e2e specs spell it out, so a rename must reach them too.
const EVENT_NAME = 'kanarche:paint'

// A map with just enough surface for installHexRise, plus a real container for the event.
function hexMap() {
  const handlers = {}
  const columns = { setData: vi.fn() }
  return {
    pitch: 0,
    columns,
    container: new EventTarget(),
    setFeatureState: vi.fn(),
    setPaintProperty: vi.fn(),
    getPitch() { return this.pitch },
    on: (type, fn) => { (handlers[type] ??= []).push(fn) },
    fire(type) { for (const fn of handlers[type] ?? []) fn() },
    getContainer() { return this.container },
    getLayer: () => ({}),
    getSource(id) { return id === HEX_COLUMN_SOURCE_ID ? columns : undefined },
  }
}

describe('paint event wiring', () => {
  it('is named kanarche:paint', () => {
    expect(PAINT_EVENT).toBe(EVENT_NAME)
  })

  it('paintSource announces each repaint on the container under that name', () => {
    const container = new EventTarget()
    const seen = []
    container.addEventListener(EVENT_NAME, (e) => seen.push(e.detail))
    const map = { getSource: () => ({ setData: vi.fn() }), getContainer: () => container }
    const features = [{ id: 7 }]
    paintSource(map, HEX_SOURCE_ID, features)
    expect(seen).toEqual([{ source: HEX_SOURCE_ID, features }])
  })

  it('installHexRise reacts to a paintSource repaint of the hex grid', () => {
    const map = hexMap()
    installHexRise(map, { raf: (fn) => fn(), reducedMotion: () => false })
    paintSource(map, HEX_SOURCE_ID, [{ id: 1, geometry: { type: 'Polygon' } }])
    map.pitch = 50
    map.fire('pitch')
    expect(map.setFeatureState).toHaveBeenCalledWith({ source: HEX_COLUMN_SOURCE_ID, id: 1 }, { rise: 1 })
  })
})
