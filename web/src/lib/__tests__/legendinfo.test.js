// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { buildPollenRows, buildSeaRows, legendRows, renderLegend, setPollenLegend } from '../legend.js'
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

  // Pollen on hides the metric's heading, and its (i) inside it. The section's must stay.
  it('setPollenLegend hides the metric heading (i) but not the pollen one', () => {
    const el = document.createElement('div')
    const label = document.createElement('span')
    label.className = 'scale__label'
    const metricInfo = document.createElement('button')
    metricInfo.className = 'scale__info'
    label.appendChild(metricInfo)
    el.appendChild(label)
    const rows = buildPollenRows(pollen, vi.fn())
    el.appendChild(rows)
    setPollenLegend(el, true, 'Pollen')
    expect(label.hidden).toBe(true)
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
    expect(t.info).toEqual({ label: 'L', title: 'T', body: 'B', linkMap: 'M', linkEea: 'E', linkDatahub: '' })
  })
})

// Each (i) sits in the heading row it explains, after the heading text in DOM order.
describe('(i) buttons live in their heading rows', () => {
  const bands = [{ upper: 15, colour: '#3c9', label: 'Good', label_bg: 'Добро' }, { upper: null, colour: '#c33', label: 'Poor', label_bg: 'Лошо' }]
  const draw = () => {
    const el = document.createElement('details')
    renderLegend(el, { title: 'PM2.5, µg/m³', toggleLabel: 't', ...legendRows(bands, { noDataColour: '#999', noDataLabel: 'n', lang: 'en' }), info: { label: 'About the scale', onOpen: vi.fn() } })
    return el
  }

  it('metric (i) is inside the metric label, not a child of the legend', () => {
    const el = draw()
    expect(el.querySelector(':scope > .scale__info')).toBeNull()
    const label = el.querySelector(':scope > .scale__label')
    const btn = label.querySelector('button.scale__info')
    expect(btn.getAttribute('aria-label')).toBe('About the scale')
    expect(label.firstChild.nodeType).toBe(Node.TEXT_NODE)
    expect(label.textContent).toBe('PM2.5, µg/m³')
  })

  it('sea and pollen (i) are inside their heading rows', () => {
    const sea = buildSeaRows({ legend: 'Bathing water', classes: {}, info: { label: 'Sea info' } }, {}, vi.fn())
    const pollen = buildPollenRows({ legend: 'Pollen', levels: {}, info: { label: 'Pollen info' } }, vi.fn())
    expect(sea.querySelector('.scale__sea-head > button.scale__info')).not.toBeNull()
    expect(pollen.querySelector('.scale__pollen-head > button.scale__info')).not.toBeNull()
    expect(sea.querySelector(':scope > .scale__info')).toBeNull()
    expect(pollen.querySelector(':scope > .scale__info')).toBeNull()
  })

  it('aria-labels are distinct per section', () => {
    const el = draw()
    const sea = buildSeaRows({ legend: 'B', classes: {}, info: { label: 'Sea info' } }, {}, vi.fn())
    const pollen = buildPollenRows({ legend: 'P', levels: {}, info: { label: 'Pollen info' } }, vi.fn())
    const names = [el, sea, pollen].map((n) => n.querySelector('button.scale__info').getAttribute('aria-label'))
    expect(new Set(names).size).toBe(3)
  })

  // The metric (i) must go with the metric label when pollen mode hides it.
  it('pollen mode hides the metric (i) with its label', () => {
    const el = draw()
    setPollenLegend(el, true, 'Pollen')
    const label = el.querySelector(':scope > .scale__label')
    expect(label.hidden).toBe(true)
    expect(label.querySelector('.scale__info')).not.toBeNull()
  })
})
