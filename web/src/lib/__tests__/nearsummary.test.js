// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest'
import { nearSummaryText, readNearCopy, applyNearSummary } from '../nearsummary.js'

const t = {
  km: 'Near you: {metric} {value} {unit}, {band}. Nearest sensor is {n} km {dir}.',
  m: 'Near you: {metric} {value} {unit}, {band}. Nearest sensor is {n} m {dir}.',
  dirs: ['north', 'northeast', 'east', 'southeast', 'south', 'southwest', 'west', 'northwest'],
}
const reading = { metric: 'PM2.5', value: 6, unit: 'micrograms per cubic metre', band: 'good', lang: 'en' }

describe('nearSummaryText', () => {
  it('states the reading and a distance in kilometres from a kilometre out', () => {
    expect(nearSummaryText({ t, ...reading, metres: 2400, octant: 1 })).toBe(
      'Near you: PM2.5 6 micrograms per cubic metre, good. Nearest sensor is 2.4 km northeast.')
  })

  it('uses metres under a kilometre, rounded to ten', () => {
    expect(nearSummaryText({ t, ...reading, metres: 347, octant: 4 })).toBe(
      'Near you: PM2.5 6 micrograms per cubic metre, good. Nearest sensor is 350 m south.')
  })

  it('never says zero metres', () => {
    expect(nearSummaryText({ t, ...reading, metres: 2, octant: 0 })).toContain('is 10 m north')
  })

  it('leaves out the band for a metric with no scale', () => {
    const text = nearSummaryText({ t, ...reading, band: '', metres: 5000, octant: 6 })
    expect(text).toContain('micrograms per cubic metre. Nearest')
    expect(text).not.toContain(', .')
  })
})

describe('the home summary node', () => {
  beforeEach(() => {
    document.body.innerHTML = `<p data-home-summary data-t-near-km="${t.km}" data-t-near-m="${t.m}" ${t.dirs
      .map((d, i) => `data-t-dir-${['n', 'ne', 'e', 'se', 's', 'sw', 'w', 'nw'][i]}="${d}"`).join(' ')}>
      Bulgaria right now. <span data-home-second>Highest province: Plovdiv.</span></p>`
  })

  it('reads the copy carried on the paragraph', () => {
    const copy = readNearCopy(document.querySelector('[data-home-summary]'))
    expect(copy.km).toBe(t.km)
    expect(copy.dirs).toEqual(t.dirs)
  })

  it('swaps the second sentence for the near-you one', () => {
    expect(applyNearSummary(document, { ...reading, metres: 2400, octant: 1 })).toBe(true)
    expect(document.querySelector('[data-home-second]').textContent).toContain('Near you: PM2.5 6')
    expect(document.querySelector('[data-home-summary]').textContent).toContain('Bulgaria right now.')
  })

  it('does nothing on a page without the summary', () => {
    document.body.innerHTML = ''
    expect(applyNearSummary(document, { ...reading, metres: 2400, octant: 1 })).toBe(false)
  })
})
