// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { mount, unmount, flushSync } from 'svelte'
import SeaSampleChart from '../SeaSampleChart.svelte'
import { readSeaTexts } from '../../lib/sea.js'
import { seaT } from './seafixtures.js'

let app
afterEach(() => {
  if (app) unmount(app)
  app = null
  document.body.innerHTML = ''
})
function render(over = {}) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  const points = [
    { date: '2024-05-20', value: 600, below: false, preSeason: true, source: '' },
    { date: '2024-06-10', value: 300, below: false, preSeason: false, source: '' },
    { date: '2024-07-02', value: 15, below: true, preSeason: false, source: '' },
  ]
  app = mount(SeaSampleChart, { target, props: { points, limits: [250, 500], metricName: 'E. coli', season: 2024, t: seaT, lang: 'en', ...over } })
  flushSync()
  return target
}

describe('SeaSampleChart', () => {
  it('is an image whose name is the text summary', () => {
    const svg = render().querySelector('svg.sea-chart__svg')
    expect(svg.getAttribute('role')).toBe('img')
    expect(svg.getAttribute('aria-label')).toBe('E. coli, season 2024. Samples: 3. Above the excellent limit: 2. Above the good limit: 1.')
  })

  it('draws one dot per sample and the below-detection one hollow', () => {
    const el = render()
    expect(el.querySelectorAll('circle.sea-chart__dot')).toHaveLength(3)
    expect(el.querySelectorAll('circle.sea-chart__dot--below')).toHaveLength(1)
  })

  it('draws three bands and both limits with their values', () => {
    const el = render()
    expect(el.querySelectorAll('svg.sea-chart__svg rect.sea-chart__band')).toHaveLength(3)
    expect([...el.querySelectorAll('.sea-chart__limit-label')].map((n) => n.textContent)).toEqual(['250', '500'])
  })

  it('explains the bands and the hollow dot in text under the chart', () => {
    const legend = render().querySelector('.sea-chart__legend')
    const items = [...legend.querySelectorAll('li')].map((li) => li.textContent.trim())
    expect(items).toEqual([
      'Within the excellent limit',
      'Between the excellent and good limits',
      'Above the good limit',
      'Hollow dot: below the detection limit',
    ])
  })

  it('labels the axis with the unit', () => {
    expect(render().querySelector('.sea-chart__unit').textContent).toBe('cfu/100 ml')
  })

  it('swaps to a table and back with the toggle', () => {
    const el = render()
    const toggle = el.querySelector('.sea-chart__toggle')
    expect(toggle.textContent).toBe('Show as table')
    toggle.click()
    flushSync()
    expect(el.querySelector('svg.sea-chart__svg')).toBeNull()
    expect(el.querySelector('.sea-chart__legend')).toBeNull()
    expect(el.querySelector('table.sea-chart__table')).not.toBeNull()
    expect(el.querySelector('.sea-chart__toggle').textContent).toBe('Show as chart')
    el.querySelector('.sea-chart__toggle').click()
    flushSync()
    expect(el.querySelector('svg.sea-chart__svg')).not.toBeNull()
    expect(el.querySelector('table.sea-chart__table')).toBeNull()
  })

  it('puts the same marks and below-detection text in the table', () => {
    const el = render()
    el.querySelector('.sea-chart__toggle').click()
    flushSync()
    expect(el.querySelector('caption').textContent).toBe('E. coli, season 2024')
    const rows = [...el.querySelectorAll('tbody tr')]
    expect(rows.map((r) => r.querySelector('th').textContent)).toEqual(['May 20, 2024', 'Jun 10, 2024', 'Jul 2, 2024'])
    expect(rows[0].textContent).toContain('600')
    expect(rows[0].textContent).toContain('▲▲')
    expect(rows[0].textContent).toContain('above the good limit')
    expect(rows[1].textContent).toContain('▲')
    expect(rows[1].textContent).not.toContain('▲▲')
    expect(rows[2].textContent).toContain('<15')
    expect(rows[2].textContent).toContain('below the detection limit')
    expect(rows[2].textContent).not.toContain('▲')
    expect(rows[0].textContent).toContain('pre-season')
  })

  it('draws and tabulates two samples on the same date', () => {
    const same = [
      { date: '2024-06-10', value: 40, below: false, preSeason: false, source: 'rzi_varna' },
      { date: '2024-06-10', value: 80, below: false, preSeason: false, source: 'rzi_varna' },
    ]
    const el = render({ points: same })
    expect(el.querySelectorAll('circle.sea-chart__dot')).toHaveLength(2)
    el.querySelector('.sea-chart__toggle').click()
    flushSync()
    expect(el.querySelectorAll('tbody tr')).toHaveLength(2)
  })

  it('draws zero, a sub-1 reading and a huge one with finite coordinates', () => {
    const el = render({ points: [
      { date: '2024-05-01', value: 0, below: false, preSeason: false, source: '' },
      { date: '2024-05-02', value: 0.4, below: false, preSeason: false, source: '' },
      { date: '2024-05-03', value: 2000000, below: false, preSeason: false, source: '' },
    ] })
    for (const c of el.querySelectorAll('circle.sea-chart__dot')) {
      expect(Number.isFinite(Number(c.getAttribute('cx')))).toBe(true)
      expect(Number.isFinite(Number(c.getAttribute('cy')))).toBe(true)
    }
  })

  it('mounts with no copy at all without throwing', () => {
    expect(() => render({ t: readSeaTexts({}), metricName: '' })).not.toThrow()
  })

  it('shows a message instead of a chart for an empty season', () => {
    let el
    expect(() => { el = render({ points: [] }) }).not.toThrow()
    expect(el.querySelector('svg.sea-chart__svg')).toBeNull()
    expect(el.querySelector('.sea-chart__toggle')).toBeNull()
    expect(el.querySelector('.sea-chart__empty').textContent).toBe('No samples published for this site.')
  })

  it('credits each distinct source once', () => {
    const mixed = [
      { date: '2024-06-01', value: 10, below: false, preSeason: false, source: 'rzi_varna' },
      { date: '2024-06-02', value: 20, below: false, preSeason: false, source: 'rzi_varna' },
      { date: '2024-06-03', value: 30, below: false, preSeason: false, source: '' },
    ]
    const credits = [...render({ points: mixed }).querySelectorAll('.sea-chart__credit')].map((n) => n.textContent)
    expect(credits).toEqual(['Source: RHI Varna', 'Data: EEA, CC BY 4.0'])
  })
})
