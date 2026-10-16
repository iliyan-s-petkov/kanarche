// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, unmount } from 'svelte'
import SensorPanel from '../SensorPanel.svelte'
import { needlePoint } from '../../lib/gauge.js'

const scaledModel = {
  fraction: 0.4,
  colour: '#fa0',
  stops: [{ fraction: 1, colour: '#fa0' }],
  range: { min: 0, max: 50 },
}
const missingModel = { fraction: null, colour: null, stops: [{ fraction: 1, colour: '#fa0' }], range: { min: 0, max: 50 } }

const rows = [
  { metric: 'P2', label: 'PM2.5', value: 12.4, unit: 'µg/m³', missing: false, model: scaledModel },
  { metric: 'P1', label: 'PM10', value: null, unit: 'µg/m³', missing: true, model: missingModel },
]

let component
afterEach(() => { if (component) unmount(component) })

function render(props) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  component = mount(SensorPanel, {
    target,
    props: { rows, title: 'Sensor 42', flagText: '', closeLabel: 'Close', noValue: 'no reading', onclose: () => {}, ...props },
  })
  return target
}

describe('SensorPanel.svelte spoken units', () => {
  afterEach(() => { document.head.innerHTML = '' })

  // The symbol stays on screen, the name a screen reader gets says it in words.
  it('names the gauge with the spoken unit and keeps the symbol visible', () => {
    document.head.innerHTML = '<meta name="spoken-units" content=\'{"µg/m³":"micrograms per cubic metre"}\'>'
    const target = render()
    const gauge = target.querySelectorAll('.gauge')[0]
    expect(gauge.getAttribute('aria-label')).toBe('PM2.5: 12.4 micrograms per cubic metre')
    expect(gauge.textContent).toContain('µg/m³')
  })
})

