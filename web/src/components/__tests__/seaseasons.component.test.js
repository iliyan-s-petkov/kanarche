// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest'
import { mount, unmount, flushSync } from 'svelte'
import SeaSwatch from '../SeaSwatch.svelte'
import SeaClassKey from '../SeaClassKey.svelte'
import SeaSeasonStrip from '../SeaSeasonStrip.svelte'
import { seaT, textsWith, colours, COLOURS } from './seafixtures.js'

let app
afterEach(() => {
  if (app) unmount(app)
  app = null
  document.body.innerHTML = ''
})
function render(Component, props) {
  const target = document.createElement('div')
  document.body.appendChild(target)
  app = mount(Component, { target, props })
  flushSync()
  return target
}

describe('SeaSwatch', () => {
  it('draws the class colour with its initial', () => {
    const el = render(SeaSwatch, { quality: 'excellent', t: seaT, colours })
    expect(el.querySelector('rect').getAttribute('fill')).toBe(COLOURS[0])
    expect(el.querySelector('text').textContent).toBe('E')
    expect(el.querySelector('svg').getAttribute('aria-hidden')).toBe('true')
  })
  it('draws the legacy class as sufficient', () => {
    const el = render(SeaSwatch, { quality: 'good_or_sufficient', t: seaT, colours })
    expect(el.querySelector('rect').getAttribute('fill')).toBe(COLOURS[2])
    expect(el.querySelector('text').textContent).toBe('S')
  })
  it('draws no class as the grey not-classified swatch', () => {
    const el = render(SeaSwatch, { quality: null, t: seaT, colours })
    expect(el.querySelector('rect').getAttribute('fill')).toBe(COLOURS[4])
    expect(el.querySelector('text').textContent).toBe('')
  })
  it('sets the letter light on the dark swatch and dark on the pale ones', () => {
    expect(render(SeaSwatch, { quality: 'excellent', t: seaT, colours }).querySelector('text').getAttribute('class')).toContain('sea-swatch__ink--light')
    unmount(app)
    document.body.innerHTML = ''
    expect(render(SeaSwatch, { quality: 'sufficient', t: seaT, colours }).querySelector('text').getAttribute('class')).toContain('sea-swatch__ink--dark')
  })
})

describe('SeaClassKey', () => {
  it('lists all five classes with swatch, initial and label', () => {
    const el = render(SeaClassKey, { t: seaT, colours })
    const items = [...el.querySelectorAll('.sea-key__item')]
    expect(items.map((li) => li.querySelector('text').textContent)).toEqual(['E', 'G', 'S', 'P', ''])
    expect(items.map((li) => li.querySelector('span').textContent)).toEqual(['Excellent', 'Good', 'Sufficient', 'Poor', 'Not classified'])
    expect(items.map((li) => li.querySelector('rect').getAttribute('fill'))).toEqual(COLOURS)
  })
  it('names the list with the legend text', () => {
    expect(render(SeaClassKey, { t: seaT, colours }).querySelector('ul').getAttribute('aria-label')).toBe('Bathing water, EEA class')
  })
})

describe('SeaSeasonStrip', () => {
  const classes = [
    { season: 2025, quality: 'excellent', source: '' },
    { season: 2023, quality: 'poor', source: '' },
  ]
  const cells = (el) => [...el.querySelectorAll('.sea-cell')]

  it('runs oldest left to newest right and fills the gap year as not classified', () => {
    const el = render(SeaSeasonStrip, { classes, t: seaT, colours, variant: 'strip' })
    expect(cells(el).map((c) => c.getAttribute('aria-label'))).toEqual(['2023 Poor', '2024 Not classified', '2025 Excellent'])
  })
  it('makes every cell a focusable image with a name', () => {
    const el = render(SeaSeasonStrip, { classes, t: seaT, colours, variant: 'strip' })
    for (const c of cells(el)) {
      expect(c.getAttribute('role')).toBe('img')
      expect(c.getAttribute('tabindex')).toBe('0')
      expect(c.getAttribute('aria-label')).not.toBe('')
    }
  })
  it('shows the class initial in each cell, so colour is not the only signal', () => {
    const el = render(SeaSeasonStrip, { classes, t: seaT, colours, variant: 'strip' })
    expect(cells(el).map((c) => c.querySelector('text').textContent)).toEqual(['P', '', 'E'])
  })
  it('puts the first and last year under the strip', () => {
    const el = render(SeaSeasonStrip, { classes, t: seaT, colours, variant: 'strip' })
    expect([...el.querySelectorAll('.sea-seasons__ticks span')].map((s) => s.textContent)).toEqual(['2023', '2025'])
  })
  it('has no ticks for a single season', () => {
    const el = render(SeaSeasonStrip, { classes: [classes[0]], t: seaT, colours, variant: 'strip' })
    expect(el.querySelector('.sea-seasons__ticks')).toBeNull()
  })
  it('names the legacy class by its full label and draws it as sufficient', () => {
    const el = render(SeaSeasonStrip, { classes: [{ season: 2017, quality: 'good_or_sufficient', source: '' }], t: seaT, colours, variant: 'strip' })
    const c = cells(el)[0]
    expect(c.getAttribute('aria-label')).toBe('2017 Good or sufficient')
    expect(c.querySelector('text').textContent).toBe('S')
    expect(c.querySelector('rect').getAttribute('fill')).toBe(COLOURS[2])
  })
  it('lays the grid out newest first with the year visible and the same names', () => {
    const el = render(SeaSeasonStrip, { classes, t: seaT, colours, variant: 'grid' })
    expect(cells(el).map((c) => c.getAttribute('aria-label'))).toEqual(['2025 Excellent', '2024 Not classified', '2023 Poor'])
    expect([...el.querySelectorAll('.sea-cell__year')].map((y) => y.textContent)).toEqual(['2025', '2024', '2023'])
    expect(el.querySelector('.sea-seasons__ticks')).toBeNull()
  })
  it('renders nothing for no classes', () => {
    expect(render(SeaSeasonStrip, { classes: [], t: seaT, colours, variant: 'strip' }).querySelector('ul')).toBeNull()
    unmount(app)
    document.body.innerHTML = ''
    expect(render(SeaSeasonStrip, { classes: null, t: seaT, colours, variant: 'strip' }).querySelector('ul')).toBeNull()
  })
  // Review Focus 1: many seasons.
  it('names and focuses every one of twenty seasons with a gap in the middle', () => {
    const many = Array.from({ length: 20 }, (_, i) => ({ season: 2025 - i, quality: 'good', source: '' })).filter((c) => c.season !== 2015)
    const el = render(SeaSeasonStrip, { classes: many, t: seaT, colours, variant: 'strip' })
    const all = cells(el)
    expect(all).toHaveLength(20)
    expect(all.find((c) => c.getAttribute('aria-label') === '2015 Not classified')).toBeTruthy()
    expect(all.every((c) => c.getAttribute('tabindex') === '0')).toBe(true)
  })
  // Review Focus 4: empty copy.
  it('mounts with no copy at all without throwing', () => {
    expect(() => render(SeaSeasonStrip, { classes, t: textsWith(Object.fromEntries(Object.keys(seaT).map(() => ['x', '']))), colours: {}, variant: 'strip' })).not.toThrow()
    unmount(app)
    document.body.innerHTML = ''
    expect(() => render(SeaClassKey, { t: textsWith({ tSeaLegend: '', tSeaInitialExcellent: '' }), colours: {} })).not.toThrow()
  })
})
