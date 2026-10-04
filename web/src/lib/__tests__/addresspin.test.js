import { describe, it, expect } from 'vitest'
import { cameraFor } from '../addresspin.js'

describe('cameraFor', () => {
  it('fits the bbox when the server sent one', () => {
    const cam = cameraFor({ lat: 1, lon: 2, bbox: [10, 20, 11, 21] })
    expect(cam.bounds).toEqual([[10, 20], [11, 21]])
    expect(cam.center).toBeUndefined()
  })

  it('zooms to 16 on the point when the bbox is all zeros or missing', () => {
    for (const bbox of [[0, 0, 0, 0], undefined]) {
      const cam = cameraFor({ lat: 42.5, lon: 23.3, bbox })
      expect(cam).toEqual({ center: [23.3, 42.5], zoom: 16 })
    }
  })
})