describe('SensorPanel.svelte', () => {
  it('renders one gauge per row, with its label, value and unit', () => {
    const target = render()
    const gauges = target.querySelectorAll('.gauge')
    expect(gauges.length).toBe(2)
    expect(gauges[0].getAttribute('aria-label')).toBe('PM2.5: 12.4 µg/m³')
    expect(gauges[0].textContent).toContain('PM2.5')
    expect(gauges[0].textContent).toContain('12.4')
    expect(gauges[0].textContent).toContain('µg/m³')
  })

  // A blank cell reads as zero on an air-quality page. It must say so in words.
  it('spells out a missing value instead of leaving a blank', () => {
    const target = render()
    const missingGauge = target.querySelectorAll('.gauge')[1]
    expect(missingGauge.textContent).toContain('no reading')
    expect(missingGauge.getAttribute('aria-label')).toBe('PM10: no reading')
  })

  it('omits the needle for a missing row', () => {
    const target = render()
    const missingGauge = target.querySelectorAll('.gauge')[1]
    expect(missingGauge.querySelector('.gauge__needle')).toBeNull()
  })

  it('draws the needle to the model fraction for a scaled, present row', () => {
    const target = render()
    const presentGauge = target.querySelectorAll('.gauge')[0]
    const needle = presentGauge.querySelector('.gauge__needle')
    expect(needle).not.toBeNull()
    const tip = needlePoint(scaledModel.fraction)
    expect(Number(needle.getAttribute('x2'))).toBeCloseTo(tip.x, 5)
    expect(Number(needle.getAttribute('y2'))).toBeCloseTo(tip.y, 5)
  })

  it('shows the quality warning only when there is one', () => {
    expect(render({ flagText: 'These readings look suspect.' }).textContent).toContain('suspect')
    expect(render({ flagText: '' }).querySelector('.panel-flag')).toBeNull()
  })

  it('closes on the close control and on Escape', () => {
    const onclose = vi.fn()
    const target = render({ onclose })
    target.querySelector('[data-close]').click()
    expect(onclose).toHaveBeenCalledTimes(1)
    target.querySelector('.sensor-panel').dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }),
    )
    expect(onclose).toHaveBeenCalledTimes(2)
  })

  // Closed by default: the readings are what the reader came for, and the
  // hardware inventory pushed them below the chart on a phone.
  it('lists the station description in a disclosure that starts closed', () => {
    const target = render({
      detailsLabel: 'About this station',
      details: [
        { key: 'devices', label: 'Devices', value: '42, 43' },
        { key: 'since', label: 'In our data since', value: '5 Mar 2024' },
      ],
    })
    const details = target.querySelector('.panel-details')
    expect(details.tagName).toBe('DETAILS')
    expect(details.open).toBe(false)
    expect(details.querySelector('summary').textContent).toBe('About this station')
    expect([...details.querySelectorAll('dt')].map((n) => n.textContent))
      .toEqual(['Devices', 'In our data since'])
    expect([...details.querySelectorAll('dd')].map((n) => n.textContent))
      .toEqual(['42, 43', '5 Mar 2024'])
  })

  // An empty description list under a heading is a promise the panel cannot
  // keep: a station we know nothing about shows the readings and stops.
  it('renders no description list when there is nothing to describe', () => {
    expect(render({ details: [] }).querySelector('.panel-details')).toBeNull()
  })

  // Task 12: an official station's EEA classification (EoI code, type,
  // area), precomposed by the caller into `meta` rows — see islands/panel.js.
  it('names an official station rather than a number', () => {
    const target = render({
      title: 'Пловдив Каменица',
      meta: [
        { key: 'code', label: 'EoI code', value: 'BG0070A' },
        { key: 'type', label: 'Station type', value: 'background' },
        { key: 'area', label: 'Area type', value: 'urban' },
      ],
    })
    expect(target.textContent).toContain('Пловдив Каменица')
    expect(target.textContent).toContain('BG0070A')
  })

  it('says which network the reading came from', () => {
    const target = render({ network: 'Network: sensor.community' })
    expect(target.textContent).toContain('sensor.community')
  })

  it('shows no network line when there is none to show', () => {
    expect(render({ network: '' }).querySelector('.panel-network')).toBeNull()
  })

  // A citizen device has no EEA classification: `meta` arrives empty, and the
  // panel must show no station-meta block at all rather than an empty list.
  it('shows no station fields for a citizen device', () => {
    const target = render({ meta: [] })
    expect(target.querySelector('.panel-meta')).toBeNull()
    expect(target.textContent).not.toContain('background')
  })
})

describe('SensorPanel.svelte favourite star', () => {
  const star = (target) => target.querySelector('button.panel-star')

  it('is a pressed-state toggle named for what the next click does', () => {
    const off = render({ favourite: false, favSave: 'Save as my sensor', favRemove: 'Remove my sensor', onstar: () => {} })
    expect(star(off).getAttribute('aria-pressed')).toBe('false')
    expect(star(off).getAttribute('aria-label')).toBe('Save as my sensor')
    expect(star(off).getAttribute('title')).toBe('Save as my sensor')
    expect(star(off).textContent).toBe('\u2606')
    unmount(component)

    const on = render({ favourite: true, favSave: 'Save as my sensor', favRemove: 'Remove my sensor', onstar: () => {} })
    expect(star(on).getAttribute('aria-pressed')).toBe('true')
    expect(star(on).getAttribute('aria-label')).toBe('Remove my sensor')
    expect(star(on).textContent).toBe('\u2605')
  })

  it('sits in the header beside the close button', () => {
    const target = render({ onstar: () => {}, favSave: 'Save', favRemove: 'Remove' })
    const actions = target.querySelector('header .panel-actions')
    expect(actions.contains(star(target))).toBe(true)
    expect(actions.contains(target.querySelector('.panel-close'))).toBe(true)
  })

  it('calls onstar when clicked', () => {
    const onstar = vi.fn()
    const target = render({ onstar, favSave: 'Save', favRemove: 'Remove' })
    star(target).click()
    expect(onstar).toHaveBeenCalledOnce()
  })

  it('renders no star when no handler is given', () => {
    expect(star(render())).toBeNull()
  })
})

