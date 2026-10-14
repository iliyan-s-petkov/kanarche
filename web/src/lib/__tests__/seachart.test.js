import { describe, it, expect } from 'vitest'
import { chartLayout, chartSummary } from '../seachart.js'

const LIMITS = [250, 500]
const pt = (date, value, below = false) => ({ date, value, below, preSeason: false, source: '' })
const label = (iso) => iso
const inPlot = (l, p) => p.cx >= l.plot.x && p.cx <= l.plot.x + l.plot.w && p.cy >= l.plot.y && p.cy <= l.plot.y + l.plot.h

describe('chartLayout', () => {
  const points = [pt('2024-05-20', 600), pt('2024-06-10', 30), pt('2024-07-02', 15, true)]
  const l = chartLayout(points, LIMITS, label)

  it('puts later dates to the right and larger values higher', () => {
    expect(l.dots[0].cx).toBeLessThan(l.dots[1].cx)
    expect(l.dots[1].cx).toBeLessThan(l.dots[2].cx)
    expect(l.dots[0].cy).toBeLessThan(l.dots[1].cy)
  })
  it('draws the excellent limit below the good limit and both inside the plot', () => {
    const [ex, good] = l.limitLines
    expect(ex.value).toBe(250)
    expect(good.value).toBe(500)
    expect(ex.y).toBeGreaterThan(good.y)
    for (const line of l.limitLines) {
      expect(line.y).toBeGreaterThanOrEqual(l.plot.y)
      expect(line.y).toBeLessThanOrEqual(l.plot.y + l.plot.h)
    }
  })
  it('tiles the plot with three bands, in the order within, between, above', () => {
    expect(l.bands.map((b) => b.kind)).toEqual(['within', 'between', 'above'])
    expect(l.bands.reduce((sum, b) => sum + b.h, 0)).toBeCloseTo(l.plot.h, 5)
  })
  it('carries the below-detection flag and the date label into each dot', () => {
    expect(l.dots.map((d) => d.below)).toEqual([false, false, true])
    expect(l.dots[2].label).toBe('2024-07-02')
  })
  it('labels the axis in decades', () => {
    expect(l.ticks.map((t) => t.label)).toEqual(['1', '10', '100', '1000'])
  })
  it('labels the first and last date only when they differ', () => {
    expect(l.xLabels.map((x) => x.label)).toEqual(['2024-05-20', '2024-07-02'])
    expect(chartLayout([pt('2024-05-20', 5)], LIMITS, label).xLabels).toHaveLength(1)
  })
  it('anchors the first label at the plot start and the last at the plot end, so neither clips', () => {
    expect(l.xLabels.map((x) => x.anchor)).toEqual(['start', 'end'])
    expect(l.xLabels[0].x).toBe(l.plot.x)
    expect(l.xLabels[1].x).toBe(l.plot.x + l.plot.w)
    expect(l.xLabels[1].x).toBeLessThanOrEqual(l.width)
    const one = chartLayout([pt('2024-05-20', 5)], LIMITS, label)
    expect(one.xLabels[0].anchor).toBe('middle')
  })
  it('centres a single point', () => {
    const one = chartLayout([pt('2024-05-20', 5)], LIMITS, label)
    expect(one.dots[0].cx).toBeCloseTo(one.plot.x + one.plot.w / 2, 5)
  })
  // Review Focus 2: a log axis has no zero.
  it('keeps zero, sub-1 and huge values finite and inside the plot', () => {
    const wild = chartLayout([pt('2024-05-01', 0), pt('2024-05-02', 0.4), pt('2024-05-03', 2_000_000)], LIMITS, label)
    for (const d of wild.dots) {
      expect(Number.isFinite(d.cx)).toBe(true)
      expect(Number.isFinite(d.cy)).toBe(true)
      expect(inPlot(wild, d)).toBe(true)
    }
    expect(wild.ticks.at(-1).label).toBe('10000000')
  })
  it('grows the axis to keep a value over the good limit visible', () => {
    const tall = chartLayout([pt('2024-05-01', 100000)], LIMITS, label)
    expect(inPlot(tall, tall.dots[0])).toBe(true)
  })
})

describe('chartSummary', () => {
  const template = '{metric}, season {season}. Samples: {count}. Above the excellent limit: {over_excellent}. Above the good limit: {over_good}.'
  it('counts samples above each limit, below-detection values never above', () => {
    const points = [pt('2024-05-20', 600), pt('2024-06-01', 300), pt('2024-06-10', 250), pt('2024-07-02', 9999, true)]
    expect(chartSummary({ points, limits: LIMITS, template, metric: 'E. coli', season: 2024 }))
      .toBe('E. coli, season 2024. Samples: 4. Above the excellent limit: 2. Above the good limit: 1.')
  })
  it('reads zero of everything for no points', () => {
    expect(chartSummary({ points: [], limits: LIMITS, template, metric: 'E. coli', season: 2024 }))
      .toBe('E. coli, season 2024. Samples: 0. Above the excellent limit: 0. Above the good limit: 0.')
  })
})
