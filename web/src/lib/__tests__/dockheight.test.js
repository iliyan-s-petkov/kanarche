// The bottom panel's draggable height (OpenProject #697): bounds, clamping, keys and the saved choice.
import { describe, it, expect } from 'vitest'
import { HEIGHT_KEY, heightBounds, defaultHeight, clampHeight, keyHeight, readHeight, writeHeight, clearHeight } from '../dockheight.js'

function memory(init = {}) {
  const map = new Map(Object.entries(init))
  return {
    map,
    getItem: (k) => (map.has(k) ? map.get(k) : null),
    setItem: (k, v) => map.set(k, String(v)),
    removeItem: (k) => map.delete(k),
  }
}

describe('heightBounds', () => {
  it('floors at 14rem and leaves 8rem of map above the panel', () => {
    expect(heightBounds(900, 16)).toEqual({ min: 224, max: 772 })
  })

  it('never lets the max fall under the floor on a short map', () => {
    expect(heightBounds(300, 16)).toEqual({ min: 224, max: 224 })
  })

  it('scales with the root font size', () => {
    expect(heightBounds(900, 20)).toEqual({ min: 280, max: 740 })
  })
})

describe('defaultHeight', () => {
  it('matches the CSS default: 45% of the map, capped at 24rem, floored at 14rem', () => {
    expect(defaultHeight(900, 16)).toBe(384)
    expect(defaultHeight(700, 16)).toBe(315)
    expect(defaultHeight(403, 16)).toBe(224)
  })
})

describe('clampHeight', () => {
  const b = { min: 224, max: 600 }
  it('keeps a height inside the bounds', () => {
    expect(clampHeight(300, b)).toBe(300)
    expect(clampHeight(100, b)).toBe(224)
    expect(clampHeight(900, b)).toBe(600)
  })

  it('rounds to whole pixels', () => {
    expect(clampHeight(300.6, b)).toBe(301)
  })
})

describe('keyHeight', () => {
  const b = { min: 224, max: 600 }
  it('steps with the arrows and jumps with Home and End', () => {
    expect(keyHeight('ArrowUp', 300, b)).toBe(316)
    expect(keyHeight('ArrowDown', 300, b)).toBe(284)
    expect(keyHeight('Home', 300, b)).toBe(224)
    expect(keyHeight('End', 300, b)).toBe(600)
  })

  it('stays inside the bounds at either end', () => {
    expect(keyHeight('ArrowUp', 595, b)).toBe(600)
    expect(keyHeight('ArrowDown', 230, b)).toBe(224)
  })

  it('ignores any other key', () => {
    expect(keyHeight('Enter', 300, b)).toBeNull()
    expect(keyHeight('ArrowLeft', 300, b)).toBeNull()
  })
})

describe('the saved height', () => {
  it('uses a kanarche: key', () => {
    expect(HEIGHT_KEY).toBe('kanarche:panel-height')
  })

  it('round-trips a whole number of pixels', () => {
    const s = memory()
    writeHeight(412.4, s)
    expect(s.map.get(HEIGHT_KEY)).toBe('412')
    expect(readHeight(s)).toBe(412)
  })

  it('reads nothing back from an empty, junk or non-positive value', () => {
    expect(readHeight(memory())).toBeNull()
    expect(readHeight(memory({ [HEIGHT_KEY]: 'tall' }))).toBeNull()
    expect(readHeight(memory({ [HEIGHT_KEY]: '0' }))).toBeNull()
    expect(readHeight(memory({ [HEIGHT_KEY]: '-40' }))).toBeNull()
  })

  it('clears back to the default', () => {
    const s = memory({ [HEIGHT_KEY]: '400' })
    clearHeight(s)
    expect(readHeight(s)).toBeNull()
  })

  it('survives a storage that throws', () => {
    const broken = { getItem() { throw new Error('denied') }, setItem() { throw new Error('denied') }, removeItem() { throw new Error('denied') } }
    expect(readHeight(broken)).toBeNull()
    expect(() => writeHeight(300, broken)).not.toThrow()
    expect(() => clearHeight(broken)).not.toThrow()
    expect(readHeight(null)).toBeNull()
  })
})
