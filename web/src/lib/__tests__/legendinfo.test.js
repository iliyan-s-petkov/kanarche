// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { buildPollenRows, buildSeaRows, setPollenLegend } from '../legend.js'
import { readPollenTexts } from '../pollen.js'
import { readSeaTexts } from '../sea.js'

describe('section (i) buttons', () => {
  const sea = { legend: 'Bathing water', classes: {}, info: { label: 'What the bathing water classes mean' } }
  const pollen = { legend: 'Pollen', levels: {}, info: { label: 'What the pollen levels mean' } }

  it('puts a named, focusable button in the sea section and fires the callback', () => {
    const onInfo = vi.fn()
    const btn = buildSeaRows(sea, {}, onInfo).querySelector('button.scale__info')
    expect(btn.type).toBe('button')
    expect(btn.getAttribute('aria-label')).toBe('What the bathing water classes mean')
    btn.click()
    expect(onInfo).toHaveBeenCalledTimes(1)
  })

  it('puts a named button in the pollen section', () => {
    const onInfo = vi.fn()
    const btn = buildPollenRows(pollen, onInfo).querySelector('button.scale__info')
    expect(btn.getAttribute('aria-label')).toBe('What the pollen levels mean')
    btn.click()
    expect(onInfo).toHaveBeenCalledTimes(1)
  })

  it('adds no button without a label or a callback', () => {
    expect(buildPollenRows({ ...pollen, info: undefined }, vi.fn()).querySelector('button')).toBeNull()
    expect(buildSeaRows(sea, {}).querySelector('button')).toBeNull()
  })

  // Pollen on hides the metric's own (i), a direct child. The section's must stay.
  it('setPollenLegend hides the metric (i) but not the pollen one', () => {
    const el = document.createElement('div')
    const metricInfo = document.createElement('button')
    metricInfo.className = 'scale__info'
    el.appendChild(metricInfo)
    const rows = buildPollenRows(pollen, vi.fn())
    el.appendChild(rows)
    setPollenLegend(el, true, 'Pollen')
    expect(metricInfo.hidden).toBe(true)
    expect(rows.querySelector('button').hidden).toBe(false)
  })
})

describe('popup copy attributes', () => {
  it('reads the pollen popup copy under info', () => {
    const t = readPollenTexts({
      tPollenInfoLabel: 'L', tPollenInfoTitle: 'T', tPollenInfoBody: 'B',
      tPollenInfoMean: 'H', tPollenInfoLinkThresholds: 'A', tPollenInfoLinkChart: 'C',
    })
    expect(t.info).toEqual({ label: 'L', title: 'T', body: 'B', hourly: 'H', linkThresholds: 'A', linkChart: 'C' })
  })

  it('reads the sea popup copy under info', () => {
    const t = readSeaTexts({
      tSeaInfoLabel: 'L', tSeaInfoTitle: 'T', tSeaInfoBody: 'B', tSeaInfoLinkMap: 'M', tSeaInfoLinkEea: 'E',
    })
    expect(t.info).toEqual({ label: 'L', title: 'T', body: 'B', linkMap: 'M', linkEea: 'E' })
  })
})
