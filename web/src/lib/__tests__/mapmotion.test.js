import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { describe, it, expect, afterEach, vi } from 'vitest'
import { motion } from '../mapmotion.js'

const stubMedia = (matches) =>
  vi.stubGlobal('matchMedia', (q) => ({ matches: matches && q.includes('prefers-reduced-motion') }))

afterEach(() => vi.unstubAllGlobals())

describe('motion', () => {
  it('adds animate:false under reduced motion and keeps the rest', () => {
    stubMedia(true)
    expect(motion({ center: [1, 2], zoom: 5, duration: 250 })).toEqual({ center: [1, 2], zoom: 5, duration: 250, animate: false })
  })
  it('handles missing options under reduced motion', () => {
    stubMedia(true)
    expect(motion(undefined)).toEqual({ animate: false })
  })
  it('does not mutate its input', () => {
    stubMedia(true)
    const o = { zoom: 3 }
    motion(o)
    expect(o).toEqual({ zoom: 3 })
  })
  it('leaves options untouched under normal motion', () => {
    stubMedia(false)
    const o = { zoom: 3 }
    expect(motion(o)).toBe(o)
  })
  it('leaves options untouched without matchMedia', () => {
    const o = { zoom: 3 }
    expect(motion(o)).toBe(o)
  })
})

// Every camera call in web/src must route its options through motion().
const roots = ['src/lib', 'src/islands', 'src/components']
const camera = /\.(flyTo|easeTo|fitBounds|zoomIn|zoomOut|panTo|zoomTo|panBy|rotateTo)\(/
describe('camera calls use motion()', () => {
  for (const root of roots) {
    for (const name of readdirSync(root).filter((f) => (f.endsWith('.js') || f.endsWith('.svelte')) && !f.includes('.test.'))) {
      it(`${root}/${name}`, () => {
        const lines = readFileSync(join(root, name), 'utf8').split('\n')
        const bad = lines.filter((l) => !/^\s*\/\//.test(l) && camera.test(l) && !/motion\(/.test(l))
        expect(bad).toEqual([])
      })
    }
  }
})
