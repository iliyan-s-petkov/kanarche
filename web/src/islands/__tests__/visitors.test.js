// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'

// uPlot is mocked: jsdom has no canvas, and the drawing is checked in the
// browser. What matters here is the DOM the island builds and the data it hands over.
const plots = []
vi.mock('uplot', () => ({
  default: vi.fn(function (opts, data) {
    this.opts = opts
    this.data = data
    this.setSize = vi.fn()
    this.destroy = vi.fn()
    this.redraw = vi.fn()
    plots.push(this)
  }),
}))
vi.mock('uplot/dist/uPlot.min.css', () => ({}))

const getJSON = vi.fn()
vi.mock('../../lib/api.js', () => ({ getJSON: (...a) => getJSON(...a) }))

const { mount } = await import('../visitors.js')

const ymd = (n) => new Date(Date.UTC(2026, 7, 31 + n)).toISOString().slice(0, 10)
function body(n) {
  const days = Array.from({ length: n }, (_, i) => ({ day: ymd(i), uniques: 10 + i, requests: 1, page_views: 1, partial: false }))
  days[n - 1] = { ...days[n - 1], partial: true, fetched_at: '2026-11-20T11:05:00Z' }
  return { days }
}

function setup() {
  document.body.innerHTML = '<div><div id="c" role="img" aria-label="Графика"></div></div>'
  const el = document.getElementById('c')
  el.setAttribute('data-t-range-30', '30 дни')
  el.setAttribute('data-t-range-90', '90 дни')
  el.setAttribute('data-t-range-365', '1 година')
  Object.assign(el.dataset, { tRangeLegend: 'Период', tToday: 'днес до {time} ч.', tAvg: 'Средно за 7 дни', tTitle: 'Посетители', tDate: 'Ден', tEmpty: 'Няма', tUnavailable: 'Грешка' })
  return el
}

beforeEach(() => {
  plots.length = 0
  getJSON.mockReset()
  globalThis.ResizeObserver = class { observe() {} disconnect() {} }
  globalThis.matchMedia = () => ({ addEventListener() {} })
})

describe('visitors island', () => {
  it('fetches once for 365 days and builds a labelled button group, 30 days pressed', async () => {
    getJSON.mockResolvedValue(body(82))
    const el = setup()
    await mount(el)
    expect(getJSON).toHaveBeenCalledTimes(1)
    expect(getJSON).toHaveBeenCalledWith('/api/v1/visitors?days=365')
    const group = el.previousElementSibling
    expect(group.getAttribute('role')).toBe('group')
    expect(group.getAttribute('aria-label')).toBe('Период')
    const buttons = [...group.querySelectorAll('button')]
    expect(buttons.map((b) => b.textContent)).toEqual(['30 дни', '90 дни', '1 година'])
    expect(buttons.map((b) => b.getAttribute('aria-pressed'))).toEqual(['true', 'false', 'false'])
    expect(plots[0].data[0]).toHaveLength(30)
  })

  it('re-slices on a range click without another request', async () => {
    getJSON.mockResolvedValue(body(82))
    const el = setup()
    await mount(el)
    const buttons = [...el.previousElementSibling.querySelectorAll('button')]
    buttons[1].click()
    expect(getJSON).toHaveBeenCalledTimes(1)
    expect(plots).toHaveLength(2)
    expect(plots[0].destroy).toHaveBeenCalled()
    expect(plots[1].data[0]).toHaveLength(82)
    expect(buttons.map((b) => b.getAttribute('aria-pressed'))).toEqual(['false', 'true', 'false'])
  })

  it('names the range, the partial day and the average in the text alternative', async () => {
    getJSON.mockResolvedValue(body(82))
    const el = setup()
    await mount(el)
    const label = el.getAttribute('aria-label')
    expect(label).toContain('30 дни')
    expect(label).toContain('днес до 13:05 ч.')
    expect(label).toContain('Средно за 7 дни')
    el.previousElementSibling.querySelectorAll('button')[2].click()
    expect(el.getAttribute('aria-label')).toContain('1 година')
  })

  it('shows the visible "so far" label from the fetch time in Sofia', async () => {
    getJSON.mockResolvedValue(body(82))
    const el = setup()
    await mount(el)
    expect(el.querySelector('.visitors-today').textContent).toBe('днес до 13:05 ч.')
  })

  it('shows no today label when the newest day is closed', async () => {
    const b = body(40)
    b.days[39] = { ...b.days[39], partial: false }
    getJSON.mockResolvedValue(b)
    const el = setup()
    await mount(el)
    expect(el.querySelector('.visitors-today')).toBeNull()
  })

  it('shows the unavailable and empty messages without buttons', async () => {
    getJSON.mockRejectedValue(new Error('down'))
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const el = setup()
    await mount(el)
    expect(el.textContent).toBe('Грешка')
    expect(el.previousElementSibling).toBeNull()
    getJSON.mockResolvedValue({ days: [] })
    const el2 = setup()
    await mount(el2)
    expect(el2.textContent).toBe('Няма')
  })
})
