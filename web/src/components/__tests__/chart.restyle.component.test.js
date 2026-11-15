// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, unmount } from 'svelte'
import Chart from '../Chart.svelte'
import { clearCache } from '../../lib/api.js'

// uPlot stores axis strokes as functions and calls axis.stroke(self, i) on every
// redraw. The mock does the same, so a restyle that assigns a plain string fails
// here the way it throws in the browser ("t.stroke is not a function").
const fn = (v) => (typeof v === 'function' ? v : () => v)
const instances = []
vi.mock('uplot', () => ({
  default: vi.fn(function (opts, data) {
    this.axes = opts.axes.map((a) => ({
      ...a, stroke: fn(a.stroke), ticks: { ...a.ticks, stroke: fn(a.ticks?.stroke) }, grid: { ...a.grid, stroke: fn(a.grid?.stroke) },
    }))
    this.redraw = vi.fn(() => this.axes.forEach((a, i) => [a.stroke(this, i), a.ticks.stroke(this, i), a.grid.stroke(this, i)]))
    this.setSize = vi.fn()
    this.setData = vi.fn()
    this.scales = { x: { min: data[0][0], max: data[0].at(-1) } }
    instances.push(this)
  }),
}))

let component
afterEach(() => {
  clearCache()
  if (component) unmount(component)
  vi.restoreAllMocks()
  instances.length = 0
  document.documentElement.removeAttribute('data-theme')
  document.documentElement.style.removeProperty('--fg-2')
})

describe('Chart.svelte theme restyle', () => {
  it('repaints axes with callable strokes in the new theme colour', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ t: ['2026-08-14T00:00:00Z', '2026-08-14T01:00:00Z'], v: [12.3, 13.1] }), { status: 200 }),
    )
    vi.stubGlobal('ResizeObserver', class { observe() {} disconnect() {} })
    const target = document.createElement('div')
    document.body.appendChild(target)
    component = mount(Chart, { target, props: { url: '/api/v1/area/restyle/series', lineColour: '#2563eb', title: 'PM2.5', valueLabel: 'µg/m³', empty: '', unavailable: '' } })
    await vi.waitFor(() => expect(instances).toHaveLength(1))
    const u = instances[0]

    document.documentElement.style.setProperty('--fg-2', '#c6c6c6')
    document.documentElement.setAttribute('data-theme', 'dark')
    await vi.waitFor(() => expect(u.redraw).toHaveBeenCalled())

    expect(u.redraw).toHaveReturned()
    expect(u.axes[0].stroke(u, 0)).toBe('#c6c6c6')
    expect(u.axes[0].ticks.stroke(u, 0)).toBe('#c6c6c6')
  })
})
